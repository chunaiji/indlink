<template>
  <div>
    <div class="scope" :class="isGlobal ? 'scope-global' : 'scope-tenant'">
      <template v-if="isGlobal">{{ t('config.scopeGlobal') }}</template>
      <template v-else>{{ t('config.scopeTenant', { name: tenantName }) }}</template>
    </div>
    <!-- 上线模式一键切换:审核模式=隐社交tab+关社交入口+开页面覆盖;运营模式=全部恢复 -->
    <div class="launch-panel" :class="reviewMode ? 'reviewing' : 'live'">
      <div class="lp-info">
        <div class="lp-title">{{ reviewMode ? t('config.modeReview') : t('config.modeLive') }}</div>
        <div class="lp-desc">
          {{ reviewMode ? t('config.modeReviewDesc') : t('config.modeLiveDesc') }}
        </div>
      </div>
      <button class="lp-btn" :class="{ danger: !reviewMode }" :disabled="switching" @click="toggleLaunch">
        {{ switching ? t('config.switching') : (reviewMode ? t('config.backToLive') : t('config.toReview')) }}
      </button>
    </div>

    <!-- 搜索**跨分区**。分区解决"太多",搜索解决"不知道在哪个分区"——
         只在当前 Tab 里搜就退化成前者的附属品了。 -->
    <div class="search">
      <input
        v-model="query"
        class="ipt search-ipt"
        type="search"
        :placeholder="t('config.searchPlaceholder')"
        @keydown.esc="query = ''"
      />
      <button v-if="searching" class="btn-sm" @click="query = ''">{{ t('config.clearSearch') }}</button>
    </div>

    <!-- 分区 Tab。两百多项挤一页找不着东西,按功能内聚分区;
         section code 与路由挂钩(/config/:section),链接可收藏可分享。 -->
    <nav class="tabs" role="tablist">
      <button
        v-for="s in visibleSections"
        :key="s.code"
        type="button"
        role="tab"
        :aria-selected="!searching && s.code === activeSection"
        :class="{ on: !searching && s.code === activeSection }"
        @click="goSection(s.code)"
      >
        {{ s.label }}
        <span class="tab-n">{{ countOf(s.code) }}</span>
      </button>
    </nav>

    <!-- 搜索时 Tab 选中态会熄灭,这条横幅解释为什么结果不受 Tab 约束 -->
    <div v-if="searching" class="hits">
      {{ t('config.searchHits', { n: hitCount }) }}
    </div>

    <!-- 支付分区顶部:微信/支付宝商户配置在 app_credentials 表里,由「租户凭证」页维护。
         这里只读引用,让人一眼看清这个租户支付到底配没配,而不用两个界面写同一批行。 -->
    <div v-if="activeSection === 'pay' && !searching && payCreds.length" class="cred-ref">
      <span v-for="c in payCreds" :key="c.id" class="cred-item">
        {{ c.platform === 'wx' ? t('credentials.platformWx') : t('credentials.platformAlipay') }}
        <span class="badge" :class="c.mch_id ? 'green' : 'gray'">
          {{ c.mch_id ? t('config.payConfigured') : t('config.payUnconfigured') }}
        </span>
      </span>
      <router-link to="/credentials" class="cred-link">{{ t('config.payGoCredentials') }}</router-link>
    </div>

    <div v-if="searching && !grouped.length" class="empty">{{ t('config.searchEmpty') }}</div>

    <div v-for="g in grouped" :key="g.code" class="block" :class="{ collapsed: !isOpen(g.code) }">
      <div class="gt gt-toggle" @click="toggle(g.code)">
        <span class="gt-arrow" :class="{ open: isOpen(g.code) }">▸</span>
        <!-- 搜索跨了分区,得告诉用户这一组住在哪 -->
        <span v-if="searching" class="gt-sec">{{ g.sectionLabel }} ·</span>
        {{ g.label }}
        <span class="gt-count">{{ t('config.itemCount', { n: g.fields.length }) }}</span>
      </div>
      <div v-show="isOpen(g.code)" class="rows">
        <div v-for="f in g.fields" :key="f.key" class="row" :class="{ 'row-area': f.type === 'textarea' }">
          <div class="lab">
            <div class="name">{{ f.label }}</div>
            <div class="key">{{ f.key }}</div>
          </div>
          <div class="ctl" :class="{ 'ctl-area': f.type === 'textarea' }">
            <!-- 机密项:值不出服务端,只显示配没配。
                 输入框恒为空,填了才写;要真清掉得点「清除」。 -->
            <div v-if="f.secret" class="secret">
              <span class="badge" :class="f.is_set ? 'green' : 'gray'">
                {{ f.is_set ? t('config.secretSet') : t('config.secretUnset') }}
              </span>
              <!-- 多行机密(如 .p8 私钥)必须用 textarea:PEM 粘进单行 input
                   会被吞掉换行,存进去的是一把废密钥,而且很难看出来 -->
              <textarea
                v-if="f.type === 'textarea'"
                class="ipt ipt-area"
                rows="4"
                :placeholder="f.is_set ? t('config.secretKeep') : t('config.secretEnter')"
                @blur="save(f, $event.target.value); $event.target.value = ''"
              ></textarea>
              <input
                v-else
                class="ipt"
                type="password"
                autocomplete="new-password"
                :placeholder="f.is_set ? t('config.secretKeep') : t('config.secretEnter')"
                @blur="save(f, $event.target.value); $event.target.value = ''"
                @keyup.enter="$event.target.blur()"
              />
              <button v-if="f.is_set" class="btn-sm del" @click="clearSecret(f)">{{ t('config.clear') }}</button>
            </div>
            <label v-else-if="f.type === 'bool'" class="switch">
              <input type="checkbox" :checked="f.value === '1'" @change="save(f, $event.target.checked ? '1' : '0')" />
              <span>{{ f.value === '1' ? t('common.on') : t('common.off') }}</span>
            </label>
            <textarea
              v-else-if="f.type === 'textarea'"
              class="ipt ipt-area"
              v-model="f.value"
              rows="4"
              @blur="save(f, f.value)"
            />
            <div v-else-if="f.type === 'image'" class="img-ctl">
              <img v-if="f.value" :src="f.value" class="img-preview" @click="previewImg = f.value" />
              <label class="btn sm">
                {{ uploadingKey === f.key ? t('config.uploading') : (f.value ? t('config.replaceImage') : t('config.uploadImage')) }}
                <input type="file" accept="image/*" hidden :disabled="!!uploadingKey" @change="onPickImage(f, $event)" />
              </label>
              <button v-if="f.value" class="btn-sm del" @click="save(f, ''); f.value = ''">{{ t('config.clear') }}</button>
            </div>
            <input
              v-else
              class="ipt"
              :type="f.type === 'int' ? 'number' : 'text'"
              v-model="f.value"
              @blur="save(f, f.value)"
              @keyup.enter="save(f, f.value)"
            />
          </div>
        </div>
      </div>
      <!-- AI 引擎：在该 group 末尾显示测试按钮。
           判断用稳定 code,不要用 g.label —— 它会随语言变化。 -->
      <div v-if="g.code === 'ai' && expanded[g.code]" class="row llm-test-row">
        <div class="lab">
          <div class="name">{{ t('config.llmTest') }}</div>
          <div class="key">{{ t('config.llmTestDesc') }}</div>
        </div>
        <div class="ctl">
          <button class="btn" :disabled="llmTesting" @click="testLLM">
            {{ llmTesting ? t('config.llmTesting') : t('config.llmTestBtn') }}
          </button>
        </div>
      </div>
    </div>

    <!-- LLM 测试结果弹框 -->
    <div v-if="llmResult !== null" class="overlay" @click.self="llmResult = null">
      <div class="dlg">
        <div class="dlg-hd" :class="{ ok: llmOk, bad: !llmOk }">
          {{ llmOk ? t('config.llmOk') : t('config.llmBad') }}
        </div>
        <div class="dlg-body">{{ llmResult }}</div>
        <button class="btn" @click="llmResult = null">{{ t('common.close') }}</button>
      </div>
    </div>

    <!-- 上线模式切换确认(自定义弹层,不用 window.confirm 防被浏览器屏蔽) -->
    <div v-if="launchConfirm" class="overlay" @click.self="launchConfirm = null">
      <div class="dlg">
        <div class="dlg-hd" :class="launchConfirm.toReview ? 'bad' : 'ok'">
          {{ launchConfirm.toReview ? t('config.confirmToReview') : t('config.confirmToLive') }}
        </div>
        <div class="dlg-body">
          <div class="lc-target">{{ t('config.scope', { target: launchConfirm.target }) }}</div>
          <ul class="lc-lines">
            <li v-for="(l, i) in launchConfirm.lines" :key="i">{{ l }}</li>
          </ul>
        </div>
        <div class="lc-btns">
          <button class="btn ghost" @click="launchConfirm = null">{{ t('common.cancel') }}</button>
          <button class="btn" @click="doToggleLaunch">{{ t('config.confirmSwitch') }}</button>
        </div>
      </div>
    </div>

    <!-- 图片放大预览 -->
    <div v-if="previewImg" class="overlay" @click="previewImg = ''">
      <img :src="previewImg" class="img-big" />
    </div>

    <div v-if="msg" class="toast" :class="{ bad: msgBad }">{{ msg }}</div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { api } from '../api'
