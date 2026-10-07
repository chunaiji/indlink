<template>
  <div>
    <div class="toolbar">
      <button class="btn" @click="openAdd">{{ t('packages.addBtn') }}</button>
      <span class="hint">{{ t('packages.hint') }}</span>
    </div>

    <div class="table-wrap">
    <table class="tbl">
      <thead>
        <tr>
          <th>ID</th>
          <th>{{ t('common.colName') }}</th>
          <th>{{ t('packages.colCoins') }}</th>
          <th>{{ t('packages.colBonus') }}</th>
          <th>{{ t('packages.colPrice') }}</th>
          <th>{{ t('packages.colIOSProduct') }}</th>
          <th>{{ t('common.colSort') }}</th>
          <th>{{ t('common.status') }}</th>
          <th>{{ t('common.actions') }}</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="r in list" :key="r.package_id">
          <td class="id">{{ r.package_id }}</td>
          <td>{{ r.name }}</td>
          <td>{{ r.coins }}</td>
          <td class="soft">{{ r.bonus_coins }}</td>
          <td>¥{{ (r.price_fen / 100).toFixed(2) }}</td>
          <!-- 空着就标出来:iOS 内购按它反查档位,没配等于这一档在 iOS 上收不了钱 -->
          <td>
            <span v-if="r.ios_product_id" class="mono small">{{ r.ios_product_id }}</span>
            <span v-else class="badge orange">{{ t('packages.iosProductMissing') }}</span>
          </td>
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
    <div v-if="!list.length && !loading" class="empty">{{ t('packages.empty') }}</div>

    <!-- 新增/编辑弹窗 -->
    <div v-if="modal" class="overlay" @click.self="modal = null">
      <div class="dlg">
        <div class="dlg-hd">{{ editing ? t('packages.editTitle') : t('packages.addTitle') }}</div>
        <div class="field">
          <label>{{ t('common.colName') }}</label>
          <input v-model="form.name" class="ipt" :placeholder="t('packages.namePlaceholder')" />
        </div>
        <div class="field">
          <label>{{ t('packages.colCoins') }}</label>
          <input v-model.number="form.coins" type="number" class="ipt" :placeholder="t('packages.coinsPlaceholder')" />
        </div>
        <div class="field">
          <label>{{ t('packages.bonusLabel') }}</label>
          <input v-model.number="form.bonus_coins" type="number" class="ipt" :placeholder="t('packages.bonusPlaceholder')" />
        </div>
        <div class="field">
          <label>{{ t('packages.priceLabel') }}</label>
          <input v-model.number="form.priceYuan" type="number" class="ipt" :placeholder="t('packages.pricePlaceholder')" />
        </div>
        <!-- iap.go 按 ios_product_id 反查档位,留空则 iOS 内购永远入账失败 -->
        <div class="field">
          <label>{{ t('packages.colIOSProduct') }}</label>
          <input v-model="form.ios_product_id" class="ipt" placeholder="com.ambertu.bottles.coins60" />
          <span class="hint">{{ t('packages.iosProductHint') }}</span>
        </div>
        <div class="field">
          <label>{{ t('packages.colPlayProduct') }}</label>
          <input v-model="form.play_product_id" class="ipt" placeholder="coins_60" />
          <span class="hint">{{ t('packages.playProductHint') }}</span>
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
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api'

const { t } = useI18n()
const list = ref([])
const loading = ref(false)
const modal = ref(null)
const editing = ref(null)
const form = ref({ name: '', coins: 0, bonus_coins: 0, priceYuan: 0, ios_product_id: '', play_product_id: '', sort: 0, status: 'active' })
const msg = ref('')
const toast = ref('')

onMounted(load)

async function load() {
  loading.value = true
  try { list.value = await api.listPackages() || [] } catch (e) { flash2(e.message) }
  loading.value = false
}

function openAdd() {
  editing.value = null
  form.value = { name: '', coins: 0, bonus_coins: 0, priceYuan: 0, ios_product_id: '', play_product_id: '', sort: 0, status: 'active' }
  modal.value = true
  msg.value = ''
}

function openEdit(r) {
  editing.value = r
  form.value = {
    name: r.name, coins: r.coins, bonus_coins: r.bonus_coins,
    priceYuan: r.price_fen / 100,
    ios_product_id: r.ios_product_id || '', play_product_id: r.play_product_id || '',
    sort: r.sort, status: r.status
  }
  modal.value = true
  msg.value = ''
}

async function submit() {
  const f = form.value
  if (!f.name.trim()) { msg.value = t('common.errName'); return }
  if (!(f.coins > 0)) { msg.value = t('packages.errCoins'); return }
  if (!(f.priceYuan > 0)) { msg.value = t('packages.errPrice'); return }
  const payload = {
    name: f.name.trim(),
    coins: Math.round(f.coins),
    bonus_coins: Math.round(f.bonus_coins || 0),
    price_fen: Math.round(f.priceYuan * 100),
    ios_product_id: (f.ios_product_id || '').trim(),
    play_product_id: (f.play_product_id || '').trim(),
    sort: Math.round(f.sort || 0),
    status: f.status
  }
  try {
    if (editing.value) await api.updatePackage(editing.value.package_id, payload)
    else await api.createPackage(payload)
    modal.value = null
    flash2(editing.value ? t('common.updated') : t('common.added'))
    load()
  } catch (e) { msg.value = e.message }
}

async function del(r) {
  if (!confirm(t('packages.confirmDel', { name: r.name }))) return
  try { await api.deletePackage(r.package_id); flash2(t('common.deleted')); load() } catch (e) { flash2(e.message) }
}

function flash2(text) { toast.value = text; setTimeout(() => { toast.value = '' }, 2000) }
</script>

<style scoped>
/* 组件样式统一来自 styles/base.css，这里只留本页特有的部分 */
.tbl { min-width: 760px; }
.id { color: var(--ink-3); font-size: var(--t-tag); }
.btn-sm + .btn-sm { margin-left: var(--s-1); }
.small { font-size: var(--t-tag); }
</style>
