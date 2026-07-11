<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  GetSettings,
  SaveSettings,
  GetDefaultExclusions,
  OpenDataFolder,
  GetServiceStatus,
  InstallService,
  UninstallService,
  StartService,
  StopService,
  RestartService,
  ExportConfigDialog,
  ImportConfigDialog,
  TestSMTP,
  HasSMTPPassword,
  GetContactInfo,
  OpenTelegramContact,
} from '../../wailsjs/go/main/App'
import { BrowserOpenURL } from '../../wailsjs/runtime/runtime'
import { models } from '../../wailsjs/go/models'
import { setAppLocale } from '../i18n'
import { serviceStateText as formatServiceState } from '../utils/format'

const { t } = useI18n()

const form = reactive({
  language: 'ru',
  bandwidth_mbps: 0,
  chunk_workers: 0,
  network_timeout_sec: 120,
  network_retries: 3,
  exclusions: '',
  webhook_url: '',
  restore_overwrite: 'ask',
  notify_backup: 'off',
  notify_restore: 'off',
  smtp_host: '',
  smtp_port: 587,
  smtp_username: '',
  smtp_password: '',
  smtp_from: '',
  smtp_to: '',
  smtp_insecure_tls: false,
})

const serviceStatus = ref<models.ServiceStatus | null>(null)
const svcBusy = ref(false)
const svcMessage = ref('')
const svcMessageOk = ref(true)
const saveMessage = ref('')
const smtpTestMessage = ref('')
const smtpTestOk = ref(true)
const smtpPasswordSaved = ref(false)
const smtpTesting = ref(false)
const telegramHandle = ref('')
const authorName = ref('')
const githubURL = ref('')

const githubIssuesURL = computed(() => {
  const base = githubURL.value.replace(/\/$/, '')
  return base ? `${base}/issues` : ''
})

async function openTelegram() {
  await OpenTelegramContact()
}

function openGitHubIssues() {
  if (githubIssuesURL.value) BrowserOpenURL(githubIssuesURL.value)
}

let pollTimer: ReturnType<typeof setInterval> | null = null

const serviceStateLabel = computed(() => {
  const s = serviceStatus.value
  if (!s) return t('common.dash')
  return formatServiceState(s.state, s.message, s.installed, t)
})

const serviceRunning = computed(() => serviceStatus.value?.running === true)
const serviceInstalled = computed(() => {
  const s = serviceStatus.value
  if (!s) return false
  return s.installed || s.pending_delete
})
const needsAdminHint = computed(() => serviceStatus.value?.needs_admin === true)

async function refreshServiceStatus() {
  try {
    serviceStatus.value = await GetServiceStatus()
  } catch {
    serviceStatus.value = null
  }
}

function showSvcResult(res: models.ServiceActionResult) {
  svcMessage.value = res.message
  svcMessageOk.value = res.ok
}

async function runSvcAction(fn: () => Promise<models.ServiceActionResult>) {
  svcBusy.value = true
  svcMessage.value = ''
  try {
    const res = await fn()
    showSvcResult(res)
    await refreshServiceStatus()
  } catch (e: any) {
    svcMessage.value = e?.message || String(e)
    svcMessageOk.value = false
  } finally {
    svcBusy.value = false
  }
}

function buildSettings() {
  return models.AppSettings.createFrom({
    language: form.language,
    bandwidth_mbps: form.bandwidth_mbps,
    chunk_workers: form.chunk_workers,
    network_timeout_sec: form.network_timeout_sec,
    network_retries: form.network_retries,
    webhook_url: form.webhook_url,
    restore_overwrite: form.restore_overwrite,
    notify_backup: form.notify_backup,
    notify_restore: form.notify_restore,
    default_exclusions: form.exclusions.split('\n').map((x) => x.trim()).filter(Boolean),
    smtp: {
      host: form.smtp_host.trim(),
      port: form.smtp_port || 587,
      username: form.smtp_username.trim(),
      from: form.smtp_from.trim(),
      to: form.smtp_to.trim(),
      insecure_tls: form.smtp_insecure_tls,
    },
  })
}

