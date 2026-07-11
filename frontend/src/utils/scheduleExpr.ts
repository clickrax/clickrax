import { i18n } from '../i18n'

export type ScheduleType = 'daily' | 'weekly'

export interface ParsedSchedule {
  type: ScheduleType
  times: string[]
  weekdays: number[]
}

export interface SchedulePreset {
  type: ScheduleType
  times: string[]
  weekdays: number[]
  label: string
  presetKey: string
}

function t(key: string, params?: Record<string, unknown>): string {
  return i18n.global.t(key, params ?? {})
}

const WEEKDAY_TO_NUM: Record<string, number> = {
  mon: 1, tue: 2, wed: 3, thu: 4, fri: 5, sat: 6, sun: 7,
  пн: 1, вт: 2, ср: 3, чт: 4, пт: 5, сб: 6, вс: 7,
  понедельник: 1, вторник: 2, среда: 3, четверг: 4, пятница: 5, суббота: 6, воскресенье: 7,
}

export function weekdayShort(day: number): string {
  const keys = ['', 'mon', 'tue', 'wed', 'thu', 'fri', 'sat', 'sun'] as const
  const k = keys[day]
  return k ? t(`jobs.weekday_${k}`) : ''
}

export function weekdayFull(day: number): string {
  const keys = ['', 'mon', 'tue', 'wed', 'thu', 'fri', 'sat', 'sun'] as const
  const k = keys[day]
  return k ? t(`schedule.weekday_full_${k}`) : ''
}

export function normalizeClock(value: string): string {
  const raw = value.trim()
  if (!raw) throw new Error(t('schedule.empty_time'))
  const parts = raw.split(':')
  if (parts.length < 2) throw new Error(t('schedule.invalid_time', { value }))
  const hour = Number(parts[0])
  const minute = Number(parts[1])
  if (!Number.isInteger(hour) || !Number.isInteger(minute) || hour < 0 || hour > 23 || minute < 0 || minute > 59) {
    throw new Error(t('schedule.invalid_time', { value }))
  }
  return `${String(hour).padStart(2, '0')}:${String(minute).padStart(2, '0')}`
}

function formatTimes(times: string[]): string {
  const norm = times.map((x) => normalizeClock(x))
  if (norm.length === 1) return t('schedule.at_one', { time: norm[0] })
  if (norm.length === 2) return t('schedule.at_two', { a: norm[0], b: norm[1] })
  return t('schedule.at_many', { list: norm.slice(0, -1).join(', '), last: norm[norm.length - 1] })
}

function formatWeekdays(weekdays: number[]): string {
  if (weekdays.length === 7) return t('schedule.every_day')
  if (weekdays.length === 5 && weekdays[0] === 1 && weekdays[4] === 5) return t('schedule.weekdays_mon_fri')
  if (weekdays.length === 2 && weekdays[0] === 6 && weekdays[1] === 7) return t('schedule.sat_sun')
  if (weekdays.length === 1) return weekdayFull(weekdays[0])
  return weekdays.map((d) => weekdayShort(d)).join(', ')
}

export function formatSchedule(
  type: ScheduleType,
  times: string[],
  weekdays: number[],
): string {
  const validTimes = times.filter(Boolean)
  if (!validTimes.length) return t('schedule.not_set')
  const when = formatTimes(validTimes)
  if (type !== 'weekly' || !weekdays.length) return t('schedule.daily', { when })
  return `${formatWeekdays(weekdays)} ${when}`
}

/** @deprecated use formatSchedule */
export const formatScheduleRu = formatSchedule

/** @deprecated use weekdayShort */
export const weekdayRuShort = weekdayShort

export function schedulesMatch(
  a: { type: ScheduleType; times: string[]; weekdays: number[] },
  b: { type: ScheduleType; times: string[]; weekdays: number[] },
): boolean {
  if (a.type !== b.type) return false
  const aTimes = a.times.map((x) => normalizeClock(x)).join(',')
  const bTimes = b.times.map((x) => normalizeClock(x)).join(',')
  if (aTimes !== bTimes) return false
  if (a.type !== 'weekly') return true
  const aDays = [...a.weekdays].sort((x, y) => x - y).join(',')
  const bDays = [...b.weekdays].sort((x, y) => x - y).join(',')
  return aDays === bDays
}

const PRESET_DEFS: Omit<SchedulePreset, 'label'>[] = [
  { type: 'daily', times: ['02:00'], weekdays: [], presetKey: 'daily_0200' },
  { type: 'daily', times: ['02:00', '14:00'], weekdays: [], presetKey: 'daily_0200_1400' },
  { type: 'weekly', times: ['02:00'], weekdays: [1, 2, 3, 4, 5], presetKey: 'weekdays_0200' },
  { type: 'weekly', times: ['03:00'], weekdays: [6, 7], presetKey: 'sat_sun_0300' },
  { type: 'weekly', times: ['03:00'], weekdays: [7], presetKey: 'sun_0300' },
  { type: 'weekly', times: ['02:00'], weekdays: [1], presetKey: 'mon_0200' },
]

export function getSchedulePresets(): SchedulePreset[] {
  return PRESET_DEFS.map((p) => ({
    ...p,
    label: t(`schedule.preset.${p.presetKey}`),
  }))
}

/** @deprecated use getSchedulePresets() */
export const SCHEDULE_PRESETS: SchedulePreset[] = PRESET_DEFS.map((p) => ({
  ...p,
  label: '',
}))
