<script setup lang="ts">
import { computed, onActivated, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import {
  StopBackup, ResumeBackup, DismissBackupCheckpoint,
} from '../../wailsjs/go/main/App'
import { models } from '../../wailsjs/go/models'
import PaginationBar from '../components/PaginationBar.vue'
import { useExecutionState } from '../composables/useExecutionState'
import { usePagination } from '../composables/usePagination'
import { formatBytes, formatSpeed, formatDuration as formatDurationI18n, formatBackupType, formatDateTime, formatJobName, formatLogStatus } from '../utils/format'

defineOptions({ name: 'ProgressView' })

const { t, locale } = useI18n()

const { state, stopping, loading, refresh, prime } = useExecutionState()
const liveProgress = ref<models.ExecutionRun | null>(null)
const resuming = ref<string | null>(null)
const tick = ref(Date.now())
let pollTimer: ReturnType<typeof setInterval> | null = null
let elapsedTimer: ReturnType<typeof setInterval> | null = null

const phaseLabel: Record<string, string> = {
  preparing: 'progress.phase_preparing',
  analyzing: 'progress.phase_analyzing',
  vss: 'progress.phase_vss',
  transfer: 'progress.phase_transfer',
  finalizing: 'progress.phase_finalizing',
  verify: 'progress.phase_verify',
  done: 'progress.phase_done',
  cancelled: 'progress.phase_cancelled',
  error: 'progress.phase_error',
}

const activeRuns = computed(() => {
  const list = [...(state.value?.active || [])]
  if (liveProgress.value) {
    const idx = list.findIndex((r) => r.job_id === liveProgress.value!.job_id)
    if (idx >= 0) {
      const prev = list[idx]
      const { message: _liveMsg, ...liveRest } = liveProgress.value
      list[idx] = {
        ...prev,
        ...liveRest,
        message: prev.message || liveProgress.value.message,
      }
      if (!list[idx].started_at && prev.started_at) list[idx].started_at = prev.started_at
    } else if (liveProgress.value.phase && liveProgress.value.phase !== 'idle') {
      list.unshift(liveProgress.value)
    }
  }
  return list
})

const manualActive = computed(() => activeRuns.value.filter((r) => r.trigger !== 'scheduled'))
const scheduledActive = computed(() => activeRuns.value.filter((r) => r.trigger === 'scheduled'))
const interruptedRuns = computed(() => state.value?.interrupted ?? [])

const recentManual = computed(() => state.value?.recent_manual ?? [])
const recentScheduled = computed(() => state.value?.recent_scheduled ?? [])
const upcoming = computed(() => state.value?.upcoming ?? [])
const queued = computed(() => state.value?.queued ?? [])

const manualPag = usePagination(recentManual, 'pbs-exec-manual-page-size', 10)
const scheduledPag = usePagination(recentScheduled, 'pbs-exec-scheduled-page-size', 10)
const upcomingPag = usePagination(upcoming, 'pbs-exec-upcoming-page-size', 10)

function formatEta(sec: number) {
  if (!sec) return t('common.dash')
  const formatted = formatDuration(sec)
  return formatted === t('common.dash') ? t('common.dash') : `~${formatted}`
}

function formatDuration(sec: number) {
  if (!sec || sec < 0) return t('common.dash')
  return formatDurationI18n(sec)
}

function formatWhen(s: string) {
  return formatDateTime(s)
}

function elapsedSec(startedAt?: string) {
  if (!startedAt) return 0
  const start = new Date(startedAt).getTime()
  if (isNaN(start)) return 0
  return Math.max(0, Math.floor((tick.value - start) / 1000))
}

function formatElapsed(startedAt?: string) {
  const sec = elapsedSec(startedAt)
  return sec > 0 ? formatDuration(sec) : t('common.dash')
}

function phaseName(p: string, message?: string) {
  if (message && (p === 'transfer' || p === 'preparing' || p === 'analyzing')) {
    return message
  }
  const key = phaseLabel[p]
  return key ? t(key) : (p || t('common.dash'))
}

function isPBSChunks(run: models.ExecutionRun) {
  return (run.chunks_new || 0) > 0 || (run.chunks_reused || 0) > 0
}

function fmtCount(n: number) {
  return (n || 0).toLocaleString()
}

function isPBSCacheProgress(run: models.ExecutionRun) {
  return (run.files_from_cache || 0) > 0
    || (run.files_skipped || 0) > 0
    || (isPBSChunks(run) && (run.files_total || 0) > 0)
}

function filesFromCache(run: models.ExecutionRun) {
  return run.files_from_cache || 0
}

function filesNewChanged(run: models.ExecutionRun) {
  const done = run.files_done || 0
  const fromCache = filesFromCache(run)
  return Math.max(0, done - fromCache)
}

function filesBeyondCache(run: models.ExecutionRun) {
  const done = run.files_done || 0
  const cache = run.files_total || 0
  return cache > 0 && done > cache ? done - cache : 0
}

function showFilesProgress(run: models.ExecutionRun) {
  return (run.files_done || 0) > 0 || (run.files_total || 0) > 0
}

function reusedLabel(run: models.ExecutionRun) {
  return isPBSChunks(run) ? t('progress.reused') : t('progress.skipped_bytes')
}

function triggerLabel(trigger: string) {
  return trigger === 'scheduled' ? t('progress.trigger_scheduled') : t('progress.trigger_manual')
}

function statusClass(s: string) {
  if (s === 'ok' || s === 'warning') return 'status-ok'
  if (s === 'error') return 'status-error'
  if (s === 'cancelled') return 'status-warn'
  return ''
}

function statusText(s: string) {
  return formatLogStatus(s) || s
}

function typeLabel(tpe: string) {
  return formatBackupType(tpe, true)
}

function progressFromEvent(p: any): models.ExecutionRun {
  return models.ExecutionRun.createFrom({
    job_id: p.job_id,
    job_name: p.job_name,
    trigger: p.trigger || 'manual',
    phase: p.phase,
    backup_type: p.backup_type,
    percent: p.percent,
    bytes_transferred: p.bytes_transferred,
    bytes_reused: p.bytes_reused,
    speed_bps: p.speed_bps,
    eta_sec: p.eta_sec,
    started_at: p.started_at,
    chunks_new: p.chunks_new,
    chunks_reused: p.chunks_reused,
    files_done: p.files_done,
    files_total: p.files_total,
    files_skipped: p.files_skipped,
    files_from_cache: p.files_from_cache,
    current_path: p.current_path,
    message: p.message,
    can_stop: true,
  })
}

onMounted(() => {
  prime()
  void refresh()
  pollTimer = setInterval(() => { void refresh() }, 2000)
  elapsedTimer = setInterval(() => { tick.value = Date.now() }, 1000)

  watch(locale, () => { void refresh() })

  EventsOn('progress', (p: any) => {
    if (!p?.phase || p.phase === 'idle') return
    if (p.phase === 'error') {
      liveProgress.value = progressFromEvent(p)
      refresh()
      return
    }
    liveProgress.value = progressFromEvent(p)
    if (p.phase === 'done' || p.phase === 'cancelled') {
      liveProgress.value = null
      setTimeout(refresh, 400)
    }
  })
  EventsOn('backup-finished', () => {
    liveProgress.value = null
    refresh()
  })
})

onActivated(() => {
  prime()
  void refresh()
})

onUnmounted(() => {
  if (pollTimer) clearInterval(pollTimer)
  if (elapsedTimer) clearInterval(elapsedTimer)
})

async function stop(run: models.ExecutionRun) {
  if (!run.can_stop) return
  if (!confirm(t('progress_ext.stop_confirm'))) return
  stopping.value = true
  await StopBackup(run.job_id)
}

async function retry(run: models.ExecutionRun) {
  if (!run.can_retry || resuming.value) return
  resuming.value = run.job_id
  try {
    await ResumeBackup(run.job_id)
  } finally {
    resuming.value = null
    await refresh()
  }
}

async function dismiss(run: models.ExecutionRun) {
  if (!run.can_dismiss) return
  if (!confirm(t('progress_ext.dismiss_confirm'))) return
  await DismissBackupCheckpoint(run.job_id)
  await refresh()
}
</script>

<template>
  <div class="page execution-page">
    <header class="page-header">
      <div>
        <h2>{{ t('progress.title') }}</h2>
        <p class="page-sub">{{ t('progress.subtitle') }}</p>
      </div>
    </header>

    <!-- Active: manual -->
    <section class="exec-section">
      <div class="section-head">
        <h3>{{ t('progress.manual_active') }}</h3>
        <span class="badge-count">{{ manualActive.length }}</span>
      </div>
      <div v-if="manualActive.length" class="run-cards">
        <article v-for="run in manualActive" :key="run.job_id + '-m'" class="run-card manual">
          <header class="run-head">
            <div>
              <strong>{{ formatJobName(run.job_name) }}</strong>
              <span class="trigger-pill manual">{{ triggerLabel(run.trigger) }}</span>
            </div>
            <button
              v-if="run.can_stop"
              class="btn sm danger"
              :disabled="stopping"
              @click="stop(run)"
            >{{ stopping ? t('progress.stopping') : t('progress.stop') }}</button>
          </header>
          <div class="phase-line" :class="'phase-' + run.phase">{{ phaseName(run.phase, run.message) }}</div>
          <p v-if="run.phase === 'error'" class="error-line">{{ run.message }}</p>
          <div class="progress-big">
            <div class="progress-fill" :style="{ width: Math.min(run.percent || 0, 100) + '%' }" />
          </div>
          <div class="progress-meta">
            <span>{{ (run.percent || 0).toFixed(1) }}%</span>
            <span>{{ formatSpeed(run.speed_bps) }}</span>
            <span v-if="run.started_at">{{ t('progress.elapsed') }}: {{ formatElapsed(run.started_at) }}</span>
            <span>{{ t('progress.eta') }}: {{ formatEta(run.eta_sec) }}</span>
          </div>
          <div class="stats-row">
            <div v-if="showFilesProgress(run)" class="files-detail">
              <template v-if="isPBSCacheProgress(run)">
                <span>{{ t('progress.files_done') }}: {{ fmtCount(run.files_done) }}</span>
                <span v-if="run.files_total">{{ t('progress.files_cache_total') }}: {{ fmtCount(run.files_total) }}</span>
                <span v-if="filesFromCache(run)">{{ t('progress.files_from_cache') }}: {{ fmtCount(filesFromCache(run)) }}</span>
                <span v-if="filesNewChanged(run)">{{ t('progress.files_new_changed') }}: {{ fmtCount(filesNewChanged(run)) }}</span>
                <span v-if="filesBeyondCache(run)" class="files-beyond">{{ t('progress.files_beyond_cache', { n: fmtCount(filesBeyondCache(run)) }) }}</span>
              </template>
              <template v-else>
                <span>{{ t('progress.files') }}: {{ fmtCount(run.files_done) }}<template v-if="run.files_total"> / {{ fmtCount(run.files_total) }} {{ t('progress.files_scan_total') }}</template></span>
              </template>
            </div>
            <span>{{ t('progress.transferred') }}: {{ formatBytes(run.bytes_transferred) }}</span>
            <span>{{ reusedLabel(run) }}: {{ formatBytes(run.bytes_reused) }}</span>
            <span v-if="isPBSChunks(run)">
              {{ t('progress.chunks') }}: {{ run.chunks_new }} / {{ run.chunks_reused }}
            </span>
          </div>
          <p v-if="run.current_path" class="path-line">{{ t('progress.current') }}: {{ run.current_path }}</p>
          <p v-if="run.message && phaseName(run.phase, run.message) !== run.message" class="msg-line">{{ run.message }}</p>
        </article>
      </div>
      <p v-else-if="loading" class="muted empty-hint">{{ t('common.loading') }}</p>
      <p v-else class="muted empty-hint">{{ t('progress.manual_idle') }}</p>
    </section>

    <!-- Interrupted -->
    <section v-if="interruptedRuns.length" class="exec-section interrupted-section">
      <div class="section-head">
        <h3>{{ t('progress.interrupted') }}</h3>
        <span class="badge-count warn">{{ interruptedRuns.length }}</span>
      </div>
      <p class="muted section-hint">{{ t('progress.interrupted_hint') }}</p>
      <div class="run-cards">
        <article v-for="run in interruptedRuns" :key="run.job_id + '-i'" class="run-card interrupted">
          <header class="run-head">
            <div>
              <strong>{{ formatJobName(run.job_name) }}</strong>
              <span class="trigger-pill manual">{{ triggerLabel(run.trigger) }}</span>
            </div>
            <div class="run-actions">
              <button
                class="btn sm primary"
                :disabled="!!resuming"
                @click="retry(run)"
              >{{ resuming === run.job_id ? '…' : t('progress.retry') }}</button>
              <button
                class="btn sm"
                :disabled="!!resuming"
                @click="dismiss(run)"
              >{{ t('progress.dismiss') }}</button>
            </div>
          </header>
          <div class="phase-line" :class="'phase-' + run.phase">{{ phaseName(run.phase, run.message) }}</div>
          <p v-if="run.message" class="msg-line">{{ run.message }}</p>
          <div class="progress-big">
            <div class="progress-fill interrupted-fill" :style="{ width: Math.min(run.percent || 0, 100) + '%' }" />
          </div>
          <div class="progress-meta">
            <span>{{ (run.percent || 0).toFixed(1) }}%</span>
            <span v-if="run.started_at">{{ t('progress.elapsed') }}: {{ formatElapsed(run.started_at) }}</span>
          </div>
          <div class="stats-row">
            <div v-if="showFilesProgress(run)" class="files-detail">
              <template v-if="isPBSCacheProgress(run)">
                <span>{{ t('progress.files_done') }}: {{ fmtCount(run.files_done) }}</span>
                <span v-if="run.files_total">{{ t('progress.files_cache_total') }}: {{ fmtCount(run.files_total) }}</span>
                <span v-if="filesFromCache(run)">{{ t('progress.files_from_cache') }}: {{ fmtCount(filesFromCache(run)) }}</span>
                <span v-if="filesNewChanged(run)">{{ t('progress.files_new_changed') }}: {{ fmtCount(filesNewChanged(run)) }}</span>
                <span v-if="filesBeyondCache(run)" class="files-beyond">{{ t('progress.files_beyond_cache', { n: fmtCount(filesBeyondCache(run)) }) }}</span>
              </template>
              <template v-else>
                <span>{{ t('progress.files') }}: {{ fmtCount(run.files_done) }}<template v-if="run.files_total"> / {{ fmtCount(run.files_total) }} {{ t('progress.files_scan_total') }}</template></span>
              </template>
            </div>
            <span>{{ t('progress.transferred') }}: {{ formatBytes(run.bytes_transferred) }}</span>
            <span>{{ reusedLabel(run) }}: {{ formatBytes(run.bytes_reused) }}</span>
            <span v-if="isPBSChunks(run)">
              {{ t('progress.chunks') }}: {{ run.chunks_new }} / {{ run.chunks_reused }}
            </span>
          </div>
          <p v-if="run.updated_at" class="started-at muted">
            {{ t('progress.interrupted_at') }}: {{ formatWhen(run.updated_at) }}
          </p>
        </article>
      </div>
    </section>

    <!-- Active: scheduled -->
    <section class="exec-section">
      <div class="section-head">
        <h3>{{ t('progress.scheduled_active') }}</h3>
        <span class="badge-count">{{ scheduledActive.length }}</span>
      </div>
      <div v-if="scheduledActive.length" class="run-cards">
        <article v-for="run in scheduledActive" :key="run.job_id + '-s'" class="run-card scheduled">
          <header class="run-head">
            <div>
              <strong>{{ formatJobName(run.job_name) }}</strong>
              <span class="trigger-pill scheduled">{{ triggerLabel(run.trigger) }}</span>
            </div>
            <span class="started-at" v-if="run.started_at">{{ formatWhen(run.started_at) }}</span>
          </header>
          <div class="phase-line" :class="'phase-' + run.phase">{{ phaseName(run.phase, run.message) }}</div>
          <p v-if="run.phase === 'error'" class="error-line">{{ run.message }}</p>
          <div class="progress-big">
            <div class="progress-fill scheduled-fill" :style="{ width: Math.min(run.percent || 0, 100) + '%' }" />
          </div>
          <div class="progress-meta">
            <span>{{ (run.percent || 0).toFixed(1) }}%</span>
            <span v-if="run.started_at">{{ t('progress.elapsed') }}: {{ formatElapsed(run.started_at) }}</span>
          </div>
          <div class="stats-row">
            <div v-if="showFilesProgress(run)" class="files-detail">
              <template v-if="isPBSCacheProgress(run)">
                <span>{{ t('progress.files_done') }}: {{ fmtCount(run.files_done) }}</span>
                <span v-if="run.files_total">{{ t('progress.files_cache_total') }}: {{ fmtCount(run.files_total) }}</span>
                <span v-if="filesFromCache(run)">{{ t('progress.files_from_cache') }}: {{ fmtCount(filesFromCache(run)) }}</span>
                <span v-if="filesNewChanged(run)">{{ t('progress.files_new_changed') }}: {{ fmtCount(filesNewChanged(run)) }}</span>
                <span v-if="filesBeyondCache(run)" class="files-beyond">{{ t('progress.files_beyond_cache', { n: fmtCount(filesBeyondCache(run)) }) }}</span>
              </template>
              <template v-else>
                <span>{{ t('progress.files') }}: {{ fmtCount(run.files_done) }}<template v-if="run.files_total"> / {{ fmtCount(run.files_total) }} {{ t('progress.files_scan_total') }}</template></span>
              </template>
            </div>
            <span>{{ t('progress.transferred') }}: {{ formatBytes(run.bytes_transferred) }}</span>
            <span>{{ reusedLabel(run) }}: {{ formatBytes(run.bytes_reused) }}</span>
          </div>
          <p v-if="run.message && phaseName(run.phase, run.message) !== run.message" class="msg-line">{{ run.message }}</p>
        </article>
      </div>
      <p v-else-if="loading" class="muted empty-hint">{{ t('common.loading') }}</p>
      <p v-else class="muted empty-hint">{{ t('progress.scheduled_idle') }}</p>
    </section>

    <section class="exec-section">
      <div class="section-head">
        <h3>{{ t('progress.queued') }}</h3>
        <span class="badge-count">{{ queued.length }}</span>
      </div>
      <div v-if="queued.length" class="table-wrap">
        <table class="data-table compact">
          <thead>
            <tr>
              <th>#</th>
              <th>{{ t('jobs.name') }}</th>
              <th>{{ t('progress.when') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="q in queued" :key="q.job_id + '-' + q.position">
              <td>{{ q.position }}</td>
              <td>{{ formatJobName(q.job_name || q.job_id) }}</td>
              <td>{{ formatWhen(q.enqueued_at) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
      <p v-else-if="loading" class="muted empty-hint">{{ t('common.loading') }}</p>
      <p v-else class="muted empty-hint">{{ t('progress.no_queued') }}</p>
    </section>

    <!-- Upcoming -->
    <section class="exec-section">
      <div class="section-head">
        <h3>{{ t('progress.upcoming') }}</h3>
      </div>
      <table v-if="upcoming.length" class="table compact">
        <thead>
          <tr>
            <th>{{ t('jobs.name') }}</th>
            <th>{{ t('progress.when') }}</th>
            <th>{{ t('logs.backup_type') }}</th>
            <th>{{ t('jobs.schedule') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(u, i) in upcomingPag.pageItems.value" :key="u.job_id + '-' + i">
            <td><strong>{{ formatJobName(u.job_name) }}</strong></td>
            <td>{{ u.run_at }}</td>
            <td>{{ typeLabel(u.backup_type) }}</td>
            <td class="muted">{{ u.times_label }}</td>
          </tr>
        </tbody>
      </table>
      <PaginationBar
        v-if="upcoming.length"
        :page="upcomingPag.page.value"
        :page-size="upcomingPag.pageSize.value"
        :page-sizes="upcomingPag.pageSizes"
        :total="upcomingPag.total.value"
        :total-pages="upcomingPag.totalPages.value"
        :range-from="upcomingPag.rangeFrom.value"
        :range-to="upcomingPag.rangeTo.value"
        :page-numbers="upcomingPag.pageNumbers.value"
        @update:page="upcomingPag.setPage"
        @update:page-size="upcomingPag.setPageSize"
      />
      <p v-else-if="loading" class="muted empty-hint">{{ t('common.loading') }}</p>
      <p v-else class="muted empty-hint">{{ t('progress.no_upcoming') }}</p>
    </section>

    <!-- Recent manual -->
    <section class="exec-section">
      <div class="section-head">
        <h3>{{ t('progress.recent_manual') }}</h3>
        <span v-if="recentManual.length" class="badge-count">{{ recentManual.length }}</span>
      </div>
      <table v-if="recentManual.length" class="table compact">
        <thead>
          <tr>
            <th>{{ t('jobs.name') }}</th>
            <th>{{ t('logs.status') }}</th>
            <th>{{ t('logs.backup_type') }}</th>
            <th>{{ t('logs.when') }}</th>
            <th>{{ t('logs.duration') }}</th>
            <th>{{ t('logs.details') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(h, i) in manualPag.pageItems.value" :key="'m' + h.started_at + i">
            <td>{{ formatJobName(h.job_name) }}</td>
            <td><span :class="statusClass(h.status)">{{ statusText(h.status) }}</span></td>
            <td>{{ typeLabel(h.backup_type) }}</td>
            <td>{{ formatWhen(h.started_at) }}</td>
            <td>{{ h.duration_sec }} {{ t('common.sec') }}</td>
            <td class="details">{{ formatBytes(h.bytes_transferred) }} / {{ formatBytes(h.bytes_reused) }}</td>
          </tr>
        </tbody>
      </table>
      <PaginationBar
        v-if="recentManual.length"
        :page="manualPag.page.value"
        :page-size="manualPag.pageSize.value"
        :page-sizes="manualPag.pageSizes"
        :total="manualPag.total.value"
        :total-pages="manualPag.totalPages.value"
        :range-from="manualPag.rangeFrom.value"
        :range-to="manualPag.rangeTo.value"
        :page-numbers="manualPag.pageNumbers.value"
        @update:page="manualPag.setPage"
        @update:page-size="manualPag.setPageSize"
      />
      <p v-else-if="loading" class="muted empty-hint">{{ t('common.loading') }}</p>
      <p v-else class="muted empty-hint">{{ t('progress.no_recent_manual') }}</p>
    </section>

    <!-- Recent scheduled -->
    <section class="exec-section">
      <div class="section-head">
        <h3>{{ t('progress.recent_scheduled') }}</h3>
        <span v-if="recentScheduled.length" class="badge-count">{{ recentScheduled.length }}</span>
      </div>
      <table v-if="recentScheduled.length" class="table compact">
        <thead>
          <tr>
            <th>{{ t('jobs.name') }}</th>
            <th>{{ t('logs.status') }}</th>
            <th>{{ t('logs.backup_type') }}</th>
            <th>{{ t('logs.when') }}</th>
            <th>{{ t('logs.duration') }}</th>
            <th>{{ t('logs.details') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(h, i) in scheduledPag.pageItems.value" :key="'s' + h.started_at + i">
            <td>{{ formatJobName(h.job_name) }}</td>
            <td><span :class="statusClass(h.status)">{{ statusText(h.status) }}</span></td>
            <td>{{ typeLabel(h.backup_type) }}</td>
            <td>{{ formatWhen(h.started_at) }}</td>
            <td>{{ h.duration_sec }} {{ t('common.sec') }}</td>
            <td class="details">{{ formatBytes(h.bytes_transferred) }} / {{ formatBytes(h.bytes_reused) }}</td>
          </tr>
        </tbody>
      </table>
      <PaginationBar
        v-if="recentScheduled.length"
        :page="scheduledPag.page.value"
        :page-size="scheduledPag.pageSize.value"
        :page-sizes="scheduledPag.pageSizes"
        :total="scheduledPag.total.value"
        :total-pages="scheduledPag.totalPages.value"
        :range-from="scheduledPag.rangeFrom.value"
        :range-to="scheduledPag.rangeTo.value"
        :page-numbers="scheduledPag.pageNumbers.value"
        @update:page="scheduledPag.setPage"
        @update:page-size="scheduledPag.setPageSize"
      />
      <p v-else-if="loading" class="muted empty-hint">{{ t('common.loading') }}</p>
      <p v-else class="muted empty-hint">{{ t('progress.no_recent_scheduled') }}</p>
    </section>
  </div>
</template>

<style scoped>
.execution-page {
  display: flex;
  flex-direction: column;
  gap: 1.25rem;
}
.exec-section {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 1rem 1.1rem;
}
.section-head {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  margin-bottom: 0.75rem;
}
.section-head h3 {
  margin: 0;
  font-size: 1rem;
}
.badge-count {
  font-size: 0.75rem;
  padding: 0.1rem 0.45rem;
  border-radius: 999px;
  background: #1e293b;
  color: #94a3b8;
}
.badge-count.warn {
  background: #422006;
  color: #fdba74;
}
.section-hint {
  margin: -0.25rem 0 0.75rem;
  font-size: 0.85rem;
}
.run-cards {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}
.run-card {
  border: 1px solid var(--border);
  border-radius: 10px;
  padding: 0.85rem 1rem;
  background: #0f172a;
}
.run-card.manual { border-left: 3px solid #3b82f6; }
.run-card.scheduled { border-left: 3px solid #8b5cf6; }
.run-card.interrupted { border-left: 3px solid #f59e0b; }
.run-actions {
  display: flex;
  gap: 0.4rem;
  flex-shrink: 0;
}
.run-head {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 0.75rem;
  margin-bottom: 0.5rem;
}
.trigger-pill {
  display: inline-block;
  margin-left: 0.5rem;
  font-size: 0.7rem;
  padding: 0.1rem 0.4rem;
  border-radius: 4px;
  vertical-align: middle;
}
.trigger-pill.manual { background: #1e3a5f; color: #93c5fd; }
.trigger-pill.scheduled { background: #3b1f6e; color: #c4b5fd; }
.started-at { font-size: 0.8rem; color: var(--muted); }
.phase-line { font-weight: 600; margin-bottom: 0.4rem; }
.phase-done { color: var(--ok); }
.phase-error { color: var(--danger); }
.phase-cancelled { color: var(--warn); }
.progress-big {
  height: 8px;
  background: #1e293b;
  border-radius: 4px;
  overflow: hidden;
}
.progress-fill {
  height: 100%;
  background: linear-gradient(90deg, #2563eb, #3b82f6);
  transition: width 0.3s ease;
}
.scheduled-fill {
  background: linear-gradient(90deg, #6d28d9, #8b5cf6);
}
.interrupted-fill {
  background: linear-gradient(90deg, #d97706, #f59e0b);
}
.progress-meta {
  display: flex;
  gap: 1rem;
  font-size: 0.85rem;
  color: var(--muted);
  margin: 0.35rem 0;
}
.stats-row {
  display: flex;
  flex-wrap: wrap;
  gap: 1rem;
  font-size: 0.85rem;
}
.files-detail {
  display: flex;
  flex-wrap: wrap;
  gap: 0.65rem 1rem;
  width: 100%;
  flex-basis: 100%;
}
.files-beyond {
  color: var(--warn);
  font-size: 0.82rem;
}
.muted-inline {
  color: var(--muted);
  font-size: 0.82rem;
}
.path-line, .msg-line {
  font-size: 0.82rem;
  color: var(--muted);
  margin: 0.35rem 0 0;
  word-break: break-all;
}
.error-line {
  font-size: 0.88rem;
  color: #fca5a5;
  margin: 0.35rem 0 0.5rem;
  line-height: 1.4;
}
.empty-hint { margin: 0.25rem 0 0; }
.table.compact th, .table.compact td { padding: 0.45rem 0.6rem; font-size: 0.88rem; }
.details { color: var(--muted); font-size: 0.82rem; }
.status-ok { color: #86efac; }
.status-error { color: #fca5a5; }
.status-warn { color: #fcd34d; }
</style>
