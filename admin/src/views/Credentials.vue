<template>
  <div>
    <div class="toolbar">
      <button class="btn" @click="openCreate()">{{ t('credentials.addBtn') }}</button>
    </div>
    <!-- 按租户分组:没有任何凭证行的租户也要出现,否则新建的租户在这页根本找不到 -->
    <div v-for="g in groups" :key="g.tenant_id" class="tenant-group">
      <div class="tenant-hd">
        <span class="tenant-name">{{ g.name || t('common.dash') }}</span>
        <span class="badge" :class="g.type === 'app' ? 'blue' : 'gray'">{{ tenantTypeLabel(g.type) }}</span>
        <span class="mono tenant-id">{{ g.tenant_id }}</span>
        <button class="btn sm ghost" @click="openCreate(g.tenant_id, g.type)">{{ t('credentials.addForTenant') }}</button>
      </div>
      <div v-if="!g.rows.length" class="empty small">{{ t('credentials.noRows') }}</div>
    <div v-for="row in g.rows" :key="row.id" class="card">
      <div class="card-hd">
        <span class="plat" :class="row.platform">{{ platLabel(row.platform) }}</span>
        <span class="appid">{{ row.appid }}</span>
      </div>
      <div class="fields">
        <div class="field"><span class="lab">{{ t('credentials.loginSecret') }}</span><span :class="row.secret_set ? 'set' : 'unset'">{{ setLabel(row.secret_set) }}</span></div>
      </div>
      <!-- 支付字段已迁到「服务商 → 支付」页。留在这里编辑不会生效:
           支付链路只读 provider_configs,在这改 APIv3 密钥会显示「已设置」而回调仍用旧 key。 -->
      <p class="moved">{{ t('credentials.payMoved') }} <router-link to="/providers/pay">{{ t('credentials.payMovedLink') }}</router-link></p>
      <button class="btn" @click="openEdit(row)">{{ t('credentials.editBtn') }}</button>
    </div>
    </div>
    <div v-if="!groups.length && !loading" class="empty">{{ t('credentials.empty') }}</div>

    <div v-if="modal" class="overlay" @click.self="modal = null">
      <div class="dlg">
        <div class="dlg-hd">{{ t('credentials.editTitle', { platform: editing && platLabel(editing.platform) }) }}</div>
        <p class="tip">{{ t('credentials.tipLoginOnly') }}</p>
        <div class="field"><label>AppID</label><input v-model="form.appid" class="ipt" /></div>
        <div class="field"><label>{{ t('credentials.loginSecretEdit') }}</label><input v-model="form.secret" type="password" class="ipt" /></div>
        <div class="dlg-ft">
          <button class="btn ghost" @click="modal = null">{{ t('common.cancel') }}</button>
          <button class="btn" @click="save">{{ t('common.save') }}</button>
        </div>
        <div v-if="msg" class="err">{{ msg }}</div>
      </div>
    </div>

    <div v-if="createModal" class="overlay" @click.self="createModal = false">
      <div class="dlg">
        <div class="dlg-hd">{{ t('credentials.addTitle') }}</div>
        <div class="field"><label>{{ t('credentials.tenant') }}</label>
          <select v-model="createForm.tenant_id" class="ipt">
            <option v-for="tn in tenants" :key="tn.tenant_id" :value="tn.tenant_id">{{ tn.name }} ({{ tn.tenant_id }})</option>
          </select>
        </div>
        <div class="field"><label>{{ t('credentials.platform') }}</label>
          <select v-model="createForm.platform" class="ipt">
            <option v-for="p in platformOptions" :key="p" :value="p">{{ platLabel(p) }}</option>
          </select>
        </div>
        <div class="field"><label>AppID</label><input v-model="createForm.appid" class="ipt" /></div>
        <div class="dlg-ft">
          <button class="btn ghost" @click="createModal = false">{{ t('common.cancel') }}</button>
          <button class="btn" @click="create">{{ t('common.save') }}</button>
        </div>
        <div v-if="createMsg" class="err">{{ createMsg }}</div>
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

function setLabel(v) { return v ? t('credentials.isSet') : t('credentials.notSet') }

const list = ref([])
const loading = ref(false)
const modal = ref(false)
const editing = ref(null)
const form = ref({})
const msg = ref('')
const toast = ref('')

// 平台标签 / 分组。wx_app、alipay_app 是国内版 App 的开放平台凭证,字段形状与小程序同族。
const platformOptions = ['wx', 'alipay', 'app', 'wx_app', 'alipay_app']
const platformLabels = { wx: 'platformWx', alipay: 'platformAlipay', app: 'platformApp', wx_app: 'platformWxApp', alipay_app: 'platformAlipayApp' }
function platLabel(p) { return t('credentials.' + (platformLabels[p] || 'platformWx')) }
const isWx = (p) => p === 'wx' || p === 'wx_app'
const isAli = (p) => p === 'alipay' || p === 'alipay_app'