onMounted(async () => {
  const s = await GetSettings()
  form.language = s.language
  setAppLocale(s.language)
  form.bandwidth_mbps = s.bandwidth_mbps
  form.chunk_workers = s.chunk_workers || 0
  form.network_timeout_sec = s.network_timeout_sec || 120
  form.network_retries = s.network_retries || 3
  form.webhook_url = s.webhook_url || ''
  form.restore_overwrite = s.restore_overwrite || 'ask'
  form.notify_backup = s.notify_backup || 'off'
  form.notify_restore = s.notify_restore || 'off'
  form.smtp_host = s.smtp?.host || ''
  form.smtp_port = s.smtp?.port || 587
  form.smtp_username = s.smtp?.username || ''
  form.smtp_from = s.smtp?.from || ''
  form.smtp_to = s.smtp?.to || ''
  form.smtp_insecure_tls = !!s.smtp?.insecure_tls
  smtpPasswordSaved.value = await HasSMTPPassword()
  const exc = s.default_exclusions?.length
    ? s.default_exclusions
    : await GetDefaultExclusions()
  form.exclusions = exc.join('\n')
  await refreshServiceStatus()
  pollTimer = setInterval(refreshServiceStatus, 5000)
  try {
    const contact = await GetContactInfo()
    telegramHandle.value = contact.telegram_handle
    authorName.value = contact.author_name
    githubURL.value = contact.github_url
  } catch { /* ignore */ }
})

onUnmounted(() => {
  if (pollTimer) clearInterval(pollTimer)
})

async function save() {
  await SaveSettings(buildSettings(), form.smtp_password)
  setAppLocale(form.language)
  if (form.smtp_password) {
    smtpPasswordSaved.value = true
    form.smtp_password = ''
  }
  saveMessage.value = t('settings.saved')
  setTimeout(() => { saveMessage.value = '' }, 3000)
}

async function testSmtp() {
  smtpTesting.value = true
  smtpTestMessage.value = ''
  try {
    const res = await TestSMTP(buildSettings(), form.smtp_password)
    smtpTestOk.value = res.ok
    smtpTestMessage.value = res.message
    if (form.smtp_password && res.ok) {
      await SaveSettings(buildSettings(), form.smtp_password)
      smtpPasswordSaved.value = true
      form.smtp_password = ''
    }
  } catch (e: any) {
    smtpTestOk.value = false
    smtpTestMessage.value = e?.message || String(e)
  } finally {
    smtpTesting.value = false
  }
}

async function exportCfg() {
  const p = await ExportConfigDialog()
  if (p) {
    saveMessage.value = t('settings.exported', { path: p })
    setTimeout(() => { saveMessage.value = '' }, 5000)
  }
}

async function confirmUninstall() {
  if (!window.confirm(t('settings.service_uninstall_confirm'))) return
  svcMessage.value = t('settings.service_uninstalling')
  svcMessageOk.value = true
  await runSvcAction(UninstallService)
}

async function importCfg() {
  const p = await ImportConfigDialog()
  if (p) {
    saveMessage.value = t('settings.imported', { path: p })
    setTimeout(() => { saveMessage.value = '' }, 5000)
  }
}
</script>

