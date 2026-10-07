<template>
  <div>
    <div class="toolbar">
      <input v-model="toFilter" class="inp" :placeholder="t('emailLogs.toPlaceholder')" @keyup.enter="doSearch" />
      <select v-model="statusFilter" class="sel" @change="doSearch">
        <option value="">{{ t('common.allStatus') }}</option>
        <option value="sent">{{ t('emailLogs.statusSent') }}</option>
        <option value="failed">{{ t('emailLogs.statusFailed') }}</option>
        <option value="skipped">{{ t('emailLogs.statusSkippedOpt') }}</option>
      </select>
      <select v-model="purposeFilter" class="sel" @change="doSearch">
        <option value="">{{ t('emailLogs.allPurposes') }}</option>
        <option v-for="p in PURPOSES" :key="p" :value="p">{{ purposeLabel(p) }}</option>
      </select>
      <button class="btn" @click="doSearch">{{ t('emailLogs.refresh') }}</button>
      <span class="total">{{ t('common.total', { n: total }) }}</span>
    </div>

    <div class="warn">{{ t('emailLogs.warning') }}</div>

    <div class="table-wrap">
      <table class="tbl">
        <thead>
          <tr>
            <th>{{ t('common.colTime') }}</th>
            <th>{{ t('emailLogs.colTo') }}</th>
            <th>{{ t('emailLogs.colPurpose') }}</th>
            <th>{{ t('emailLogs.colCode') }}</th>
            <th>{{ t('emailLogs.colSubject') }}</th>
            <th>{{ t('common.status') }}</th>
            <th>{{ t('emailLogs.colError') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="l in list" :key="l.id">
            <td class="soft nowrap">{{ shortTime(l.created_at) }}</td>
            <td class="nowrap">{{ l.to }}</td>
            <td><span class="badge" :class="purposeClass(l.purpose)">{{ purposeLabel(l.purpose) }}</span></td>
            <td>
              <code class="code" @click="copy(l.code)" :title="t('emailLogs.clickToCopy')">{{ l.code || t('common.dash') }}</code>
            </td>
            <td class="soft">{{ l.subject || t('common.dash') }}</td>
            <td><span class="badge" :class="statusClass(l.status)">{{ statusLabel(l.status) }}</span></td>
            <td class="detail-cell soft">{{ l.error || t('common.dash') }}</td>
          </tr>
          <tr v-if="!list.length && !loading">
            <td colspan="7" class="empty">{{ t('emailLogs.empty') }}</td>
          </tr>
        </tbody>
      </table>
    </div>

    <div class="pager">
      <button class="btn ghost sm" :disabled="page <= 1" @click="goPage(page - 1)">{{ t('common.prev') }}</button>
      <span class="pg">{{ t('emailLogs.pageInfo', { page, pages: totalPages }) }}</span>
      <button class="btn ghost sm" :disabled="page >= totalPages" @click="goPage(page + 1)">{{ t('common.next') }}</button>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api'

const { t } = useI18n()

// purpose / status 的**值**存库,不翻;只翻显示名
const PURPOSES = ['register', 'reset', 'login']

const list = ref([])
const total = ref(0)
const page = ref(1)
const size = ref(50)
const loading = ref(false)
const toFilter = ref('')
const statusFilter = ref('')
const purposeFilter = ref('')
const totalPages = computed(() => Math.max(1, Math.ceil(total.value / size.value)))

onMounted(load)

async function load() {
  loading.value = true
  try {
    const res = await api.listEmailLogs({
      page: page.value, size: size.value,
      to: toFilter.value, status: statusFilter.value, purpose: purposeFilter.value,
    })
    list.value = res.list || []
    total.value = res.total || 0
  } catch (e) {}
  loading.value = false
}
function doSearch() { page.value = 1; load() }
function goPage(p) { page.value = p; load() }

// 验证码点一下就复制：客服场景下手抄六位数字最容易出错
async function copy(code) {
  if (!code) return
  try { await navigator.clipboard.writeText(code) } catch (e) {}
}

function purposeLabel(p) { return PURPOSES.includes(p) ? t('emailLogs.purpose_' + p) : (p || '—') }
function purposeClass(p) {
  return { register: 'blue', reset: 'orange', login: 'cyan' }[p] || 'gray'
}

function statusLabel(s) {
  const key = { sent: 'statusSent', failed: 'statusFailed', skipped: 'statusSkipped' }[s]
  return key ? t('emailLogs.' + key) : (s || '—')
}
function statusClass(s) {
  return { sent: 'green', failed: 'red', skipped: 'gray' }[s] || 'gray'
}
function shortTime(ts) { return ts ? String(ts).slice(5, 19).replace('T', ' ') : '—' }
</script>

<style scoped>
/* 组件样式统一来自 styles/base.css，这里只留本页特有的部分 */
.inp { min-width: 200px; }

/* 「这张表存明文验证码」的告警条 */
.warn {
  margin-bottom: var(--s-4);
  padding: var(--s-3) var(--s-4);
  border: 1px solid var(--warn-line);
  border-radius: var(--radius-sm);
  background: var(--warn-bg);
  color: var(--warn);
  font-size: var(--t-value);
  line-height: 1.6;
}

.tbl { min-width: 900px; }
.tbl td { vertical-align: top; }
.nowrap { white-space: nowrap; }
.detail-cell { max-width: 300px; word-break: break-all; }

/* 验证码：可整段选中复制。用中性等宽块而不是蓝底蓝字 ——
   品牌蓝是「可以点」的唯一信号，用在展示型内容上会稀释它。 */
.code {
  display: inline-block;
  padding: 2px var(--s-2);
  border: 1px solid var(--line);
  border-radius: var(--radius-xs);
  background: var(--panel-3);
  color: var(--ink);
  font-family: var(--f-mono);
  font-variant-numeric: tabular-nums;
  font-size: var(--t-body);
  font-weight: 600;
  letter-spacing: 2px;
  cursor: pointer;
  user-select: all;
  transition: background-color var(--dur-1) var(--ease);
}

.code:hover { background: var(--panel-2); }
</style>