import { tenantStore } from '../tenant.js'

const { t, locale } = useI18n()
const route = useRoute()
const router = useRouter()

const list = ref([])
const msg = ref('')
const msgBad = ref(false)
const llmTesting = ref(false)
const llmResult = ref(null)
const llmOk = ref(false)
const globalEditConfirmed = ref(false)
const uploadingKey = ref('')  // 正在上传的配置 key(防重复点)
const previewImg = ref('')    // 放大预览的图片 URL
const expanded = ref({})      // group -> 是否展开(默认全部收起)

function toggle(group) {
  expanded.value = { ...expanded.value, [group]: !expanded.value[group] }
}

const isGlobal = computed(() => !tenantStore.currentTenantID || tenantStore.currentTenantID === '0')
const tenantName = computed(() => {
  const cur = String(tenantStore.currentTenantID)
  const t = tenantStore.tenants.find(x => String(x.tenant_id) === cur)
  return t ? `${t.name} (${t.tenant_id})` : cur
})

// —— 分区 Tab ——
// section 与 group 一样是服务端给的稳定 code;顺序也由服务端定,
// 因为 configMeta 是按分组排的,前端按首次出现推导出来的顺序是乱的。
const sections = ref([])

// 一项都不可见的分区不出 Tab。租户类型过滤之后这是常态 ——
// App 租户在「广告」下一项都没有,留个「广告 0」只会让人点进去看空页。
// list 还没回来时先全给,否则首屏 Tab 会闪一下才长出来。
const visibleSections = computed(() =>
  list.value.length === 0 ? sections.value : sections.value.filter((s) => countOf(s.code) > 0),
)

