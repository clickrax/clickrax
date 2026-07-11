<script setup lang="ts">
import { computed, onActivated, onMounted, reactive, ref, toRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import {
  ListDestinations, ListJobs, GetHistory, IsBackupRunning, GetNextScheduledRun,
  RunQuickBackup, PickFolder, ListVolumes, GetHostname, TestDestinationConnection, EstimatePath,
  GetLastSuccessfulBackup, ClearBackupLock, GetExecutionState,
} from '../../wailsjs/go/main/App'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import { models } from '../../wailsjs/go/models'
import { useBackdropDismiss } from '../composables/useBackdropDismiss'
import { useJobExclusions } from '../composables/useJobExclusions'
import { formatBytes, formatBackupType, formatJobName, formatLogStatus } from '../utils/format'

defineOptions({ name: 'Dashboard' })

const { t } = useI18n()
const router = useRouter()

const serverCount = ref(0)
const jobCount = ref(0)
const jobsEnabledCount = ref(0)
const jobsDisabledCount = ref(0)
const runningJobName = ref('')
const lastRun = ref('')
const lastType = ref('')
const lastTransferred = ref('')
const lastReused = ref('')
const nextRun = ref('')
const running = ref(false)
const destinations = ref<models.BackupDestination[]>([])
const volumes = ref<string[]>([])
const quickOpen = ref(false)
const { onBackdropPointerDown, onBackdropClick } = useBackdropDismiss(() => { quickOpen.value = false })
const quickError = ref('')
const lockBlocked = computed(() => {
  const pat = t('dashboard_ext.lock_pattern')
  try {
    return new RegExp(pat, 'i').test(quickError.value)
  } catch {
    return /lock|process|running|blocked|блокир|процесс|выполняется/i.test(quickError.value)
  }
})
const serverStatus = ref<{ ok: boolean; message: string; checking: boolean }>({
  ok: false, message: '', checking: false,
})

type Est = { files: number; bytes: number; loading: boolean; error?: string; approx?: boolean; volume?: boolean }
const estimates = ref<Record<string, Est>>({})

const quick = reactive({
  destination_id: '',
  source_mode: 'paths' as 'volume' | 'paths',
  volume: '',
  pathList: [] as string[],
  vss_enabled: true,
  name: '',
  backup_id: '',
})

const sourcesList = computed(() =>
  quick.source_mode === 'volume' ? (quick.volume ? [quick.volume] : []) : quick.pathList
)

function rescanSources() {
  for (const p of sourcesList.value) {
    scanPath(p)
  }
}

const {
  exclusion_patterns,
  volumeFolders,
  volumeFolderExclude,
  exclusionsList,
  loadVolumeFolders,
  toggleFolderExclude,
  isFolderExcluded,
  addExcludeFolder,
  loadDefaultExclusions,
  resetExclusions,
  onSourceModeVolume,
  onSourceModePaths,
} = useJobExclusions(
  toRef(quick, 'source_mode'),
  toRef(quick, 'volume'),
  sourcesList,
  rescanSources,
)

const totalEstimate = computed(() => {
  let files = 0, bytes = 0, loading = false, volume = false
  for (const p of sourcesList.value) {
    const e = estimates.value[p]
    if (!e) continue
    if (e.loading) loading = true
    if (e.volume) volume = true
    files += e.files || 0
    bytes += e.bytes || 0
  }
  return { files, bytes, loading, volume }
})

function typeLabel(tpe: string) {
  return formatBackupType(tpe)
}

async function refreshHistory() {
  const history = await GetHistory()
  if (history?.length) {
    const h = history[0]
    lastRun.value = `${formatJobName(h.job_name)} — ${formatLogStatus(h.status) || h.status} (${h.duration_sec} ${t('common.sec')})`
    lastType.value = typeLabel(h.backup_type)
    lastTransferred.value = formatBytes(h.bytes_transferred)
    lastReused.value = formatBytes(h.bytes_reused)
  } else {
    lastRun.value = ''
    lastType.value = ''
    lastTransferred.value = ''
    lastReused.value = ''
  }
}

async function loadSummary() {
  const [dests, jobs, isRunning, state, nextRunText, history] = await Promise.all([
    ListDestinations(),
    ListJobs(),
    IsBackupRunning(),
    GetExecutionState(),
    GetNextScheduledRun(),
    GetHistory(),
  ])
  destinations.value = dests
  serverCount.value = dests?.length || 0
  jobCount.value = jobs?.length || 0
  jobsEnabledCount.value = jobs?.filter((j) => j.schedule?.enabled).length || 0
  jobsDisabledCount.value = jobCount.value - jobsEnabledCount.value
  running.value = isRunning
  runningJobName.value = ''
  if (state.active?.length) {
    runningJobName.value = state.active[0].job_name || ''
  }
  nextRun.value = nextRunText
  if (history?.length) {
    const h = history[0]
    lastRun.value = `${formatJobName(h.job_name)} — ${formatLogStatus(h.status) || h.status} (${h.duration_sec} ${t('common.sec')})`
    lastType.value = typeLabel(h.backup_type)
    lastTransferred.value = formatBytes(h.bytes_transferred)
    lastReused.value = formatBytes(h.bytes_reused)
  } else {
    lastRun.value = ''
    lastType.value = ''
    lastTransferred.value = ''
    lastReused.value = ''
  }
  if (!quick.destination_id && destinations.value.length) quick.destination_id = destinations.value[0].id
}

async function ensureQuickBackupMeta() {
  if (!volumes.value.length) {
    volumes.value = await ListVolumes()
    if (!quick.volume && volumes.value.length) quick.volume = volumes.value[0]
  }
  if (!quick.backup_id) quick.backup_id = await GetHostname()
}

async function checkServer() {
  if (!quick.destination_id) return
  const dest = destinations.value.find((d) => d.id === quick.destination_id)
  if (!dest) return
  serverStatus.value = { ok: false, message: t('common.checking'), checking: true }
  try {
    const r = await TestDestinationConnection(dest, '')
    serverStatus.value = { ok: r.ok, message: r.message, checking: false }
  } catch (e: any) {
    serverStatus.value = { ok: false, message: e?.message || String(e), checking: false }
  }
}

async function scanPath(path: string) {
  if (!path) return
  estimates.value[path] = { files: 0, bytes: 0, loading: true }
  try {
    const r = await EstimatePath(path, exclusionsList.value)
    estimates.value[path] = {
      files: r.files || 0, bytes: r.bytes || 0, loading: false, error: r.error || undefined,
      approx: !!r.approx, volume: !!r.volume,
    }
  } catch (e: any) {
    estimates.value[path] = { files: 0, bytes: 0, loading: false, error: e?.message || String(e) }
  }
}

watch(() => quick.destination_id, () => { if (quickOpen.value) checkServer() })
watch(() => quick.volume, async (v) => {
  if (!quickOpen.value || quick.source_mode !== 'volume' || !v) return
  await loadVolumeFolders()
  scanPath(v)
})
watch(() => quick.source_mode, async (mode) => {
  if (!quickOpen.value) return
  if (mode === 'volume') {
    await onSourceModeVolume()
    if (quick.volume) scanPath(quick.volume)
  } else {
    onSourceModePaths()
  }
})
watch(exclusion_patterns, () => {
  if (quickOpen.value) rescanSources()
})

function destLabel(d: models.BackupDestination) {
  const tp = d.type === 'smb' ? 'SMB' : d.type === 'ftp' ? 'FTP' : 'PBS'
  return `${d.name} (${tp})`
}

function openQuick() {
  if (!destinations.value.length) {
    alert(t('dashboard_ext.add_destination_first'))
    router.push('/servers')
    return
  }
  quick.pathList = []
  quick.name = ''
  quickError.value = ''
  estimates.value = {}
  resetExclusions()
  quickOpen.value = true
  void ensureQuickBackupMeta()
  checkServer()
  if (quick.source_mode === 'volume') {
    onSourceModeVolume().then(() => {
      if (quick.volume) scanPath(quick.volume)
    })
  }
}

async function addQuickFolder() {
  const path = await PickFolder()
  if (path && !quick.pathList.includes(path)) {
    quick.pathList.push(path)
    await scanPath(path)
  }
}

function removePath(i: number) {
  const p = quick.pathList[i]
  quick.pathList.splice(i, 1)
  if (p) delete estimates.value[p]
}

async function clearLock() {
  const r = await ClearBackupLock()
  if (r.ok) {
    quickError.value = ''
  } else {
    quickError.value = r.message
  }
}

async function startQuick() {
  quickError.value = ''
  if (!serverStatus.value.ok) {
    await checkServer()
    if (!serverStatus.value.ok) {
      quickError.value = t('dashboard_ext.destination_unavailable', { message: serverStatus.value.message })
      return
    }
  }
  const sources = sourcesList.value
  if (!sources.length) {
    quickError.value = quick.source_mode === 'volume' ? t('jobs_ext.validate_volume') : t('jobs_ext.validate_paths')
    return
  }
  try {
    await RunQuickBackup(models.QuickBackupRequest.createFrom({
      name: quick.name.trim(),
      destination_id: quick.destination_id,
      server_id: quick.destination_id,
      source_mode: quick.source_mode,
      sources,
      exclusions: exclusionsList.value,
      vss_enabled: quick.vss_enabled,
      backup_id: quick.backup_id,
      force_full: false,
    }))
    quickOpen.value = false
    running.value = true
    router.push('/progress')
  } catch (e: any) {
    quickError.value = e?.message || String(e)
  }
}

async function restoreLast() {
  try {
    const last = await GetLastSuccessfulBackup()
    router.push({ path: '/restore', query: { job: last.job_id } })
  } catch (e: any) {
    alert(e?.message || t('dashboard_ext.no_successful_backups'))
  }
}

onMounted(() => {
  loadSummary()
  EventsOn('backup-finished', async () => {
    running.value = false
    await refreshHistory()
    jobCount.value = (await ListJobs())?.length || 0
  })
  EventsOn('progress', (p: any) => {
    const active = p.phase && !['done', 'idle', 'error', 'cancelled'].includes(p.phase)
    running.value = active
  })
})

onActivated(() => {
  loadSummary()
})
</script>

<template>
  <div class="page">
    <header class="page-header">
      <div>
        <h2>{{ t('dashboard.title') }}</h2>
        <p class="page-sub">{{ t('dashboard_ext.page_sub') }}</p>
      </div>
    </header>

    <div class="cards">
      <div class="card">
        <div class="card-label">{{ t('dashboard.last_backup') }}</div>
        <div class="card-value">{{ lastRun || t('dashboard.no_runs') }}</div>
        <div v-if="lastType" class="card-sub">
          {{ t('dashboard.backup_type') }}: {{ lastType }}
          · {{ t('dashboard.transferred') }}: {{ lastTransferred }}
          · {{ t('dashboard.reused') }}: {{ lastReused }}
        </div>
      </div>
      <div class="card">
        <div class="card-label">{{ t('dashboard.next_run') }}</div>
        <div class="card-value">{{ nextRun || t('common.dash') }}</div>
      </div>
      <div class="card">
        <div class="card-label">{{ t('dashboard.servers') }}</div>
        <div class="card-value">{{ serverCount }}</div>
      </div>
      <div class="card">
        <div class="card-label">{{ t('dashboard.jobs') }}</div>
        <div class="card-value">{{ jobCount }}</div>
        <div v-if="jobCount" class="card-sub">
          {{ t('dashboard.jobs_summary', { enabled: jobsEnabledCount, disabled: jobsDisabledCount }) }}
          <template v-if="runningJobName"> · {{ t('dashboard.running_job', { name: formatJobName(runningJobName) }) }}</template>
        </div>
      </div>
    </div>

    <div class="actions" v-if="running">
      <button class="btn primary" @click="router.push('/progress')">{{ t('dashboard.running') }}</button>
    </div>
    <div class="actions">
      <button class="btn primary" @click="openQuick">{{ t('dashboard.quick_backup') }}</button>
      <button class="btn ghost" @click="restoreLast">{{ t('dashboard.restore_last') }}</button>
      <button class="btn ghost" @click="router.push('/servers')">{{ t('nav.servers') }}</button>
      <button class="btn ghost" @click="router.push('/jobs')">{{ t('nav.jobs') }}</button>
    </div>

    <div
      v-if="quickOpen"
      class="sheet-backdrop"
      @pointerdown.self="onBackdropPointerDown"
      @click.self="onBackdropClick"
    >
      <div class="sheet">
        <header class="sheet-header">
          <h3>{{ t('dashboard.quick_backup') }}</h3>
          <button type="button" class="icon-btn" @click="quickOpen = false">×</button>
        </header>

        <div class="sheet-body">
          <section class="form-section">
            <label class="field">
              <span>{{ t('jobs.destination') }}</span>
              <select v-model="quick.destination_id">
                <option v-for="d in destinations" :key="d.id" :value="d.id">{{ destLabel(d) }}</option>
              </select>
            </label>
            <div class="server-status" :class="serverStatus.ok ? 'ok' : serverStatus.checking ? '' : 'bad'">
              <template v-if="serverStatus.checking">⏳ {{ t('common.checking') }}</template>
              <template v-else-if="serverStatus.ok">🟢 {{ serverStatus.message }}</template>
              <template v-else>🔴 {{ serverStatus.message || t('common.unavailable') }}</template>
              <button type="button" class="btn sm ghost" @click="checkServer">{{ t('common.check') }}</button>
            </div>
          </section>

          <section class="form-section">
            <h4>{{ t('jobs.source_mode') }}</h4>
            <div class="segment">
              <button type="button" :class="{ active: quick.source_mode === 'volume' }" @click="quick.source_mode = 'volume'">
                {{ t('jobs.mode_volume') }}
              </button>
              <button type="button" :class="{ active: quick.source_mode === 'paths' }" @click="quick.source_mode = 'paths'">
                {{ t('jobs.mode_paths') }}
              </button>
            </div>

            <label v-if="quick.source_mode === 'volume'" class="field">
              <span>{{ t('jobs.volume') }}</span>
              <select v-model="quick.volume">
                <option v-for="v in volumes" :key="v" :value="v">{{ v }}</option>
              </select>
              <p v-if="quick.volume && estimates[quick.volume]?.loading" class="hint">{{ t('jobs.volume_estimating') }}</p>
              <p v-else-if="quick.volume && estimates[quick.volume]?.volume" class="hint">
                {{ t('jobs.volume_used', { size: formatBytes(estimates[quick.volume].bytes) }) }}
              </p>
            </label>

            <div v-else class="paths-area">
              <button type="button" class="btn ghost" @click="addQuickFolder">+ {{ t('jobs.add_folder') }}</button>
              <div v-for="(p, i) in quick.pathList" :key="p" class="path-card">
                <div class="path-main">
                  <span class="path-icon">📁</span>
                  <div>
                    <div class="path-name">{{ p }}</div>
                    <div class="path-meta" v-if="estimates[p]?.loading">{{ t('common.counting') }}</div>
                    <div class="path-meta" v-else-if="estimates[p]">≈ {{ t('common.files_count', { n: estimates[p].files.toLocaleString() }) }} · {{ formatBytes(estimates[p].bytes) }}</div>
                  </div>
                </div>
                <button type="button" class="icon-btn danger" @click="removePath(i)">×</button>
              </div>
            </div>

            <div v-if="sourcesList.length" class="estimate-total">
              <template v-if="totalEstimate.loading">{{ t('jobs.estimating') }}</template>
              <template v-else-if="totalEstimate.volume">
                {{ t('jobs.volume_used', { size: formatBytes(totalEstimate.bytes) }) }}
              </template>
              <template v-else>
                {{ t('jobs.total_estimate', { files: totalEstimate.files.toLocaleString(), size: formatBytes(totalEstimate.bytes) }) }}
              </template>
            </div>
          </section>

          <section class="form-section">
            <h4>{{ t('jobs.exclusions') }}</h4>
            <p v-if="quick.source_mode === 'volume'" class="hint">{{ t('jobs.exclusions_volume_hint') }}</p>

            <div v-if="quick.source_mode === 'volume'" class="volume-excludes">
              <span class="field-label">{{ t('jobs.exclusions_folders') }}</span>
              <p v-if="!volumeFolders.length" class="hint">{{ t('jobs.exclusions_folders_empty') }}</p>
              <div v-else class="folder-exclude-list">
                <label
                  v-for="f in volumeFolders"
                  :key="f.path"
                  class="folder-exclude-row"
                  :class="{ system: f.system }"
                >
                  <input
                    type="checkbox"
                    :checked="f.system || isFolderExcluded(f.path)"
                    :disabled="f.system"
                    @change="toggleFolderExclude(f.path)"
                  />
                  <span class="folder-exclude-name">{{ f.name }}</span>
                  <span v-if="f.system" class="folder-exclude-badge">{{ t('jobs.exclusions_system_badge') }}</span>
                </label>
              </div>
              <button type="button" class="btn sm ghost" @click="addExcludeFolder">+ {{ t('jobs.add_folder') }}</button>
              <div v-if="volumeFolderExclude.size" class="extra-excludes">
                <span
                  v-for="p in [...volumeFolderExclude].filter((p) => !volumeFolders.some((f) => f.path === p))"
                  :key="p"
                  class="chip removable"
                  @click="toggleFolderExclude(p)"
                >{{ p }} ×</span>
              </div>
            </div>

            <label class="field">
              <span>{{ t('jobs.exclusions_patterns') }}</span>
              <textarea v-model="exclusion_patterns" rows="3" class="field-input" placeholder="*.tmp" />
            </label>
            <button type="button" class="btn sm ghost" @click="loadDefaultExclusions">{{ t('jobs.load_exclusions') }}</button>
          </section>

          <section class="form-section">
            <label class="field">
              <span>{{ t('jobs.name') }} <span class="optional">({{ t('common.optional') }})</span></span>
              <input v-model="quick.name" :placeholder="t('dashboard_ext.quick_name_ph')" />
            </label>
            <label class="toggle"><input type="checkbox" v-model="quick.vss_enabled" /><span>{{ t('jobs.vss') }}</span></label>
            <p class="hint">{{ t('dashboard_ext.quick_hint_save') }}</p>
          </section>

          <p v-if="quickError" class="alert error">✗ {{ quickError }}</p>
          <button v-if="lockBlocked" type="button" class="btn sm ghost" @click="clearLock">{{ t('dashboard_ext.clear_lock') }}</button>
        </div>

        <footer class="sheet-footer">
          <button type="button" class="btn primary" :disabled="serverStatus.checking" @click="startQuick">
            {{ t('jobs.run') }}
          </button>
          <button type="button" class="btn ghost" @click="quickOpen = false">{{ t('servers.cancel') }}</button>
        </footer>
      </div>
    </div>
  </div>
</template>

<style scoped>
.card-sub { margin-top: 0.5rem; font-size: 0.85rem; color: var(--muted); }
.server-status {
  display: flex; align-items: center; gap: 0.75rem; flex-wrap: wrap;
  padding: 0.6rem 0.75rem; border-radius: 8px; font-size: 0.85rem;
  background: var(--bg); border: 1px solid var(--border); margin-top: 0.5rem;
}
.server-status.ok { border-color: #166534; background: #052e16; }
.server-status.bad { border-color: #991b1b; background: #450a0a; }
.optional { color: var(--muted); font-weight: normal; }
.volume-excludes { margin: 0.75rem 0; }
.folder-exclude-list {
  display: flex; flex-direction: column; gap: 0.35rem;
  max-height: 200px; overflow-y: auto; margin: 0.5rem 0;
  padding: 0.5rem; border: 1px solid var(--border); border-radius: 8px;
}
.folder-exclude-row {
  display: flex; align-items: center; gap: 0.5rem;
  font-size: 0.9rem; cursor: pointer;
}
.folder-exclude-row.system { opacity: 0.7; cursor: default; }
.folder-exclude-name { flex: 1; }
.folder-exclude-badge {
  font-size: 0.7rem; padding: 0.1rem 0.4rem; border-radius: 4px;
  background: var(--border); color: var(--muted);
}
.extra-excludes { display: flex; flex-wrap: wrap; gap: 0.35rem; margin-top: 0.5rem; }
.chip.removable {
  font-size: 0.8rem; padding: 0.2rem 0.5rem; border-radius: 6px;
  background: var(--bg); border: 1px solid var(--border); cursor: pointer;
}
.field-label { display: block; font-size: 0.85rem; color: var(--muted); margin-bottom: 0.25rem; }
</style>