const tenants = ref([])
const createModal = ref(false)
const createForm = ref({ tenant_id: '', platform: 'wx_app', appid: '' })
const createMsg = ref('')
async function openCreate(tenantId, tenantType) {
  createMsg.value = ''
  if (!tenants.value.length) {
    try { tenants.value = await api.listTenants() || [] } catch (e) {}
  }
  createForm.value = {
    tenant_id: tenantId ? String(tenantId) : '',
    // App 租户默认建 wx_app(国内版第一步要配的就是它);小程序租户默认 wx
    platform: tenantType === 'app' ? 'wx_app' : 'wx',
    appid: '',
  }
  createModal.value = true
}
async function create() {
  try {
    await api.createCredential(createForm.value)
    createModal.value = false
    toast.value = t('credentials.created')
    setTimeout(() => { toast.value = '' }, 3000)
    load()
  } catch (e) { createMsg.value = e.message }
}

onMounted(load)

async function load() {
  loading.value = true
  try {
    const [rows, tns] = await Promise.all([api.getCredentials(), api.listTenants()])
    list.value = rows || []
    tenants.value = tns || []
  } catch (e) {}
  loading.value = false
}

function tenantTypeLabel(v) {
  return t(v === 'app' ? 'credentials.tenantTypeApp' : 'credentials.tenantTypeMiniProgram')
}

// 租户 → 其凭证行。凭证行引用了不存在的租户时也单独成组,别让它消失。
const groups = computed(() => {
  const byId = new Map()
  for (const tn of tenants.value) {
    byId.set(String(tn.tenant_id), { tenant_id: String(tn.tenant_id), name: tn.name, type: tn.type, rows: [] })
  }
  for (const row of list.value) {
    const id = String(row.tenant_id)
    if (!byId.has(id)) byId.set(id, { tenant_id: id, name: row.tenant_name, type: '', rows: [] })
    byId.get(id).rows.push(row)
  }
  return [...byId.values()]
})

function openEdit(row) {
  editing.value = row
  // 只改登录相关:支付字段归「服务商 → 支付」页,两处都能改等于两个真相源
  form.value = { appid: row.appid, secret: '' }
  modal.value = true
  msg.value = ''
}

async function save() {
  try {
    await api.updateCredential(editing.value.id, form.value)
    modal.value = false
    toast.value = t('credentials.saved')
    setTimeout(() => { toast.value = '' }, 3000)
    load()
  } catch (e) { msg.value = e.message }
}
</script>

<style scoped>
/* 组件样式统一来自 styles/base.css，这里只留本页特有的部分 */
.card { padding: var(--s-5); margin-bottom: var(--s-5); }

.card-hd {
  display: flex;
  align-items: center;
  gap: var(--s-3);
  margin-bottom: var(--s-4);
  flex-wrap: wrap;
}

/* 平台标记用中性徽章：微信绿、支付宝蓝是**第三方品牌色**，
   放进来会和「品牌色 = 可以点」「绿 = 通过」两条规则同时打架。
   文字本身已经写着是哪个平台了。 */
.plat {
  padding: 2px var(--s-2);
  border: 1px solid var(--line);
  border-radius: var(--radius-xs);
  background: var(--panel-3);
  color: var(--ink-2);
  font-size: var(--t-tag);
  font-weight: 500;
}

.appid {
  font-family: var(--f-mono);
  font-size: var(--t-value);
  color: var(--ink-2);
}

.tenant {
  margin-left: auto;
  font-size: var(--t-tag);
  color: var(--ink-3);
}

.toolbar {
  display: flex;
  justify-content: flex-end;
  margin-bottom: var(--s-4);
}

.moved {
  margin-top: var(--s-3);
  font-size: var(--t-tag);
  color: var(--ink-3);
}

.tenant-group { margin-bottom: var(--s-6); }

.tenant-hd {
  display: flex;
  align-items: center;
  gap: var(--s-3);
  padding: var(--s-2) 0;
  margin-bottom: var(--s-3);
  border-bottom: 1px solid var(--line);
}

.tenant-name { font-weight: 600; }

.tenant-id { color: var(--ink-3); font-size: var(--t-tag); }

.tenant-hd .btn { margin-left: auto; }

.empty.small { padding: var(--s-3) 0; font-size: var(--t-tag); }

.fields {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
  gap: var(--s-2);
  margin-bottom: var(--s-4);
}

/* 这一页的 .field 是「标签 : 值」的横排，不是表单里的竖排 */
.fields .field {
  flex-direction: row;
  gap: var(--s-2);
  font-size: var(--t-value);
  margin-bottom: 0;
}

.lab { color: var(--ink-2); min-width: 80px; }
.fields .field span:last-child { min-width: 0; overflow-wrap: anywhere; }

.ta { font-family: var(--f-mono); font-size: var(--t-label); }
.err { margin-top: var(--s-2); }
</style>