const activeSection = computed(() => {
  const want = route.params.section
  if (want && visibleSections.value.some((s) => s.code === want)) return want
  return visibleSections.value[0]?.code || ''
})

function goSection(code) {
  query.value = '' // 点 Tab 是"我要去那儿",不是"在那儿里面搜"
  router.push(`/config/${code}`)
}

function countOf(code) {
  return list.value.filter((f) => f.section === code).length
}

// —— 跨分区搜索 ——
const query = ref('')
const searching = computed(() => query.value.trim() !== '')

// 名称与 key 都能搜:运营记得住的是名称,排查问题时手上拿到的往往是 key。
const matched = computed(() => {
  if (!searching.value) return list.value.filter((f) => f.section === activeSection.value)
  const q = query.value.trim().toLowerCase()
  return list.value.filter(
    (f) =>
      (f.label || '').toLowerCase().includes(q) ||
      (f.key || '').toLowerCase().includes(q) ||
      (f.group_label || '').toLowerCase().includes(q),
  )
})

const hitCount = computed(() => matched.value.length)

// 搜索时强制展开:搜出结果还要再点一次才能看见,等于没搜。
function isOpen(code) {
  return searching.value || !!expanded.value[code]
}

// 产出 [{ code, label, sectionLabel, fields }]。
// code 是稳定标识(前端逻辑用),label 是服务端按语言解析好的显示名。
// **不要**拿 label 做判断 —— 它会随语言变化。
const grouped = computed(() => {
  const order = []
  const byCode = new Map()
  for (const f of matched.value) {
    if (!byCode.has(f.group)) {
      byCode.set(f.group, {
        code: f.group,
        label: f.group_label || f.group,
        sectionLabel: f.section_label || f.section,
        fields: [],
      })
      order.push(f.group)
    }
    byCode.get(f.group).fields.push(f)
  }
  return order.map((c) => byCode.get(c))
})

