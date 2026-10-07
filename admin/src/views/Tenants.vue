<template>
  <div>
    <div class="bar">
      <div class="hint">{{ t('tenants.hint') }}</div>
      <button class="btn" @click="openCreate">{{ t('tenants.addBtn') }}</button>
    </div>

    <div class="table-wrap">
    <table class="tbl">
      <thead>
        <tr>
          <th>{{ t('tenants.colId') }}</th>
          <th>{{ t('common.colName') }}</th>
          <th>{{ t('tenants.colType') }}</th>
          <th>AppID</th>
          <th>{{ t('common.status') }}</th>
          <th>{{ t('common.createdAt') }}</th>
          <th>{{ t('common.actions') }}</th>
        </tr>
      </thead>
      <tbody>
        <!-- 循环变量用 tn,不用 t —— 那是翻译函数 -->
        <tr v-for="tn in list" :key="tn.tenant_id">
          <td class="mono">{{ tn.tenant_id }}</td>
          <td>{{ tn.name }}</td>
          <td><span class="badge" :class="tn.type === 'app' ? 'blue' : 'gray'">{{ typeLabel(tn.type) }}</span></td>
          <td class="mono">
            <div v-if="tn.wx_appid">{{ t('tenants.wxPrefix') }} {{ tn.wx_appid }}</div>
            <div v-if="tn.alipay_appid">{{ t('tenants.alipayPrefix') }} {{ tn.alipay_appid }}</div>
            <div v-if="tn.app_appid">{{ t('tenants.appPrefix') }} {{ tn.app_appid }}</div>
            <span v-if="!tn.wx_appid && !tn.alipay_appid && !tn.app_appid" class="unset">{{ t('tenants.unset') }}</span>
          </td>
          <td><span :class="tn.status === 'active' ? 'set' : 'unset'">{{ tn.status }}</span></td>
          <td class="mono">{{ fmtTime(tn.created_at) }}</td>
          <td><button class="btn sm ghost" @click="openEdit(tn)">{{ t('common.edit') }}</button></td>
        </tr>
      </tbody>
    </table>
    </div>
    <div v-if="!list.length && !loading" class="empty">{{ t('tenants.empty') }}</div>

    <div v-if="modal" class="overlay" @click.self="modal = false">
      <div class="dlg">
        <div class="dlg-hd">{{ t('tenants.addTitle') }}</div>
        <p class="tip">{{ t('tenants.addTip') }}</p>
        <div class="field"><label>{{ t('tenants.nameLabel') }}</label><input v-model="form.name" class="ipt" :placeholder="t('tenants.namePlaceholder')" /></div>
        <div class="field">
          <label>{{ t('tenants.typeLabel') }}</label>
          <select v-model="form.type" class="sel">
            <option value="miniprogram">{{ t('tenants.typeMiniProgram') }}</option>
            <option value="app">{{ t('tenants.typeApp') }}</option>
          </select>
        </div>
        <template v-if="form.type !== 'app'">
          <div class="field"><label>{{ t('tenants.appidLabel') }}</label><input v-model="form.appid" class="ipt" placeholder="wx..." /></div>
          <div class="field"><label>{{ t('tenants.secretLabel') }}</label><input v-model="form.secret" type="password" class="ipt" :placeholder="t('tenants.secretPlaceholder')" /></div>
        </template>
        <template v-else>
          <!-- App 租户:appid 是 App 端 --dart-define=APP_ID 的值,落 platform=app 行,不需要 secret -->
          <div class="field"><label>{{ t('tenants.appAppidLabel') }}</label><input v-model="form.appid" class="ipt" placeholder="drift_app_cn" /></div>
          <p class="tip">{{ t('tenants.appHint') }}</p>
        </template>
        <div class="dlg-ft">
          <button class="btn ghost" @click="modal = false">{{ t('common.cancel') }}</button>
          <button class="btn" :disabled="saving" @click="save">{{ saving ? t('tenants.creating') : t('tenants.create') }}</button>
        </div>
        <div v-if="msg" class="err">{{ msg }}</div>
      </div>
    </div>

    <!-- 编辑租户 -->
    <div v-if="editModal" class="overlay" @click.self="editModal = false">
      <div class="dlg">
        <div class="dlg-hd">{{ t('tenants.editTitle') }} <span class="mono">{{ editForm.tenant_id }}</span></div>
        <p class="tip">{{ t('tenants.editTip') }}</p>
        <div class="field"><label>{{ t('tenants.nameLabel') }}</label><input v-model="editForm.name" class="ipt" /></div>
        <div class="field">
          <label>{{ t('tenants.typeLabel') }}</label>
          <select v-model="editForm.type" class="sel">
            <option value="miniprogram">{{ t('tenants.typeMiniProgram') }}</option>
            <option value="app">{{ t('tenants.typeApp') }}</option>
          </select>
        </div>
        <div class="field"><label>{{ t('common.status') }}</label>
          <select v-model="editForm.status" class="ipt">
            <option value="active">{{ t('tenants.statusActive') }}</option>
            <option value="disabled">{{ t('tenants.statusDisabled') }}</option>
          </select>
        </div>
        <template v-if="editForm.type !== 'app'">
          <div class="field"><label>{{ t('tenants.appidLabel') }}</label><input v-model="editForm.appid" class="ipt" placeholder="wx..." /></div>
          <div class="field"><label>{{ t('tenants.secretLabel') }}</label><input v-model="editForm.secret" type="password" class="ipt" :placeholder="t('tenants.secretKeepPlaceholder')" /></div>
        </template>
        <template v-else>
          <div class="field"><label>{{ t('tenants.appAppidLabel') }}</label><input v-model="editForm.appid" class="ipt" placeholder="drift_app_cn" /></div>
          <p class="tip">{{ t('tenants.appHint') }}</p>
        </template>
        <div class="dlg-ft">
          <button class="btn ghost" @click="editModal = false">{{ t('common.cancel') }}</button>
          <button class="btn" :disabled="saving" @click="saveEdit">{{ saving ? t('common.saving') : t('common.save') }}</button>
        </div>
        <div v-if="msg" class="err">{{ msg }}</div>
      </div>
    </div>

    <div v-if="toast" class="gtoast">{{ toast }}</div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api'

