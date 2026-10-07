<template>
  <div>
    <div class="toolbar">
      <span style="flex:1"></span>
      <button class="btn" @click="openAdd">{{ t('keywordRules.addBtn') }}</button>
    </div>

    <div class="table-wrap">
    <table class="tbl">
      <thead>
        <tr>
          <th>{{ t('keywordRules.colPriority') }}</th>
          <th>{{ t('keywordRules.colKeywords') }}</th>
          <th>{{ t('keywordRules.colMatch') }}</th>
          <th>{{ t('keywordRules.colHits') }}</th>
          <th>{{ t('common.status') }}</th>
          <th>{{ t('common.actions') }}</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="r in pagedList" :key="r.rule_id">
          <td class="soft">{{ r.priority }}</td>
          <td class="kw-cell">{{ keywordsPreview(r.keywords_json) }}</td>
          <td class="soft">{{ matchLabel(r.match_type) }}</td>
          <td class="soft">{{ r.hit_count }}</td>
          <td>
            <span class="badge" :class="r.status === 'active' ? 'st-active' : 'st-paused'">
              {{ r.status === 'active' ? t('keywordRules.statusActive') : t('keywordRules.statusDisabled') }}
            </span>
          </td>
          <td>
            <button class="btn-sm" @click="openEdit(r)">{{ t('common.edit') }}</button>
            <button class="btn-sm" @click="toggle(r)">
              {{ r.status === 'active' ? t('keywordRules.statusDisabled') : t('keywordRules.statusActive') }}
            </button>
            <button class="btn-sm del" @click="del(r)">{{ t('common.delete') }}</button>
          </td>
        </tr>
      </tbody>
    </table>
    </div>
    <div v-if="!list.length && !loading" class="empty">{{ t('keywordRules.empty') }}</div>

    <div v-if="list.length > 0" class="pager">
      <button class="btn ghost btn-pg" :disabled="page <= 1" @click="goPage(page - 1)">{{ t('common.prev') }}</button>
      <span class="pg-info">{{ t('common.pageInfo', { page, pages: totalPages, total: list.length }) }}</span>
      <button class="btn ghost btn-pg" :disabled="page >= totalPages" @click="goPage(page + 1)">{{ t('common.next') }}</button>
      <select v-model="size" @change="onSizeChange" class="sel-pg">
        <option :value="20">{{ t('common.perPage', { n: 20 }) }}</option>
        <option :value="50">{{ t('common.perPage', { n: 50 }) }}</option>
        <option :value="100">{{ t('common.perPage', { n: 100 }) }}</option>
      </select>
    </div>

    <div v-if="modal" class="overlay" @click.self="modal = null">
      <div class="dlg">
        <div class="dlg-hd">{{ editing ? t('keywordRules.editTitle') : t('keywordRules.addTitle') }}</div>
        <div class="field">
          <label>{{ t('keywordRules.colPriority') }}</label>
          <input v-model.number="form.priority" type="number" class="ipt" :placeholder="t('keywordRules.priorityPlaceholder')" />
        </div>
        <div class="field">
          <label>{{ t('keywordRules.colMatch') }}</label>
          <select v-model="form.match_type" class="sel">
            <option v-for="v in MATCH_TYPES" :key="v" :value="v">{{ matchLabel(v) }}</option>
          </select>
        </div>
        <div class="field">
          <label>{{ t('keywordRules.keywordsLabel') }}</label>
          <textarea v-model="form.keywords_text" class="ta" rows="4" :placeholder="t('keywordRules.keywordsHint')"></textarea>
          <span class="hint">{{ t('keywordRules.keywordsHint') }}</span>
        </div>
        <div class="field">
          <label>{{ t('keywordRules.defaultLabel') }}</label>
          <textarea v-model="form.default_text" class="ta" rows="4" :placeholder="t('keywordRules.defaultHint')"></textarea>
          <span class="hint">{{ t('keywordRules.defaultHint') }}</span>
        </div>
        <div class="field">
          <label>{{ t('keywordRules.warmSoftLabel') }} <span class="opt">{{ t('keywordRules.optional') }}</span></label>
          <textarea v-model="form.warm_soft_text" class="ta" rows="3" :placeholder="t('keywordRules.warmSoftHint')"></textarea>
          <span class="hint">{{ t('keywordRules.warmSoftHint') }}</span>
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
const list = ref([])
const loading = ref(false)
const page = ref(1)
const size = ref(20)
const totalPages = computed(() => Math.max(1, Math.ceil(list.value.length / size.value)))
const pagedList = computed(() => list.value.slice((page.value - 1) * size.value, page.value * size.value))
function goPage(p) { page.value = p }
function onSizeChange() { page.value = 1 }
const modal = ref(null)
const editing = ref(null)
const form = ref({ priority: 0, match_type: 'contains', keywords_text: '', default_text: '', warm_soft_text: '' })
const msg = ref('')
const toast = ref('')

