<script setup lang="ts">
import { computed, onActivated, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import {
  ListJobs, ListDestinations, SaveJob, DeleteJob, StartBackup, StartForceFullBackup,
  SaveJobAndRun, GetHostname, PickFolder, ListVolumes, ListVolumeFolders, GetDefaultExclusions, EstimatePath,
  GetExecutionState,
} from '../../wailsjs/go/main/App'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import { models } from '../../wailsjs/go/models'
import JobActionIcons from '../components/JobActionIcons.vue'
import { useResizableColumns } from '../composables/useResizableColumns'
import { useBackdropDismiss } from '../composables/useBackdropDismiss'
import {
  formatSchedule,
  getSchedulePresets,
  schedulesMatch,
  weekdayShort,
  type SchedulePreset,
} from '../utils/scheduleExpr'
import { formatBytes } from '../utils/format'

defineOptions({ name: 'Jobs' })

const jobsTableRef = ref<HTMLElement | null>(null)

const JOB_COLUMNS = [
  { key: 'dest', defaultPercent: 9, minPercent: 7 },
  { key: 'name', defaultPercent: 11, minPercent: 8 },
  { key: 'status', defaultPercent: 8, minPercent: 6 },
  { key: 'sources', defaultPercent: 16, minPercent: 10 },
  { key: 'bid', defaultPercent: 7, minPercent: 5 },
  { key: 'vss', defaultPercent: 5, minPercent: 4 },
  { key: 'sched', defaultPercent: 20, minPercent: 12 },
  { key: 'actions', fixedPx: 108 },
]

const { colStyle, startResize } = useResizableColumns('pbs-jobs-col-pct-v3', JOB_COLUMNS, jobsTableRef)

const { t } = useI18n()
const router = useRouter()

const jobs = ref<models.BackupJob[]>([])
const activeJobIds = ref<Set<string>>(new Set())
const queuedJobIds = ref<Set<string>>(new Set())
const destinations = ref<models.BackupDestination[]>([])
const volumes = ref<string[]>([])
const editing = ref(false)
const { onBackdropPointerDown, onBackdropClick } = useBackdropDismiss(() => { editing.value = false })
const errorMsg = ref('')

type Est = { files: number; bytes: number; loading: boolean; error?: string; approx?: boolean; volume?: boolean }
const estimates = ref<Record<string, Est>>({})
const volumeFolders = ref<models.VolumeFolder[]>([])
const volumeFolderExclude = ref<Set<string>>(new Set())

const form = reactive({
  id: '', name: '', destination_id: '',
  source_mode: 'paths' as 'volume' | 'paths',
  volume: '', pathList: [] as string[], exclusion_patterns: '',
  backup_id: '', vss_enabled: true, split_enabled: false,
  schedule_enabled: true,
  schedule_times: ['02:00'] as string[], schedule_type: 'daily' as 'daily' | 'weekly',
  schedule_weekdays: [1, 2, 3, 4, 5, 6, 7] as number[],
  schedule_full_mode: 'weekly' as 'weekly' | 'biweekly' | 'monthly' | 'never',
  schedule_full_weekday: 7,
  schedule_full_anchor: '',
  schedule_full_time: '02:00',
  schedule_skip_if_running: true,
  verify_after_backup: true, encryption_enabled: false, passphrase: '', comment: '',
  notify_backup: 'inherit', notify_restore: 'inherit',
})

const selectedDest = computed(() =>
  destinations.value.find((d) => d.id === form.destination_id)
)
const isPBSDest = computed(() => !selectedDest.value || selectedDest.value.type === 'pbs' || !selectedDest.value.type)

function destLabel(d: models.BackupDestination) {
  const t = d.type === 'smb' ? 'SMB' : d.type === 'ftp' ? 'FTP' : 'PBS'
  return `${d.name} (${t})`
}

function jobDestName(j: models.BackupJob) {
  const id = j.destination_id || j.server_id
  const d = destinations.value.find((x) => x.id === id)
  return d ? destLabel(d) : id || '—'
}

const sourcesList = computed(() =>
  form.source_mode === 'volume' ? (form.volume ? [form.volume] : []) : form.pathList.filter(Boolean)
)
const exclusionsList = computed(() => {
  const patterns = form.exclusion_patterns.split('\n').map((s) => s.trim()).filter(Boolean)
  const folders = [...volumeFolderExclude.value]
  return [...folders, ...patterns]
})

function normPathKey(p: string) {
  return p.replace(/\\+$/, '').toLowerCase()
}

function isChildOfVolume(path: string, volume: string) {
  if (!volume) return false
  const vol = normPathKey(volume.endsWith('\\') ? volume : `${volume}\\`)
  const p = normPathKey(path)
  const root = vol.slice(0, -1)
  if (!p.startsWith(root)) return false
  const rest = p.slice(root.length).replace(/^\\/, '')
  return rest !== '' && !rest.includes('\\')
}

function splitJobExclusions(exclusions: string[], volume: string) {
  const folders: string[] = []
  const patterns: string[] = []
  for (const e of exclusions || []) {
    const line = (e || '').trim()
    if (!line) continue
    if (volume && isChildOfVolume(line, volume)) {
      folders.push(line)
    } else {
      patterns.push(line)
    }
  }
  return { folders, patterns }
}

async function loadVolumeFolders() {
  if (form.source_mode !== 'volume' || !form.volume) {
    volumeFolders.value = []
    return
  }
  try {
    volumeFolders.value = await ListVolumeFolders(form.volume)
  } catch {
    volumeFolders.value = []
  }
}

function toggleFolderExclude(path: string) {
  const next = new Set(volumeFolderExclude.value)
  if (next.has(path)) next.delete(path)
  else next.add(path)
  volumeFolderExclude.value = next
}

function isFolderExcluded(path: string) {
  return volumeFolderExclude.value.has(path)
}

async function addExcludeFolder() {
  const p = await PickFolder()
  if (!p) return
  const next = new Set(volumeFolderExclude.value)
  next.add(p)
  volumeFolderExclude.value = next
}

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

const weekdayOptions = computed(() =>
  [1, 2, 3, 4, 5, 6, 7].map((v) => ({
    v,
    l: t(`jobs.weekday_${['', 'mon', 'tue', 'wed', 'thu', 'fri', 'sat', 'sun'][v]}`),
  })),
)

const schedulePresets = computed(() => getSchedulePresets())

const scheduleSummary = computed(() =>
  formatSchedule(
    form.schedule_type,
    form.schedule_times,
    form.schedule_type === 'weekly' ? form.schedule_weekdays : [],
  ),
)

function currentScheduleShape() {
  return {
    type: form.schedule_type,
    times: [...form.schedule_times],
    weekdays: form.schedule_type === 'weekly' ? [...form.schedule_weekdays] : [],
  }
}

function isActivePreset(preset: SchedulePreset) {
  return schedulesMatch(currentScheduleShape(), preset)
}

function toggleWeekday(day: number) {
  const i = form.schedule_weekdays.indexOf(day)
  if (i >= 0) {
    if (form.schedule_weekdays.length > 1) form.schedule_weekdays.splice(i, 1)
  } else {
    form.schedule_weekdays.push(day)
    form.schedule_weekdays.sort((a, b) => a - b)
  }
}

function addScheduleTime() {
  form.schedule_times.push('12:00')
}

function removeScheduleTime(idx: number) {
  if (form.schedule_times.length <= 1) return
  form.schedule_times.splice(idx, 1)
}

function applySchedulePreset(preset: SchedulePreset) {
  form.schedule_type = preset.type
  form.schedule_times = [...preset.times]
  form.schedule_weekdays = preset.type === 'weekly' ? [...preset.weekdays] : [1, 2, 3, 4, 5, 6, 7]
  form.schedule_full_time = preset.times[0] || '02:00'
  if (preset.type === 'weekly' && preset.weekdays.length === 1) {
    form.schedule_full_weekday = preset.weekdays[0]
  }
}

function onScheduleTypeChange() {
  if (form.schedule_type === 'daily') {
    form.schedule_weekdays = [1, 2, 3, 4, 5, 6, 7]
  } else if (!form.schedule_weekdays.length) {
    form.schedule_weekdays = [1, 2, 3, 4, 5]
  }
}

function applyWeeklyFullBackupSchedule() {
  const time = form.schedule_full_time || '02:00'
  if (form.schedule_times.length) form.schedule_times[0] = time
  else form.schedule_times = [time]
}

function isScheduleEnabled(j: models.BackupJob) {
  return !!j.schedule?.enabled
}

const runningPhases = new Set([
  'preparing', 'analyzing', 'vss', 'transfer', 'finalizing', 'verify',
])

function jobStatusLabel(j: models.BackupJob) {
  if (activeJobIds.value.has(j.id)) return t('jobs.status_running')
  if (queuedJobIds.value.has(j.id)) return t('jobs.status_queued')
  if (isScheduleEnabled(j)) return t('jobs.status_enabled')
  return t('jobs.status_disabled')
}

function jobStatusClass(j: models.BackupJob) {
  if (activeJobIds.value.has(j.id)) return 'pill-run'
  if (queuedJobIds.value.has(j.id)) return 'pill-queue'
  if (isScheduleEnabled(j)) return 'pill-ok'
  return 'pill-off'
}

async function refreshExecutionState() {
  try {
    const state = await GetExecutionState()
    const active = new Set<string>()
    for (const run of state.active || []) {
      if (run.job_id && runningPhases.has(run.phase)) active.add(run.job_id)
    }
    activeJobIds.value = active
    const queued = new Set<string>()
    for (const q of state.queued || []) {
      if (q.job_id) queued.add(q.job_id)
    }
    queuedJobIds.value = queued
  } catch { /* ignore */ }
}

function jobTimes(j: models.BackupJob): string[] {
  const s = j.schedule
  if (s?.times?.length) return s.times.map((t) => (t || '').slice(0, 5))
  if (s?.time) return [(s.time || '').slice(0, 5)]
  return []
}

function fullBackupShortLabel(mode: string | undefined, fullDay: number) {
  const day = weekdayShort(fullDay)
  switch (mode) {
    case 'never':
      return t('jobs_ext.full_never')
    case 'biweekly':
      return t('jobs_ext.full_biweekly', { day })
    case 'monthly':
      return t('jobs_ext.full_monthly', { day })
    default:
      return t('jobs_ext.full_weekly', { day })
  }
}

function scheduleLabel(j: models.BackupJob) {
  const s = j.schedule
  const times = jobTimes(j)
  const label = formatSchedule(
    s?.type === 'weekly' ? 'weekly' : 'daily',
    times,
    s?.weekdays || [],
  )
  if (!s?.enabled) {
    if (times.length) return t('jobs_ext.schedule_disabled', { label })
    return t('common.dash')
  }
  if (!times.length) return t('common.dash')
  const fullDay = s?.full_backup_weekday || 7
  const full = fullBackupShortLabel(s?.full_backup_mode, fullDay)
  return `${label} (${full})`
}

function resetForm() {
  Object.assign(form, {
    id: '', name: '', source_mode: 'paths', volume: volumes.value[0] || '',
    pathList: [], exclusion_patterns: '', vss_enabled: true, split_enabled: false,
    schedule_enabled: true,
    schedule_times: ['02:00'], schedule_type: 'daily' as 'daily' | 'weekly',
    schedule_weekdays: [1, 2, 3, 4, 5, 6, 7], schedule_full_mode: 'weekly',
    schedule_full_weekday: 7, schedule_full_anchor: '', schedule_full_time: '02:00', schedule_skip_if_running: true,
    verify_after_backup: true, encryption_enabled: false, passphrase: '', comment: '',
    notify_backup: 'inherit', notify_restore: 'inherit',
  })
  volumeFolderExclude.value = new Set()
  volumeFolders.value = []
  estimates.value = {}
  errorMsg.value = ''
}

async function load() {
  const [jobsList, dests, vols] = await Promise.all([
    ListJobs(),
    ListDestinations(),
    ListVolumes(),
  ])
  jobs.value = jobsList
  destinations.value = dests
  volumes.value = vols
  await refreshExecutionState()
  if (!form.backup_id) form.backup_id = await GetHostname()
  if (!form.destination_id && destinations.value.length) form.destination_id = destinations.value[0].id
  if (!form.volume && volumes.value.length) form.volume = volumes.value[0]
}

async function scanPath(path: string) {
  if (!path) return
  estimates.value[path] = { files: 0, bytes: 0, loading: true }
  try {
    const r = await EstimatePath(path, exclusionsList.value)
    estimates.value[path] = {
      files: r.files || 0,
      bytes: r.bytes || 0,
      loading: false,
      error: r.error || undefined,
      approx: !!r.approx,
      volume: !!r.volume,
    }
  } catch (e: any) {
    estimates.value[path] = { files: 0, bytes: 0, loading: false, error: e?.message || String(e) }
  }
}

function nextFullBackupAnchor(weekday: number) {
  const now = new Date()
  const today = now.getDay() === 0 ? 7 : now.getDay()
  let delta = weekday - today
  if (delta < 0) delta += 7
  const d = new Date(now.getFullYear(), now.getMonth(), now.getDate() + delta)
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${y}-${m}-${day}`
}

function normalizeFullBackupMode(mode: string | undefined): 'weekly' | 'biweekly' | 'monthly' | 'never' {
  if (mode === 'never' || mode === 'biweekly' || mode === 'monthly') return mode
  return 'weekly'
}

watch(() => form.volume, (v) => {
  if (form.source_mode === 'volume' && v) {
    loadVolumeFolders()
    scanPath(v)
  }
})
watch(() => form.source_mode, (mode) => {
  if (mode === 'volume') {
    loadVolumeFolders()
    if (!form.exclusion_patterns.trim()) {
      loadDefaultExclusions()
    }
    if (form.volume) scanPath(form.volume)
  } else if (!form.pathList.length) {
    volumeFolders.value = []
    volumeFolderExclude.value = new Set()
  }
})
watch(() => form.schedule_full_mode, (mode) => {
  if (mode === 'biweekly' && !form.schedule_full_anchor) {
    form.schedule_full_anchor = nextFullBackupAnchor(form.schedule_full_weekday)
  }
})

watch(() => form.schedule_full_weekday, (weekday) => {
  if (form.schedule_full_mode === 'biweekly' && !form.schedule_full_anchor) {
    form.schedule_full_anchor = nextFullBackupAnchor(weekday)
  }
})

function openNew() { resetForm(); editing.value = true }

function edit(j: models.BackupJob) {
  form.id = j.id
  form.name = j.name
  form.destination_id = j.destination_id || j.server_id || ''
  form.source_mode = j.source_mode === 'volume' ? 'volume' : 'paths'
  if (form.source_mode === 'volume' && j.sources?.length) {
    form.volume = j.sources[0]
    form.pathList = []
  } else {
    form.pathList = [...(j.sources || [])]
    form.volume = volumes.value[0] || ''
  }
  const vol = form.source_mode === 'volume' && j.sources?.length ? j.sources[0] : ''
  const split = splitJobExclusions(j.exclusions || [], vol)
  volumeFolderExclude.value = new Set(split.folders)
  form.exclusion_patterns = split.patterns.join('\n')
  if (form.source_mode === 'volume') {
    loadVolumeFolders()
  }
  form.backup_id = j.backup_id
  form.vss_enabled = j.vss_enabled
  form.split_enabled = j.split_enabled
  const times = jobTimes(j)
  form.schedule_enabled = !!j.schedule?.enabled
  form.schedule_times = times.length ? [...times] : ['02:00']
  form.schedule_type = (j.schedule?.type === 'weekly' ? 'weekly' : 'daily')
  form.schedule_weekdays = j.schedule?.weekdays?.length ? [...j.schedule.weekdays] : [1, 2, 3, 4, 5, 6, 7]
  form.schedule_full_mode = normalizeFullBackupMode(j.schedule?.full_backup_mode)
  form.schedule_full_weekday = j.schedule?.full_backup_weekday || 7
  form.schedule_full_anchor = j.schedule?.full_backup_anchor || ''
  if (form.schedule_full_mode === 'biweekly' && !form.schedule_full_anchor) {
    form.schedule_full_anchor = nextFullBackupAnchor(form.schedule_full_weekday)
  }
  form.schedule_full_time = form.schedule_times[0] || '02:00'
  form.schedule_skip_if_running = j.schedule?.skip_if_running ?? true
  form.verify_after_backup = j.verify_after_backup ?? true
  form.encryption_enabled = j.encryption_enabled ?? false
  form.passphrase = ''
  form.comment = j.comment || ''
  form.notify_backup = j.notify_backup || 'inherit'
  form.notify_restore = j.notify_restore || 'inherit'
  estimates.value = {}
  sourcesList.value.forEach(scanPath)
  errorMsg.value = ''
  editing.value = true
}

function buildJob() {
  const times = form.schedule_times.filter((t) => (t || '').trim())
  const scheduleEnabled = form.schedule_enabled
  return models.BackupJob.createFrom({
    id: form.id, name: form.name,
    destination_id: form.destination_id,
    server_id: form.destination_id,
    source_mode: form.source_mode, sources: sourcesList.value,
    exclusions: exclusionsList.value, backup_id: form.backup_id,
    vss_enabled: form.vss_enabled, split_enabled: form.split_enabled,
    skip_access_errors: true, verify_after_backup: form.verify_after_backup,
    encryption_enabled: form.encryption_enabled, comment: form.comment,
    notify_backup: form.notify_backup,
    notify_restore: form.notify_restore,
    schedule: {
      enabled: scheduleEnabled, type: form.schedule_type,
      time: times[0] || '02:00',
      times: [...times],
      weekdays: form.schedule_type === 'weekly' ? [...form.schedule_weekdays] : [],
      full_backup_mode: form.schedule_full_mode,
      full_backup_weekday: form.schedule_full_weekday,
      full_backup_anchor: form.schedule_full_mode === 'biweekly' ? form.schedule_full_anchor : '',
      skip_if_running: form.schedule_skip_if_running,
    },
  })
}

function validate(): string | null {
  if (!form.name.trim()) return t('jobs_ext.validate_name')
  if (!form.destination_id) return t('jobs_ext.validate_destination')
  if (!sourcesList.value.length) {
    return form.source_mode === 'volume' ? t('jobs_ext.validate_volume') : t('jobs_ext.validate_paths')
  }
  if (form.schedule_enabled && !form.schedule_times.some((x) => (x || '').trim())) {
    return t('jobs_ext.validate_time')
  }
  if (form.schedule_enabled && form.schedule_type === 'weekly' && !form.schedule_weekdays.length) {
    return t('jobs_ext.validate_weekday')
  }
  return null
}

async function save() {
  const err = validate()
  if (err) { errorMsg.value = err; return }
  await SaveJob(buildJob(), form.passphrase)
  editing.value = false
  await load()
}

async function saveAndRun() {
  const err = validate()
  if (err) { errorMsg.value = err; return }
  try {
    await SaveJobAndRun(buildJob(), form.passphrase, false)
    editing.value = false
    router.push('/progress')
  } catch (e: any) { errorMsg.value = e?.message || String(e) }
}

async function remove(id: string) {
  if (!confirm(t('jobs_ext.delete_confirm'))) return
  await DeleteJob(id)
  await load()
}

async function run(id: string) {
  try { await StartBackup(id); router.push('/progress') }
  catch (e: any) {
    const raw = e?.message || String(e)
    if (raw.toLowerCase().includes('file already exists') || raw.includes('очередь бэкапов') || raw.toLowerCase().includes('backup queue')) {
      alert(t('jobs_ext.queue_blocked'))
    } else {
      alert(raw)
    }
  }
}

async function runFull(id: string) {
  if (!confirm(t('jobs_ext.full_backup_confirm'))) return
  try { await StartForceFullBackup(id); router.push('/progress') }
  catch (e: any) { alert(e?.message || String(e)) }
}

async function addFolder() {
  const path = await PickFolder()
  if (path && !form.pathList.includes(path)) {
    form.pathList.push(path)
    await scanPath(path)
  }
}

function removePath(idx: number) {
  const p = form.pathList[idx]
  form.pathList.splice(idx, 1)
  if (p) delete estimates.value[p]
}

async function loadDefaultExclusions() {
  const defaults = await GetDefaultExclusions() || []
  form.exclusion_patterns = defaults
    .filter((d) => !d.includes('\\') && !d.includes('/'))
    .join('\n')
  sourcesList.value.forEach(scanPath)
}

function sourcesPreview(j: models.BackupJob) {
  if (!j.sources?.length) return t('common.dash')
  return j.sources.length === 1 ? j.sources[0] : `${j.sources[0]} (+${j.sources.length - 1})`
}

function sourcesTitle(j: models.BackupJob) {
  if (!j.sources?.length) return ''
  return j.sources.join('\n')
}

onMounted(() => {
  load()
  EventsOn('progress', () => { refreshExecutionState() })
  EventsOn('backup-queued', () => { refreshExecutionState() })
  EventsOn('backup-finished', () => { refreshExecutionState() })
})
onActivated(() => {
  load()
})
onUnmounted(() => {})
</script>

<template>
  <div class="page jobs-page">
    <header class="page-header">
      <div>
        <h2>{{ t('jobs.title') }}</h2>
        <p class="page-sub">{{ t('jobs_ext.page_sub') }}</p>
      </div>
      <button class="btn primary" @click="openNew">{{ t('jobs.add') }}</button>
    </header>

    <div v-if="jobs.length" class="table-card jobs-table-wrap">
      <table ref="jobsTableRef" class="table jobs-table">
        <colgroup>
          <col :style="colStyle('dest')" />
          <col :style="colStyle('name')" />
          <col :style="colStyle('status')" />
          <col :style="colStyle('sources')" />
          <col :style="colStyle('bid')" class="col-hide-narrow" />
          <col :style="colStyle('vss')" class="col-hide-medium" />
          <col :style="colStyle('sched')" />
          <col :style="colStyle('actions')" />
        </colgroup>
        <thead>
          <tr>
            <th class="resizable-th">
              <span>{{ t('jobs.destination') }}</span>
              <span class="col-resizer" @mousedown="startResize('dest', $event)" />
            </th>
            <th class="resizable-th">
              <span>{{ t('jobs.name') }}</span>
              <span class="col-resizer" @mousedown="startResize('name', $event)" />
            </th>
            <th class="resizable-th">
              <span>{{ t('jobs.status') }}</span>
              <span class="col-resizer" @mousedown="startResize('status', $event)" />
            </th>
            <th class="resizable-th">
              <span>{{ t('jobs.sources') }}</span>
              <span class="col-resizer" @mousedown="startResize('sources', $event)" />
            </th>
            <th class="resizable-th col-hide-narrow">
              <span>Backup ID</span>
              <span class="col-resizer" @mousedown="startResize('bid', $event)" />
            </th>
            <th class="resizable-th col-hide-medium">
              <span>VSS</span>
              <span class="col-resizer" @mousedown="startResize('vss', $event)" />
            </th>
            <th class="resizable-th">
              <span>{{ t('jobs.schedule') }}</span>
              <span class="col-resizer" @mousedown="startResize('sched', $event)" />
            </th>
            <th class="actions-head"></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="j in jobs" :key="j.id">
            <td class="cell-dest">{{ jobDestName(j) }}</td>
            <td class="cell-name"><strong>{{ j.name }}</strong></td>
            <td class="cell-status">
              <span class="pill" :class="jobStatusClass(j)">{{ jobStatusLabel(j) }}</span>
            </td>
            <td class="path-cell" :title="sourcesTitle(j)">{{ sourcesPreview(j) }}</td>
            <td class="cell-bid col-hide-narrow">{{ j.backup_id }}</td>
            <td class="cell-vss col-hide-medium">
              <span class="pill" :class="j.vss_enabled ? 'pill-ok' : ''">{{ j.vss_enabled ? 'VSS' : '—' }}</span>
            </td>
            <td class="cell-sched" :title="scheduleLabel(j)">{{ scheduleLabel(j) }}</td>
            <td class="actions-cell">
              <JobActionIcons
                :run-title="t('jobs.run')"
                :full-title="t('jobs.full_backup_mode')"
                :edit-title="t('servers.edit')"
                :delete-title="t('jobs.delete')"
                @run="run(j.id)"
                @full="runFull(j.id)"
                @edit="edit(j)"
                @remove="remove(j.id)"
              />
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <div v-else class="empty-card">
      <p>{{ t('jobs_ext.no_jobs') }}</p>
      <button class="btn primary" @click="openNew">{{ t('jobs.add') }}</button>
    </div>

    <div
      v-if="editing"
      class="sheet-backdrop"
      @pointerdown.self="onBackdropPointerDown"
      @click.self="onBackdropClick"
    >
      <div class="sheet">
        <header class="sheet-header">
          <h3>{{ form.id ? t('servers.edit') : t('jobs.add') }}</h3>
          <button class="icon-btn" @click="editing = false" :aria-label="t('common.close')">×</button>
        </header>

        <div class="sheet-body">
          <section class="form-section">
            <h4>{{ t('jobs_ext.section_main') }}</h4>
            <div class="field-grid">
              <label class="field">
                <span>{{ t('jobs.name') }}</span>
                <input v-model="form.name" :placeholder="t('jobs_ext.placeholder_name')" />
              </label>
              <label class="field">
                <span>{{ t('jobs.destination') }}</span>
                <select v-model="form.destination_id">
                  <option v-for="d in destinations" :key="d.id" :value="d.id">{{ destLabel(d) }}</option>
                </select>
              </label>
              <label class="field">
                <span>{{ t('jobs.backup_id') }}</span>
                <input v-model="form.backup_id" />
              </label>
            </div>
          </section>

          <section class="form-section">
            <h4>{{ t('jobs.source_mode') }}</h4>
            <div class="segment">
              <button type="button" :class="{ active: form.source_mode === 'volume' }" @click="form.source_mode = 'volume'">
                {{ t('jobs.mode_volume') }}
              </button>
              <button type="button" :class="{ active: form.source_mode === 'paths' }" @click="form.source_mode = 'paths'">
                {{ t('jobs.mode_paths') }}
              </button>
            </div>

            <div v-if="form.source_mode === 'volume'" class="field">
              <span>{{ t('jobs.volume') }}</span>
              <select v-model="form.volume">
                <option v-for="v in volumes" :key="v" :value="v">{{ v }}</option>
              </select>
              <p v-if="form.volume && estimates[form.volume]?.loading" class="hint">{{ t('jobs.volume_estimating') }}</p>
              <p v-else-if="form.volume && estimates[form.volume]?.error" class="hint warn">{{ estimates[form.volume].error }}</p>
              <p v-else-if="form.volume && estimates[form.volume]?.volume" class="hint">
                {{ t('jobs.volume_used', { size: formatBytes(estimates[form.volume].bytes) }) }}
              </p>
            </div>

            <div v-else class="paths-area">
              <button type="button" class="btn ghost" @click="addFolder">+ {{ t('jobs.add_folder') }}</button>
              <div v-for="(p, i) in form.pathList" :key="p" class="path-card">
                <div class="path-main">
                  <span class="path-icon">📁</span>
                  <div>
                    <div class="path-name">{{ p }}</div>
                    <div class="path-meta" v-if="estimates[p]?.loading">{{ t('common.counting') }}</div>
                    <div class="path-meta error" v-else-if="estimates[p]?.error">{{ estimates[p].error }}</div>
                    <div class="path-meta" v-else-if="estimates[p]">
                      ≈ {{ t('common.files_count', { n: estimates[p].files.toLocaleString() }) }} · {{ formatBytes(estimates[p].bytes) }}
                    </div>
                  </div>
                </div>
                <button type="button" class="icon-btn danger" @click="removePath(i)">×</button>
              </div>
              <p v-if="!form.pathList.length" class="hint">{{ t('jobs.no_paths') }}</p>
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
            <p v-if="form.source_mode === 'volume'" class="hint">{{ t('jobs.exclusions_volume_hint') }}</p>

            <div v-if="form.source_mode === 'volume'" class="volume-excludes">
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
              <textarea v-model="form.exclusion_patterns" rows="3" class="field-input" placeholder="*.tmp" />
            </label>
            <button type="button" class="btn sm ghost" @click="loadDefaultExclusions">{{ t('jobs.load_exclusions') }}</button>
          </section>

          <section class="form-section cols-2">
            <h4>{{ t('jobs_ext.section_params') }}</h4>
            <label v-if="isPBSDest" class="toggle"><input type="checkbox" v-model="form.vss_enabled" /><span>{{ t('jobs.vss') }}</span></label>
            <label v-if="isPBSDest" class="toggle"><input type="checkbox" v-model="form.verify_after_backup" /><span>{{ t('jobs.verify') }}</span></label>
            <label v-if="isPBSDest" class="toggle"><input type="checkbox" v-model="form.split_enabled" /><span>{{ t('jobs.split') }}</span></label>
            <label v-if="isPBSDest" class="toggle"><input type="checkbox" v-model="form.encryption_enabled" /><span>{{ t('jobs.encryption') }}</span></label>
            <p v-if="!isPBSDest" class="hint">{{ t('jobs.file_dest_hint') }}</p>
            <p v-if="form.encryption_enabled" class="hint warn">{{ t('jobs.encryption_warn') }}</p>
            <label v-if="form.encryption_enabled" class="field full">
              <span>{{ t('jobs.passphrase') }}</span>
              <input v-model="form.passphrase" type="password" />
            </label>
            <label class="field full">
              <span>{{ t('jobs.comment') }}</span>
              <input v-model="form.comment" :placeholder="t('jobs.comment')" />
            </label>
          </section>

          <section class="form-section">
            <h4>{{ t('jobs.schedule') }}</h4>
            <label class="toggle schedule-toggle"><input type="checkbox" v-model="form.schedule_enabled" /><span>{{ t('jobs.enabled') }}</span></label>

            <div class="schedule-presets">
              <span class="field-label">{{ t('jobs.schedule_presets') }}</span>
              <div class="preset-chips">
                <button
                  v-for="preset in schedulePresets"
                  :key="preset.label"
                  type="button"
                  class="chip"
                  :class="{ active: isActivePreset(preset) }"
                  @click="applySchedulePreset(preset)"
                >
                  {{ preset.label }}
                </button>
              </div>
            </div>

            <div class="field-grid">
              <label class="field">
                <span>{{ t('jobs.schedule_type') }}</span>
                <select v-model="form.schedule_type" @change="onScheduleTypeChange">
                  <option value="daily">{{ t('jobs.daily') }}</option>
                  <option value="weekly">{{ t('jobs.weekly') }}</option>
                </select>
              </label>
            </div>

            <div v-if="form.schedule_type === 'weekly'" class="weekday-row">
              <span class="field-label">{{ t('jobs.schedule_weekdays') }}</span>
              <div class="weekday-chips">
                <button
                  v-for="d in weekdayOptions"
                  :key="d.v"
                  type="button"
                  class="chip"
                  :class="{ active: form.schedule_weekdays.includes(d.v) }"
                  @click="toggleWeekday(d.v)"
                >
                  {{ d.l }}
                </button>
              </div>
            </div>

            <div class="schedule-times">
              <span class="field-label">{{ t('jobs.times') }}</span>
              <div v-for="(_tm, idx) in form.schedule_times" :key="idx" class="time-row">
                <input v-model="form.schedule_times[idx]" type="time" />
                <button
                  type="button"
                  class="btn sm ghost icon"
                  :disabled="form.schedule_times.length <= 1"
                  :title="t('jobs_ext.remove_time')"
                  @click="removeScheduleTime(idx)"
                >×</button>
              </div>
              <button type="button" class="btn sm ghost" @click="addScheduleTime">+ {{ t('jobs.add_time') }}</button>
            </div>

            <p class="schedule-summary">{{ t('jobs.schedule_result') }}: <strong>{{ scheduleSummary }}</strong></p>

            <div class="full-backup-block">
              <label class="field">
                <span>{{ t('jobs.full_backup_mode') }}</span>
                <select v-model="form.schedule_full_mode">
                  <option value="weekly">{{ t('jobs.full_backup_weekly') }}</option>
                  <option value="biweekly">{{ t('jobs.full_backup_biweekly') }}</option>
                  <option value="monthly">{{ t('jobs.full_backup_monthly') }}</option>
                  <option value="never">{{ t('jobs.full_backup_never') }}</option>
                </select>
              </label>
              <label v-if="form.schedule_full_mode !== 'never'" class="field">
                <span>{{ t('jobs.full_backup_when') }}</span>
                <div class="full-backup-when">
                  <select v-model.number="form.schedule_full_weekday">
                    <option v-for="d in weekdayOptions" :key="d.v" :value="d.v">{{ d.l }}</option>
                  </select>
                  <input
                    v-model="form.schedule_full_time"
                    type="time"
                    @change="applyWeeklyFullBackupSchedule"
                  />
                </div>
              </label>
            </div>
            <p v-if="form.schedule_full_mode === 'weekly'" class="hint">{{ t('jobs.full_backup_schedule_hint') }}</p>
            <p v-else-if="form.schedule_full_mode === 'biweekly'" class="hint">{{ t('jobs.full_backup_biweekly_hint') }}</p>
            <p v-else-if="form.schedule_full_mode === 'monthly'" class="hint">{{ t('jobs.full_backup_monthly_hint') }}</p>
            <label class="toggle"><input type="checkbox" v-model="form.schedule_skip_if_running" /><span>{{ t('jobs.skip_if_running') }}</span></label>
          </section>

          <section class="form-section">
            <h4>{{ t('jobs.notify_title') }}</h4>
            <p class="hint">{{ t('jobs.notify_hint') }}</p>
            <div class="field-grid">
              <label class="field">
                <span>{{ t('jobs.notify_backup') }}</span>
                <select v-model="form.notify_backup">
                  <option value="inherit">{{ t('jobs.notify_inherit') }}</option>
                  <option value="off">{{ t('jobs.notify_off') }}</option>
                  <option value="always">{{ t('jobs.notify_always') }}</option>
                  <option value="failure">{{ t('jobs.notify_failure') }}</option>
                </select>
              </label>
              <label class="field">
                <span>{{ t('jobs.notify_restore') }}</span>
                <select v-model="form.notify_restore">
                  <option value="inherit">{{ t('jobs.notify_inherit') }}</option>
                  <option value="off">{{ t('jobs.notify_off') }}</option>
                  <option value="always">{{ t('jobs.notify_always') }}</option>
                  <option value="failure">{{ t('jobs.notify_failure') }}</option>
                </select>
              </label>
            </div>
          </section>

          <p v-if="errorMsg" class="alert error">✗ {{ errorMsg }}</p>
        </div>

        <footer class="sheet-footer">
          <button class="btn primary" @click="saveAndRun">{{ t('jobs.save_and_run') }}</button>
          <button class="btn ghost" @click="save">{{ t('jobs.save') }}</button>
          <button class="btn ghost" @click="editing = false">{{ t('servers.cancel') }}</button>
        </footer>
      </div>
    </div>
  </div>
</template>

<style scoped>
.field-label {
  display: block;
  font-size: 0.85rem;
  color: var(--muted);
  margin-bottom: 0.35rem;
}
.schedule-presets { margin-bottom: 0.85rem; }
.preset-chips,
.weekday-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
}
.chip {
  padding: 0.3rem 0.65rem;
  border-radius: 6px;
  border: 1px solid var(--border);
  background: transparent;
  color: inherit;
  cursor: pointer;
  font-size: 0.85rem;
}
.chip.active {
  background: #1e3a5f;
  border-color: #3b82f6;
  color: #93c5fd;
}
.weekday-row { margin: 0.75rem 0; }
.schedule-times { margin: 0.75rem 0; }
.schedule-summary {
  margin: 0.75rem 0;
  padding: 0.65rem 0.85rem;
  border-radius: 10px;
  background: rgba(15, 23, 42, 0.55);
  border: 1px solid rgba(51, 65, 85, 0.7);
  font-size: 0.9rem;
}
.full-backup-block {
  display: grid;
  gap: 0.75rem;
}
.full-backup-when {
  display: flex;
  gap: 0.5rem;
  align-items: center;
}
.full-backup-when select,
.full-backup-when input[type="time"] {
  flex: 1;
  min-width: 0;
}
.schedule-toggle { margin-bottom: 0.75rem; }
.time-row {
  display: flex;
  align-items: center;
  gap: 0.35rem;
  margin-bottom: 0.35rem;
}
.time-row input[type="time"] {
  flex: 1;
  max-width: 160px;
}
.volume-excludes { margin: 0.75rem 0; }
.folder-exclude-list {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  max-height: 220px;
  overflow: auto;
  margin: 0.5rem 0;
  padding: 0.5rem;
  border: 1px solid var(--border);
  border-radius: 8px;
}
.folder-exclude-row {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.9rem;
  cursor: pointer;
}
.folder-exclude-row.system {
  opacity: 0.65;
  cursor: default;
}
.folder-exclude-name { flex: 1; }
.folder-exclude-badge {
  font-size: 0.75rem;
  color: var(--muted);
}
.extra-excludes {
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
  margin-top: 0.5rem;
}
.chip.removable {
  cursor: pointer;
}
</style>
