<template>
  <div>
    <div class="toolbar">
      <select v-model="filterRole" @change="onFilterChange" class="sel">
        <option value="">{{ t('replyCache.allRoles') }}</option>
        <option v-for="v in ROLES" :key="v" :value="v">{{ t('replyCache.role_' + v) }}</option>
      </select>
      <span style="flex:1"></span>
      <span class="total-hint" v-if="total > 0">{{ t('common.total', { n: total }) }}</span>
    </div>

    <div class="table-wrap">
    <table class="tbl">
      <thead>
        <tr>
          <th>{{ t('replyCache.colQuestion') }}</th>
          <th>{{ t('replyCache.colRole') }}</th>
          <th>{{ t('replyCache.colVariants') }}</th>
          <th>{{ t('replyCache.colHits') }}</th>
          <th>{{ t('replyCache.colSource') }}</th>
          <th>{{ t('common.status') }}</th>
          <th>{{ t('common.actions') }}</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="r in list" :key="r.cache_id">
          <td class="q-cell">{{ r.question_sample }}</td>
          <td class="soft">{{ r.persona_role }}</td>
          <td class="soft">{{ variantCount(r.responses_json) }}</td>
          <td class="soft">{{ r.hit_count }}</td>
          <td>
            <span class="badge" :class="r.source === 'ai_generated' ? 'src-ai' : 'src-manual'">
              {{ r.source === 'ai_generated' ? t('replyCache.sourceAI') : t('replyCache.sourceManual') }}
            </span>
          </td>
          <td>
            <span class="badge" :class="r.status === 'active' ? 'st-active' : 'st-paused'">
              {{ r.status === 'active' ? t('replyCache.statusActive') : t('replyCache.statusDisabled') }}
            </span>
          </td>
          <td>
            <button class="btn-sm" @click="openEdit(r)">{{ t('common.edit') }}</button>
            <button class="btn-sm" @click="toggle(r)">
              {{ r.status === 'active' ? t('replyCache.statusDisabled') : t('replyCache.statusActive') }}
            </button>
            <button class="btn-sm del" @click="del(r)">{{ t('common.delete') }}</button>
          </td>
        </tr>
      </tbody>
    </table>
    </div>
    <div v-if="!list.length && !loading" class="empty">{{ t('replyCache.empty') }}</div>

    <div v-if="total > 0" class="pager">
      <button class="btn ghost btn-pg" :disabled="page <= 1" @click="goPage(page - 1)">{{ t('common.prev') }}</button>
      <span class="pg-info">{{ t('common.pageInfo', { page, pages: totalPages, total }) }}</span>
      <button class="btn ghost btn-pg" :disabled="page >= totalPages" @click="goPage(page + 1)">{{ t('common.next') }}</button>
      <select v-model="size" @change="onFilterChange" class="sel-pg">
        <option :value="20">{{ t('common.perPage', { n: 20 }) }}</option>
        <option :value="50">{{ t('common.perPage', { n: 50 }) }}</option>
        <option :value="100">{{ t('common.perPage', { n: 100 }) }}</option>
      </select>
    </div>

    <div v-if="modal" class="overlay" @click.self="modal = null">
      <div class="dlg">
        <div class="dlg-hd">{{ t('replyCache.editTitle') }}</div>
        <div class="field">
          <label>{{ t('replyCache.colQuestion') }}</label>
          <div class="readonly-val">{{ editing && editing.question_sample }}</div>
        </div>
        <div class="field">
          <label>{{ t('replyCache.variantsLabel') }}</label>
          <textarea v-model="form.responses_text" class="ta" rows="8" :placeholder="t('replyCache.variantsPlaceholder')"></textarea>
          <span class="hint">{{ t('replyCache.variantsHint') }}</span>
        </div>
        <div class="dlg-ft">
          <button class="btn ghost" @click="modal = null">{{ t('common.cancel') }}</button>
          <button class="btn" @click="submit">{{ t('common.save') }}</button>
        </div>
        <div v-if="msg" class="toast">{{ msg }}</div>
      </div>
    </div>

    <div v-if="toast" class="gtoast">{{ toast }}</div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api'

const { t } = useI18n()

// persona_role 的**值**存库,不翻;只翻显示名
const ROLES = ['warm_soft', 'calm', 'energetic', 'playful', 'dominant']

const list = ref([])
const total = ref(0)
const loading = ref(false)
const page = ref(1)
const size = ref(20)
const filterRole = ref('')
const totalPages = computed(() => Math.max(1, Math.ceil(total.value / size.value)))

const modal = ref(null)
const editing = ref(null)
const form = ref({ responses_text: '' })
const msg = ref('')
const toast = ref('')

function variantCount(json) {
  try { return JSON.parse(json).length } catch { return 0 }
}

onMounted(load)

async function load() {
  loading.value = true
  try {
    const res = await api.listReplyCache({ persona_role: filterRole.value || undefined, page: page.value, size: size.value })
    list.value = res.list || []
    total.value = res.total || 0
  } catch (e) { flash2(e.message) }
  loading.value = false
}

function onFilterChange() {
  page.value = 1
  load()
}

function goPage(p) {
  page.value = p
  load()
}

function openEdit(r) {
  editing.value = r
  let responses_text = ''
  try {
    const arr = JSON.parse(r.responses_json)
    responses_text = Array.isArray(arr) ? arr.join('\n') : ''
  } catch {}
  form.value = { responses_text }
  modal.value = true
  msg.value = ''
}

async function submit() {
  const arr = (form.value.responses_text || '').split('\n').map(s => s.trim()).filter(Boolean)
  if (!arr.length) { msg.value = t('replyCache.errVariants'); return }
  try {
    await api.updateReplyCache(editing.value.cache_id, { responses_json: JSON.stringify(arr) })
    modal.value = null
    flash2(t('common.updated'))
    load()
  } catch (e) { msg.value = e.message }
}

async function toggle(r) {
  const next = r.status === 'active' ? 'disabled' : 'active'
  try { await api.toggleReplyCache(r.cache_id, next); flash2(t('replyCache.toggled')); load() } catch (e) { flash2(e.message) }
}

async function del(r) {
  if (!confirm(t('replyCache.confirmDel', { excerpt: r.question_sample?.slice(0, 20) }))) return
  try { await api.deleteReplyCache(r.cache_id); flash2(t('common.deleted')); load() } catch (e) { flash2(e.message) }
}

function flash2(text) { toast.value = text; setTimeout(() => { toast.value = '' }, 2000) }
</script>

<style scoped>
/* 组件样式统一来自 styles/base.css，这里只留本页特有的部分。
   来源标记（AI / 人工）原本占了蓝与橙两个色相——那是分类不是状态，
   蓝还和「可以点」撞车，所以都收敛为中性徽章，靠文字区分。 */
.total-hint { font-size: var(--t-label); color: var(--ink-3); }
.tbl { min-width: 720px; }
.q-cell { max-width: 280px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.ta { width: 100%; }
.btn-sm + .btn-sm { margin-left: var(--s-1); }
.pager { justify-content: flex-end; }
.sel-pg { margin-left: 0; }

/* 只读字段：形状与输入框一致，但用 panel-3 底表明不可编辑 */
.readonly-val {
  display: flex;
  align-items: center;
  min-height: var(--d-ctl-h);
  padding: var(--s-1) var(--s-3);
  background: var(--panel-3);
  border: 1px solid var(--line);
  border-radius: var(--radius-sm);
  font-size: var(--t-value);
  color: var(--ink-2);
}
</style>