<template>
  <div class="page">
    <h2>{{ t('settings.title') }}</h2>

    <section class="panel service-panel">
      <h3>{{ t('settings.service_title') }}</h3>
      <p class="service-status-line">
        <span class="label">{{ t('settings.service_state') }}:</span>
        <span :class="serviceRunning ? 'status-ok' : (serviceInstalled ? 'status-warn' : 'status-muted')">
          {{ serviceStateLabel }}
        </span>
      </p>
      <p v-if="needsAdminHint" class="hint admin-hint">{{ t('settings.service_admin_hint') }}</p>
      <div class="actions service-actions">
        <button class="btn" :disabled="svcBusy" @click="runSvcAction(InstallService)">
          {{ serviceInstalled ? t('settings.service_reinstall') : t('settings.service_install') }}
        </button>
        <button class="btn" :disabled="svcBusy || !serviceInstalled || serviceRunning" @click="runSvcAction(StartService)">
          {{ t('settings.service_start') }}
        </button>
        <button class="btn" :disabled="svcBusy || !serviceInstalled || !serviceRunning" @click="runSvcAction(StopService)">
          {{ t('settings.service_stop') }}
        </button>
        <button class="btn" :disabled="svcBusy || !serviceInstalled" @click="runSvcAction(RestartService)">
          {{ t('settings.service_restart') }}
        </button>
        <button class="btn danger" :disabled="svcBusy" @click="confirmUninstall">
          {{ t('settings.service_uninstall') }}
        </button>
      </div>
      <p v-if="svcMessage" :class="['inline-msg', svcMessageOk ? 'ok' : 'error']">{{ svcMessage }}</p>
    </section>

    <section class="panel">
      <h3>{{ t('settings.smtp_title') }}</h3>
      <p class="hint">{{ t('settings.smtp_hint') }}</p>
      <div class="field-grid">
        <label class="field">
          <span>{{ t('settings.smtp_host') }}</span>
          <input v-model="form.smtp_host" placeholder="smtp.example.com" />
        </label>
        <label class="field">
          <span>{{ t('settings.smtp_port') }}</span>
          <input v-model.number="form.smtp_port" type="number" min="1" max="65535" />
        </label>
        <label class="field">
          <span>{{ t('settings.smtp_username') }}</span>
          <input v-model="form.smtp_username" autocomplete="off" />
        </label>
        <label class="field">
          <span>{{ t('settings.smtp_password') }}</span>
          <input v-model="form.smtp_password" type="password" :placeholder="smtpPasswordSaved ? t('settings.smtp_password_saved') : ''" />
        </label>
        <label class="field">
          <span>{{ t('settings.smtp_from') }}</span>
          <input v-model="form.smtp_from" placeholder="backup@example.com" />
        </label>
        <label class="field">
          <span>{{ t('settings.smtp_to') }}</span>
          <input v-model="form.smtp_to" placeholder="admin@example.com" />
        </label>
      </div>
      <label class="toggle">
        <input type="checkbox" v-model="form.smtp_insecure_tls" />
        <span>{{ t('settings.smtp_insecure') }}</span>
      </label>
      <div class="actions smtp-actions">
        <button class="btn" :disabled="smtpTesting" @click="testSmtp">{{ t('settings.smtp_test') }}</button>
      </div>
      <p v-if="smtpTestMessage" :class="['inline-msg', smtpTestOk ? 'ok' : 'error']">{{ smtpTestMessage }}</p>
    </section>

    <section class="panel">
      <h3>{{ t('settings.notify_title') }}</h3>
      <p class="hint">{{ t('settings.notify_default_hint') }}</p>
      <label>{{ t('settings.notify_backup') }}
        <select v-model="form.notify_backup">
          <option value="off">{{ t('settings.notify_off') }}</option>
          <option value="always">{{ t('settings.notify_always') }}</option>
          <option value="failure">{{ t('settings.notify_failure') }}</option>
        </select>
      </label>
      <label>{{ t('settings.notify_restore') }}
        <select v-model="form.notify_restore">
          <option value="off">{{ t('settings.notify_off') }}</option>
          <option value="always">{{ t('settings.notify_always') }}</option>
          <option value="failure">{{ t('settings.notify_failure') }}</option>
        </select>
      </label>
    </section>

    <label>{{ t('settings.language') }}
      <select v-model="form.language">
        <option value="ru">{{ t('settings_ext.language_ru') }}</option>
        <option value="en">{{ t('settings_ext.language_en') }}</option>
      </select>
    </label>
    <label>{{ t('settings.bandwidth') }}
      <input v-model.number="form.bandwidth_mbps" type="number" min="0" />
    </label>
    <label>{{ t('settings.chunk_workers') }}
      <input v-model.number="form.chunk_workers" type="number" min="0" max="32" />
    </label>
    <label>{{ t('settings.timeout') }}
      <input v-model.number="form.network_timeout_sec" type="number" min="30" />
    </label>
    <label>{{ t('settings.retries') }}
      <input v-model.number="form.network_retries" type="number" min="0" max="10" />
    </label>
    <label>{{ t('settings.restore_overwrite') }}
      <select v-model="form.restore_overwrite">
        <option value="ask">{{ t('settings_ext.restore_mode_ask') }}</option>
        <option value="overwrite">{{ t('settings_ext.restore_mode_overwrite') }}</option>
        <option value="backup">{{ t('settings_ext.restore_mode_backup') }}</option>
      </select>
    </label>
    <label>{{ t('settings_ext.webhook_label') }}
      <input v-model="form.webhook_url" placeholder="https://..." />
    </label>
    <label>{{ t('settings.exclusions') }}
      <textarea v-model="form.exclusions" rows="10" />
    </label>
    <div class="actions">
      <button class="btn primary" @click="save">{{ t('settings.save') }}</button>
      <button class="btn" @click="OpenDataFolder">{{ t('settings.open_data') }}</button>
      <button class="btn" @click="exportCfg">{{ t('settings.export') }}</button>
      <button class="btn" @click="importCfg">{{ t('settings.import') }}</button>
    </div>
    <p v-if="saveMessage" class="inline-msg ok">{{ saveMessage }}</p>

    <div class="panel contact-panel">
      <h3>{{ t('settings.contact_title') }}</h3>
      <p v-if="authorName" class="contact-author">{{ t('settings.contact_author', { name: authorName }) }}</p>
      <p class="contact-hint">{{ t('settings.contact_hint') }}</p>
      <p class="contact-copyright">{{ t('settings.contact_copyright') }}</p>
      <div class="contact-actions">
        <button v-if="githubIssuesURL" type="button" class="btn" @click="openGitHubIssues">
          {{ t('settings.contact_issues') }}
        </button>
        <button v-if="telegramHandle" type="button" class="btn" @click="openTelegram">
          {{ t('settings.contact_open', { handle: telegramHandle }) }}
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.panel {
  margin-bottom: 1.5rem;
  padding: 1rem;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: rgba(0, 0, 0, 0.15);
}
.panel h3 {
  margin: 0 0 0.75rem;
  font-size: 1rem;
}
.field-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.75rem;
  margin-bottom: 0.75rem;
}
.field {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
}
.field span {
  font-size: 0.85rem;
  color: var(--muted);
}
.smtp-actions {
  margin-top: 0.5rem;
}
.service-panel {
  margin-bottom: 1.5rem;
}
.service-status-line .label {
  color: var(--muted);
  margin-right: 0.5rem;
}
.status-ok { color: #86efac; }
.status-warn { color: #fcd34d; }
.status-muted { color: var(--muted); }
.admin-hint { color: #fcd34d; margin: 0.5rem 0; }
.service-actions { flex-wrap: wrap; gap: 0.5rem; }
.inline-msg { margin-top: 0.75rem; font-size: 0.9rem; }
.inline-msg.ok { color: #86efac; }
.inline-msg.error { color: #fca5a5; }
.contact-hint { color: var(--muted); margin: 0 0 0.75rem; font-size: 0.9rem; }
.contact-author { margin: 0 0 0.5rem; font-weight: 600; }
.contact-copyright {
  color: var(--muted);
  margin: 0 0 0.75rem;
  font-size: 0.85rem;
  line-height: 1.45;
  max-width: 52rem;
}
.contact-panel { margin-top: 1.5rem; }
.contact-actions { display: flex; flex-wrap: wrap; gap: 0.5rem; }
@media (max-width: 720px) {
  .field-grid { grid-template-columns: 1fr; }
}
</style>