onMounted(load)

// 配置项标签由服务端按 Accept-Language 下发,切语言要重取。
// 只有这一页依赖服务端文案,所以 watch 写在这里而不是做成全局刷新。
watch(locale, () => { load() })

async function load() {
  try {
    const d = await api.getConfig()
    list.value = d.list || []
    sections.value = d.sections || []
    savedValues.value = Object.fromEntries(list.value.map((f) => [f.key, f.value]))
  } catch (e) { flash(e.message, true) }
}

// 服务端当前值快照,用来判断这次 blur 到底改没改。
// 原来那个「防重复保存」的守卫是个空语句块,等于每次失焦都写一遍。
const savedValues = ref({})

async function save(f, value, opts = {}) {
  const clear = opts.clear === true

  if (!clear) {
    // 机密项的输入框永远是空的(值不下发),空提交只可能是划过去触发的 blur。
    // 放行的话就会把真密钥冲掉 —— 服务端也挡了一道,这里是第一道。
    if (f.secret && String(value).trim() === '') return
    // 没改就别写:blur 即保存,不拦住的话点一下看看也会产生一次写入与一条日志。
    if (!f.secret && String(value) === String(savedValues.value[f.key] ?? '')) return
  }

  // 「全部租户」下编辑写的是全局默认,首次保存前二次确认(本次会话只确认一次)
  if (isGlobal.value && !globalEditConfirmed.value) {
    if (!window.confirm(t('config.confirmGlobalEdit'))) {
      load() // 还原开关/输入框为服务端值
      return
    }
    globalEditConfirmed.value = true
  }
  try {
    await api.setConfig(f.key, value, clear)
    if (f.secret) {
      // 机密项写完不回显,只更新「配没配」并清掉输入框
      f.is_set = !clear
      f.value = ''
    } else {
      f.value = String(value)
      savedValues.value[f.key] = String(value)
    }
    flash(t('config.savedItem', { label: f.label }))
  } catch (e) {
    flash(e.message, true)
    load()
  }
}

// 支付分区顶部的凭证只读引用。
// 复用「租户凭证」的现成接口,不新开 API,也不在这里提供编辑 —— 那会变成
// 两个界面写同一批 app_credentials 行。
const payCreds = ref([])

watch(
  () => [activeSection.value, tenantStore.currentTenantID],
  async ([sec]) => {
    if (sec !== 'pay') return
    try { payCreds.value = (await api.getCredentials()) || [] } catch (e) { payCreds.value = [] }
  },
  { immediate: true },
)

// 机密项唯一的清除途径。写空是「保持不变」,所以清除必须是个明确动作。
async function clearSecret(f) {
  if (!window.confirm(t('config.confirmClearSecret', { label: f.label }))) return
  await save(f, '', { clear: true })
}

