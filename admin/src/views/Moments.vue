<template>
  <div>
    <div class="toolbar">
      <input v-model="keyword" class="ipt" :placeholder="t('moments.searchPlaceholder')" @keydown.enter="doSearch" />
      <select v-model="robotFilter" class="sel">
        <option value="">{{ t('common.allTypes') }}</option>
        <option value="human">{{ t('moments.typeHuman') }}</option>
        <option value="robot">{{ t('moments.typeRobot') }}</option>
      </select>
      <button class="btn" @click="doSearch">{{ t('common.filter') }}</button>
      <button class="btn ghost" @click="reset">{{ t('common.reset') }}</button>
      <span class="total">{{ t('common.total', { n: total }) }}</span>
    </div>

    <div class="table-wrap">
      <table class="tbl">
        <thead>
          <tr>
            <th>{{ t('moments.colContent') }}</th>
            <th>{{ t('moments.colImages') }}</th>
            <th>{{ t('moments.colAuthor') }}</th>
            <th>{{ t('moments.colLikes') }}</th>
            <th>{{ t('moments.colComments') }}</th>
            <th>{{ t('moments.colVisible') }}</th>
            <th>{{ t('common.colTime') }}</th>
            <th>{{ t('common.actions') }}</th>
          </tr>
        </thead>
        <tbody>
          <template v-for="m in list" :key="m.moment_id">
            <tr :class="{ 'row-bot': m.is_robot }">
              <td class="content-cell">{{ m.content || t('common.dash') }}</td>
              <td>
                <div v-if="imgs(m).length" class="thumbs">
                  <img v-for="(img, i) in imgs(m).slice(0, 3)" :key="i" :src="img" class="thumb" @click="previewImg = img" />
                  <span v-if="imgs(m).length > 3" class="soft">+{{ imgs(m).length - 3 }}</span>
                </div>
                <span v-else class="soft">{{ t('common.dash') }}</span>
              </td>
              <td>
                <div class="nick">{{ m.nickname || t('common.dash') }}</div>
                <span v-if="m.is_robot" class="badge purple">{{ t('moments.badgeRobot') }}</span>
              </td>
              <td>{{ m.like_count || 0 }}</td>
              <td>{{ m.comment_count || 0 }}</td>
              <td>
                <span class="badge" :class="m.visible === 'public' ? 'green' : 'gray'">
                  {{ m.visible === 'public' ? t('moments.visiblePublic') : t('moments.visibleSelf') }}
                </span>
              </td>
              <td class="soft">{{ shortTime(m.created_at) }}</td>
              <td>
                <div class="acts">
                  <button class="btn-sm" @click="toggleExpand(m)">
                    {{ expandedId === m.moment_id ? t('moments.collapse') : t('moments.comments') }}{{ m.comment_count ? '(' + m.comment_count + ')' : '' }}
                  </button>
                  <button class="btn-sm del" @click="delMoment(m)">{{ t('common.delete') }}</button>
                </div>
              </td>
            </tr>
            <tr v-if="expandedId === m.moment_id" class="expand-row">
              <td colspan="8">
                <div v-if="cmtsLoading" class="soft">{{ t('common.loading') }}</div>
                <div v-else-if="!(cmtsMap[m.moment_id] || []).length" class="soft">{{ t('moments.emptyComments') }}</div>
                <div v-else class="replies">
                  <div v-for="c in cmtsMap[m.moment_id]" :key="c.comment_id" class="reply">
                    <span class="r-user">{{ c.nickname || t('common.dash') }}</span>
                    <span class="r-content">{{ c.content }}</span>
                    <span class="r-time">{{ shortTime(c.created_at) }}</span>
                    <button class="btn-sm del" @click="delComment(m, c)">{{ t('moments.delCommentShort') }}</button>
                  </div>
                </div>
              </td>
            </tr>
          </template>
          <tr v-if="!list.length && !loading">
            <td colspan="8" class="empty">{{ t('moments.emptyMoments') }}</td>
          </tr>
        </tbody>
      </table>
    </div>

    <div class="pager">
      <button class="btn ghost sm" :disabled="page <= 1" @click="goPage(page - 1)">{{ t('common.prev') }}</button>
      <span class="pg">{{ t('common.pageInfo', { page, pages: totalPages, total }) }}</span>
      <button class="btn ghost sm" :disabled="page >= totalPages" @click="goPage(page + 1)">{{ t('common.next') }}</button>
    </div>

    <div v-if="previewImg" class="overlay" @click="previewImg = ''">
      <img :src="previewImg" class="img-big" />
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
const robotFilter = ref('human') // 默认只看真人动态
const totalPages = computed(() => Math.max(1, Math.ceil(total.value / size.value)))

