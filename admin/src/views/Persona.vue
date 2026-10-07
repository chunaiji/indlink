<template>
  <div>
    <div class="toolbar">
      <span style="flex:1"></span>
      <button class="btn ghost" @click="openBatch">{{ t('persona.batchBtn') }}</button>
      <button class="btn" @click="openAdd">{{ t('persona.addBtn') }}</button>
    </div>

    <div class="table-wrap">
    <table class="tbl">
      <thead>
        <tr>
          <th>{{ t('common.colName') }}</th>
          <th>{{ t('persona.colRole') }}</th>
          <th>{{ t('persona.colAffect') }}</th>
          <th>{{ t('persona.colVoice') }}</th>
          <th>{{ t('common.status') }}</th>
          <th>{{ t('common.actions') }}</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="r in pagedList" :key="r.persona_id">
          <td>{{ r.name }}</td>
          <td class="soft">{{ roleLabel(r.relationship_role) }}</td>
          <td class="soft">{{ affectLabel(r.affective_style) }}</td>
          <td class="soft">{{ voiceLabel(r.voice_style) }}</td>
          <td>
            <span class="badge" :class="r.status === 'active' ? 'st-active' : 'st-paused'">
              {{ r.status === 'active' ? t('persona.statusActive') : t('persona.statusPaused') }}
            </span>
          </td>
          <td>
            <button class="btn-sm" @click="openEdit(r)">{{ t('common.edit') }}</button>
            <button class="btn-sm del" @click="del(r)">{{ t('common.delete') }}</button>
          </td>
        </tr>
      </tbody>
    </table>
    </div>
    <div v-if="!list.length && !loading" class="empty">{{ t('persona.empty') }}</div>

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
        <div class="dlg-hd">{{ editing ? t('persona.editTitle') : t('persona.addTitle') }}</div>
        <div class="field">
          <label>{{ t('persona.nameLabel') }}</label>
          <input v-model="form.name" class="ipt" :placeholder="t('persona.namePlaceholder')" />
        </div>
        <div class="field">
          <label>{{ t('persona.colRole') }}</label>
          <select v-model="form.relationship_role" class="sel">
            <option v-for="v in ROLES" :key="v" :value="v">{{ roleLabel(v) }}</option>
          </select>
        </div>
        <div class="field">
          <label>{{ t('persona.colAffect') }}</label>
          <select v-model="form.affective_style" class="sel">
            <option v-for="v in AFFECTS" :key="v" :value="v">{{ affectLabel(v) }}</option>
          </select>
        </div>
        <div class="field">
          <label>{{ t('persona.colVoice') }}</label>
          <select v-model="form.voice_style" class="sel">
            <option v-for="v in VOICES" :key="v" :value="v">{{ voiceLabel(v) }}</option>
          </select>
        </div>
        <div class="field">
          <label>{{ t('persona.rulesLabel') }}</label>
          <textarea v-model="form.rules_json" class="ta" rows="5" placeholder='{"do":["..."],"dont":["..."]}'></textarea>
        </div>
        <div class="field">
          <label>{{ t('common.status') }}</label>
          <select v-model="form.status" class="sel">
            <option value="active">{{ t('persona.statusActive') }}</option>
            <option value="paused">{{ t('persona.statusPaused') }}</option>
          </select>
        </div>
        <div class="dlg-ft">
          <button class="btn ghost" @click="modal = null">{{ t('common.cancel') }}</button>
          <button class="btn" @click="submit">{{ t('common.save') }}</button>
        </div>
        <div v-if="msg" class="toast">{{ msg }}</div>
      </div>
    </div>

    <div v-if="batchModal" class="overlay" @click.self="batchModal = null">
      <div class="dlg">
        <div class="dlg-hd">{{ t('persona.batchTitle') }}</div>
        <div class="batch-info">
          <div>{{ t('persona.batchSource', { n: batchAvailable }) }}</div>
          <div>{{ t('persona.batchCurrent', { n: list.length }) }}</div>
        </div>
        <div class="field">
          <label>{{ t('persona.countLabel') }}</label>
          <input v-model.number="batchCount" type="number" class="ipt"
                 :min="1" :max="batchAvailable || 1" />
        </div>
        <div class="batch-tip">{{ t('persona.batchTip') }}</div>
        <div class="dlg-ft">
          <button class="btn ghost" @click="batchModal = null">{{ t('common.cancel') }}</button>
          <button class="btn" :disabled="batchSubmitting" @click="submitBatch">
            {{ batchSubmitting ? t('persona.processing') : t('common.confirm') }}
          </button>
        </div>
        <div v-if="batchMsg" class="toast">{{ batchMsg }}</div>
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
const form = ref({ name: '', relationship_role: 'friend', affective_style: 'warm_soft', voice_style: 'casual', rules_json: '', status: 'active' })
const msg = ref('')
const toast = ref('')

