<template>
  <div>
    <div class="toolbar">
      <span class="total">{{ t('robotProfile.total', { n: list.length }) }}</span>
      <span style="flex:1"></span>
      <button class="btn" @click="openCreate">{{ t('robotProfile.addBtn') }}</button>
    </div>

    <div class="table-wrap">
    <table class="tbl">
      <thead>
        <tr>
          <th>{{ t('robotProfile.colNickname') }}</th>
          <th>{{ t('robotProfile.colGender') }}</th>
          <th>{{ t('robotProfile.colAge') }}</th>
          <th>{{ t('robotProfile.colCity') }}</th>
          <th>{{ t('robotProfile.colLanguage') }}</th>
          <th>{{ t('robotProfile.colInterests') }}</th>
          <th>{{ t('common.status') }}</th>
          <th>{{ t('robotProfile.colPersona') }}</th>
          <th>{{ t('common.actions') }}</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="r in pagedList" :key="r.user_id">
          <td>
            <div class="user-cell">
              <img v-if="r.avatar" :src="r.avatar" class="av" />
              <span v-else class="av-ph bot">🤖</span>
              <div>
                <div class="nick">{{ r.nickname }}</div>
                <div class="uid">ID: {{ r.user_id }}</div>
              </div>
            </div>
          </td>
          <td class="soft">{{ genderLabel(r.gender) }}</td>
          <td class="soft">{{ r.age || t('common.dash') }}</td>
          <td class="soft">{{ r.city || t('common.dash') }}</td>
          <td class="soft">{{ r.language || t('common.dash') }}</td>
          <td>
            <!-- 兴趣存的是 key(music/travel…),显示名按界面语言翻;未知 key 原样显示 -->
            <span v-for="k in splitCsv(r.interests)" :key="k" class="badge gray" style="margin-right:4px">{{ interestLabel(k) }}</span>
            <span v-if="!splitCsv(r.interests).length" class="soft">{{ t('common.dash') }}</span>
          </td>
          <td>
            <span class="badge" :class="r.status === 'active' ? 'st-active' : 'st-paused'">
              {{ r.status === 'active' ? t('robotProfile.statusActive') : t('robotProfile.statusPaused') }}
            </span>
          </td>
          <td class="soft">{{ r.persona_name || t('robotProfile.defaultPersona') }}</td>
          <td>
            <button class="btn-sm" @click="openEdit(r)">{{ t('robotProfile.editBtn') }}</button>
          </td>
        </tr>
      </tbody>
    </table>
    </div>
    <div v-if="!list.length && !loading" class="empty">{{ t('robotProfile.empty') }}</div>

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

    <!-- 新增机器人:选语言,昵称/城市/兴趣/简介按语言生成 -->
    <div v-if="createModal" class="overlay" @click.self="createModal = false">
      <div class="dlg">
        <div class="dlg-hd">{{ t('robotProfile.createTitle') }}</div>
        <div class="field">
          <label>{{ t('robotProfile.countLabel') }}</label>
          <input v-model.number="createForm.count" type="number" min="1" max="50" class="ipt full" />
        </div>
        <div class="field">
          <label>{{ t('robotProfile.langLabel') }}</label>
          <select v-model="createForm.language" class="sel">
            <option value="zh">{{ t('robotProfile.langZh') }}</option>
            <option value="en">{{ t('robotProfile.langEn') }}</option>
          </select>
        </div>
        <div class="field">
          <label>{{ t('robotProfile.colGender') }}</label>
          <select v-model.number="createForm.gender" class="sel">
            <option :value="0">{{ t('robotProfile.genderRandom') }}</option>
            <option :value="1">{{ t('robotProfile.male') }}</option>
            <option :value="2">{{ t('robotProfile.female') }}</option>
          </select>
        </div>
        <div class="field">
          <label>{{ t('robotProfile.cityLabel') }}</label>
          <input v-model="createForm.city" class="ipt full" :placeholder="t('robotProfile.cityPlaceholder')" />
        </div>
        <div class="hint">{{ t('robotProfile.createHint') }}</div>
        <div class="dlg-ft">
          <button class="btn ghost" @click="createModal = false">{{ t('common.cancel') }}</button>
          <button class="btn" :disabled="creating || !createValid" @click="doCreate">
            {{ creating ? t('robotProfile.creating') : t('robotProfile.createBtn') }}
          </button>
        </div>
        <div v-if="msg" class="toast">{{ msg }}</div>
      </div>
    </div>

    <!-- 编辑机器人资料 + 绑定人格 -->
    <div v-if="modal" class="overlay" @click.self="modal = null">
      <div class="dlg dlg-wide">
        <div class="dlg-hd">{{ t('robotProfile.editTitle') }}</div>
        <div class="grid2">
          <div class="field">
            <label>{{ t('robotProfile.nicknameLabel') }}</label>
            <input v-model="form.nickname" class="ipt full" maxlength="32" />
          </div>
          <div class="field">
            <label>{{ t('robotProfile.colGender') }}</label>
            <select v-model.number="form.gender" class="sel">
              <option :value="1">{{ t('robotProfile.male') }}</option>
              <option :value="2">{{ t('robotProfile.female') }}</option>
            </select>
          </div>
          <div class="field">
            <label>{{ t('robotProfile.ageLabel') }}</label>
            <input v-model.number="form.age" type="number" min="18" max="80" class="ipt full" />
          </div>
          <div class="field">
            <label>{{ t('robotProfile.colCity') }}</label>
            <input v-model="form.city" class="ipt full" maxlength="32" />
          </div>
        </div>
        <div class="field">
          <label>{{ t('robotProfile.colLanguage') }}</label>
          <div class="checks">
            <label v-for="lang in LANGS" :key="lang" class="chk">
              <input type="checkbox" :value="lang" v-model="form.languages" /> {{ lang }}
            </label>
          </div>
        </div>
        <div class="field">
          <label>{{ t('robotProfile.interestsLabel') }}</label>
          <div class="checks">
            <label v-for="k in INTERESTS" :key="k" class="chk">
              <input type="checkbox" :value="k" v-model="form.interests" /> {{ interestLabel(k) }}
            </label>
          </div>
        </div>
        <div class="field">
          <label>{{ t('robotProfile.bioLabel') }}</label>
          <textarea v-model="form.bio" class="ipt full" rows="2" maxlength="200"></textarea>
        </div>
        <div class="grid2">
          <div class="field">
            <label>{{ t('robotProfile.colPersona') }}</label>
            <select v-model="form.persona_id" class="sel">
              <option :value="null">{{ t('robotProfile.noBind') }}</option>
              <option v-for="p in personas" :key="p.persona_id" :value="p.persona_id">{{ p.name }}</option>
            </select>
          </div>
          <div class="field">
            <label>{{ t('common.status') }}</label>
            <select v-model="form.status" class="sel">
              <option value="active">{{ t('robotProfile.statusActive') }}</option>
              <option value="paused">{{ t('robotProfile.statusPaused') }}</option>
            </select>
          </div>
        </div>
        <div class="dlg-ft">
          <button class="btn ghost" @click="modal = null">{{ t('common.cancel') }}</button>
          <button class="btn" :disabled="saving" @click="submit">{{ saving ? t('common.saving') : t('common.save') }}</button>
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

