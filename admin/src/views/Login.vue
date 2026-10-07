<template>
  <div class="wrap">
    <div class="card">
      <div class="lang"><LangToggle /></div>
      <div class="logo">{{ t('login.brand') }}</div>
      <div class="sub">{{ t('login.subtitle') }}</div>
      <input v-model="username" class="ipt" :placeholder="t('login.username')" @keyup.enter="submit" />
      <input v-model="password" class="ipt" type="password" :placeholder="t('login.password')" @keyup.enter="submit" />
      <button class="btn" :disabled="loading" @click="submit">
        {{ loading ? t('login.submitting') : t('login.submit') }}
      </button>
      <div v-if="err" class="err">{{ err }}</div>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { api, setToken } from '../api'
import LangToggle from '../components/LangToggle.vue'

const { t } = useI18n()
const router = useRouter()
const username = ref('')
const password = ref('')
const loading = ref(false)
const err = ref('')

async function submit() {
  if (!username.value || !password.value) { err.value = t('login.needBoth'); return }
  loading.value = true
  err.value = ''
  try {
    const data = await api.login(username.value, password.value)
    setToken(data.token)
    router.push('/dashboard')
  } catch (e) {
    err.value = e.message
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
/* 改造前这一屏有两处渐变（页面底 + 按钮底），规范 §9 明确「不做渐变、
   发光、霓虹」——付钱那侧不能把产品读成投机工具。
   层次改由 --bg / --panel 两档底色 + --shadow-2 承载。 */
.wrap {
  min-height: 100vh;
  min-height: 100dvh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: var(--s-4);
  background: var(--bg);
}

.card {
  width: min(360px, 92vw);
  padding: var(--s-8) var(--s-7);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-2);
  text-align: center;
}

.lang { display: flex; justify-content: flex-end; margin-bottom: var(--s-2); }

.logo { font-size: var(--t-display); font-weight: 650; letter-spacing: -0.02em; }
.sub { margin: var(--s-1) 0 var(--s-6); color: var(--ink-2); font-size: var(--t-value); }

/* 登录是单目的表单，控件放到 lg 档（40px）比后台的紧档更好按 */
.ipt {
  width: 100%;
  height: 40px;
  margin-bottom: var(--s-3);
  text-align: left;
}

.btn {
  width: 100%;
  height: 40px;
  margin-top: var(--s-1);
  font-size: var(--t-body);
}

.err { margin-top: var(--s-3); }
</style>
