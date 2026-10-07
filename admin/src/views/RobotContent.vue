<template>
  <div>
    <div class="toolbar">
      <select v-model="filterType" @change="load" class="sel">
        <option value="">{{ t('common.allTypes') }}</option>
        <option v-for="v in TYPES" :key="v" :value="v">{{ t('robotContent.type_' + v) }}</option>
      </select>
      <button class="btn" @click="openAdd">{{ t('robotContent.addBtn') }}</button>
    </div>

    <div class="table-wrap">
    <table class="tbl">
      <thead>
        <tr>
          <th>ID</th>
          <th>{{ t('robotContent.colType') }}</th>
          <th>{{ t('robotContent.colContent') }}</th>
          <th>{{ t('robotContent.colTags') }}</th>
          <th>{{ t('robotContent.colWeight') }}</th>
          <th>{{ t('common.actions') }}</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="r in pagedList" :key="r.id">
          <td class="id">{{ r.id }}</td>
          <td><span class="badge" :class="r.type">{{ r.type }}</span></td>
          <td class="txt">{{ r.text }}</td>
          <td class="soft">{{ r.tags }}</td>
          <td class="soft">{{ r.weight }}</td>
          <td>
            <button class="btn-sm" @click="openEdit(r)">{{ t('common.edit') }}</button>
            <button class="btn-sm del" @click="del(r)">{{ t('common.delete') }}</button>
          </td>
        </tr>
      </tbody>
    </table>
    </div>
    <div v-if="!list.length && !loading" class="empty">{{ t('robotContent.empty') }}</div>

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

    <!-- 新增/编辑弹窗 -->
    <div v-if="modal" class="overlay" @click.self="modal = null">
      <div class="dlg">
        <div class="dlg-hd">{{ editing ? t('robotContent.editTitle') : t('robotContent.addTitle') }}</div>
        <div class="field">
          <label>{{ t('robotContent.colType') }}</label>
          <select v-model="form.type" class="sel" :disabled="!!editing">
            <option v-for="v in TYPES" :key="v" :value="v">{{ t('robotContent.type_' + v) }}</option>
          </select>
        </div>
        <div class="field">
          <label>{{ t('robotContent.colContent') }}</label>
          <textarea v-model="form.text" class="ta" rows="4" :placeholder="t('robotContent.contentPlaceholder')"></textarea>
        </div>
        <div class="field">
          <label>{{ t('robotContent.colTags') }}</label>
          <input v-model="form.tags" class="ipt" :placeholder="t('robotContent.tagsPlaceholder')" />
        </div>
        <div class="field">
          <label>{{ t('robotContent.colWeight') }}</label>
          <input v-model.number="form.weight" type="number" class="ipt" :placeholder="t('robotContent.weightPlaceholder')" />
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

// type 的**值**存库,不翻;只翻显示名
const TYPES = ['bottle', 'reply']

const list = ref([])
const loading = ref(false)
const page = ref(1)
const size = ref(20)
const totalPages = computed(() => Math.max(1, Math.ceil(list.value.length / size.value)))
const pagedList = computed(() => list.value.slice((page.value - 1) * size.value, page.value * size.value))
function goPage(p) { page.value = p }
function onSizeChange() { page.value = 1 }
const filterType = ref('')
const modal = ref(null)
const editing = ref(null)
const form = ref({ type: 'bottle', text: '', tags: '', weight: 1 })
const msg = ref('')
const toast = ref('')

onMounted(load)

async function load() {
  loading.value = true
  try { list.value = await api.getRobotContent(filterType.value) || [] } catch (e) { flash(e.message) }
  loading.value = false
}

function openAdd() {
  editing.value = null
  form.value = { type: 'bottle', text: '', tags: '', weight: 1 }
  modal.value = true
  msg.value = ''
}

function openEdit(r) {
  editing.value = r
  form.value = { type: r.type, text: r.text, tags: r.tags || '', weight: r.weight || 1 }
  modal.value = true
  msg.value = ''
}

async function submit() {
  if (!form.value.text.trim()) { msg.value = t('robotContent.errContent'); return }
  try {
    if (editing.value) {
      await api.updateRobotContent(editing.value.id, { text: form.value.text, tags: form.value.tags, weight: form.value.weight })
    } else {
      await api.createRobotContent(form.value)
    }
    modal.value = null
    flash2(editing.value ? t('common.updated') : t('common.added'))
    load()
  } catch (e) { msg.value = e.message }
}

async function del(r) {
  if (!confirm(t('robotContent.confirmDel', { excerpt: r.text.slice(0, 20) }))) return
  try { await api.deleteRobotContent(r.id); flash2(t('common.deleted')); load() } catch (e) { flash2(e.message) }
}

function flash(t) { msg.value = t; setTimeout(() => { msg.value = '' }, 2000) }
function flash2(t) { toast.value = t; setTimeout(() => { toast.value = '' }, 2000) }
</script>

<style scoped>
/* 组件样式统一来自 styles/base.css，这里只留本页特有的部分。
   内容类型（瓶子 / 回复）原本各占一个色相，属于分类而非状态，
   已在 base.css 收敛为中性徽章。 */
.tbl { min-width: 680px; }
.id { color: var(--ink-3); font-size: var(--t-tag); }
.txt { max-width: 320px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.btn-sm + .btn-sm { margin-left: var(--s-1); }
</style>