function flash(t, bad = false) {
  msg.value = t; msgBad.value = bad
  setTimeout(() => { msg.value = '' }, 2000)
}

// ── 上线模式一键切换 ──────────────────────────
// 审核模式(工具形态):隐社交 tab、显相机 tab、mine 功能项全关(仅留客服)、开覆盖、开内容安全
const REVIEW_MODE = {
  pages_cover_on: '1',
  tab_show_home: '0', tab_show_city: '0', tab_show_expand: '0', tab_show_message: '0',
  tab_show_mine: '1', tab_show_privacy: '1',
  fn_show_verify: '0', fn_show_avatar: '0', fn_show_viewed: '0', fn_show_items: '0',
  fn_show_collection: '0', fn_show_wallet: '0', fn_show_recharge: '0', fn_show_walletlog: '0',
  fn_show_orders: '0', fn_show_blocklist: '0', fn_show_settings: '0', fn_show_moments: '0',
  fn_show_contact: '1',
  mine_complete_tip_on: '0', // 完善资料跑马灯(文案含社交词,审核期隐藏)
  // 业务功能总开关全关(机器人/AI/签到/分享/触达/关联小程序/留存四件套/动态广场/强制认证/iOS充值隐藏)
  robot_enabled: '0', ai_bot_enabled: '0', ai_chat_enabled: '0', outreach_enabled: '0',
  checkin_enabled: '0', share_reward_enabled: '0', link_mp_enabled: '0',
  bottle_trace_enabled: '0', night_bottle_enabled: '0', charm_rank_enabled: '0',
  user_card_enabled: '0', square_enabled: '0',
  ios_recharge_off: '0', verify_required: '0'
}
// 运营模式(社交形态):与审核模式反转——社交 tab/入口/业务功能全开,相机 tab 隐藏,覆盖关;
// 内容安全不在这两套预设里:它已移到「服务商 → 内容安全」页,
// 按租户单选服务商。写这两个键会被服务端白名单拒掉,整个切换报失败。
const LIVE_MODE = {
  pages_cover_on: '0',
  tab_show_home: '1', tab_show_city: '1', tab_show_expand: '1', tab_show_message: '1',
  tab_show_mine: '1', tab_show_privacy: '0',
  fn_show_verify: '1', fn_show_avatar: '1', fn_show_viewed: '1', fn_show_items: '1',
  fn_show_collection: '1', fn_show_wallet: '1', fn_show_recharge: '1', fn_show_walletlog: '1',
  fn_show_orders: '1', fn_show_blocklist: '1', fn_show_settings: '1', fn_show_moments: '1',
  fn_show_contact: '1',
  mine_complete_tip_on: '1',
  robot_enabled: '1', ai_bot_enabled: '1', ai_chat_enabled: '1', outreach_enabled: '1',
  checkin_enabled: '1', share_reward_enabled: '1', link_mp_enabled: '1',
  bottle_trace_enabled: '1', night_bottle_enabled: '1', charm_rank_enabled: '1',
  user_card_enabled: '1', square_enabled: '1',
  ios_recharge_off: '1', verify_required: '1'
}

const switching = ref(false)
const launchConfirm = ref(null) // {toReview, target, lines[]} 自定义确认弹层(window.confirm 可能被浏览器屏蔽,勿用)
const reviewMode = computed(() => {
  const f = list.value.find(x => x.key === 'pages_cover_on')
  return f ? f.value === '1' : false
})

function toggleLaunch() {
  const toReview = !reviewMode.value
  const target = isGlobal.value
    ? t('config.targetGlobal')
    : t('config.targetTenant', { name: tenantName.value })
  const prefix = toReview ? 'config.toReviewLine' : 'config.toLiveLine'
  const lines = [1, 2, 3, 4, 5].map((i) => t(prefix + i))
  launchConfirm.value = { toReview, target, lines }
}

