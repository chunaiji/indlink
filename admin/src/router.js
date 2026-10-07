import { createRouter, createWebHashHistory } from 'vue-router'
import { getToken } from './api'

import Login from './views/Login.vue'
import Layout from './views/Layout.vue'
import Dashboard from './views/Dashboard.vue'
import Config from './views/Config.vue'
import Account from './views/Account.vue'
import RobotContent from './views/RobotContent.vue'
import Credentials from './views/Credentials.vue'
import ProviderPage from './views/ProviderPage.vue'
import Tenants from './views/Tenants.vue'
import Persona from './views/Persona.vue'
import RobotProfile from './views/RobotProfile.vue'
import KeywordRules from './views/KeywordRules.vue'
import ReplyCache from './views/ReplyCache.vue'
import RobotChats from './views/RobotChats.vue'
import Users from './views/Users.vue'
import Messages from './views/Messages.vue'
import Bottles from './views/Bottles.vue'
import Moments from './views/Moments.vue'
import ApiLogs from './views/ApiLogs.vue'
import EmailLogs from './views/EmailLogs.vue'
import Orders from './views/Orders.vue'
import WalletTxns from './views/WalletTxns.vue'
import Packages from './views/Packages.vue'
import Items from './views/Items.vue'

const routes = [
  { path: '/login', component: Login },
  {
    path: '/',
    component: Layout,
    children: [
      { path: '', redirect: '/dashboard' },
      { path: 'dashboard', component: Dashboard, meta: { titleKey: 'nav.dashboard' } },
      // :section 可选 —— 配置页按分区分 Tab,带上它链接才能收藏与分享
      { path: 'config/:section?', component: Config, meta: { titleKey: 'nav.config' } },
      { path: 'account', component: Account, meta: { titleKey: 'nav.account' } },
      { path: 'robot', component: RobotContent, meta: { titleKey: 'nav.robot' } },
      { path: 'tenants', component: Tenants, meta: { titleKey: 'nav.tenants' } },
      { path: 'credentials', component: Credentials, meta: { titleKey: 'nav.credentials' } },
      { path: 'providers/:kind', component: ProviderPage, meta: { titleKey: 'nav.providers' } },
      { path: 'persona', component: Persona, meta: { titleKey: 'nav.persona' } },
      { path: 'robot-profiles', component: RobotProfile, meta: { titleKey: 'nav.robotProfiles' } },
      { path: 'keyword-rules', component: KeywordRules, meta: { titleKey: 'nav.keywordRules' } },
      { path: 'reply-cache', component: ReplyCache, meta: { titleKey: 'nav.replyCache' } },
      { path: 'robot-chats', component: RobotChats, meta: { titleKey: 'nav.robotChats' } },
      { path: 'users', component: Users, meta: { titleKey: 'nav.users' } },
      { path: 'messages', component: Messages, meta: { titleKey: 'nav.messages' } },
      { path: 'bottles', component: Bottles, meta: { titleKey: 'nav.bottles' } },
      { path: 'moments', component: Moments, meta: { titleKey: 'nav.moments' } },
      { path: 'apilogs', component: ApiLogs, meta: { titleKey: 'nav.apilogs' } },
      { path: 'email-logs', component: EmailLogs, meta: { titleKey: 'nav.emailLogs' } },
      { path: 'orders', component: Orders, meta: { titleKey: 'nav.orders' } },
      { path: 'wallet-txns', component: WalletTxns, meta: { titleKey: 'nav.walletTxns' } },
      { path: 'packages', component: Packages, meta: { titleKey: 'nav.packages' } },
      { path: 'items', component: Items, meta: { titleKey: 'nav.items' } },
    ]
  }
]

const router = createRouter({ history: createWebHashHistory(), routes })

router.beforeEach((to) => {
  if (to.path !== '/login' && !getToken()) return '/login'
  if (to.path === '/login' && getToken()) return '/dashboard'
  return true
})

export default router
