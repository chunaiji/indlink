<template>
  <div>
    <div class="toolbar">
      <input v-model="userKw" class="ipt" :placeholder="t('common.userKwPlaceholder')" @keydown.enter="doSearch" />
      <select v-model="directionFilter" class="sel">
        <option value="">{{ t('walletTxns.allDirections') }}</option>
        <option value="credit">{{ t('walletTxns.dirCredit') }}</option>
        <option value="debit">{{ t('walletTxns.dirDebit') }}</option>
      </select>
      <select v-model="sceneFilter" class="sel">
        <option value="">{{ t('walletTxns.allScenes') }}</option>
        <option v-for="s in SCENES" :key="s" :value="s">{{ sceneLabel(s) }}</option>
      </select>
      <input v-model="startDate" type="date" class="ipt date" @change="doSearch" />
      <span class="date-sep">{{ t('walletTxns.dateSep') }}</span>
      <input v-model="endDate" type="date" class="ipt date" @change="doSearch" />
      <button class="btn" @click="doSearch">{{ t('common.filter') }}</button>
      <button class="btn ghost" @click="reset">{{ t('common.reset') }}</button>
      <span class="total">{{ t('common.total', { n: total }) }}</span>
    </div>

    <div class="table-wrap">
      <table class="tbl">
        <thead>
          <tr>
            <th>{{ t('walletTxns.colTxnId') }}</th>
            <th>{{ t('common.colUser') }}</th>
            <th>{{ t('walletTxns.colDirection') }}</th>
            <th>{{ t('walletTxns.colCoins') }}</th>
            <th>{{ t('walletTxns.colScene') }}</th>
            <th>{{ t('walletTxns.colBizNo') }}</th>
            <th>{{ t('walletTxns.colBalanceAfter') }}</th>
            <th>{{ t('walletTxns.colRemark') }}</th>
            <th>{{ t('common.colTime') }}</th>
          </tr>
        </thead>
        <tbody>
          <!-- 循环变量用 tx,不用 t —— 那是翻译函数 -->
          <tr v-for="tx in list" :key="tx.txn_id">
            <td class="mono small">{{ tx.txn_id }}</td>
            <td>
              <div class="uid-cell">
                <div>{{ tx.nickname || t('common.dash') }}</div>
                <div class="uid">ID: {{ tx.user_id }}</div>
              </div>
            </td>
            <td>
              <span class="badge" :class="tx.direction === 'credit' ? 'green' : 'red'">
                {{ tx.direction === 'credit' ? t('walletTxns.creditBadge') : t('walletTxns.debitBadge') }}
              </span>
            </td>
            <td class="coins" :class="tx.direction === 'credit' ? 'plus' : 'minus'">
              {{ tx.direction === 'credit' ? '+' : '-' }}{{ tx.coins }}
            </td>
            <td>
              <span class="badge gray">{{ sceneLabel(tx.scene) }}</span>
            </td>
            <td class="mono small">{{ tx.biz_no || t('common.dash') }}</td>
            <td class="coins">{{ tx.balance_after }}</td>
            <td class="small">{{ tx.remark || t('common.dash') }}</td>
            <td>{{ shortTime(tx.created_at) }}</td>
          </tr>
          <tr v-if="!list.length && !loading">
            <td colspan="9" class="empty">{{ t('walletTxns.empty') }}</td>
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

// scene 的**值**存库,不翻;只翻显示名
const SCENES = ['recharge', 'chat', 'unlock', 'gift', 'reward', 'checkin', 'share', 'rewind', 'admin']

const list = ref([])
const total = ref(0)
const page = ref(1)
const size = ref(20)
const loading = ref(false)
const directionFilter = ref('')
const sceneFilter = ref('')
const userKw = ref('')
const startDate = ref('')
const endDate = ref('')
const totalPages = computed(() => Math.max(1, Math.ceil(total.value / size.value)))

onMounted(() => load())

async function load() {
  loading.value = true
  try {
    const res = await api.listWalletTxns({ page: page.value, size: size.value, direction: directionFilter.value, scene: sceneFilter.value, user: userKw.value, start: startDate.value, end: endDate.value })
    list.value = res.list || []
    total.value = res.total || 0
  } catch (e) {}
  loading.value = false
}

function doSearch() { page.value = 1; load() }
function reset() { directionFilter.value = ''; sceneFilter.value = ''; userKw.value = ''; startDate.value = ''; endDate.value = ''; doSearch() }
function goPage(p) { page.value = p; load() }

function sceneLabel(s) {
  return SCENES.includes(s) ? t('walletTxns.scene_' + s) : s
}
function shortTime(t) { return t ? String(t).slice(0, 16).replace('T', ' ') : '—' }
</script>

<style scoped>
/* 组件样式（toolbar / ipt / sel / tbl / badge / btn / pager / empty）
   全部来自 styles/base.css，这里只留本页特有的部分 */
.ipt { width: 180px; }
.ipt.date { width: 150px; }
.date-sep { color: var(--ink-3); font-size: var(--t-label); }

.uid-cell div { line-height: 1.4; }
.uid { font-size: var(--t-tag); color: var(--ink-3); }
.small { font-size: var(--t-tag); }

/* 金额不着色（S-7）。方向已经由「▲ 收入 / ▼ 支出」徽章、箭头和正负号
   表达了三遍，再给数字染绿红只会和语义色抢注意力 ——
   一屏里语义色超过三处就没有「异常」了。 */
.coins {
  font-family: var(--f-mono);
  font-variant-numeric: tabular-nums;
  font-weight: 600;
  white-space: nowrap;
}
</style>