async function doToggleLaunch() {
  const { toReview } = launchConfirm.value
  launchConfirm.value = null
  switching.value = true
  const plan = toReview ? REVIEW_MODE : LIVE_MODE
  let done = 0
  try {
    for (const [k, v] of Object.entries(plan)) {
      await api.setConfig(k, v)
      done++
    }
    flash(t(toReview ? 'config.enteredReview' : 'config.restoredLive', { n: done }))
    await load()
  } catch (e) {
    flash(t('config.switchFailed', { n: done }) + e.message, true)
  } finally {
    switching.value = false
  }
}

// image 类型:选文件 → 上传拿 URL → 保存到该配置项
async function onPickImage(f, e) {
  const file = e.target.files && e.target.files[0]
  e.target.value = '' // 允许连续选同一文件
  if (!file) return
  uploadingKey.value = f.key
  try {
    const r = await api.upload(file)
    if (r && r.url) { await save(f, r.url); f.value = r.url }
  } catch (err) {
    flash(err.message, true)
  } finally {
    uploadingKey.value = ''
  }
}

async function testLLM() {
  llmTesting.value = true
  llmResult.value = null
  try {
    // 提示词跟随界面语言:回复会原样显示在结果弹框里,英文用户该看到看得懂的回复
    const res = await api.testLLM(t('config.llmTestPrompt'))
    llmOk.value = true
    llmResult.value = res.reply || JSON.stringify(res)
  } catch (e) {
    llmOk.value = false
    llmResult.value = e.message
  } finally {
    llmTesting.value = false
  }
}
</script>

<style scoped>
/* 按钮 / 输入 / 对话框 / toast 统一来自 styles/base.css。
   这一页原本有三处渐变（两个 launch-panel 状态底 + 按钮），
   规范 §9「不做渐变」，改为语义色的 bg + line 两件套。 */

/* 作用范围提示条。「全局」是需要留意的状态（改动会影响所有租户）→ warn；
   「当前租户」只是陈述事实，不该抢注意力 → 中性（S-10 信息态不占色相）。 */
.scope {
  padding: var(--s-3) var(--s-4);
  margin-bottom: var(--s-4);
  border-radius: var(--radius-sm);
  font-size: var(--t-value);
  line-height: 1.6;
}

.scope-global {
  border: 1px solid var(--warn-line);
  background: var(--warn-bg);
  color: var(--warn);
}

.scope-tenant {
  border: 1px solid var(--line);
  background: var(--panel-3);
  color: var(--ink-2);
}

.launch-panel {
  display: flex;
  align-items: center;
  gap: var(--s-4);
  padding: var(--s-4) var(--s-5);
  margin-bottom: var(--s-4);
  border: 1px solid var(--line);
  border-radius: var(--radius);
  background: var(--panel);
}

/* 运营模式=正常运行，审核模式=刻意压制功能，后者才需要被一眼看到 */
.launch-panel.live { border-color: var(--ok-line); background: var(--ok-bg); }
.launch-panel.reviewing { border-color: var(--warn-line); background: var(--warn-bg); }

.lp-info { flex: 1; min-width: 0; }
.lp-title { font-size: var(--t-h2); font-weight: 600; }
.lp-desc { margin-top: var(--s-1); font-size: var(--t-label); color: var(--ink-2); }

.lp-btn {
  height: var(--d-ctl-h);
  padding: 0 var(--s-4);
  border: 1px solid var(--brand);
  border-radius: var(--radius-sm);
  background: var(--brand);
  color: var(--brand-on);
  font-size: var(--t-value);
  font-weight: 500;
  white-space: nowrap;
  transition:
    background-color var(--dur-1) var(--ease),
    border-color var(--dur-1) var(--ease),
    color var(--dur-1) var(--ease);
}

.lp-btn:hover:not(:disabled) { background: var(--brand-hover); border-color: var(--brand-hover); }
.lp-btn:active:not(:disabled) { background: var(--brand-press); border-color: var(--brand-press); }

