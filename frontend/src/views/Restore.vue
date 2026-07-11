<script setup lang="ts">
import { onMounted, onUnmounted, ref, computed, nextTick, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import {
  ListJobs,
  ListDestinations,
  ListSnapshots,
  ListSnapshotFiles,
  RestoreBatch,
  GetPBSWebURL,
  PickRestoreFolder,
  CancelRestore,
  GetLastSuccessfulBackup,
} from '../../wailsjs/go/main/App'
import { BrowserOpenURL, EventsOn, EventsOnce } from '../../wailsjs/runtime/runtime'
import { models } from '../../wailsjs/go/models'
import RestoreExplorer from '../components/RestoreExplorer.vue'
import RestoreConfirmModal from '../components/RestoreConfirmModal.vue'
import {
  breadcrumbs,
  sortItems,
  parentPath,
  type TreeNode,
  type SortKey,
  type SortDir,
} from '../utils/restoreTree'
import { formatDateTime } from '../utils/format'

const { t } = useI18n()
const route = useRoute()

const jobs = ref<models.BackupJob[]>([])
const destinations = ref<models.BackupDestination[]>([])
const jobId = ref('')
const snapshots = ref<any[]>([])
const snapshot = ref('')
const search = ref('')
const currentItems = ref<TreeNode[]>([])
const checked = ref<Set<string>>(new Set())
const currentPath = ref('')
const activePath = ref<string | null>(null)
const sortKey = ref<SortKey>('name')
const sortDir = ref<SortDir>('asc')
const message = ref('')
const errorMsg = ref('')
const loading = ref(false)
const restoring = ref(false)
const restoreProgress = ref('')
const showOriginalConfirm = ref(false)

const currentJob = computed(() => jobs.value.find((j) => j.id === jobId.value))
const currentDestination = computed(() => {
  const id = currentJob.value?.destination_id || currentJob.value?.server_id
  return destinations.value.find((d) => d.id === id)
})
const isPBSJob = computed(() => !currentDestination.value || currentDestination.value.type === 'pbs' || !currentDestination.value.type)
const jobSources = computed(() => currentJob.value?.sources?.filter(Boolean) ?? [])

const searchMode = computed(() => search.value.trim().length > 0)
const crumbs = computed(() => breadcrumbs(currentPath.value))
const canGoUp = computed(() => !searchMode.value && currentPath.value !== '')

const displayItems = computed(() => currentItems.value)

const selectedCount = computed(() => checked.value.size)
const selectedFileCount = computed(() => {
  let files = 0
  for (const p of checked.value) {
    const item = currentItems.value.find((n) => n.path === p)
    if (!item || !item.isDir) files++
  }
  if (files === 0 && checked.value.size > 0) return checked.value.size
  return files
})
const previewFilePaths = computed(() => Array.from(checked.value))

function snapshotFilesToNodes(files: models.SnapshotFile[]): TreeNode[] {
  return files.map((f) => {
    const parts = f.path.split('\\').filter(Boolean)
    return {
      path: f.path,
      name: parts[parts.length - 1] || f.path,
      isDir: !!f.is_dir,
      size: f.size || 0,
      modified: f.modified || '',
      owner: f.owner || '',
      attributes: f.attributes || '',
      children: [],
    }
  })
}

let searchTimer: ReturnType<typeof setTimeout> | null = null
watch(search, () => {
  if (searchTimer) clearTimeout(searchTimer)
  searchTimer = setTimeout(() => {
    loadFiles(searchMode.value ? '' : currentPath.value)
  }, 400)
})

function formatSnapshot(s: { time: string; backup: string; comment?: string; has_catalog?: boolean }) {
  const when = formatDateTime(s.time)
  const broken = s.has_catalog === false ? t('restore_ext.no_catalog_suffix') : ''
  return `${when} — ${s.backup}${s.comment ? ' (' + s.comment + ')' : ''}${broken}`
}

const restorableSnapshots = computed(() => snapshots.value.filter((s) => s.has_catalog !== false))

function friendlyError(raw: string): string {
  if (raw.includes('certificate signed by unknown authority') || raw.includes('x509')) {
    return t('restore_ext.tls_error')
  }
  if (raw.includes('Authentication error')) {
    return t('restore_ext.access_error')
  }
  if (raw.includes('backup.pxar.didx') && (raw.includes('No such file') || raw.includes('отсутствует') || raw.includes('missing'))) {
    return t('restore_ext.didx_missing')
  }
  if (raw.includes('снапшот создан без каталога') || raw.includes('каталог бэкапа пуст')
    || raw.toLowerCase().includes('snapshot created without catalog') || raw.toLowerCase().includes('catalog is empty')) {
    return t('restore_ext.no_file_catalog')
  }
  if (raw.includes('catalog.pcat1.didx') || raw.includes('индекс без chunks') || raw.toLowerCase().includes('index without chunks')) {
    return t('restore_ext.catalog_load_failed')
  }
  if (raw.includes('catalog-cache') && raw.toLowerCase().includes('access is denied')) {
    return t('restore_ext.cache_access')
  }
  if (raw.toLowerCase().includes('access is denied')) {
    return t('restore_ext.access_denied')
  }
  return raw
}

function setError(raw: string) {
  errorMsg.value = friendlyError(raw)
  message.value = ''
}

function setSuccess(msg: string) {
  message.value = msg
  errorMsg.value = ''
}

onMounted(async () => {
  jobs.value = await ListJobs()
  destinations.value = await ListDestinations()
  let target = route.query.job as string | undefined
  if (target && !jobs.value.some((j) => j.id === target)) target = undefined
  if (!target) {
    try {
      const last = await GetLastSuccessfulBackup()
      if (jobs.value.some((j) => j.id === last.job_id)) target = last.job_id
    } catch { /* no history */ }
  }
  if (target) jobId.value = target
  else if (jobs.value.length) jobId.value = jobs.value[0].id
  if (jobId.value) await loadSnapshots()
  EventsOn('restore-progress', (p: any) => {
    restoring.value = true
    const msg = p.current_path || t('restore.restoring')
    restoreProgress.value = p.files_total > 1 ? `${p.files_done}/${p.files_total}: ${msg}` : msg
  })
})

onUnmounted(() => {})

async function loadSnapshots() {
  errorMsg.value = ''
  message.value = t('restore.loading')
  snapshots.value = []
  snapshot.value = ''
  currentItems.value = []
  checked.value = new Set()
  try {
    snapshots.value = await ListSnapshots(jobId.value)
    const ok = snapshots.value.filter((s) => s.has_catalog !== false)
    if (ok.length) {
      snapshot.value = ok[0].time
      message.value = ''
      await loadFiles()
    } else if (snapshots.value.length) {
      setError(t('restore_ext.no_catalog_snapshots'))
    } else {
      message.value = t('restore.no_snapshots')
    }
  } catch (e: any) {
    setError(e?.message || String(e))
  }
}

async function onSnapshotChange() {
  checked.value = new Set()
  await loadFiles('')
}

async function loadFiles(dirPath = '') {
  loading.value = true
  errorMsg.value = ''
  if (!searchMode.value) {
    currentPath.value = dirPath
    activePath.value = null
  }
  try {
    const list = await ListSnapshotFiles(
      jobId.value,
      snapshot.value,
      searchMode.value ? '' : dirPath,
      searchMode.value ? search.value.trim() : '',
    )
    const nodes = snapshotFilesToNodes(list || [])
    currentItems.value = sortItems(nodes, sortKey.value, sortDir.value)
    if (!currentItems.value.length) {
      message.value = searchMode.value ? t('restore_ext.search_nothing') : t('restore_ext.catalog_empty')
    } else {
      message.value = searchMode.value && list.length >= 500
        ? t('restore_ext.search_truncated')
        : ''
    }
  } catch (e: any) {
    currentItems.value = []
    setError(e?.message || String(e))
  } finally {
    loading.value = false
  }
}

type RestoreBatchDone = { ok: boolean; message: string; count?: number }

function isRestoreCancelled(msg: string) {
  return msg === t('restore_ext.restore_cancelled') || msg === 'Восстановление отменено' || msg === 'Restore cancelled'
}

function waitRestoreBatch(req: models.RestoreBatchRequest): Promise<RestoreBatchDone> {
  return withRestoreTimeout(new Promise((resolve) => {
    EventsOnce('restore-batch-done', (r: RestoreBatchDone) => {
      resolve({ ok: !!r?.ok, message: r?.message || '', count: r?.count })
    })
    RestoreBatch(req)
  }))
}

function withRestoreTimeout<T>(promise: Promise<T>): Promise<T> {
  const ms = 20 * 60 * 1000
  return Promise.race([
    promise,
    new Promise<T>((_, reject) =>
      setTimeout(() => reject(new Error(t('restore_ext.restore_timeout'))), ms)
    ),
  ])
}

function toggleCheck(node: TreeNode, value: boolean) {
  const next = new Set(checked.value)
  if (value) next.add(node.path)
  else next.delete(node.path)
  checked.value = next
}

function navigate(path: string) {
  search.value = ''
  loadFiles(path)
}

function goUp() {
  loadFiles(parentPath(currentPath.value))
  activePath.value = null
}

function openFolder(node: TreeNode) {
  if (!node.isDir) return
  search.value = ''
  loadFiles(node.path)
  activePath.value = node.path
}

function onSort(key: SortKey) {
  if (sortKey.value === key) {
    sortDir.value = sortDir.value === 'asc' ? 'desc' : 'asc'
  } else {
    sortKey.value = key
    sortDir.value = key === 'name' ? 'asc' : 'desc'
  }
  currentItems.value = sortItems(currentItems.value, sortKey.value, sortDir.value)
}

function selectAllVisible() {
  const next = new Set(checked.value)
  for (const n of displayItems.value) next.add(n.path)
  checked.value = next
}
function clearSelection() { checked.value = new Set() }
function selectedPaths(): string[] { return Array.from(checked.value) }

async function beginRestore() {
  restoring.value = true
  restoreProgress.value = t('restore.loading_archive')
  errorMsg.value = ''
  message.value = ''
  await nextTick()
}

function endRestore() {
  restoring.value = false
  restoreProgress.value = ''
}

function requestRestoreOriginal() {
  if (!selectedCount.value) {
    setError(t('restore.nothing_selected'))
    return
  }
  showOriginalConfirm.value = true
}

async function confirmRestoreOriginal() {
  showOriginalConfirm.value = false
  const paths = selectedPaths()
  await beginRestore()
  try {
    const req = models.RestoreBatchRequest.createFrom({
      job_id: jobId.value,
      snapshot: snapshot.value,
      paths,
      to_original: true,
      overwrite: false,
    })
    const r = await waitRestoreBatch(req)
    if (r.ok) setSuccess(`✓ ${r.message}`)
    else if (isRestoreCancelled(r.message)) message.value = t('restore_ext.restore_cancelled')
    else setError(r.message || t('restore_ext.restore_error'))
  } catch (e: any) {
    setError(e?.message || String(e))
  } finally {
    endRestore()
  }
}

async function restoreToFolder() {
  const paths = selectedPaths()
  if (!paths.length) {
    setError(t('restore.nothing_selected'))
    return
  }
  let dest: string
  try {
    dest = await PickRestoreFolder()
  } catch (e: any) {
    setError(e?.message || t('restore_ext.folder_pick_error'))
    return
  }
  if (!dest) return

  await beginRestore()
  try {
    const req = models.RestoreBatchRequest.createFrom({
      job_id: jobId.value,
      snapshot: snapshot.value,
      paths,
      dest_path: dest,
      to_original: false,
      overwrite: false,
    })
    const r = await waitRestoreBatch(req)
    if (r.ok) setSuccess(`✓ ${r.message} → ${dest}`)
    else if (isRestoreCancelled(r.message)) message.value = t('restore_ext.restore_cancelled')
    else setError(r.message || t('restore_ext.restore_error'))
  } catch (e: any) {
    setError(e?.message || String(e))
  } finally {
    endRestore()
  }
}

async function cancelRestore() {
  if (!restoring.value) return
  restoreProgress.value = t('restore_ext.cancelling')
  try {
    await CancelRestore()
  } catch { /* ignore */ }
}

async function openPBS() {
  const url = await GetPBSWebURL(jobId.value)
  if (url) BrowserOpenURL(url)
}
</script>

<template>
  <div class="page restore-page">
    <header class="restore-header">
      <div class="restore-header-text">
        <h2>{{ t('restore.title') }}</h2>
        <p>{{ t('restore.subtitle') }}</p>
      </div>
      <button v-if="isPBSJob" type="button" class="btn ghost header-btn" @click="openPBS">
        <svg viewBox="0 0 20 20" fill="currentColor" width="16" height="16"><path d="M11 3a1 1 0 1 0 0 2h2.586l-6.293 6.293a1 1 0 1 0 1.414 1.414L15 6.414V9a1 1 0 1 0 2 0V4a1 1 0 0 0-1-1h-5z"/><path d="M5 5a2 2 0 0 0-2 2v8a2 2 0 0 0 2 2h8a2 2 0 0 0 2-2v-3a1 1 0 1 0-2 0v3H5V7h3a1 1 0 0 0 0-2H5z"/></svg>
        PBS Web
      </button>
    </header>

    <section class="restore-panel filters-panel">
      <div class="field-grid">
        <label class="field">
          <span>{{ t('restore.job') }}</span>
          <select v-model="jobId" @change="loadSnapshots">
            <option v-for="j in jobs" :key="j.id" :value="j.id">{{ j.name }}</option>
          </select>
        </label>
        <label class="field">
          <span>{{ t('restore.snapshot') }}</span>
          <select v-model="snapshot" @change="onSnapshotChange" :disabled="!restorableSnapshots.length">
            <option v-for="s in snapshots" :key="s.time" :value="s.time" :disabled="s.has_catalog === false">
              {{ formatSnapshot(s) }}
            </option>
          </select>
        </label>
        <label class="field full search-field">
          <span>{{ t('restore.search') }}</span>
          <div class="search-input-wrap">
            <svg class="search-icon" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M8 4a4 4 0 1 0 0 8 4 4 0 0 0 0-8zM2 8a6 6 0 1 1 10.89 3.476l4.817 4.817a1 1 0 0 1-1.414 1.414l-4.816-4.816A6 6 0 0 1 2 8z" clip-rule="evenodd"/></svg>
            <input v-model="search" :placeholder="t('restore.search_hint')" />
          </div>
        </label>
      </div>
    </section>

    <p v-if="errorMsg" class="alert error">✗ {{ errorMsg }}</p>
    <p v-else-if="message && !loading" :class="['alert', message.startsWith('✓') ? 'success' : 'info']">{{ message }}</p>

    <section class="restore-panel tree-panel" v-if="!loading && currentItems.length">
      <div class="tree-toolbar">
        <div class="toolbar-group">
          <button type="button" class="tool-btn" @click="selectAllVisible">{{ t('restore.select_visible') }}</button>
          <button type="button" class="tool-btn" :disabled="!checked.size" @click="clearSelection">{{ t('restore.clear_selection') }}</button>
        </div>
        <div v-if="checked.size" class="selection-badge">
          {{ t('restore.selected', { items: selectedCount, files: selectedFileCount }) }}
        </div>
      </div>
      <RestoreExplorer
        :items="displayItems"
        :crumbs="crumbs"
        :checked="checked"
        :active-path="activePath"
        :sort-key="sortKey"
        :sort-dir="sortDir"
        :search-mode="searchMode"
        :can-go-up="canGoUp"
        @navigate="navigate"
        @go-up="goUp"
        @sort="onSort"
        @toggle-check="toggleCheck"
        @activate="(p) => activePath = p"
        @open="openFolder"
      />
    </section>
    <div v-else-if="!loading" class="tree-empty restore-panel">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M3 7v10a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2V9a2 2 0 0 0-2-2h-6l-2-2H5a2 2 0 0 0-2 2z"/></svg>
      <p>{{ t('restore.empty') }}</p>
    </div>

    <div v-else class="loading-state">
      <div class="loading-spinner" />
      <p>{{ t('restore.loading_catalog') }}</p>
    </div>

    <footer class="restore-footer" :class="{ active: selectedCount > 0 || restoring }">
      <div v-if="restoring" class="footer-progress">
        <div class="progress-dot" />
        <span>{{ restoreProgress || t('restore.restoring') }}</span>
      </div>
      <div v-else-if="selectedCount" class="footer-hint">
        {{ t('restore.footer_hint', { files: selectedFileCount }) }}
      </div>
      <div class="footer-actions">
        <button
          v-if="restoring"
          type="button"
          class="btn ghost cancel-btn"
          @click="cancelRestore"
        >
          {{ t('restore.cancel_restore') }}
        </button>
        <button
          type="button"
          class="btn warn"
          :disabled="!selectedCount || restoring"
          @click="requestRestoreOriginal"
        >
          <svg viewBox="0 0 20 20" fill="currentColor" width="16" height="16"><path fill-rule="evenodd" d="M8.257 3.099c.765-1.36 2.722-1.36 3.486 0l5.58 9.92c.75 1.334-.213 2.98-1.742 2.98H4.42c-1.53 0-2.493-1.646-1.743-2.98l5.58-9.92zM11 13a1 1 0 1 1-2 0 1 1 0 0 1 2 0zm-1-8a1 1 0 0 0-1 1v3a1 1 0 0 0 2 0V6a1 1 0 0 0-1-1z" clip-rule="evenodd"/></svg>
          {{ t('restore.restore_original') }}
        </button>
        <button type="button" class="btn primary" :disabled="!selectedCount || restoring" @click="restoreToFolder">
          <svg viewBox="0 0 20 20" fill="currentColor" width="16" height="16"><path d="M4 3a2 2 0 0 0-2 2v10a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V5a2 2 0 0 0-2-2H4zm0 2h5v4H4V5zm7 0h5v4h-5V5zM4 11h5v4H4v-4zm7 0h5v4h-5v-4z"/></svg>
          {{ t('restore.restore_to') }}
        </button>
      </div>
    </footer>

    <RestoreConfirmModal
      :open="showOriginalConfirm"
      :job-name="currentJob?.name || '—'"
      :sources="jobSources"
      :file-paths="previewFilePaths"
      :item-count="selectedCount"
      @confirm="confirmRestoreOriginal"
      @cancel="showOriginalConfirm = false"
    />
  </div>
</template>

<style scoped>
.restore-page {
  padding-bottom: 5.5rem;
}
.restore-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
  margin-bottom: 1.25rem;
}
.restore-header h2 {
  margin: 0;
  font-size: 1.45rem;
  font-weight: 700;
  letter-spacing: -0.02em;
}
.restore-header p {
  margin: 0.35rem 0 0;
  color: var(--muted);
  font-size: 0.9rem;
}
.header-btn {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  flex-shrink: 0;
}
.restore-panel {
  border-radius: 14px;
  border: 1px solid rgba(51, 65, 85, 0.65);
  background: linear-gradient(180deg, rgba(30, 41, 59, 0.92) 0%, rgba(15, 23, 42, 0.55) 100%);
  box-shadow: 0 4px 24px rgba(0, 0, 0, 0.18);
}
.filters-panel {
  padding: 1rem 1.15rem;
  margin-bottom: 0.85rem;
}
.search-input-wrap {
  position: relative;
}
.search-icon {
  position: absolute;
  left: 0.7rem;
  top: 50%;
  transform: translateY(-50%);
  width: 1rem;
  height: 1rem;
  color: #64748b;
  pointer-events: none;
}
.search-field input {
  padding-left: 2.2rem !important;
}
.tree-panel {
  overflow: hidden;
}
.tree-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  padding: 0.65rem 0.85rem;
  border-bottom: 1px solid rgba(51, 65, 85, 0.55);
  background: rgba(15, 23, 42, 0.35);
}
.toolbar-group {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.35rem;
}
.toolbar-divider {
  width: 1px;
  height: 1.25rem;
  background: rgba(71, 85, 105, 0.6);
  margin: 0 0.25rem;
}
.tool-btn {
  padding: 0.35rem 0.7rem;
  border: 1px solid transparent;
  border-radius: 7px;
  background: transparent;
  color: #cbd5e1;
  font-size: 0.8rem;
  cursor: pointer;
  transition: background 0.15s, border-color 0.15s, color 0.15s;
}
.tool-btn:hover:not(:disabled) {
  background: rgba(51, 65, 85, 0.55);
  border-color: rgba(71, 85, 105, 0.5);
  color: #f1f5f9;
}
.tool-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}
.selection-badge {
  padding: 0.3rem 0.7rem;
  border-radius: 999px;
  font-size: 0.78rem;
  font-weight: 500;
  background: rgba(59, 130, 246, 0.15);
  color: #93c5fd;
  border: 1px solid rgba(59, 130, 246, 0.25);
  white-space: nowrap;
}
.tree-frame {
  display: flex;
  flex-direction: column;
}
.tree-header {
  display: grid;
  grid-template-columns: auto 1.1rem 1.6rem 1.35rem 1fr 5rem 8.5rem;
  gap: 0.4rem;
  padding: 0.45rem 0.65rem 0.45rem 0;
  font-size: 0.7rem;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  color: #64748b;
  font-weight: 600;
  border-bottom: 1px solid rgba(51, 65, 85, 0.45);
  background: rgba(15, 23, 42, 0.25);
}
.h-size,
.h-mtime {
  text-align: right;
}
.tree-scroll {
  max-height: min(52vh, 520px);
  overflow: auto;
  padding: 0.35rem 0 0.5rem;
}
.tree-scroll::-webkit-scrollbar {
  width: 8px;
}
.tree-scroll::-webkit-scrollbar-thumb {
  background: #334155;
  border-radius: 4px;
}
.tree-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 0.75rem;
  padding: 3rem 1.5rem;
  color: var(--muted);
}
.tree-empty svg {
  width: 2.5rem;
  height: 2.5rem;
  opacity: 0.35;
}
.loading-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 1rem;
  padding: 3rem;
  color: var(--muted);
}
.loading-spinner {
  width: 2rem;
  height: 2rem;
  border: 2px solid rgba(59, 130, 246, 0.2);
  border-top-color: var(--primary);
  border-radius: 50%;
  animation: spin 0.7s linear infinite;
}
@keyframes spin {
  to { transform: rotate(360deg); }
}
.restore-footer {
  position: fixed;
  bottom: 44px;
  left: 200px;
  right: 0;
  z-index: 50;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 0.85rem 1.25rem;
  background: rgba(15, 23, 42, 0.88);
  border-top: 1px solid rgba(51, 65, 85, 0.65);
  backdrop-filter: blur(10px);
  transform: translateY(100%);
  opacity: 0;
  transition: transform 0.25s ease, opacity 0.25s ease;
  pointer-events: none;
}
.restore-footer.active {
  transform: translateY(0);
  opacity: 1;
  pointer-events: auto;
}
.footer-progress {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  font-size: 0.85rem;
  color: #93c5fd;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.progress-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--primary);
  animation: pulse-dot 1.2s ease infinite;
  flex-shrink: 0;
}
@keyframes pulse-dot {
  0%, 100% { opacity: 1; transform: scale(1); }
  50% { opacity: 0.5; transform: scale(0.85); }
}
.footer-hint {
  font-size: 0.85rem;
  color: var(--muted);
}
.footer-actions {
  display: flex;
  gap: 0.6rem;
  margin-left: auto;
  flex-shrink: 0;
}
.footer-actions .btn.cancel-btn {
  border-color: rgba(248, 113, 113, 0.45);
  color: #fca5a5;
}
.footer-actions .btn.cancel-btn:hover {
  background: rgba(239, 68, 68, 0.15);
  border-color: rgba(248, 113, 113, 0.65);
}
.footer-actions .btn {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
}
.btn.warn {
  background: rgba(234, 179, 8, 0.12);
  border-color: rgba(234, 179, 8, 0.35);
  color: #fcd34d;
}
.btn.warn:hover:not(:disabled) {
  background: rgba(234, 179, 8, 0.22);
  border-color: rgba(234, 179, 8, 0.5);
}
</style>
