import { createI18n } from 'vue-i18n'
import ru from './ru.json'
import en from './en.json'

export type AppLocale = 'ru' | 'en'

export const i18n = createI18n({
  legacy: false,
  locale: 'ru',
  fallbackLocale: 'en',
  messages: { ru, en },
})

export function setAppLocale(lang: string) {
  const norm = lang === 'ru' || lang?.startsWith('ru-') ? 'ru' : 'en'
  i18n.global.locale.value = norm
  document.documentElement.lang = norm
  return norm
}

export function detectBrowserLocale(): AppLocale {
  const nav = (navigator.language || 'en').toLowerCase()
  return nav.startsWith('ru') ? 'ru' : 'en'
}