// 批量新增(从默认租户的人格模板复制到当前租户)
const batchModal = ref(null)
const batchCount = ref(10)
const batchAvailable = ref(0)
const batchSubmitting = ref(false)
const batchMsg = ref('')

// 三组枚举的**值**存库,不翻;只翻显示名
const ROLES = ['friend', 'partner', 'companion', 'mentor']
const AFFECTS = ['warm_soft', 'calm', 'energetic', 'playful', 'dominant']
const VOICES = ['short_sentence', 'casual', 'structured', 'expressive']

function roleLabel(v) { return ROLES.includes(v) ? t('persona.role_' + v) : v }
function affectLabel(v) { return AFFECTS.includes(v) ? t('persona.affect_' + v) : v }
function voiceLabel(v) { return VOICES.includes(v) ? t('persona.voice_' + v) : v }

onMounted(load)

async function load() {
  loading.value = true
  try { list.value = await api.listPersonas() || [] } catch (e) { flash2(e.message) }
  loading.value = false
}

async function openBatch() {
  batchMsg.value = ''
  batchCount.value = 10
  batchAvailable.value = 0
  batchModal.value = true
  try {
    const info = await api.personaBatchInfo()
    batchAvailable.value = info.available || 0
    if (info.is_template_tenant) {
      batchMsg.value = t('persona.isTemplateTenant')
    } else if (batchAvailable.value === 0) {
      batchMsg.value = t('persona.noTemplates')
    } else if (batchCount.value > batchAvailable.value) {
      batchCount.value = batchAvailable.value
    }
  } catch (e) { batchMsg.value = e.message }
}

async function submitBatch() {
  const n = Number(batchCount.value)
  if (!n || n < 1) { batchMsg.value = t('persona.errCount'); return }
  if (batchAvailable.value && n > batchAvailable.value) {
    batchMsg.value = t('persona.errMax', { n: batchAvailable.value }); return
  }
  batchSubmitting.value = true
  try {
    const r = await api.batchCreatePersonas(n)
    batchModal.value = null
    flash2(t('persona.batchDone', { created: r.created, skipped: r.skipped }))
    load()
  } catch (e) { batchMsg.value = e.message }
  batchSubmitting.value = false
}

function openAdd() {
  editing.value = null
  form.value = { name: '', relationship_role: 'friend', affective_style: 'warm_soft', voice_style: 'casual', rules_json: '', status: 'active' }
  modal.value = true
  msg.value = ''
}

function openEdit(r) {
  editing.value = r
  form.value = {
    name: r.name,
    relationship_role: r.relationship_role,
    affective_style: r.affective_style,
    voice_style: r.voice_style,
    rules_json: r.rules_json || '',
    status: r.status
  }
  modal.value = true
  msg.value = ''
}

async function submit() {
  if (!form.value.name.trim()) { msg.value = t('persona.errName'); return }
  if (form.value.rules_json.trim()) {
    try { JSON.parse(form.value.rules_json) } catch { msg.value = t('persona.errRulesJson'); return }
  }
  try {
    if (editing.value) {
      await api.updatePersona(editing.value.persona_id, form.value)
    } else {
      await api.createPersona(form.value)
    }
    modal.value = null
    flash2(editing.value ? t('common.updated') : t('common.added'))
    load()
    api.reloadPersona().catch(() => {})
  } catch (e) { msg.value = e.message }
}

async function del(r) {
  if (!confirm(t('persona.confirmDel', { name: r.name }))) return
  try {
    await api.deletePersona(r.persona_id)
    flash2(t('common.deleted'))
    load()
    api.reloadPersona().catch(() => {})
  } catch (e) { flash2(e.message) }
}

function flash2(text) { toast.value = text; setTimeout(() => { toast.value = '' }, 2000) }
</script>

<style scoped>
/* 组件样式统一来自 styles/base.css，这里只留本页特有的部分 */
.tbl { min-width: 680px; }
.ta { width: 100%; }
.btn-sm + .btn-sm { margin-left: var(--s-1); }

.batch-info {
  margin-bottom: var(--s-4);
  padding: var(--s-3) var(--s-4);
  border-radius: var(--radius-sm);
  background: var(--panel-3);
  color: var(--ink-2);
  font-size: var(--t-value);
  line-height: 1.9;
}

.batch-tip { margin: calc(-1 * var(--s-1)) 0 var(--s-4); font-size: var(--t-label); color: var(--ink-3); }
</style>
