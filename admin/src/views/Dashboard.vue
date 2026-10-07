<template>
  <div>
    <!-- 总量 -->
    <div class="grid">
      <div v-for="c in cards" :key="c.key" class="stat">
        <div class="v">{{ c.fmt ? c.fmt(stats[c.key]) : (stats[c.key] ?? '—') }}</div>
        <div class="l">{{ c.label }}</div>
      </div>
    </div>

    <div class="detail-bar">
      <button class="btn sm" @click="openOrders">{{ t('dashboard.ordersDetailBtn') }}</button>
    </div>

    <!-- 当期 KPI:今日/本周/本月/本年 -->
    <h3 class="sec">{{ t('dashboard.secPeriod') }}</h3>
    <div class="grid">
      <div v-for="k in kpiCards" :key="k.key" class="stat kpi">
        <div class="kpi-title">{{ k.label }}</div>
        <div class="kpi-rows">
          <div class="kpi-row"><span class="p">{{ t('dashboard.today') }}</span><span class="n">{{ k.fmt(period(k.key).today) }}</span></div>
          <div class="kpi-row"><span class="p">{{ t('dashboard.week') }}</span><span class="n">{{ k.fmt(period(k.key).week) }}</span></div>
          <div class="kpi-row"><span class="p">{{ t('dashboard.month') }}</span><span class="n">{{ k.fmt(period(k.key).month) }}</span></div>
          <div class="kpi-row"><span class="p">{{ t('dashboard.year') }}</span><span class="n">{{ k.fmt(period(k.key).year) }}</span></div>
        </div>
      </div>
    </div>

    <!-- 近 30 天趋势 -->
    <h3 class="sec">{{ t('dashboard.secTrend') }}</h3>
    <div class="charts">
      <div class="card"><div class="card-t">{{ t('dashboard.chartNewUsers') }}</div><Chart :option="newUsersOpt" /></div>
      <div class="card"><div class="card-t">{{ t('dashboard.chartRecharge') }}</div><Chart :option="rechargeOpt" /></div>
      <div class="card"><div class="card-t">{{ t('dashboard.chartConsume') }}</div><Chart :option="consumeOpt" /></div>
    </div>

    <p class="hint">{{ t('dashboard.hint') }}</p>

    <!-- 充值订单明细弹框 -->
    <div v-if="ordersModal" class="modal-mask" @click.self="ordersModal = false">
      <div class="modal">
        <div class="modal-hd">{{ t('dashboard.ordersModalTitle', { n: orders.length }) }}</div>
        <div class="modal-body">
          <table class="otbl">
            <thead>
              <tr>
                <th>{{ t('dashboard.colOrderNo') }}</th>
                <th>{{ t('common.colUser') }}</th>
                <th>{{ t('dashboard.colAmount') }}</th>
                <th>{{ t('dashboard.colCoins') }}</th>
                <th>{{ t('dashboard.colPaidAt') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="o in orders" :key="o.order_no">
                <td class="mono">{{ o.order_no }}</td>
                <td>{{ o.nickname || t('common.dash') }}<div class="uid">ID: {{ o.user_id }}</div></td>
                <td class="money">¥{{ (o.price_fen / 100).toFixed(2) }}</td>
                <td>{{ o.coins }} 🪙</td>
                <td>{{ o.paid_at ? String(o.paid_at).slice(0,16).replace('T',' ') : t('common.dash') }}</td>
              </tr>
              <tr v-if="!orders.length"><td colspan="5" class="empty">{{ t('dashboard.emptyOrders') }}</td></tr>
            </tbody>
          </table>
        </div>
        <div class="modal-ft"><button class="btn ghost sm" @click="ordersModal = false">{{ t('common.close') }}</button></div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api'
import Chart from '../components/Chart.vue'

const { t, locale } = useI18n()

// 千分位跟随当前语言。¥ 是币种不是语言,保持不变。
const fmtInt = (v) => Number(v || 0).toLocaleString(locale.value)
const fmtYuan = (v) =>
  '¥' + Number(v || 0).toLocaleString(locale.value, { minimumFractionDigits: 2, maximumFractionDigits: 2 })

const stats = ref({})

// cards / kpiCards 用 computed —— 常量存翻译后的字符串不会响应语言切换。
const cards = computed(() => [
  { key: 'users', label: t('dashboard.cardUsers') },
  { key: 'bottles', label: t('dashboard.cardBottles') },
  { key: 'bottles_today', label: t('dashboard.cardBottlesToday') },
  { key: 'replies', label: t('dashboard.cardReplies') },
  { key: 'chats', label: t('dashboard.cardChats') },
  { key: 'orders', label: t('dashboard.cardOrders') },
  { key: 'recharge_yuan', label: t('dashboard.cardRechargeYuan'), fmt: fmtYuan },
  { key: 'robots', label: t('dashboard.cardRobots') }
])

const kpiCards = computed(() => [
  { key: 'new_users', label: t('dashboard.kpiNewUsers'), fmt: fmtInt },
  { key: 'active_users', label: t('dashboard.kpiActiveUsers'), fmt: fmtInt },
  { key: 'recharge_yuan', label: t('dashboard.kpiRechargeYuan'), fmt: fmtYuan },
  { key: 'recharge_orders', label: t('dashboard.kpiRechargeOrders'), fmt: fmtInt },
  { key: 'consume_coins', label: t('dashboard.kpiConsumeCoins'), fmt: fmtInt },
  { key: 'paying_users', label: t('dashboard.kpiPayingUsers'), fmt: fmtInt },
  { key: 'arpu_yuan', label: t('dashboard.kpiArpu'), fmt: fmtYuan }
])

const empty = { today: 0, week: 0, month: 0, year: 0 }
const kpi = ref({})
const trend = ref({ dates: [], new_users: [], recharge_yuan: [], consume_coins: [] })

const period = (key) => kpi.value[key] || empty

/**
 * 图表配色从 CSS token 实时读取，不写死。
 *
 * 写死的后果不是「颜色不统一」，是**切到深色主题时坐标轴还是浅色值** ——
 * 深底上的浅灰字基本看不见，而 JS 里的颜色不会跟着 CSS 变量走。
 *
 * 取色遵循规范 Q-3：用三个语义色 + 中性，不引序列色（现在也确实只有三条线）。
 */
const themeTick = ref(0)

function token(name) {
  return getComputedStyle(document.documentElement).getPropertyValue(name).trim()
}

const chartColors = computed(() => {
  themeTick.value // 依赖：主题一变就重算
  return {
    axis: token('--ink-3'),
    split: token('--line-2'),
    series: [token('--brand'), token('--ok'), token('--warn')],
  }
})

let themeObserver = null
let mediaQuery = null
const onThemeChange = () => { themeTick.value++ }

onMounted(() => {
  // 显式切换主题会改 <html data-theme>；跟随系统时则要听 media query。
  themeObserver = new MutationObserver(onThemeChange)
  themeObserver.observe(document.documentElement, { attributeFilter: ['data-theme'] })
  mediaQuery = window.matchMedia('(prefers-color-scheme: dark)')
  mediaQuery.addEventListener('change', onThemeChange)
})

onBeforeUnmount(() => {
  themeObserver?.disconnect()
  mediaQuery?.removeEventListener('change', onThemeChange)
})

function lineOption(name, data, color, yuan) {
  const fmt = yuan ? fmtYuan : fmtInt
  const c = chartColors.value
  return {
    tooltip: { trigger: 'axis', valueFormatter: (v) => fmt(v) },
    grid: { left: 52, right: 16, top: 20, bottom: 28 },
    xAxis: { type: 'category', data: trend.value.dates, boundaryGap: false, axisLabel: { fontSize: 11, color: c.axis } },
    yAxis: { type: 'value', axisLabel: { fontSize: 11, color: c.axis }, splitLine: { lineStyle: { color: c.split } } },
    series: [{
      name, type: 'line', smooth: true, showSymbol: false,
      data, lineStyle: { width: 2, color }, itemStyle: { color },
      areaStyle: { color, opacity: 0.08 }
    }]
  }
}

// option 是 computed,locale 变化会连带重算 —— 图例与 tooltip 跟着切换,无需手动 setOption
const newUsersOpt = computed(() => lineOption(t('dashboard.seriesNewUsers'), trend.value.new_users, chartColors.value.series[0], false))
const rechargeOpt = computed(() => lineOption(t('dashboard.seriesRecharge'), trend.value.recharge_yuan, chartColors.value.series[1], true))
const consumeOpt = computed(() => lineOption(t('dashboard.seriesConsume'), trend.value.consume_coins, chartColors.value.series[2], false))

const ordersModal = ref(false)
const orders = ref([])
async function openOrders() {
  ordersModal.value = true
  try {
    const res = await api.listPayOrders({ status: 'paid', page: 1, size: 50 })
    orders.value = res.list || []
  } catch (e) { orders.value = [] }
}

onMounted(async () => {
  try { stats.value = await api.stats() } catch (e) {}
  try {
    const o = await api.statsOverview()
    if (o) { kpi.value = o.kpi || {}; trend.value = o.trend || trend.value }
  } catch (e) {}
})
</script>

<style scoped>
.grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: var(--d-gap);
}

.stat {
  padding: var(--s-6);
  background: var(--panel);
  border: 1px solid var(--line);
  border-radius: var(--radius);
  box-shadow: var(--shadow-1);
}

/* 数值不着色。品牌色只用于「可以点」——统计数字不可点，
   染成蓝色会让蓝色不再意味着可交互。强调交给字号 + 字重 + 等宽。 */
.v {
  font-size: 28px;
  font-weight: 650;
  color: var(--ink);
  font-family: var(--f-mono);
  font-variant-numeric: tabular-nums;
  line-height: 1.2;
}

.l { margin-top: var(--s-2); color: var(--ink-2); font-size: var(--t-label); }

.sec {
  margin: var(--s-7) 0 var(--s-3);
  font-size: var(--t-h1);
  font-weight: 600;
  color: var(--ink);
}

.kpi { padding: var(--s-4) var(--s-5); }
.kpi-title { margin-bottom: var(--s-2); color: var(--ink-2); font-size: var(--t-label); }
.kpi-rows { display: flex; flex-direction: column; gap: var(--s-1); }
.kpi-row { display: flex; justify-content: space-between; align-items: baseline; gap: var(--s-3); }
.kpi-row .p { color: var(--ink-2); font-size: var(--t-value); }

.kpi-row .n {
  font-size: var(--t-money);
  font-weight: 600;
  color: var(--ink);
  font-family: var(--f-mono);
  font-variant-numeric: tabular-nums;
}

.charts {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(min(360px, 100%), 1fr));
  gap: var(--d-gap);
}

