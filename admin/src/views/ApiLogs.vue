<template>
  <div>
    <div class="toolbar">
      <select v-model="kindFilter" class="sel" @change="doSearch">
        <option value="">{{ t('common.allTypes') }}</option>
        <option v-for="k in FILTER_KINDS" :key="k" :value="k">{{ t('apiLogs.type_' + k + '_opt') }}</option>
      </select>
      <button class="btn" @click="doSearch">{{ t('apiLogs.refresh') }}</button>
      <span class="hint">{{ t('apiLogs.hint') }}</span>
      <span class="total">{{ t('common.total', { n: total }) }}</span>
    </div>

    <div class="table-wrap">
      <table class="tbl">
        <thead>
          <tr>
            <th>{{ t('common.colTime') }}</th>
            <th>{{ t('apiLogs.colType') }}</th>
            <th>{{ t('apiLogs.colReq') }}</th>
            <th>{{ t('apiLogs.colCode') }}</th>
            <th>{{ t('apiLogs.colResp') }}</th>
            <th>{{ t('apiLogs.colResult') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="l in list" :key="l.id">
            <td class="soft nowrap">{{ shortTime(l.created_at) }}</td>
            <td><span class="badge" :class="kindClass(l.kind)">{{ kindLabel(l.kind) }}</span></td>
            <td class="detail-cell">{{ l.detail }}</td>
            <td class="soft">{{ l.resp_code }}</td>
            <td class="detail-cell soft">{{ l.resp_body || t('common.dash') }}</td>
            <td><span class="badge" :class="l.ok ? 'green' : 'red'">{{ l.ok ? t('apiLogs.ok') : t('apiLogs.fail') }}</span></td>
          </tr>
          <tr v-if="!list.length && !loading">
            <td colspan="6" class="empty">{{ t('apiLogs.empty') }}</td>
          </tr>
        </tbody>
      </table>
    </div>

    <div class="pager">
      <button class="btn ghost sm" :disabled="page <= 1" @click="goPage(page - 1)">{{ t('common.prev') }}</button>
      <span class="pg">{{ t('apiLogs.pageInfo', { page, pages: totalPages }) }}</span>
      <button class="btn ghost sm" :disabled="page >= totalPages" @click="goPage(page + 1)">{{ t('common.next') }}</button>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api'

const { t } = useI18n()

// kind 的**值**存库,不翻;只翻显示名。
// 下拉只列可筛选的几种,access_token 仅在表格里出现。
const FILTER_KINDS = ['msg_sec_check', 'media_check_async', 'media_check_callback', 'subscribe_send', 'geo_regeo']
const KINDS = [...FILTER_KINDS, 'access_token']

const list = ref([])
const total = ref(0)
const page = ref(1)
const size = ref(50)
const loading = ref(false)
const kindFilter = ref('')
const totalPages = computed(() => Math.max(1, Math.ceil(total.value / size.value)))

onMounted(load)

async function load() {
  loading.value = true
  try {
    const res = await api.listApiLogs({ page: page.value, size: size.value, kind: kindFilter.value })
    list.value = res.list || []
    total.value = res.total || 0
  } catch (e) {}
  loading.value = false
}
function doSearch() { page.value = 1; load() }
function goPage(p) { page.value = p; load() }

function kindLabel(k) { return KINDS.includes(k) ? t('apiLogs.type_' + k) : k }
function kindClass(k) {
  return { msg_sec_check: 'blue', media_check_async: 'purple', media_check_callback: 'purple', subscribe_send: 'orange', geo_regeo: 'cyan' }[k] || 'gray'
}
function shortTime(ts) { return ts ? String(ts).slice(5, 19).replace('T', ' ') : '—' }
</script>

<style scoped>
/* 组件样式统一来自 styles/base.css，这里只留本页特有的部分。
   原本 badge 有 7 种颜色在给「接口类型」做分类——那是用色相做装饰，
   而色相只留给状态（S-10）。装饰性的几种已在 base.css 收敛为中性。 */
.tbl { min-width: 860px; }
.tbl td { vertical-align: top; }
.nowrap { white-space: nowrap; }
.detail-cell { max-width: 340px; word-break: break-all; }
</style>
