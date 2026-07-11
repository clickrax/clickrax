import { createRouter, createWebHashHistory } from 'vue-router'
import Dashboard from '../views/Dashboard.vue'
import Servers from '../views/Servers.vue'
import Jobs from '../views/Jobs.vue'
import ProgressView from '../views/ProgressView.vue'
import Restore from '../views/Restore.vue'
import Logs from '../views/Logs.vue'
import Settings from '../views/Settings.vue'
import Diagnostics from '../views/Diagnostics.vue'

export const router = createRouter({
  history: createWebHashHistory(),
  routes: [
    { path: '/', name: 'dashboard', component: Dashboard },
    { path: '/servers', name: 'servers', component: Servers },
    { path: '/jobs', name: 'jobs', component: Jobs },
    { path: '/progress', name: 'progress', component: ProgressView },
    { path: '/restore', name: 'restore', component: Restore, meta: { noCache: true } },
    { path: '/logs', name: 'logs', component: Logs },
    { path: '/settings', name: 'settings', component: Settings },
    { path: '/diagnostics', name: 'diagnostics', component: Diagnostics },
  ],
})
