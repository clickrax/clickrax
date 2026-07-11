<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  ListDestinations,
  SaveDestination,
  DeleteDestination,
  TestDestinationConnection,
  TestDestinationByID,
  FetchFingerprint,
  GetHostname,
} from '../../wailsjs/go/main/App'
import { models } from '../../wailsjs/go/models'
import { useBackdropDismiss } from '../composables/useBackdropDismiss'

const { t } = useI18n()

const destinations = ref<models.BackupDestination[]>([])
const editing = ref(false)
const { onBackdropPointerDown, onBackdropClick } = useBackdropDismiss(() => { editing.value = false })
const testResult = ref('')
const hostname = ref('')
const refreshing = ref(false)

type StatusInfo = { online: boolean; message: string; checking: boolean }
const statuses = ref<Record<string, StatusInfo>>({})

const emptyForm = () => ({
  id: '',
  type: 'pbs' as 'pbs' | 'smb' | 'ftp',
  name: '',
  description: '',
  url: '',
  fingerprint: '',
  datastore: '',
  namespace: '',
  token_id: '',
  host: '',
  port: 0,
  share: '',
  remote_path: '',
  domain: '',
  username: '',
  tls: false,
  passive: true,
  secret: '',
})

const form = reactive(emptyForm())

const isPBS = computed(() => form.type === 'pbs')
const isSMB = computed(() => form.type === 'smb')
const isFTP = computed(() => form.type === 'ftp')

function typeLabel(t: string) {
  if (t === 'smb') return 'SMB'
  if (t === 'ftp') return 'FTP'
  return 'PBS'
}

function destSummary(d: models.BackupDestination) {
  if (d.type === 'smb') return `\\\\${d.host}\\${d.share || ''}`
  if (d.type === 'ftp') return `${d.tls ? 'ftps' : 'ftp'}://${d.host}${d.port ? ':' + d.port : ''}`
  return d.url || t('common.dash')
}

async function checkStatus(id: string) {
  statuses.value[id] = { online: false, message: t('common.checking'), checking: true }
  try {
    const r = await TestDestinationByID(id)
    statuses.value[id] = { online: r.ok, message: r.message, checking: false }
  } catch (e: any) {
    statuses.value[id] = { online: false, message: e?.message || String(e), checking: false }
  }
}

async function load() {
  destinations.value = await ListDestinations()
  hostname.value = await GetHostname()
  await Promise.all(destinations.value.map((d) => checkStatus(d.id)))
}

async function refreshStatuses() {
  if (!destinations.value.length) return
  refreshing.value = true
  await Promise.all(destinations.value.map((d) => checkStatus(d.id)))
  refreshing.value = false
}

function resetForm() {
  Object.assign(form, emptyForm())
  testResult.value = ''
}

function edit(d: models.BackupDestination) {
  form.id = d.id
  form.type = (d.type || 'pbs') as 'pbs' | 'smb' | 'ftp'
  form.name = d.name
  form.description = d.description || ''
  form.url = d.url || ''
  form.fingerprint = d.fingerprint || ''
  form.datastore = d.datastore || ''
  form.namespace = d.namespace || ''
  form.token_id = d.token_id || ''
  form.host = d.host || ''
  form.port = d.port || 0
  form.share = d.share || ''
  form.remote_path = d.remote_path || ''
  form.domain = d.domain || ''
  form.username = d.username || ''
  form.tls = !!d.tls
  form.passive = d.passive !== false
  form.secret = ''
  editing.value = true
}

function buildDestination() {
  return models.BackupDestination.createFrom({
    id: form.id,
    type: form.type,
    name: form.name,
    description: form.description,
    url: form.url,
    fingerprint: form.fingerprint,
    datastore: form.datastore,
    namespace: form.namespace,
    token_id: form.token_id,
    host: form.host,
    port: form.port || 0,
    share: form.share,
    remote_path: form.remote_path,
    domain: form.domain,
    username: form.username,
    tls: form.tls,
    passive: form.passive,
  })
}

