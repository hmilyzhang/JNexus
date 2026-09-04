// AutoOps 运维平台 — By JJ Zhang, Version 1.0
import { createI18n } from 'vue-i18n'
import zhCN from './zh-CN'
import enUS from './en-US'

export const locales = [
  { value: 'zh-CN', label: '中文' },
  { value: 'en-US', label: 'English' }
]

const i18n = createI18n({
  legacy: false,
  locale: localStorage.getItem('locale') || 'en-US',
  fallbackLocale: 'zh-CN',
  messages: {
    'zh-CN': zhCN,
    'en-US': enUS
  }
})

export function setLocale(locale) {
  i18n.global.locale.value = locale
  localStorage.setItem('locale', locale)
}

export default i18n
