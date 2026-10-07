// 主题:白天沙滩 / 夜间星空。按本地时间自动选择,用户也可手动切换。
const KEY = 'theme'

export function getTheme() {
  const saved = uni.getStorageSync(KEY)
  if (saved) return saved
  const hour = new Date().getHours()
  // 19:00 - 06:00 默认夜间
  return (hour >= 19 || hour < 6) ? 'night' : 'day'
}

export function setTheme(theme) {
  uni.setStorageSync(KEY, theme)
}

export function toggleTheme() {
  const next = getTheme() === 'day' ? 'night' : 'day'
  setTheme(next)
  return next
}

export function initTheme() {
  // 占位:具体页面通过 getTheme() 决定根节点 class。
  return getTheme()
}