async function save() {
  await SaveDestination(buildDestination(), form.secret)
  editing.value = false
  resetForm()
  await load()
}

async function remove(id: string) {
  if (!confirm(t('servers_ext.delete_confirm'))) return
  await DeleteDestination(id)
  await load()
}

async function test() {
  const r = await TestDestinationConnection(buildDestination(), form.secret)
  testResult.value = r.ok ? `✓ ${r.message}` : `✗ ${r.message}`
}

async function fetchFP() {
  if (!form.url) {
    testResult.value = t('servers_ext.test_need_url')
    return
  }
  try {
    form.fingerprint = await FetchFingerprint(form.url)
    testResult.value = t('servers_ext.fingerprint_ok')
  } catch (e: any) {
    testResult.value = '✗ ' + (e?.message || String(e))
  }
}

function statusLabel(id: string) {
  const s = statuses.value[id]
  if (!s || s.checking) return t('servers.checking')
  return s.online ? t('servers.online') : t('servers.offline')
}

onMounted(load)
</script>

<template>
  <div class="page servers-page">
    <header class="page-header">
      <div>
        <h2>{{ t('servers.title') }}</h2>
        <p class="page-sub">{{ t('servers.subtitle') }}</p>
      </div>
      <div class="btn-group">
        <button class="btn ghost" :disabled="refreshing || !destinations.length" @click="refreshStatuses">
          {{ refreshing ? t('common.checking') : t('servers_ext.refresh_status') }}
        </button>
        <button class="btn primary" @click="editing = true; resetForm()">{{ t('servers.add') }}</button>
      </div>
    </header>

    <div v-if="destinations.length" class="table-card">
      <table class="table">
        <thead>
          <tr>
            <th>{{ t('servers.status') }}</th>
            <th>{{ t('servers.type') }}</th>
            <th>{{ t('servers.name') }}</th>
            <th>{{ t('servers.target') }}</th>
            <th>{{ t('servers.remote_path') }}</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="d in destinations" :key="d.id">
            <td class="server-status-cell">
              <span
                class="status-dot"
                :class="statuses[d.id]?.checking ? 'pending' : (statuses[d.id]?.online ? 'ok' : 'err')"
              />
              {{ statusLabel(d.id) }}
              <span v-if="statuses[d.id]?.message && !statuses[d.id]?.checking" class="status-hint" :title="statuses[d.id].message">
                {{ statuses[d.id].message }}
              </span>
            </td>
            <td><span class="tag">{{ typeLabel(d.type || 'pbs') }}</span></td>
            <td><strong>{{ d.name }}</strong></td>
            <td>{{ destSummary(d) }}</td>
            <td>{{ d.remote_path || (d.type === 'pbs' ? d.datastore : '—') }}</td>
            <td class="actions-cell">
              <div class="btn-group">
                <button class="btn sm ghost" @click="edit(d)">{{ t('servers.edit') }}</button>
                <button class="btn sm danger" @click="remove(d.id)">{{ t('servers.delete') }}</button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <div v-else class="empty-card">
      <p>{{ t('servers.empty') }}</p>
      <button class="btn primary" @click="editing = true; resetForm()">{{ t('servers.add') }}</button>
    </div>

    <div
      v-if="editing"
      class="sheet-backdrop"
      @pointerdown.self="onBackdropPointerDown"
      @click.self="onBackdropClick"
    >
      <div class="sheet">
        <header class="sheet-header">
          <h3>{{ form.id ? t('servers.edit') : t('servers.add') }}</h3>
          <button class="icon-btn" @click="editing = false" :aria-label="t('common.close')">×</button>
        </header>

        <div class="sheet-body">
          <section class="form-section">
            <h4>{{ t('servers_ext.section_common') }}</h4>
            <div class="field-grid">
              <label class="field">
                <span>{{ t('servers.type') }}</span>
                <select v-model="form.type">
                  <option value="pbs">Proxmox PBS</option>
                  <option value="smb">SMB / CIFS</option>
                  <option value="ftp">FTP / FTPS</option>
                </select>
              </label>
              <label class="field">
                <span>{{ t('servers.name') }}</span>
                <input v-model="form.name" :placeholder="t('servers.ph_name')" />
              </label>
              <label class="field full">
                <span>{{ t('servers.description') }}</span>
                <input v-model="form.description" />
              </label>
            </div>
          </section>

          <section v-if="isPBS" class="form-section">
            <h4>PBS</h4>
            <div class="field-grid">
              <label class="field">
                <span>{{ t('servers.url') }}</span>
                <input v-model="form.url" placeholder="https://pbs.example.com:8007" />
              </label>
              <label class="field full">
                <span>{{ t('servers.fingerprint') }}</span>
                <div class="btn-row">
                  <input v-model="form.fingerprint" placeholder="SHA-256" class="field-input" style="flex:1" />
                  <button class="btn ghost" type="button" @click="fetchFP">{{ t('servers.fetch_fp') }}</button>
                </div>
              </label>
              <label class="field">
                <span>{{ t('servers.datastore') }}</span>
                <input v-model="form.datastore" placeholder="backup" />
              </label>
              <label class="field">
                <span>{{ t('servers.namespace') }}</span>
                <input v-model="form.namespace" :placeholder="hostname || t('servers.ph_namespace')" />
              </label>
              <label class="field">
                <span>{{ t('servers.token') }}</span>
                <input v-model="form.token_id" placeholder="user@pbs!token-name" />
              </label>
              <label class="field">
                <span>{{ t('servers.secret') }}</span>
                <input v-model="form.secret" type="password" placeholder="••••••" />
              </label>
            </div>
            <p class="hint">{{ t('servers.acl_hint') }}</p>
          </section>

          <section v-if="isSMB || isFTP" class="form-section">
            <h4>{{ isSMB ? 'SMB' : 'FTP' }}</h4>
            <div class="field-grid">
              <label class="field">
                <span>{{ t('servers.host') }}</span>
                <input v-model="form.host" placeholder="192.168.1.10" />
              </label>
              <label class="field">
                <span>{{ t('servers.port') }}</span>
                <input v-model.number="form.port" type="number" :placeholder="isSMB ? '445' : (form.tls ? '990' : '21')" />
              </label>
              <label v-if="isSMB" class="field">
                <span>{{ t('servers.share') }}</span>
                <input v-model="form.share" placeholder="backup" />
              </label>
              <label class="field">
                <span>{{ t('servers.remote_path') }}</span>
                <input v-model="form.remote_path" :placeholder="t('servers.remote_path_ph')" />
              </label>
              <label v-if="isSMB" class="field">
                <span>{{ t('servers.domain') }}</span>
                <input v-model="form.domain" placeholder="CORP" />
              </label>
              <label class="field">
                <span>{{ t('servers.username') }}</span>
                <input v-model="form.username" />
              </label>
              <label class="field">
                <span>{{ t('servers.password') }}</span>
                <input v-model="form.secret" type="password" placeholder="••••••" />
              </label>
            </div>
            <div v-if="isFTP" class="option-toggles">
              <label class="toggle">
                <input v-model="form.tls" type="checkbox" />
                <span>{{ t('servers.ftp_tls') }}</span>
              </label>
              <label class="toggle">
                <input v-model="form.passive" type="checkbox" />
                <span>{{ t('servers.ftp_passive') }}</span>
              </label>
            </div>
            <p class="hint">{{ t('servers.file_backup_hint') }}</p>
          </section>

          <p v-if="testResult" :class="['alert', testResult.startsWith('✓') ? 'success' : 'error']">{{ testResult }}</p>
        </div>

        <footer class="sheet-footer">
          <button class="btn ghost" type="button" @click="test">{{ t('servers.test') }}</button>
          <button class="btn primary" type="button" @click="save">{{ t('servers.save') }}</button>
          <button class="btn ghost" type="button" @click="editing = false">{{ t('servers.cancel') }}</button>
        </footer>
      </div>
    </div>
  </div>
</template>
