<template>
  <div>
    <p class="tip">{{ t('providers.hint.' + kind) }}</p>
    <div v-if="!tenantStore.currentTenantID" class="empty">{{ t('providers.needTenant') }}</div>
    <template v-else>
      <div v-for="card in cards" :key="card.provider" class="card pcard" :class="{ off: !card.enabled }">
        <div class="card-hd">
          <span class="pname">{{ card.label }}</span>
          <span class="badge" :class="card.complete ? 'green' : 'gray'">
            {{ card.complete ? t('providers.complete') : t('providers.missing', { n: card.missing.length }) }}
          </span>
          <span v-if="single && card.active" class="badge blue">{{ t('providers.current') }}</span>
          <a v-if="card.doc_url" class="doc" :href="card.doc_url" target="_blank" rel="noopener">{{ t('providers.apply') }}</a>
          <label class="switch">
            <input type="checkbox" :checked="card.enabled" @change="toggleEnabled(card, $event.target.checked)" />
            <span>{{ t('providers.enabled') }}</span>
          </label>
          <button v-if="single && !card.active" class="btn sm ghost" :disabled="!card.enabled" @click="setActive(card)">
            {{ t('providers.setCurrent') }}
          </button>
        </div>
        <div class="fields">
          <!-- 循环变量用 fd,不用 t —— 那是翻译函数 -->
          <div v-for="fd in card.fields" :key="fd.key" class="field" :class="{ wide: fd.type === 'textarea' }">
            <label>{{ fd.label }}<span v-if="fd.required" class="req">*</span></label>
            <select v-if="fd.type === 'bool'" v-model="draft[card.provider][fd.key]" class="ipt">
              <option value="1">{{ t('common.on') }}</option>
              <option value="0">{{ t('common.off') }}</option>
            </select>
            <textarea
              v-else-if="fd.type === 'textarea'"
              v-model="draft[card.provider][fd.key]"
              class="ta"
              rows="4"
              :placeholder="fd.secret && fd.value === 'set' ? t('providers.secretSet') : ''"
            ></textarea>
            <input
              v-else
              v-model="draft[card.provider][fd.key]"
              class="ipt"
              :type="fd.secret ? 'password' : 'text'"
              :placeholder="fd.secret && fd.value === 'set' ? t('providers.secretSet') : ''"
            />
            <small v-if="fd.help" class="help">{{ fd.help }}</small>
          </div>
        </div>
        <div class="card-ft">
          <button class="btn" :disabled="saving === card.provider" @click="save(card)">{{ t('common.save') }}</button>
          <button
            class="btn ghost"
            :disabled="probing === card.provider || !card.complete || (single && !card.active)"
            @click="probe(card)"
          >{{ t('providers.test') }}</button>
          <span v-if="result[card.provider]" :class="result[card.provider].ok ? 'set' : 'unset'">
            {{ result[card.provider].message }}
          </span>
        </div>
      </div>
      <div v-if="!cards.length && !loading" class="empty">{{ t('providers.none') }}</div>
    </template>
    <div v-if="toast" class="gtoast">{{ toast }}</div>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { api } from '../api'
import { tenantStore } from '../tenant.js'

const { t } = useI18n()
const route = useRoute()
const kind = computed(() => route.params.kind)
// 地图与内容安全是「同一时刻只用一家」,支付可多家并存
const single = computed(() => kind.value === 'map' || kind.value === 'moderation')

const cards = ref([])
const draft = ref({}) // provider -> { key: value };机密字段初始为空串(= 不改)
const loading = ref(false)
const saving = ref('')
const probing = ref('')
const result = ref({})
const toast = ref('')

function flash(msg) {
  toast.value = msg
  setTimeout(() => { toast.value = '' }, 3000)
}

async function load() {
  if (!tenantStore.currentTenantID) { cards.value = []; return }
  loading.value = true
  try {
    const d = await api.getProviders(kind.value)
    cards.value = d.cards || []
    const next = {}
    for (const c of cards.value) {
      next[c.provider] = {}
      for (const fd of c.fields) next[c.provider][fd.key] = fd.secret ? '' : fd.value
    }
    draft.value = next
    result.value = {}
  } catch (e) { flash(e.message) } finally { loading.value = false }
}

onMounted(load)
watch([kind, () => tenantStore.currentTenantID], load)

async function save(card) {
  saving.value = card.provider
  try {
    await api.saveProvider(kind.value, card.provider, { fields: draft.value[card.provider] })
    flash(t('providers.saved'))
    await load()
  } catch (e) { flash(e.message) } finally { saving.value = '' }
}

async function toggleEnabled(card, enabled) {
  try {
    await api.saveProvider(kind.value, card.provider, { enabled, fields: {} })
    await load()
  } catch (e) { flash(e.message); await load() }
}

async function setActive(card) {
  try {
    await api.saveProvider(kind.value, card.provider, { active: true, fields: {} })
    await load()
  } catch (e) { flash(e.message) }
}

async function probe(card) {
  probing.value = card.provider
  try {
    const r = await api.probeProvider(kind.value, card.provider)
    result.value = { ...result.value, [card.provider]: r }
  } catch (e) {
    result.value = { ...result.value, [card.provider]: { ok: false, message: e.message } }
  } finally { probing.value = '' }
}
</script>

<style scoped>
.pcard { padding: var(--s-5); margin-bottom: var(--s-5); }
.pcard.off { opacity: .75; }

.card-hd {
  display: flex;
  align-items: center;
  gap: var(--s-3);
  flex-wrap: wrap;
  margin-bottom: var(--s-4);
}

.pname { font-weight: 650; font-size: var(--t-3); }
.doc { font-size: var(--t-tag); color: var(--brand-text); }

.switch {
  margin-left: auto;
  display: flex;
  align-items: center;
  gap: var(--s-2);
  font-size: var(--t-tag);
}

.fields {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: var(--s-3) var(--s-5);
}

.field.wide { grid-column: 1 / -1; }

.field label {
  display: block;
  font-size: var(--t-tag);
  color: var(--ink-2);
  margin-bottom: 4px;
}

.req { color: var(--danger); margin-left: 2px; }

.help {
  display: block;
  color: var(--ink-3);
  font-size: var(--t-tag);
  margin-top: 2px;
}

.card-ft {
  display: flex;
  align-items: center;
  gap: var(--s-3);
  margin-top: var(--s-4);
}
</style>
