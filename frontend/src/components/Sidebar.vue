<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { useRouter, useRoute } from 'vue-router'

const { t } = useI18n()
const router = useRouter()
const route = useRoute()

const items = [
  { path: '/', key: 'dashboard' },
  { path: '/servers', key: 'servers' },
  { path: '/jobs', key: 'jobs' },
  { path: '/progress', key: 'progress' },
  { path: '/restore', key: 'restore' },
  { path: '/logs', key: 'logs' },
  { path: '/settings', key: 'settings' },
  { path: '/diagnostics', key: 'diagnostics' },
]
</script>

<template>
  <nav class="sidebar">
    <button
      v-for="item in items"
      :key="item.path"
      class="nav-btn"
      :class="{ active: route.path === item.path }"
      @click="router.push(item.path)"
    >
      {{ t(`nav.${item.key}`) }}
    </button>
  </nav>
</template>
