<template>
  <div>
    <div class="toolbar">
      <input v-model="userKw" class="ipt" :placeholder="t('common.userKwPlaceholder')" @keydown.enter="doSearch" />
      <select v-model="statusFilter" class="sel">
        <option value="">{{ t('common.allStatus') }}</option>
        <option value="pending">{{ t('orders.statusPending') }}</option>
        <option value="paid">{{ t('orders.statusPaid') }}</option>
        <option value="failed">{{ t('orders.statusFailed') }}</option>
        <option value="refunded">{{ t('orders.statusRefunded') }}</option>
      </select>
      <button class="btn" @click="doSearch">{{ t('common.filter') }}</button>
      <button class="btn ghost" @click="reset">{{ t('common.reset') }}</button>
      <span class="total">{{ t('common.total', { n: total }) }}</span>
    </div>

    <div class="table-wrap">
      <table class="tbl">
        <thead>
          <tr>
            <th>{{ t('orders.colOrderNo') }}</th>
            <th>{{ t('common.colUser') }}</th>
            <th>{{ t('orders.colPlatform') }}</th>
            <th>{{ t('orders.colAmount') }}</th>
            <th>{{ t('orders.colCoins') }}</th>
            <th>{{ t('common.status') }}</th>
            <th>{{ t('orders.colTxnId') }}</th>
            <th>{{ t('common.createdAt') }}</th>
            <th>{{ t('orders.colPaidAt') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="o in list" :key="o.order_no">
            <td class="mono">{{ o.order_no }}</td>
            <td>
              <div class="uid-cell">
                <div>{{ o.nickname || t('common.dash') }}</div>
                <div class="uid">ID: {{ o.user_id }}</div>
              </div>
            </td>
            <td>
              <span class="badge" :class="o.platform === 'wx' ? 'green' : 'blue'">
                {{ o.platform === 'wx' ? t('orders.platformWx') : o.platform === 'alipay' ? t('orders.platformAlipay') : o.platform }}
              </span>
            </td>
            <td class="money">¥{{ (o.price_fen / 100).toFixed(2) }}</td>
            <td class="coins">{{ o.coins }} 🪙</td>
            <td>
              <span class="badge" :class="statusClass(o.status)">{{ statusLabel(o.status) }}</span>
            </td>
            <td class="mono small">{{ o.platform_txn_id || t('common.dash') }}</td>
            <td>{{ shortTime(o.created_at) }}</td>
            <td>{{ o.paid_at ? shortTime(o.paid_at) : t('common.dash') }}</td>
          </tr>
          <tr v-if="!list.length && !loading">
            <td colspan="9" class="empty">{{ t('orders.empty') }}</td>
          </tr>
        </tbody>
      </table>
    </div>

    <div class="pager">
      <button class="btn ghost sm" :disabled="page <= 1" @click="goPage(page - 1)">{{ t('common.prev') }}</button>
      <span class="pg">{{ t('common.pageInfo', { page, pages: totalPages, total }) }}</span>
      <button class="btn ghost sm" :disabled="page >= totalPages" @click="goPage(page + 1)">{{ t('common.next') }}</button>
      <select v-model="size" @change="doSearch" class="sel-pg">
        <option :value="20">{{ t('common.perPage', { n: 20 }) }}</option>
        <option :value="50">{{ t('common.perPage', { n: 50 }) }}</option>
        <option :value="100">{{ t('common.perPage', { n: 100 }) }}</option>
      </select>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api'

const { t } = useI18n()
const list = ref([])
const total = ref(0)
const page = ref(1)
const size = ref(20)
const loading = ref(false)
const statusFilter = ref('paid') // 默认显示已支付
const userKw = ref('')
const totalPages = computed(() => Math.max(1, Math.ceil(total.value / size.value)))

onMounted(() => load())

async function load() {
  loading.value = true
  try {
    const res = await api.listPayOrders({ page: page.value, size: size.value, status: statusFilter.value, user: userKw.value })
    list.value = res.list || []
    total.value = res.total || 0
  } catch (e) {}
  loading.value = false
}

function doSearch() { page.value = 1; load() }
function reset() { statusFilter.value = 'paid'; userKw.value = ''; doSearch() }
function goPage(p) { page.value = p; load() }

// status 的**值**是枚举,不翻;只翻显示名
function statusLabel(s) {
  const key = { pending: 'statusPending', paid: 'statusPaid', failed: 'statusFailed', refunded: 'statusRefunded' }[s]
  return key ? t('orders.' + key) : s
}
function statusClass(s) {
  return { pending: 'orange', paid: 'green', failed: 'red', refunded: 'gray' }[s] || 'gray'
}
function shortTime(ts) { return ts ? String(ts).slice(0, 16).replace('T', ' ') : '—' }
</script>

<style scoped>
/* 组件样式统一来自 styles/base.css，这里只留本页特有的部分 */
.ipt { width: 180px; }

.uid-cell div { line-height: 1.4; }
.uid { font-size: var(--t-tag); color: var(--ink-3); }
.small { font-size: var(--t-tag); }

/* 金额不着色（S-7）：原本实付金额是红色，而红在这套系统里只表示「失败」。
   订单状态已有徽章承载，金额靠等宽 + 字重站住就够了。 */
.money,
.coins {
  font-family: var(--f-mono);
  font-variant-numeric: tabular-nums;
  font-weight: 600;
  color: var(--money);
  white-space: nowrap;
}
</style>
