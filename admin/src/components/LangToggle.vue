<template>
  <div class="lt" role="group" :aria-label="t('common.language')">
    <button
      v-for="o in LOCALES"
      :key="o.value"
      type="button"
      :class="{ on: locale === o.value }"
      :aria-pressed="locale === o.value"
      @click="pick(o.value)"
    >{{ o.label }}</button>
  </div>
</template>

<script setup>
import { useI18n } from 'vue-i18n'

import { LOCALES, setLocale } from '../i18n'

const { t, locale } = useI18n()

function pick(v) {
  setLocale(v)
}
</script>

<style scoped>
/* 与 ThemeToggle 同构：顶栏里两个控件视觉上要成对 */
.lt {
  display: flex;
  border: 1px solid var(--line);
  border-radius: var(--radius-sm);
  overflow: hidden;
  background: var(--panel-2);
  flex: none;
}

button {
  min-width: 28px;
  height: 26px;
  padding: 0 var(--s-1);
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