// match_type 的**值**存库,不翻;只翻显示名
const MATCH_TYPES = ['contains', 'exact', 'prefix']
function matchLabel(v) { return MATCH_TYPES.includes(v) ? t('keywordRules.match_' + v) : v }

function keywordsPreview(json) {
  try {
    const arr = JSON.parse(json)
    if (!Array.isArray(arr)) return json
    return arr.slice(0, 3).join('、') + (arr.length > 3 ? ' …' : '')
  } catch { return json }
}

function parseLines(text) {
  return (text || '').split('\n').map(s => s.trim()).filter(Boolean)
}

function linesToText(arr) {
  return Array.isArray(arr) ? arr.join('\n') : ''
}

onMounted(load)

async function load() {
  loading.value = true
  try { list.value = await api.listKeywordRules() || [] } catch (e) { flash2(e.message) }
  loading.value = false
}

function openAdd() {
  editing.value = null
  form.value = { priority: 0, match_type: 'contains', keywords_text: '', default_text: '', warm_soft_text: '' }
  modal.value = true
  msg.value = ''
}

function openEdit(r) {
  editing.value = r
  let keywords_text = ''
  let default_text = ''
  let warm_soft_text = ''
  try { keywords_text = linesToText(JSON.parse(r.keywords_json)) } catch {}
  try {
    const resp = JSON.parse(r.responses_json)
    default_text = linesToText(resp.default || [])
    warm_soft_text = linesToText(resp.warm_soft || [])
  } catch {}
  form.value = { priority: r.priority, match_type: r.match_type, keywords_text, default_text, warm_soft_text }
  modal.value = true
  msg.value = ''
}

async function submit() {
  const keywords = parseLines(form.value.keywords_text)
  if (!keywords.length) { msg.value = t('keywordRules.errKeywords'); return }
  const defaultArr = parseLines(form.value.default_text)
  if (!defaultArr.length) { msg.value = t('keywordRules.errDefault'); return }
  const warmSoftArr = parseLines(form.value.warm_soft_text)

  const data = {
    priority: form.value.priority,
    match_type: form.value.match_type,
    keywords_json: JSON.stringify(keywords),
    responses_json: JSON.stringify({
      default: defaultArr,
      ...(warmSoftArr.length ? { warm_soft: warmSoftArr } : {})
    })
  }

  try {
    if (editing.value) {
      await api.updateKeywordRule(editing.value.rule_id, data)
    } else {
      await api.createKeywordRule(data)
    }
    modal.value = null
    flash2(editing.value ? t('common.updated') : t('common.added'))
    load()
  } catch (e) { msg.value = e.message }
}

async function toggle(r) {
  const next = r.status === 'active' ? 'disabled' : 'active'
  try { await api.toggleKeywordRule(r.rule_id, next); flash2(t('keywordRules.toggled')); load() } catch (e) { flash2(e.message) }
}

async function del(r) {
  const preview = keywordsPreview(r.keywords_json)
  if (!confirm(t('keywordRules.confirmDel', { preview }))) return
  try { await api.deleteKeywordRule(r.rule_id); flash2(t('common.deleted')); load() } catch (e) { flash2(e.message) }
}

function flash2(text) { toast.value = text; setTimeout(() => { toast.value = '' }, 2000) }
</script>

<style scoped>
/* 组件样式统一来自 styles/base.css，这里只留本页特有的部分 */
.tbl { min-width: 680px; }
.kw-cell { max-width: 240px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.opt { font-weight: 400; color: var(--ink-3); }
.ta { width: 100%; }
.btn-sm + .btn-sm { margin-left: var(--s-1); }
</style>
