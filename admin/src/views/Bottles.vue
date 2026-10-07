<template>
  <div>
    <div class="toolbar">
      <input v-model="keyword" class="ipt" :placeholder="t('bottles.searchPlaceholder')" @keydown.enter="doSearch" />
      <select v-model="statusFilter" class="sel">
        <option value="">{{ t('common.allStatus') }}</option>
        <option value="active">{{ t('bottles.statusActive') }}</option>
        <option value="expired">{{ t('bottles.statusExpired') }}</option>
        <option value="deleted">{{ t('bottles.statusDeleted') }}</option>
      </select>
      <select v-model="robotFilter" class="sel">
        <option value="">{{ t('common.allTypes') }}</option>
        <option value="human">{{ t('bottles.typeHuman') }}</option>
        <option value="robot">{{ t('bottles.typeRobot') }}</option>
      </select>
      <button class="btn" @click="doSearch">{{ t('common.filter') }}</button>
      <button class="btn ghost" @click="reset">{{ t('common.reset') }}</button>
      <span class="total">{{ t('common.total', { n: total }) }}</span>
    </div>

    <div class="table-wrap">
      <table class="tbl">
        <thead>
          <tr>
            <th>{{ t('bottles.colContent') }}</th>
            <th>{{ t('bottles.colAuthor') }}</th>
            <th>{{ t('bottles.colCity') }}</th>
            <th>{{ t('bottles.colReplies') }}</th>
            <th>{{ t('common.status') }}</th>
            <th>{{ t('common.colTime') }}</th>
            <th>{{ t('common.actions') }}</th>
          </tr>
        </thead>
        <tbody>
          <template v-for="b in list" :key="b.bottle_id">
            <tr :class="{ 'row-bot': b.is_robot }">
              <td class="content-cell">{{ b.content }}</td>
              <td>
                <div class="nick">{{ b.nickname || t('common.dash') }}</div>
                <span v-if="b.is_robot" class="badge purple">{{ t('bottles.badgeRobot') }}</span>
                <span v-else class="soft">{{ genderLabel(b.gender) }}{{ b.age ? ' · ' + b.age : '' }}</span>
              </td>
              <td class="soft">{{ b.city || t('common.dash') }}</td>
              <td>{{ b.reply_count }}</td>
              <td><span class="badge" :class="statusClass(b.status)">{{ statusLabel(b.status) }}</span></td>
              <td class="soft">{{ shortTime(b.created_at) }}</td>
              <td>
                <div class="acts">
                  <button class="btn-sm" @click="toggleExpand(b)">
                    {{ expandedId === b.bottle_id ? t('bottles.collapse') : t('bottles.expand') }}{{ b.reply_count ? '(' + b.reply_count + ')' : '' }}
                  </button>
                  <button v-if="b.status !== 'deleted'" class="btn-sm del" @click="delBottle(b)">{{ t('common.delete') }}</button>
                </div>
              </td>
            </tr>
            <tr v-if="expandedId === b.bottle_id" class="expand-row">
              <td colspan="7">
                <div v-if="repliesLoading && !repliesMap[b.bottle_id]" class="soft">{{ t('common.loading') }}</div>
                <div v-else-if="!(repliesMap[b.bottle_id] || []).length" class="soft">{{ t('bottles.emptyReplies') }}</div>
                <div v-else class="replies">
                  <div v-for="r in repliesMap[b.bottle_id]" :key="r.reply_id" class="reply">
                    <span class="r-user">{{ r.nickname || t('common.dash') }}<span v-if="r.is_robot"> 🤖</span></span>
                    <span class="r-content">{{ r.content }}</span>
                    <span class="r-time">{{ shortTime(r.created_at) }}</span>
                    <button class="btn-sm del" @click="delReply(b, r)">{{ t('bottles.delReplyShort') }}</button>
                  </div>
                </div>
              </td>
            </tr>
          </template>
          <tr v-if="!list.length && !loading">
            <td colspan="7" class="empty">{{ t('bottles.emptyBottles') }}</td>
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

    <div v-if="toast" class="gtoast" :class="toastOk ? 'ok' : 'err'">{{ toast }}</div>
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
const keyword = ref('')
const statusFilter = ref('')
const robotFilter = ref('human') // 默认只看真人瓶子
const totalPages = computed(() => Math.max(1, Math.ceil(total.value / size.value)))

