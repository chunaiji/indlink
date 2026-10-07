import { createI18n } from 'vue-i18n'

import en from '../locales/en.json'
import zhCN from '../locales/zh-CN.json'

// label 用于 LangToggle 的按钮文字,刻意用极短形式(顶栏空间紧)。
export const LOCALES = [
  { value: 'zh-CN', label: '中' },
  { value: 'en', label: 'EN' },
]

// 与 ThemeToggle 的 'driftbottle.admin.theme' 保持同一命名空间。
const KEY = 'driftbottle.admin.lang'

// 默认中文,**不**按浏览器语言自动判定 —— 自动判定会让任何英文浏览器的
// 使用者一进来就看到英文后台,是个不必要的意外。切换一次即持久化。
function savedLocale() {
  try {
    const v = localStorage.getItem(KEY)
    if (LOCALES.some((l) => l.value === v)) return v
  } catch (e) { /* 无痕模式 */ }
  return 'zh-CN'
}

export const i18n = createI18n({
  legacy: false,
  locale: savedLocale(),
  fallbackLocale: 'zh-CN',
  messages: { 'zh-CN': zhCN, en },
})

// 供 api.js 取实时语言下发 Accept-Language。
// 本模块不引 api.js,不构成循环依赖。
export function currentLocale() {
  return i18n.global.locale.value
}

// 非组件模块(如 api.js)拿不到 useI18n(),走全局实例。
export function gt(key, params) {
  return i18n.global.t(key, params)
}

export function setLocale(v) {
  if (!LOCALES.some((l) => l.value === v)) return
  i18n.global.locale.value = v
  document.documentElement.setAttribute('lang', v)
  try {
    localStorage.setItem(KEY, v)
  } catch (e) { /* 无痕模式:本次会话内仍生效 */ }
}

export default i18n
