<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { GetDiagnostics, GetConfigPath, ListServers, TestServerConnection, RunHealthCheck, OpenTelegramContact } from '../../wailsjs/go/main/App'
import { BrowserOpenURL } from '../../wailsjs/runtime/runtime'
import { models } from '../../wailsjs/go/models'

const { t } = useI18n()

const info = ref<Record<string, string>>({})
const configPath = ref('')
const testResults = ref<string[]>([])
const healthReport = ref<models.HealthReport | null>(null)
const telegramHandle = ref('')
const githubURL = ref('')

onMounted(async () => {
  info.value = await GetDiagnostics()
  configPath.value = await GetConfigPath()
  telegramHandle.value = info.value.telegram || ''
  githubURL.value = info.value.github || ''
})

async function testAllServers() {
  testResults.value = []
  const servers = await ListServers()
  for (const s of servers) {
    const r = await TestServerConnection(s, '')
    testResults.value.push(`${s.name}: ${r.ok ? 'OK' : 'FAIL'} ${r.message}`)
  }
}

async function runHealth() {
  healthReport.value = await RunHealthCheck()
}

async function openTelegram() {
  await OpenTelegramContact()
}

function openGitHub() {
  if (githubURL.value) BrowserOpenURL(githubURL.value)
}
</script>

<template>
  <div class="page">
    <div class="page-head">
      <h2>{{ t('diagnostics.title') }}</h2>
      <button v-if="telegramHandle" type="button" class="btn" @click="openTelegram">
        {{ t('diagnostics.contact_open', { handle: telegramHandle }) }}
      </button>
    </div>
    <table class="table">
      <tbody>
        <tr v-for="(v, k) in info" :key="k">
          <td>{{ k }}</td>
          <td>
            <button
              v-if="k === 'telegram'"
              type="button"
              class="contact-link"
              @click="openTelegram"
            >
              {{ v }}
            </button>
            <a
              v-else-if="k === 'github'"
              class="contact-link"
              href="#"
              @click.prevent="openGitHub"
            >
              {{ v }}
            </a>
            <template v-else>{{ v }}</template>
          </td>
        </tr>
        <tr>
          <td>{{ t('diagnostics.config') }}</td>
          <td>{{ configPath }}</td>
        </tr>
      </tbody>
    </table>
    <div class="actions">
      <button class="btn primary" @click="testAllServers">{{ t('diagnostics.test_servers') }}</button>
      <button class="btn" @click="runHealth">{{ t('diagnostics.health_check') }}</button>
    </div>
    <ul v-if="testResults.length">
      <li v-for="(r, i) in testResults" :key="i">{{ r }}</li>
    </ul>
    <table class="table" v-if="healthReport">
      <thead><tr><th>Check</th><th>Status</th><th>Message</th></tr></thead>
      <tbody>
        <tr v-for="(c, i) in healthReport.checks" :key="i">
          <td>{{ c.name }}</td>
          <td>{{ c.ok ? 'OK' : 'FAIL' }}</td>
          <td>{{ c.message }}</td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<style scoped>
.page-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  flex-wrap: wrap;
  margin-bottom: 1rem;
}
.page-head h2 { margin: 0; }
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
