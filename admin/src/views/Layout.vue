<template>
  <!--
    管理后台整体走「紧」密度（S-2：后台是密集表格侧，密度即可读性）。
    写在外壳上而不是每个视图各写一遍 —— 让 23 个视图各自记得写，只会漏，
    而漏写的后果不是「密度不对」，是 padding 归零、表格挤成一片。
  -->
  <div class="shell" data-density="tight">
    <div v-if="sidebarOpen" class="scrim" @click="sidebarOpen = false"></div>

    <aside class="side" :class="{ open: sidebarOpen }">
      <div class="logo">
        <span class="mark">🌊</span>
        <div class="who">
          <b>{{ t('layout.brand') }}</b>
          <span class="ver">{{ t('layout.brandSub') }}</span>
        </div>
      </div>

      <nav>
        <div v-for="g in GROUPS" :key="g.titleKey || 'top'" class="grp">
          <div v-if="g.titleKey" class="gt">{{ t(g.titleKey) }}</div>
          <router-link
            v-for="it in g.items"
            :key="it.to"
            :to="it.to"
            class="nav"
            active-class="on"
          >
            <span class="ic">{{ it.icon }}</span>
            <span class="lb">{{ t(it.labelKey) }}</span>
          </router-link>
        </div>
      </nav>

      <div class="foot">
        <div class="who-line">{{ me.username || '—' }} · {{ me.role }}</div>
        <button class="btn-sm block" @click="logout">{{ t('layout.logout') }}</button>
      </div>
    </aside>

    <div class="main">
      <header class="top">
        <button class="burger" type="button" :aria-label="t('layout.menu')" @click="sidebarOpen = true">☰</button>
        <h1 class="tt">{{ title }}</h1>
        <div class="acts">
          <div class="tenant-wrap">
            <label class="tenant-label">{{ t('layout.tenant') }}</label>
            <select
              class="sel tenant-select"
              :value="currentTenantID"
              :disabled="tenants.length === 0"
              @change="onTenantChange"
            >
              <option value="0">{{ t('layout.allTenants') }}</option>
              <option v-for="t in tenants" :key="t.tenant_id" :value="t.tenant_id">
                {{ t.name }} ({{ t.tenant_id }})
              </option>
            </select>
          </div>
          <LangToggle />
          <ThemeToggle />
        </div>
      </header>

      <main class="body"><router-view /></main>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { api, setToken } from '../api'
import { tenantStore } from '../tenant.js'
import LangToggle from '../components/LangToggle.vue'
import ThemeToggle from '../components/ThemeToggle.vue'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const me = ref({})
const sidebarOpen = ref(false)
watch(() => route.path, () => { sidebarOpen.value = false }) // 切页自动收起抽屉
const title = computed(() => (route.meta.titleKey ? t(route.meta.titleKey) : t('layout.fallbackTitle')))
const tenants = computed(() => tenantStore.tenants)
const currentTenantID = computed(() => tenantStore.currentTenantID)

// 存 key 而不是翻译后的字符串 —— 模块级常量不会响应语言切换,
// 翻译放在模板里做,这样它可以继续当普通常量。
const GROUPS = [
  {
    titleKey: '',
    items: [
      { to: '/dashboard', labelKey: 'nav.dashboard', icon: '📊' },
      { to: '/config', labelKey: 'nav.config', icon: '⚙️' },
      { to: '/apilogs', labelKey: 'nav.apilogs', icon: '📡' },
      { to: '/email-logs', labelKey: 'nav.emailLogs', icon: '✉️' },
      { to: '/account', labelKey: 'nav.account', icon: '🔑' },
      { to: '/robot', labelKey: 'nav.robot', icon: '🤖' },
      { to: '/tenants', labelKey: 'nav.tenants', icon: '🏢' },
      { to: '/credentials', labelKey: 'nav.credentials', icon: '🔐' },
    ],
  },
  {
    titleKey: 'nav.groupAI',
    items: [
      { to: '/persona', labelKey: 'nav.persona', icon: '🎭' },
      { to: '/robot-profiles', labelKey: 'nav.robotProfiles', icon: '🤖' },
      { to: '/keyword-rules', labelKey: 'nav.keywordRules', icon: '🔑' },
      { to: '/reply-cache', labelKey: 'nav.replyCache', icon: '💾' },
      { to: '/robot-chats', labelKey: 'nav.robotChats', icon: '💬' },
    ],
  },
  {
    titleKey: 'nav.groupUser',
    items: [
      { to: '/users', labelKey: 'nav.users', icon: '👥' },
      { to: '/messages', labelKey: 'nav.messages', icon: '💬' },
      { to: '/bottles', labelKey: 'nav.bottles', icon: '🍾' },
      { to: '/moments', labelKey: 'nav.moments', icon: '🌈' },
    ],
  },
  {
    titleKey: 'nav.groupProviders',
    items: [
      { to: '/providers/pay', labelKey: 'nav.providersPay', icon: '💰' },
      { to: '/providers/map', labelKey: 'nav.providersMap', icon: '🗺️' },
      { to: '/providers/moderation', labelKey: 'nav.providersModeration', icon: '🛡️' },
      { to: '/providers/sso', labelKey: 'nav.providersSso', icon: '🔑' },
    ],
  },
  {
    titleKey: 'nav.groupFinance',
    items: [
      { to: '/packages', labelKey: 'nav.packages', icon: '💳' },
      { to: '/items', labelKey: 'nav.items', icon: '🎁' },
      { to: '/orders', labelKey: 'nav.orders', icon: '🧾' },
      { to: '/wallet-txns', labelKey: 'nav.walletTxns', icon: '🪙' },
    ],
  },
]

