<template>
  <div>
    <div class="toolbar">
      <button class="btn" @click="openAdd">{{ t('items.addBtn') }}</button>
      <span class="hint">{{ t('items.hint') }}</span>
    </div>

    <div class="table-wrap">
    <table class="tbl">
      <thead>
        <tr>
          <th>ID</th>
          <th>{{ t('items.colIcon') }}</th>
          <th>{{ t('common.colName') }}</th>
          <th>{{ t('items.colType') }}</th>
          <th>{{ t('items.colPrice') }}</th>
          <th>{{ t('common.colSort') }}</th>
          <th>{{ t('common.status') }}</th>
          <th>{{ t('common.actions') }}</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="r in list" :key="r.item_id">
          <td class="id">{{ r.item_id }}</td>
          <td><img v-if="isUrl(r.icon)" :src="r.icon" class="ic-img" /><span v-else class="soft">{{ r.icon || t('common.dash') }}</span></td>
          <td>{{ r.name }}</td>
          <td><span class="badge">{{ typeLabel(r.type) }}</span></td>
          <td>{{ r.price_coin }}</td>
          <td class="soft">{{ r.sort }}</td>
          <td>
            <span class="badge" :class="r.status">
              {{ r.status === 'active' ? t('common.statusListed') : t('common.statusUnlisted') }}
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
    <div v-if="!list.length && !loading" class="empty">{{ t('items.empty') }}</div>

    <div v-if="modal" class="overlay" @click.self="modal = null">
      <div class="dlg">
        <div class="dlg-hd">{{ editing ? t('items.editTitle') : t('items.addTitle') }}</div>
        <div class="field">
          <label>{{ t('common.colName') }}</label>
          <input v-model="form.name" class="ipt" :placeholder="t('items.namePlaceholder')" />
        </div>
        <div class="field">
          <label>{{ t('items.colType') }}</label>
          <select v-model="form.type" class="sel">
            <!-- 循环变量用 ty,不用 t —— 那是翻译函数 -->
            <option v-for="ty in TYPES" :key="ty.v" :value="ty.v">{{ ty.l }}</option>
          </select>
        </div>
        <div class="field">
          <label>{{ t('items.iconLabel') }}</label>
          <input v-model="form.icon" class="ipt" placeholder="https://.../rose.png" />
          <img v-if="isUrl(form.icon)" :src="form.icon" class="ic-preview" />
        </div>
        <div class="field">
          <label>{{ t('items.colPrice') }}</label>
          <input v-model.number="form.price_coin" type="number" class="ipt" />
        </div>
        <div class="field">
          <label>{{ t('common.colSort') }}</label>
          <input v-model.number="form.sort" type="number" class="ipt" :placeholder="t('common.sortPlaceholder')" />
        </div>
        <div class="field">
          <label>{{ t('common.status') }}</label>
          <select v-model="form.status" class="sel">
            <option value="active">{{ t('common.statusListed') }}</option>
            <option value="inactive">{{ t('common.statusUnlisted') }}</option>
          </select>
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

// 类型的**值**存库,不翻;只翻显示名。
// TYPES 用 computed 而非常量 —— 常量存翻译后的字符串不会响应语言切换。
const TYPE_VALUES = ['gift', 'vip', 'boost', 'top', 'superlike', 'quota_throw', 'quota_scoop']
const TYPES = computed(() => TYPE_VALUES.map((v) => ({ v, l: t('items.type_' + v) })))

const list = ref([])
const loading = ref(false)
const modal = ref(null)
const editing = ref(null)
const form = ref({ name: '', type: 'gift', icon: '', price_coin: 0, sort: 0, status: 'active' })
const msg = ref('')
const toast = ref('')

onMounted(load)

function isUrl(s) { return typeof s === 'string' && /^(https?:)?\/\//.test(s) }
function typeLabel(v) { return TYPE_VALUES.includes(v) ? t('items.type_' + v) : v }

async function load() {
  loading.value = true
  try { list.value = await api.listItems() || [] } catch (e) { flash2(e.message) }
  loading.value = false
}

function openAdd() {
  editing.value = null
  form.value = { name: '', type: 'gift', icon: '', price_coin: 0, sort: 0, status: 'active' }
  modal.value = true
  msg.value = ''
}

function openEdit(r) {
  editing.value = r
  form.value = { name: r.name, type: r.type, icon: r.icon || '', price_coin: r.price_coin, sort: r.sort, status: r.status }
  modal.value = true
  msg.value = ''
}

async function submit() {
  const f = form.value
  if (!f.name.trim()) { msg.value = t('common.errName'); return }
  if (!(f.price_coin >= 0)) { msg.value = t('items.errPrice'); return }
  const payload = {
    name: f.name.trim(), type: f.type, icon: f.icon.trim(),
    price_coin: Math.round(f.price_coin), sort: Math.round(f.sort || 0), status: f.status
  }
  try {
    if (editing.value) await api.updateItem(editing.value.item_id, payload)
    else await api.createItem(payload)
    modal.value = null
    flash2(editing.value ? t('common.updated') : t('common.added'))
    load()
  } catch (e) { msg.value = e.message }
}

async function del(r) {
  if (!confirm(t('items.confirmDel', { name: r.name }))) return
  try { await api.deleteItem(r.item_id); flash2(t('common.deleted')); load() } catch (e) { flash2(e.message) }
}

function flash2(text) { toast.value = text; setTimeout(() => { toast.value = '' }, 2000) }
</script>

<style scoped>
/* 组件样式统一来自 styles/base.css，这里只留本页特有的部分 */
.tbl { min-width: 760px; }
.id { color: var(--ink-3); font-size: var(--t-tag); }
.btn-sm + .btn-sm { margin-left: var(--s-1); }

/* 道具图标：内圆角取 --radius-xs，避免与容器同值造成「同心不同曲率」 */
.ic-img {
  width: 40px;
  height: 40px;
  object-fit: contain;
  border-radius: var(--radius-xs);
}

.ic-preview {
  display: block;
  margin-top: var(--s-2);
  width: 64px;
  height: 64px;
  object-fit: contain;
  border: 1px solid var(--line);
  border-radius: var(--radius-sm);
}
</style>
