import { GetSettings } from '../../wailsjs/go/main/App'
import { detectBrowserLocale, i18n, setAppLocale } from './index'

export { i18n, setAppLocale, detectBrowserLocale }

export async function bootstrapLocale(): Promise<string> {
  try {
    const s = await GetSettings()
    return setAppLocale(s.language || detectBrowserLocale())
  } catch {
    return setAppLocale(detectBrowserLocale())
  }
}