/* 切回审核模式会关掉一批用户可见功能，是危险动作：描边不实心 */
.lp-btn.danger {
  background: var(--panel);
  border-color: var(--line-strong);
  color: var(--danger);
}

.lp-btn.danger:hover:not(:disabled) {
  background: var(--danger-bg);
  border-color: var(--danger-line);
}

.lp-btn:disabled {
  background: var(--panel-3);
  border-color: var(--line);
  color: var(--ink-4);
  cursor: not-allowed;
}

/* 搜索框。放在 Tab 之上 —— 它跨分区,不属于任何一个 Tab */
.search {
  display: flex;
  align-items: center;
  gap: var(--s-2);
  margin-bottom: var(--s-3);
}

.search-ipt { flex: 1; max-width: 420px; }

/* 命中横幅:Tab 选中态在搜索时熄灭,这里解释为什么 */
.hits {
  margin-bottom: var(--s-3);
  padding: var(--s-2) var(--s-3);
  border-radius: var(--radius-sm);
  background: var(--brand-weak);
  color: var(--brand-text);
  font-size: var(--t-label);
}

/* 机密项:状态徽章 + 输入框 + 清除,横排 */
.secret {
  display: flex;
  align-items: center;
  gap: var(--s-2);
  justify-content: flex-end;
}

.secret .ipt { flex: 1; min-width: 0; }
.secret .ipt-area { text-align: left; }
.secret .badge { flex: none; }

/* 支付分区的凭证只读引用条 */
.cred-ref {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: var(--s-3);
  margin-bottom: var(--s-3);
  padding: var(--s-2) var(--s-3);
  border: 1px solid var(--line);
  border-radius: var(--radius-sm);
  background: var(--panel-2);
  font-size: var(--t-label);
}

.cred-item { display: flex; align-items: center; gap: var(--s-2); }
.cred-link { margin-left: auto; }

/* 搜索结果里的分区标记 */
.gt-sec { color: var(--ink-3); font-weight: 400; }

/* 分区 Tab。横向可滚,窄屏下不换行挤成两层 */
.tabs {
  display: flex;
  gap: var(--s-1);
  margin-bottom: var(--s-4);
  padding-bottom: var(--s-1);
  overflow-x: auto;
  border-bottom: 1px solid var(--line);
}

.tabs button {
  display: flex;
  align-items: center;
  gap: var(--s-2);
  flex: none;
  padding: var(--s-2) var(--s-3);
  border-radius: var(--radius-sm) var(--radius-sm) 0 0;
  font-size: var(--t-value);
  color: var(--ink-2);
  white-space: nowrap;
  border-bottom: 2px solid transparent;
  margin-bottom: -5px;
  transition:
    color var(--dur-1) var(--ease),
    border-color var(--dur-1) var(--ease);
}

.tabs button:hover { color: var(--ink); }
.tabs button.on { color: var(--brand-text); border-bottom-color: var(--brand); font-weight: 650; }

.tab-n {
  font-size: var(--t-tag);
  color: var(--ink-3);
  font-variant-numeric: tabular-nums;
}

.tabs button.on .tab-n { color: var(--brand-text); }

/* 配置分组 */
.block {
  margin-bottom: var(--s-3);
  overflow: hidden;
  background: var(--panel);
  border: 1px solid var(--line);
  border-radius: var(--radius);
  box-shadow: var(--shadow-1);
}

.gt {
  padding: var(--s-3) var(--s-5);
  background: var(--panel-2);
  border-bottom: 1px solid var(--line);
  font-size: var(--t-h2);
  font-weight: 600;
}

.block.collapsed .gt { border-bottom: none; }

.gt-toggle {
  display: flex;
  align-items: center;
  gap: var(--s-2);
  cursor: pointer;
  user-select: none;
  transition: background-color var(--dur-1) var(--ease);
}

.gt-toggle:hover { background: var(--panel-3); }