const expandedId = ref(null)
const cmtsMap = ref({})
const cmtsLoading = ref(false)
const previewImg = ref('')

const toast = ref('')
const toastOk = ref(true)

onMounted(load)

async function load() {
  loading.value = true
  expandedId.value = null
  try {
    const res = await api.listMoments({ page: page.value, size: size.value, keyword: keyword.value, robot: robotFilter.value })
    list.value = res.list || []
    total.value = res.total || 0
  } catch (e) { flash(e.message, false) }
  loading.value = false
}

function doSearch() { page.value = 1; load() }
function reset() { keyword.value = ''; robotFilter.value = 'human'; doSearch() }
function goPage(p) { page.value = p; load() }
function imgs(m) {
  try { const a = JSON.parse(m.images || '[]'); return Array.isArray(a) ? a : [] } catch (e) { return [] }
}

async function toggleExpand(m) {
  if (expandedId.value === m.moment_id) { expandedId.value = null; return }
  expandedId.value = m.moment_id
  cmtsLoading.value = true
  try { cmtsMap.value[m.moment_id] = await api.momentComments(m.moment_id) || [] }
  catch (e) { cmtsMap.value[m.moment_id] = [] }
  cmtsLoading.value = false
}

async function delMoment(m) {
  const excerpt = (m.content || t('moments.imageMoment')).slice(0, 20)
  if (!confirm(t('moments.confirmDelMoment', { excerpt }))) return
  try { await api.deleteMoment(m.moment_id); flash(t('moments.momentDeleted')); load() }
  catch (e) { flash(e.message, false) }
}

async function delComment(m, c) {
  if (!confirm(t('moments.confirmDelComment', { excerpt: (c.content || '').slice(0, 20) }))) return
  try {
    await api.deleteMomentComment(c.comment_id)
    cmtsMap.value[m.moment_id] = await api.momentComments(m.moment_id) || []
    m.comment_count = cmtsMap.value[m.moment_id].length
    flash(t('moments.commentDeleted'))
  } catch (e) { flash(e.message, false) }
}

function shortTime(ts) { return ts ? String(ts).slice(0, 16).replace('T', ' ') : '—' }
function flash(msg, ok = true) { toast.value = msg; toastOk.value = ok; setTimeout(() => { toast.value = '' }, 2500) }
</script>

<style scoped>
/* 组件样式统一来自 styles/base.css，这里只留本页特有的部分 */
.ipt { width: 220px; }
.tbl { min-width: 820px; }

/* 机器人发的动态：标一档更浅的面色，不占色相 */
.tbl tr.row-bot td { background: var(--panel-2); }

.content-cell { max-width: 320px; }
.nick { font-weight: 600; }
.thumbs { display: flex; gap: var(--s-1); align-items: center; }

.thumb {
  width: 42px;
  height: 42px;
  object-fit: cover;
  border: 1px solid var(--line);
  border-radius: var(--radius-xs);
  cursor: zoom-in;
}

.acts { display: flex; gap: var(--s-1); flex-wrap: wrap; }

.expand-row td { background: var(--panel-2); }
.replies { display: flex; flex-direction: column; gap: var(--s-2); }
.reply { display: flex; align-items: center; gap: var(--s-2); flex-wrap: wrap; }
.r-user { flex-shrink: 0; font-size: var(--t-value); font-weight: 600; }
.r-content { flex: 1; min-width: 120px; font-size: var(--t-value); word-break: break-word; }
.r-time { flex-shrink: 0; font-size: var(--t-tag); color: var(--ink-3); }

/* 看图弹层（比对话框更暗，图片要压得住） */
.overlay { cursor: zoom-out; }
.img-big { max-width: 86vw; max-height: 86vh; border-radius: var(--radius); }

.gtoast.err {
  border-color: var(--danger-line);
  background: var(--danger-bg);
  color: var(--danger);
}
</style>
