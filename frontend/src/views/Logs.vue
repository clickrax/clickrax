<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { GetHistory, ExportHistoryDialog, ClearHistory } from '../../wailsjs/go/main/App'
import PaginationBar from '../components/PaginationBar.vue'
import { usePagination } from '../composables/usePagination'

import { formatBytes, formatLogStatus, formatBackupType, formatDateTime, formatJobName } from '../utils/format'

const { t } = useI18n()
const history = ref<any[]>([])
const clearing = ref(false)

const pagination = usePagination(history, 'pbs-logs-page-size', 25)
const pageItems = pagination.pageItems

function formatStatus(s: string) {
  return formatLogStatus(s) || t('common.dash')
}

function detailText(h: { status?: string; error?: string; message?: string; backup_type?: string; bytes_transferred?: number; bytes_reused?: number }) {
  if (h.status === 'error') {
    return h.error || h.message || t('logs_ext.unknown_error')
  }
  if (h.message) return h.message
  if (h.backup_type !== 'restore') {
    return `${formatBytes(h.bytes_transferred || 0)} / ${formatBytes(h.bytes_reused || 0)}`
  }
  return t('common.dash')
}

function formatType(h: { backup_type?: string }) {
  return formatBackupType(h.backup_type, true)
}

function formatWhen(s: string) {
  return formatDateTime(s)
}

async function load() {
  history.value = await GetHistory()
  pagination.resetPage()
}

onMounted(load)

async function exportLog() {
  const p = await ExportHistoryDialog()
  if (p) alert(t('settings.exported', { path: p }))
}

async function clearLog() {
  if (!history.value.length) return
  if (!confirm(t('logs.clear_confirm'))) return
  clearing.value = true
  try {
    await ClearHistory()
    await load()
  } catch (e: any) {
    alert(e?.message || String(e))
  } finally {
    clearing.value = false
  }
}

const hasHistory = computed(() => history.value.length > 0)
</script>

<template>
  <div class="page logs-page">
    <div class="page-header">
      <div>
        <h2>{{ t('logs.title') }}</h2>
        <p v-if="hasHistory" class="page-sub">{{ t('logs.total', { n: history.length }) }}</p>
      </div>
      <div class="btn-group">
        <button class="btn sm ghost" :disabled="!hasHistory || clearing" @click="clearLog">
          {{ clearing ? t('logs.clearing') : t('logs.clear') }}
        </button>
        <button class="btn sm" :disabled="!hasHistory" @click="exportLog">{{ t('logs.export') }}</button>
      </div>
    </div>

    <div v-if="hasHistory" class="table-card">
      <table class="table logs-table">
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
          <tr v-for="(h, i) in pageItems" :key="h.started_at + '-' + i" :class="{ 'row-restore': h.backup_type === 'restore' }">
            <td>{{ formatJobName(h.job_name) }}</td>
            <td><span :class="'status-' + h.status">{{ formatStatus(h.status) }}</span></td>
            <td>{{ formatType(h) }}</td>
            <td>{{ formatWhen(h.started_at) }}</td>
            <td>{{ h.duration_sec }} {{ t('common.sec') }}</td>
            <td class="details-cell" :class="{ 'details-error': h.status === 'error' }">
              {{ detailText(h) }}
            </td>
          </tr>
        </tbody>
      </table>
      <PaginationBar
        :page="pagination.page.value"
        :page-size="pagination.pageSize.value"
        :page-sizes="pagination.pageSizes"
        :total="pagination.total.value"
        :total-pages="pagination.totalPages.value"
        :range-from="pagination.rangeFrom.value"
        :range-to="pagination.rangeTo.value"
        :page-numbers="pagination.pageNumbers.value"
        @update:page="pagination.setPage"
        @update:page-size="pagination.setPageSize"
      />
    </div>
    <p v-else class="muted empty-card">{{ t('logs.empty') }}</p>
  </div>
</template>

<style scoped>
.logs-page .page-header {
  align-items: flex-start;
}
.details-cell {
  max-width: 480px;
  word-break: break-word;
  font-size: 0.85rem;
  color: var(--muted);
}
.details-error {
  color: #fca5a5;
}
.row-restore td:nth-child(3) {
  color: #93c5fd;
}
.logs-table {
  table-layout: fixed;
  width: 100%;
}
</style>
