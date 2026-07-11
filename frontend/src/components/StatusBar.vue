<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import { GetLastProgress, GetVersion, GetContactInfo, OpenTelegramContact, IsBackupRunning, IsStopping, StopBackup } from '../../wailsjs/go/main/App'
import { formatBytes, formatJobName, formatSpeed } from '../utils/format'

const { t, locale } = useI18n()

const copyrightYear = new Date().getFullYear()

const appVersion = ref('')
const telegramHandle = ref('')

async function openTelegram() {
  await OpenTelegramContact()
}

const phase = ref('idle')
const percent = ref(0)
const speed = ref(0)
const transferred = ref(0)
const reused = ref(0)
const message = ref('')
const jobName = ref('')
const jobID = ref('')
const running = ref(false)
const stopping = ref(false)

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

const phaseText = computed(() => {
  if (stopping.value) return t('statusbar.stopping')
  const key = phaseLabel[phase.value]
  if (key) return t(key)
  return message.value || t('statusbar.backup')
})

function applyProgress(p: any) {
  phase.value = p.phase || 'idle'
  percent.value = p.percent || 0
  speed.value = p.speed_bps || 0
  transferred.value = p.bytes_transferred || 0
  reused.value = p.bytes_reused || 0
  message.value = p.message || ''
  if (p.job_name) jobName.value = p.job_name
  if (p.job_id) jobID.value = p.job_id
  if (['done', 'error', 'cancelled', 'idle', ''].includes(phase.value)) {
    running.value = false
    stopping.value = false
    return
  }
  running.value = true
}

async function refreshProgress() {
  try {
    const p = await GetLastProgress()
    if (p) applyProgress(p)
  } catch { /* ignore */ }
}

watch(locale, () => { refreshProgress() })

onMounted(async () => {
  try {
    appVersion.value = await GetVersion()
  } catch { /* ignore */ }
  try {
    const contact = await GetContactInfo()
    telegramHandle.value = contact.telegram_handle
  } catch { /* ignore */ }
  running.value = await IsBackupRunning()
  stopping.value = await IsStopping()
  await refreshProgress()

  EventsOn('progress', (p: any) => {
    applyProgress(p)
    if (p.phase === 'cancelled') {
      stopping.value = false
      running.value = false
    }
  })
  EventsOn('backup-finished', (rec: any) => {
    running.value = false
    stopping.value = false
    if (rec?.status === 'cancelled') {
      phase.value = 'cancelled'
    } else if (rec?.status === 'error') {
      phase.value = 'error'
    } else {
      phase.value = 'done'
    }
  })
})

async function stopBackup() {
  if (!confirm(t('statusbar.stop_confirm'))) return
  stopping.value = true
  await StopBackup(jobID.value)
}
</script>

<template>
  <footer class="statusbar">
    <div v-if="running || stopping" class="statusbar-inner">
      <span class="statusbar-phase">{{ phaseText }}</span>
      <div class="statusbar-bar" role="progressbar" :aria-valuenow="percent" aria-valuemin="0" aria-valuemax="100">
        <span class="statusbar-bar-fill" :style="{ width: percent + '%' }" />
      </div>
      <span class="statusbar-pct">{{ percent.toFixed(0) }}%</span>
      <span class="statusbar-stats">
        {{ formatBytes(transferred) }}
        <template v-if="speed > 0"> · {{ formatSpeed(speed) }}</template>
      </span>
      <button class="btn-stop" :disabled="stopping" @click="stopBackup">{{ t('progress.stop') }}</button>
      <span v-if="message || jobName" class="statusbar-msg">
        <template v-if="jobName">{{ formatJobName(jobName) }}</template>
        <template v-if="message && jobName"> — </template>
        <template v-if="message">{{ message }}</template>
      </span>
    </div>
    <span v-else class="muted statusbar-idle">
      {{ t('app.copyright', { year: copyrightYear }) }} · v{{ appVersion || '…' }}
      <template v-if="telegramHandle">
        ·
        <button type="button" class="contact-link" @click="openTelegram">
          {{ t('app.contact', { handle: telegramHandle }) }}
        </button>
      </template>
    </span>
  </footer>
</template>

<style scoped>
.btn-stop {
  padding: 0.3rem 0.75rem;
  border: 1px solid var(--danger);
  background: transparent;
  color: #fca5a5;
  border-radius: 6px;
  cursor: pointer;
  font-size: 0.75rem;
  white-space: nowrap;
}
.btn-stop:disabled { opacity: 0.5; cursor: not-allowed; }
.btn-stop:hover:not(:disabled) { background: rgba(239, 68, 68, 0.15); }
.statusbar-idle { display: inline-flex; align-items: center; gap: 0.35rem; flex-wrap: wrap; }
.contact-link {
  padding: 0;
  border: none;
  background: none;
  color: var(--primary);
  cursor: pointer;
  font: inherit;
  text-decoration: underline;
  text-underline-offset: 2px;
}
.contact-link:hover { opacity: 0.85; }
</style>
