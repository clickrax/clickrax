<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { GetServiceStatus } from '../wailsjs/go/main/App'
import { EventsOn } from '../wailsjs/runtime/runtime'
import { models } from '../wailsjs/go/models'
import Sidebar from './components/Sidebar.vue'
import StatusBar from './components/StatusBar.vue'
import { serviceStateText } from './utils/format'
import { preloadExecutionState } from './composables/useExecutionState'

defineOptions({ name: 'AppShell' })

const { t } = useI18n()
const router = useRouter()

const serviceStatus = ref<models.ServiceStatus | null>(null)
let pollTimer: ReturnType<typeof setInterval> | null = null

async function refreshServiceStatus() {
  try {
    serviceStatus.value = await GetServiceStatus()
  } catch {
    serviceStatus.value = null
  }
}

const serviceBadgeClass = computed(() => {
  const s = serviceStatus.value
  if (!s || !s.installed) return 'badge badge-muted'
  return s.running ? 'badge badge-ok' : 'badge badge-warn'
})

const serviceBadgeText = computed(() => {
  const s = serviceStatus.value
  if (!s) return t('app.service_unknown')
  if (!s.installed) return t('app.service_not_installed')
  const status = serviceStateText(s.state, s.message, s.installed, t)
  return t('app.service_status', { status })
})

onMounted(() => {
  preloadExecutionState()
  refreshServiceStatus()
  pollTimer = setInterval(refreshServiceStatus, 5000)
  EventsOn('navigate', (path: string) => {
    if (typeof path === 'string' && path) {
      router.push(path)
    }
  })
})

onUnmounted(() => {
  if (pollTimer) clearInterval(pollTimer)
})
</script>

<template>
  <div class="app-shell">
    <header class="topbar">
      <h1>{{ t('app.title') }}</h1>
      <span :class="serviceBadgeClass" :title="serviceStatus?.state || ''">{{ serviceBadgeText }}</span>
    </header>
    <div class="body">
      <Sidebar />
      <main class="content">
        <router-view v-slot="{ Component, route: viewRoute }">
          <keep-alive v-if="!viewRoute.meta.noCache" :max="8">
            <component :is="Component" v-if="Component" :key="viewRoute.path" />
          </keep-alive>
          <component v-else :is="Component" v-if="Component" :key="viewRoute.path" />
        </router-view>
      </main>
    </div>
    <StatusBar />
  </div>
</template>