const { t } = useI18n()
const list = ref([])
const loading = ref(false)
const modal = ref(false)
const saving = ref(false)
const form = ref({ name: '', type: 'miniprogram', appid: '', secret: '' })
const msg = ref('')
const toast = ref('')

// 类型的**值**存库,不翻;只翻显示名
function typeLabel(v) {
  return t(v === 'app' ? 'tenants.typeApp' : 'tenants.typeMiniProgram')
}

onMounted(load)

async function load() {
  loading.value = true
  try { list.value = await api.listTenants() || [] } catch (e) {}
  loading.value = false
}

function openCreate() {
  form.value = { name: '', type: 'miniprogram', appid: '', secret: '' }
  msg.value = ''
  modal.value = true
}

const editModal = ref(false)
const editForm = ref({ tenant_id: '', name: '', type: 'miniprogram', status: 'active', appid: '', secret: '' })

// 参数用 tn 不用 t —— 那是翻译函数
function openEdit(tn) {
  editForm.value = {
    tenant_id: String(tn.tenant_id), name: tn.name,
    type: tn.type || 'miniprogram', status: tn.status,
    appid: (tn.type === 'app' ? tn.app_appid : tn.wx_appid) || '', secret: ''
  }
  msg.value = ''
  editModal.value = true
}

async function saveEdit() {
  msg.value = ''
  saving.value = true
  try {
    const f = editForm.value
    await api.updateTenant(f.tenant_id, {
      name: f.name, type: f.type, status: f.status, appid: f.appid, secret: f.secret,
    })
    editModal.value = false
    toast.value = t('tenants.updated')
    setTimeout(() => { toast.value = '' }, 3000)
    load()
  } catch (e) { msg.value = e.message } finally { saving.value = false }
}

async function save() {
  msg.value = ''
  saving.value = true
  try {
    await api.createTenant(form.value)
    modal.value = false
    toast.value = t('tenants.created')
    setTimeout(() => { toast.value = '' }, 3000)
    load()
  } catch (e) { msg.value = e.message } finally { saving.value = false }
}

function fmtTime(ts) { return ts ? String(ts).slice(0, 19).replace('T', ' ') : '' }
</script>

<style scoped>
/* 组件样式统一来自 styles/base.css，这里只留本页特有的部分 */
.bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--s-4);
  margin-bottom: var(--s-4);
  flex-wrap: wrap;
}

.tbl { min-width: 560px; }
.err { margin-top: var(--s-2); }
</style>