.gt-arrow {
  font-size: var(--t-value);
  color: var(--ink-3);
  transition: transform var(--dur-1) var(--ease);
}

.gt-arrow.open { transform: rotate(90deg); }
.gt-count { margin-left: auto; font-size: var(--t-label); font-weight: 400; color: var(--ink-3); }

/* 两列 grid 而不是 flex + space-between:
   标签列自适应、控件列有下限,控件宽度才不受标签长短牵连。 */
.row {
  display: grid;
  grid-template-columns: minmax(220px, 1fr) minmax(260px, 440px);
  align-items: center;
  gap: var(--s-4);
  padding: var(--d-pad-y) var(--s-5);
  border-bottom: 1px solid var(--line-2);
}

.row:last-child { border-bottom: none; }
.name { font-weight: 600; }
.key { margin-top: var(--s-1); font-size: var(--t-label); color: var(--ink-3); }

/* 控件列 260–440px。原来是死的 200px + 右对齐 ——
   URL、提示词、模板 ID 这类值装不下,而右对齐让你看到的是尾巴不是开头。 */
.ctl { justify-self: end; width: 100%; max-width: 440px; text-align: right; }
.ipt { width: 100%; text-align: left; }
/* 数字右对齐才好上下比较 */
.ipt[type="number"] { text-align: right; font-variant-numeric: tabular-nums; }
.ipt-area { width: 100%; height: auto; padding: var(--s-2) var(--s-3); text-align: left; resize: vertical; }
.row-area { grid-template-columns: 1fr; }
.ctl-area { max-width: none; margin-top: var(--s-2); text-align: left; }

.switch { display: inline-flex; align-items: center; gap: var(--s-2); cursor: pointer; }
.switch input { width: 20px; height: 20px; accent-color: var(--brand); }

.img-ctl { display: flex; align-items: center; gap: var(--s-3); justify-content: flex-end; }

.img-preview {
  width: 56px;
  height: 56px;
  object-fit: cover;
  border: 1px solid var(--line);
  border-radius: var(--radius-sm);
  cursor: zoom-in;
}

.img-big { max-width: 86vw; max-height: 86vh; border-radius: var(--radius); }

/* 保存成功提示。失败时 .bad 切到 danger 一套 */
.toast {
  position: fixed;
  right: var(--s-6);
  bottom: var(--s-6);
  z-index: 200;
  margin: 0;
  padding: var(--s-3) var(--s-5);
  border: 1px solid var(--ok-line);
  border-radius: var(--radius-sm);
  background: var(--ok-bg);
  color: var(--ok);
  box-shadow: var(--shadow-2);
  font-size: var(--t-value);
}

.toast.bad {
  border-color: var(--danger-line);
  background: var(--danger-bg);
  color: var(--danger);
}

.llm-test-row { background: var(--panel-2); }

.dlg-hd.ok { color: var(--ok); }
.dlg-hd.bad { color: var(--danger); }

.dlg-body {
  margin-bottom: var(--s-4);
  padding: var(--s-3) var(--s-4);
  border-radius: var(--radius-sm);
  background: var(--panel-3);
  color: var(--ink);
  font-size: var(--t-value);
  line-height: 1.7;
  white-space: pre-wrap;
  word-break: break-all;
}

.lc-target { margin-bottom: var(--s-2); font-weight: 600; }
.lc-lines { margin: 0; padding-left: var(--s-4); list-style: disc; }
.lc-lines li { margin: var(--s-1) 0; }
.lc-btns { display: flex; justify-content: flex-end; gap: var(--s-2); }

/* 移动端:每行标签/控件竖排,输入框占满 */
@media (max-width: 768px) {
  .row { flex-direction: column; align-items: flex-start; gap: var(--s-2); }
  .ctl { width: 100%; min-width: 0; text-align: left; }
  .ipt { width: 100%; text-align: left; }
  .img-ctl { justify-content: flex-start; }
}
</style>