// 语言取值与 App 资料页一致(自称),机器人填别的值发现页匹配不上
const LANGS = ['中文', 'English']
// 兴趣 key 与 App Catalog.interests 一致;显示名走 i18n
const INTERESTS = ['music', 'movie', 'travel', 'cricket', 'food', 'art', 'photo', 'reading', 'fitness', 'gaming']

const list = ref([])
const personas = ref([])
const loading = ref(false)
const page = ref(1)
const size = ref(20)
const totalPages = computed(() => Math.max(1, Math.ceil(list.value.length / size.value)))
const pagedList = computed(() => list.value.slice((page.value - 1) * size.value, page.value * size.value))
function goPage(p) { page.value = p }
function onSizeChange() { page.value = 1 }

const modal = ref(null)
const editing = ref(null)
const form = ref({})
const saving = ref(false)

const createModal = ref(false)
const creating = ref(false)
const createForm = ref({ count: 5, language: 'zh', gender: 0, city: '' })
const createValid = computed(() => Number.isInteger(createForm.value.count) && createForm.value.count >= 1 && createForm.value.count <= 50)

const msg = ref('')
const toast = ref('')

function genderLabel(v) {
  return t(v === 1 ? 'robotProfile.male' : v === 2 ? 'robotProfile.female' : 'robotProfile.unknown')
}
function splitCsv(s) { return (s || '').split(',').map(x => x.trim()).filter(Boolean) }
function interestLabel(k) { return INTERESTS.includes(k) ? t('robotProfile.interest_' + k) : k }