onMounted(async () => {
  try { me.value = await api.me() } catch (e) {}
  await tenantStore.load(api.listTenants)
})

function onTenantChange(e) {
  tenantStore.setTenant(e.target.value) // 字符串,避免雪花 ID 丢精度
  location.reload() // router.go(0) 在 hash 路由下刷新不可靠,强制整页刷新让各页按新租户重拉数据
}

function logout() {
  setToken('')
  router.push('/login')
}
</script>

<style scoped>
.shell {
  display: flex;
  min-height: 100vh;
  min-height: 100dvh;
}

/* ── 侧栏（§7.8）──────────────────────────────────────────
 * 改造前是深色栏 + 选中态渐变（linear-gradient(135deg,#2f6fed,#1f9fd1)）。
 * 规范 §9 明确「不做渐变、发光、霓虹」，且品牌色只用于「可以点」——
 * 选中态现在是 --brand-weak 底 + --brand-text 字 + --radius-sm。 */
.side {
  width: var(--sidebar-w);
  flex: none;
  display: flex;
  flex-direction: column;
  background: var(--panel);
  border-right: 1px solid var(--line);
  position: sticky;
  top: 0;
  height: 100vh;
  height: 100dvh;
}

.logo {
  display: flex;
  align-items: center;
  gap: var(--s-2);
  height: var(--topbar-h);
  padding: 0 var(--s-4);
  border-bottom: 1px solid var(--line-2);
  flex: none;
}

.mark {
  width: 30px;
  height: 30px;
  flex: none;
  display: grid;
  place-items: center;
  border-radius: var(--radius-sm);
  background: var(--brand-weak);
  font-size: 15px;
}

.who { display: grid; line-height: 1.25; min-width: 0; }
.who b { font-size: var(--t-body); font-weight: 700; letter-spacing: -0.01em; }
.ver { font-size: var(--t-tag); color: var(--ink-3); }

nav {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: var(--s-3) var(--s-2);
}

.grp + .grp { margin-top: var(--s-3); }

.gt {
  padding: 0 var(--s-2) var(--s-1);
  font-size: var(--t-tag);
  font-weight: 700;
  letter-spacing: 0.08em;
  color: var(--ink-3);
}

.nav {
  display: flex;
  align-items: center;
  gap: var(--s-2);
  padding: var(--s-2);
  margin-bottom: 1px;
  border-radius: var(--radius-sm);
  font-size: var(--t-value);
  color: var(--ink-2);
  transition:
    background-color var(--dur-1) var(--ease),
    color var(--dur-1) var(--ease);
}

.nav:hover { background: var(--panel-2); color: var(--ink); }
.nav.on { background: var(--brand-weak); color: var(--brand-text); font-weight: 650; }

.ic { width: 18px; flex: none; text-align: center; font-size: var(--t-value); }
.lb { flex: 1; min-width: 0; }

.foot {
  flex: none;
  padding: var(--s-3);
  border-top: 1px solid var(--line-2);
}

.who-line {
  margin-bottom: var(--s-2);
  font-size: var(--t-label);
  color: var(--ink-3);
}

.block { display: flex; width: 100%; }

/* ── 主区 ── */
.main { flex: 1; min-width: 0; display: flex; flex-direction: column; }

.top {
  height: var(--topbar-h);
  display: flex;
  align-items: center;
  gap: var(--s-3);
  padding: 0 var(--s-5);
  background: var(--panel);
  border-bottom: 1px solid var(--line);
  position: sticky;
  top: 0;
  z-index: 20;
}

.burger { display: none; font-size: 17px; color: var(--ink-2); }

.tt {
  flex: 1;
  min-width: 0;
  font-size: 16px;
  font-weight: 700;
  letter-spacing: -0.02em;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.acts { display: flex; align-items: center; gap: var(--s-3); flex: none; }
.tenant-wrap { display: flex; align-items: center; gap: var(--s-2); }
.tenant-label { font-size: var(--t-label); color: var(--ink-3); }
.tenant-select { height: 28px; font-size: var(--t-label); }

.body { flex: 1; min-width: 0; padding: var(--s-5) var(--s-5) var(--s-8); }

.scrim { display: none; }

@media (max-width: 900px) {
  .side {
    position: fixed;
    z-index: 60;
    transform: translateX(-100%);
    transition: transform var(--dur-3) var(--ease);
    box-shadow: var(--shadow-2);
  }
  .side.open { transform: none; }
  .scrim {
    display: block;
    position: fixed;
    inset: 0;
    z-index: 50;
    background: var(--scrim);
  }
  .burger { display: block; }
  .top { padding: 0 var(--s-3); }
  .tenant-label { display: none; }
  .tenant-select { max-width: 40vw; }
  .body { padding: var(--s-4) var(--s-3) var(--s-7); }
}
</style>
