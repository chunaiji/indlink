<template>
  <div class="tt" role="group" :aria-label="t('theme.label')">
    <button
      v-for="o in OPTS"
      :key="o.v"
      type="button"
      :class="{ on: mode === o.v }"
      :title="o.label"
      :aria-pressed="mode === o.v"
      @click="set(o.v)"
    >{{ o.icon }}</button>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

/**
 * 主题切换：跟随系统 / 浅色 / 深色。
 *
 * **三态，不是两态。**「跟随系统」必须是独立一档而不是默认值的别名 ——
 * 用户在系统深色下选了浅色，重开浏览器不该被拉回深色。
 *
 * 对应 tokens.css 的三段结构：不写 data-theme 即跟随系统，
 * 因此没选过的用户零闪烁（裸 :root 就是完整浅色，不依赖 JS）。
 */
const KEY = 'driftbottle.admin.theme'
const mode = ref('system')

// computed 而非常量 —— 常量存翻译后的字符串不会响应语言切换。
const OPTS = computed(() => [
  { v: 'system', label: t('theme.system'), icon: '◐' },
  { v: 'light', label: t('theme.light'), icon: '☀' },
  { v: 'dark', label: t('theme.dark'), icon: '☾' },
])

function apply(m) {
  const el = document.documentElement
  if (m === 'system') el.removeAttribute('data-theme')
  else el.setAttribute('data-theme', m)
}

function set(m) {
  mode.value = m
  apply(m)
  try {
    if (m === 'system') localStorage.removeItem(KEY)
    else localStorage.setItem(KEY, m)
  } catch (e) { /* 无痕模式:本次会话内仍生效 */ }
}

onMounted(() => {
  let saved = null
  try { saved = localStorage.getItem(KEY) } catch (e) { /* ignore */ }
  mode.value = saved === 'light' || saved === 'dark' ? saved : 'system'
  apply(mode.value)
})
</script>

<style scoped>
.tt {
  display: flex;
  border: 1px solid var(--line);
  border-radius: var(--radius-sm);
  overflow: hidden;
  background: var(--panel-2);
  flex: none;
}

button {
  width: 28px;
  height: 26px;
  font-size: var(--t-label);
  color: var(--ink-3);
  transition:
    background-color var(--dur-1) var(--ease),
    color var(--dur-1) var(--ease);
}

button + button { border-left: 1px solid var(--line); }
button:hover { color: var(--ink); }
button.on { background: var(--brand-weak); color: var(--brand-text); }
</style>
