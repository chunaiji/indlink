<template>
  <div class="card">
    <div class="t">{{ t('account.title') }}</div>
    <input v-model="oldp" class="ipt" type="password" :placeholder="t('account.oldPassword')" />
    <input v-model="newp" class="ipt" type="password" :placeholder="t('account.newPassword')" />
    <input v-model="newp2" class="ipt" type="password" :placeholder="t('account.confirmPassword')" />
    <button class="btn" :disabled="loading" @click="submit">{{ t('common.save') }}</button>
    <div v-if="msg" class="msg" :class="{ bad }">{{ msg }}</div>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api'

const { t } = useI18n()
const oldp = ref(''); const newp = ref(''); const newp2 = ref('')
const loading = ref(false); const msg = ref(''); const bad = ref(false)

async function submit() {
  if (newp.value.length < 6) { show(t('account.errTooShort'), true); return }
  if (newp.value !== newp2.value) { show(t('account.errMismatch'), true); return }
  loading.value = true
  try {
    await api.changePassword(oldp.value, newp.value)
    show(t('account.changed'))
    oldp.value = newp.value = newp2.value = ''
  } catch (e) { show(e.message, true) } finally { loading.value = false }
}
function show(text, b = false) { msg.value = text; bad.value = b; setTimeout(() => { msg.value = '' }, 2500) }
</script>

<style scoped>
/* 按钮 / 输入来自 styles/base.css。原本这里的提交按钮是渐变底（§9 禁止）。 */
.card { max-width: 420px; padding: var(--s-6); }
.t { margin-bottom: var(--s-5); font-size: var(--t-h1); font-weight: 600; }
.ipt { width: 100%; margin-bottom: var(--s-3); }
.btn { padding: 0 var(--s-6); }
.msg { margin-top: var(--s-4); color: var(--ok); font-size: var(--t-value); }
.msg.bad { color: var(--danger); }
</style>
