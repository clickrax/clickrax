import { i18n } from '../i18n'

type TFn = (key: string, params?: Record<string, unknown>) => string

function t(key: string, params?: Record<string, unknown>): string {
  return i18n.global.t(key, params ?? {})
}

export function formatBytes(n: number): string {
  if (!n) return t('units.bytes_zero')
  if (n >= 1e12) return t('units.tb', { n: (n / 1e12).toFixed(2) })
  if (n >= 1e9) return t('units.gb', { n: (n / 1e9).toFixed(2) })
  if (n >= 1e6) return t('units.mb', { n: (n / 1e6).toFixed(1) })
  if (n >= 1e3) return t('units.kb', { n: (n / 1e3).toFixed(0) })
  return t('units.b', { n })
}

export function formatSpeed(bps: number): string {
  if (bps >= 1e6) return t('units.mb_per_sec', { n: (bps / 1e6).toFixed(1) })
  return t('units.kb_per_sec', { n: (bps / 1e3).toFixed(0) })
}

export function formatDuration(sec: number): string {
  const m = Math.floor(sec / 60)
  const h = Math.floor(m / 60)
  const d = Math.floor(h / 24)
  if (d) return t('units.days_hours', { d, h: h % 24 })
  if (h) return t('units.hours_min', { h, m: m % 60 })
  return t('units.minutes', { m })
}

export function formatDateTime(s: string): string {
  if (!s) return t('common.dash')
  const d = new Date(s)
  const loc = i18n.global.locale.value === 'ru' ? 'ru-RU' : 'en-US'
  return isNaN(d.getTime()) ? s : d.toLocaleString(loc, { dateStyle: 'short', timeStyle: 'short' })
}

export function formatBackupType(tpe: string | undefined, short = false): string {
  const code = normalizeBackupType(tpe)
  if (code === 'full') return t('backup_type.full')
  if (code === 'incremental') return short ? t('backup_type.incr_short') : t('backup_type.incremental')
  if (code === 'restore') return t('backup_type.restore')
  return tpe || '—'
}

function normalizeBackupType(tpe: string | undefined): string {
  const v = (tpe || '').trim().toLowerCase()
  if (v === 'full' || v === 'полный') return 'full'
  if (v === 'incremental' || v === 'инкрементальный' || v === 'инкр.' || v === 'incr.') return 'incremental'
  if (v === 'restore' || v === 'восстановление') return 'restore'
  return v
}

export function formatJobName(name: string): string {
  if (!name) return name
  if (name === 'Быстрый бэкап' || name === 'Quick backup') return t('dashboard.quick_backup')
  const ru = /^Быстрый (.+)$/.exec(name)
  if (ru) return t('dashboard_ext.quick_name_auto', { time: ru[1] })
  const en = /^Quick (.+)$/.exec(name)
  if (en) return t('dashboard_ext.quick_name_auto', { time: en[1] })
  return name
}

export function formatStoredMessage(msg: string): string {
  return msg || ''
}

export function formatLogStatus(status: string): string {
  switch (status) {
    case 'ok': return t('logs_ext.status_ok')
    case 'warning': return t('logs_ext.status_warning')
    case 'error': return t('logs_ext.status_error')
    case 'cancelled': return t('logs_ext.status_cancelled')
    default: return status
  }
}

export type ServiceStateKey =
  | 'not_installed' | 'deleting' | 'running' | 'stopped'
  | 'start_pending' | 'stop_pending' | 'continue_pending' | 'pause_pending' | 'paused' | 'unknown'

export function serviceStateText(
  state: string | undefined,
  message: string | undefined,
  installed: boolean,
  tf: TFn = t,
): string {
  if (!installed) return tf('settings.service_not_installed')
  if (!state) return message || '—'
  const key = `service.state.${state}`
  const translated = tf(key)
  if (translated !== key) return translated
  return message || '—'
}