.card { padding: var(--s-4) var(--s-4) var(--s-2); }
.card-t { margin-bottom: var(--s-1); font-size: var(--t-h2); font-weight: 600; }
.hint { margin-top: var(--s-6); }

.detail-bar { margin-top: var(--s-4); }

/* 弹层（这一页用的是自己的 modal-* 命名，不是全局的 .dlg） */
.modal-mask {
  position: fixed;
  inset: 0;
  z-index: 100;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: var(--s-4);
  background: var(--scrim);
}

.modal {
  display: flex;
  flex-direction: column;
  width: 720px;
  max-width: 94vw;
  max-height: 82vh;
  background: var(--panel);
  border: 1px solid var(--line);
  border-radius: var(--radius);
  box-shadow: var(--shadow-2);
}

.modal-hd {
  padding: var(--s-4) var(--s-5);
  border-bottom: 1px solid var(--line-2);
  font-size: var(--t-h1);
  font-weight: 600;
}

.modal-body { padding: var(--s-2) var(--s-5); overflow: auto; }

.modal-ft {
  display: flex;
  justify-content: flex-end;
  padding: var(--s-3) var(--s-5);
  border-top: 1px solid var(--line-2);
}

.otbl { min-width: 520px; }
.otbl .uid { font-size: var(--t-tag); color: var(--ink-3); }

/* 金额不着色（S-7） */
.otbl .money {
  font-family: var(--f-mono);
  font-variant-numeric: tabular-nums;
  font-weight: 600;
  color: var(--money);
}
</style>