const expandedId = ref(null)   // 当前展开的 bottle_id(手风琴,一次展开一个)
const repliesMap = ref({})     // bottle_id -> replies[]
const repliesLoading = ref(false)

const toast = ref('')
const toastOk = ref(true)

onMounted(load)

async function load() {
  loading.value = true
  expandedId.value = null
  try {
    const res = await api.listBottles({ page: page.value, size: size.value, keyword: keyword.value, status: statusFilter.value, robot: robotFilter.value })
    list.value = res.list || []
    total.value = res.total || 0
  } catch (e) { flash(e.message, false) }
  loading.value = false
}

function doSearch() { page.value = 1; load() }
function reset() { keyword.value = ''; statusFilter.value = ''; robotFilter.value = 'human'; doSearch() }
function goPage(p) { page.value = p; load() }

async function toggleExpand(b) {
  if (expandedId.value === b.bottle_id) { expandedId.value = null; return }
  expandedId.value = b.bottle_id
  if (!repliesMap.value[b.bottle_id]) {
    repliesLoading.value = true
    try { repliesMap.value[b.bottle_id] = await api.bottleReplies(b.bottle_id) || [] }
    catch (e) { repliesMap.value[b.bottle_id] = []; flash(e.message, false) }
    repliesLoading.value = false
  }
}

async function delBottle(b) {
  if (!confirm(t('bottles.confirmDelBottle', { excerpt: (b.content || '').slice(0, 20) }))) return
  try { await api.deleteBottle(b.bottle_id); flash(t('bottles.bottleDeleted')); load() }
  catch (e) { flash(e.message, false) }
}

async function delReply(b, r) {
  if (!confirm(t('bottles.confirmDelReply', { excerpt: (r.content || '').slice(0, 20) }))) return
  try {
    await api.deleteBottleReply(r.reply_id)
    repliesMap.value[b.bottle_id] = await api.bottleReplies(b.bottle_id) || []
    b.reply_count = repliesMap.value[b.bottle_id].length
    flash(t('bottles.replyDeleted'))
  } catch (e) { flash(e.message, false) }
}

function genderLabel(g) { return t(g === 1 ? 'bottles.male' : g === 2 ? 'bottles.female' : 'bottles.unknown') }
// status 的**值**是枚举,不翻;只翻显示名
function statusLabel(s) {
  const key = { active: 'statusActive', expired: 'statusExpired', deleted: 'statusDeleted' }[s]
  return key ? t('bottles.' + key) : s
}
function statusClass(s) { return { active: 'green', expired: 'orange', deleted: 'gray' }[s] || 'gray' }
function shortTime(ts) { return ts ? String(ts).slice(0, 16).replace('T', ' ') : '—' }
function flash(msg, ok = true) { toast.value = msg; toastOk.value = ok; setTimeout(() => { toast.value = '' }, 2500) }
</script>

<style scoped>
/* 组件样式统一来自 styles/base.css，这里只留本页特有的部分 */
.ipt { width: 220px; }
.tbl { min-width: 760px; }

/* 机器人发的瓶子：标一档更浅的面色，不占色相 */
.tbl tr.row-bot td { background: var(--panel-2); }

.content-cell { max-width: 360px; }
.nick { font-weight: 600; }

.acts { display: flex; gap: var(--s-1); flex-wrap: wrap; }

/* 展开区:回应列表 */
.expand-row td { background: var(--panel-2); }
.replies { display: flex; flex-direction: column; gap: var(--s-2); }
.reply { display: flex; align-items: center; gap: var(--s-2); flex-wrap: wrap; }
.r-user { flex-shrink: 0; font-size: var(--t-value); font-weight: 600; }
.r-content { flex: 1; min-width: 120px; font-size: var(--t-value); word-break: break-word; }
.r-time { flex-shrink: 0; font-size: var(--t-tag); color: var(--ink-3); }

/* 失败的 toast 切到 danger 一套 */
.gtoast.err {
  border-color: var(--danger-line);
  background: var(--danger-bg);
  color: var(--danger);
}
</style>