onMounted(load)

async function load() {
  loading.value = true
  try {
    const [profiles, ps] = await Promise.all([api.listRobotProfiles(), api.listPersonas()])
    list.value = profiles || []
    personas.value = ps || []
  } catch (e) { flash2(e.message) }
  loading.value = false
}

function openCreate() {
  createForm.value = { count: 5, language: 'zh', gender: 0, city: '' }
  createModal.value = true
  msg.value = ''
}

async function doCreate() {
  if (!createValid.value) return
  creating.value = true
  try {
    const res = await api.createRobots({ ...createForm.value, city: createForm.value.city.trim() })
    createModal.value = false
    flash2(t('robotProfile.created', { n: res.created }))
    load()
  } catch (e) { msg.value = e.message }
  creating.value = false
}

function openEdit(r) {
  editing.value = r
  form.value = {
    nickname: r.nickname || '',
    gender: r.gender === 1 ? 1 : 2,
    age: r.age || 18,
    city: r.city || '',
    languages: splitCsv(r.language),
    interests: splitCsv(r.interests),
    bio: r.bio || '',
    persona_id: r.persona_id || null,
    status: r.status,
  }
  modal.value = true
  msg.value = ''
}

async function submit() {
  saving.value = true
  try {
    const f = form.value
    await api.updateRobotProfile(editing.value.user_id, {
      persona_id: f.persona_id || null,
      status: f.status,
      nickname: f.nickname.trim(),
      gender: f.gender,
      age: f.age,
      city: f.city.trim(),
      language: f.languages.join(','),
      interests: f.interests.join(','),
      bio: f.bio.trim(),
    })
    modal.value = null
    flash2(t('common.updated'))
    load()
  } catch (e) { msg.value = e.message }
  saving.value = false
}

function flash2(text) { toast.value = text; setTimeout(() => { toast.value = '' }, 2000) }
</script>

<style scoped>
/* 组件样式统一来自 styles/base.css，这里只留本页特有的部分 */
.tbl { min-width: 900px; }
.btn-sm + .btn-sm { margin-left: var(--s-1); }
.dlg-wide { width: min(640px, 92vw); }
.grid2 { display: grid; grid-template-columns: 1fr 1fr; gap: 0 var(--s-3); }
.checks { display: flex; flex-wrap: wrap; gap: var(--s-1) var(--s-3); padding: 6px 0; }
.chk { display: inline-flex; align-items: center; gap: 4px; font-size: var(--t-value); color: var(--ink-2); cursor: pointer; }
.hint { font-size: var(--t-label); color: var(--ink-3); margin: 0 0 var(--s-2); }
.user-cell { display: flex; align-items: center; gap: 8px; }
.user-cell .av { width: 32px; height: 32px; border-radius: 50%; object-fit: cover; }
.user-cell .av-ph { width: 32px; height: 32px; border-radius: 50%; display: inline-flex; align-items: center; justify-content: center; background: var(--panel-3); }
.user-cell .uid { font-size: var(--t-label); color: var(--ink-3); }
</style>
