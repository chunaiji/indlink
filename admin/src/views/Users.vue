<template>
  <div>
    <!-- 搜索栏 -->
    <div class="toolbar">
      <input v-model="keyword" class="ipt" :placeholder="t('users.searchPlaceholder')" @keydown.enter="doSearch" />
      <input v-model="tagFilter" class="ipt tag-ipt" :placeholder="t('users.tagPlaceholder')" @keydown.enter="doSearch" />
      <select v-model="googleFilter" class="sel">
        <option value="">{{ t('users.googleAny') }}</option>
        <option value="bound">{{ t('users.googleBound') }}</option>
        <option value="unbound">{{ t('users.googleUnbound') }}</option>
      </select>
      <select v-model="robotFilter" class="sel">
        <option value="">{{ t('common.allTypes') }}</option>
        <option value="human">{{ t('users.typeHuman') }}</option>
        <option value="robot">{{ t('users.typeRobot') }}</option>
        <option value="test">{{ t('users.typeTest') }}</option>
      </select>
      <select v-model="statusFilter" class="sel">
        <option value="">{{ t('common.allStatus') }}</option>
        <option value="active">{{ t('users.statusActive') }}</option>
        <option value="banned">{{ t('users.statusBanned') }}</option>
        <option value="muted">{{ t('users.statusMuted') }}</option>
        <option value="deleted">{{ t('users.statusDeleted') }}</option>
      </select>
      <button class="btn" @click="doSearch">{{ t('common.search') }}</button>
      <button class="btn ghost" @click="reset">{{ t('common.reset') }}</button>
      <span class="total">{{ t('users.totalPeople', { n: total }) }}</span>
    </div>

    <!-- 用户表格 -->
    <div class="table-wrap">
      <table class="tbl">
        <thead>
          <tr>
            <th>{{ t('common.colUser') }}</th>
            <th>{{ t('users.colType') }}</th>
            <th>OpenID</th>
            <th>{{ t('users.colEmail') }}</th>
            <th>Google</th>
            <th>Apple</th>
            <th>{{ t('users.colGender') }}</th>
            <th>{{ t('users.colAge') }}</th>
            <th>{{ t('users.colCity') }}</th>
            <th>{{ t('users.colTags') }}</th>
            <th>{{ t('users.colTotalCoins') }}</th>
            <th>{{ t('users.colBalance') }}</th>
            <th>{{ t('users.colOnline') }}</th>
            <th>{{ t('common.status') }}</th>
            <th>{{ t('users.colMuted') }}</th>
            <th>{{ t('users.colLastActive') }}</th>
            <th>{{ t('common.actions') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="u in list" :key="u.user_id" :class="{ 'row-bot': u.is_robot }">
            <td>
              <div class="user-cell">
                <img v-if="u.avatar" :src="u.avatar" class="av" />
                <span v-else class="av-ph" :class="u.is_robot ? 'bot' : ''">
                  {{ u.is_robot ? '🤖' : (u.nickname || '?')[0] }}
                </span>
                <div>
                  <div class="nick">{{ u.nickname || '—' }}</div>
                  <div class="uid">ID: {{ u.user_id }}</div>
                </div>
              </div>
            </td>
            <td>
              <span class="badge" :class="u.is_robot ? 'purple' : 'blue'">
                {{ u.is_robot ? t('users.badgeRobot') : t('users.badgeHuman') }}
              </span>
              <span v-if="isTest(u)" class="badge orange" style="margin-left:4px">{{ t('users.badgeTest') }}</span>
            </td>
            <td>
              <span v-if="u.openid" class="openid" :title="u.openid">{{ u.openid }}</span>
              <span v-else class="badge purple">{{ t('users.emptyOpenid') }}</span>
            </td>
            <td>
              <span v-if="u.email" class="openid" :title="u.email">{{ u.email }}</span>
              <span v-else>{{ t('common.dash') }}</span>
            </td>
            <td>
              <span class="badge" :class="u.google_bound ? 'green' : 'gray'">
                {{ u.google_bound ? t('users.googleBound') : t('users.googleUnbound') }}
              </span>
            </td>
            <td>
              <span class="badge" :class="u.apple_bound ? 'green' : 'gray'">
                {{ u.apple_bound ? t('users.googleBound') : t('users.googleUnbound') }}
              </span>
            </td>
            <td>{{ genderLabel(u.gender) }}</td>
            <td>{{ u.age || '—' }}</td>
            <td>{{ u.city || '—' }}</td>
            <td>
              <!-- 「付费用户」是库里的标签值,不是界面文案 —— 翻了就对不上,徽章会失效 -->
              <span v-for="tag in tagList(u.tags)" :key="tag" class="badge" :class="tag === '付费用户' ? 'gold' : 'gray'" style="margin-right:4px">{{ tag }}</span>
              <span v-if="!tagList(u.tags).length">{{ t('common.dash') }}</span>
            </td>
            <td>{{ u.total_recharged || 0 }}</td>
            <td>{{ u.balance || 0 }}</td>
            <td>
              <span class="badge" :class="u.online ? 'green' : 'gray'">{{ u.online ? t('users.online') : t('users.offline') }}</span>
            </td>
            <td>
              <span class="badge" :class="u.status === 'banned' ? 'red' : u.status === 'deleted' ? 'gray' : 'green'">
                {{ u.status === 'banned' ? t('users.statusBanned') : u.status === 'deleted' ? t('users.statusDeleted') : t('users.statusActive') }}
              </span>
            </td>
            <td>
              <span class="badge" :class="u.is_muted ? 'orange' : 'gray'">
                {{ u.is_muted ? t('users.muting') : t('users.statusActive') }}
              </span>
            </td>
            <td>{{ shortTime(u.last_active_at) }}</td>
            <td>
              <div class="acts">
                <button class="btn sm ghost" @click="openTags(u)">{{ t('users.btnTags') }}</button>
                <template v-if="!u.is_robot">
                  <button
                    class="btn sm"
                    :class="u.status === 'banned' ? 'ghost' : 'danger'"
                    @click="toggleBan(u)"
                    :disabled="actLoading === u.user_id"
                  >{{ u.status === 'banned' ? t('users.btnUnban') : t('users.btnBan') }}</button>
                  <button
                    class="btn sm"
                    :class="u.is_muted ? 'ghost' : 'warn'"
                    @click="toggleMute(u)"
                    :disabled="actLoading === u.user_id"
                  >{{ u.is_muted ? t('users.btnUnmute') : t('users.btnMute') }}</button>
                  <button class="btn sm ghost" @click="openCoins(u)">{{ t('users.btnCoins') }}</button>
                  <button class="btn sm" @click="openChat(u)">{{ t('users.btnStartChat') }}</button>
                  <button class="btn ghost sm" @click="openPush(u)">{{ t('users.btnPush') }}</button>
                </template>
              </div>
            </td>
          </tr>
          <tr v-if="!list.length && !loading">
            <td colspan="17" class="empty">{{ t('users.emptyUsers') }}</td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- 分页 -->
    <div class="pager">
      <button class="btn ghost sm" :disabled="page <= 1" @click="goPage(page - 1)">{{ t('common.prev') }}</button>
      <span class="pg">{{ t('common.pageInfo', { page, pages: totalPages, total }) }}</span>
      <button class="btn ghost sm" :disabled="page >= totalPages" @click="goPage(page + 1)">{{ t('common.next') }}</button>
      <select v-model="size" @change="doSearch" class="sel-pg">
        <option :value="20">{{ t('common.perPage', { n: 20 }) }}</option>
        <option :value="50">{{ t('common.perPage', { n: 50 }) }}</option>
        <option :value="100">{{ t('common.perPage', { n: 100 }) }}</option>
      </select>
    </div>

    <!-- 标签编辑弹框 -->
    <div v-if="tagModal" class="modal-mask" @click.self="tagModal = null">
      <div class="modal">
        <div class="modal-hd">{{ t('users.tagModalTitle', { name: tagModal.nickname || tagModal.user_id }) }}</div>
        <div class="modal-body">
          <div class="tag-editor">
            <!-- 同上:「付费用户」是数据值,不翻 -->
            <span v-for="(tag, i) in tagDraft" :key="tag" class="badge" :class="tag === '付费用户' ? 'gold' : 'gray'">
              {{ tag }} <span class="tag-x" @click="tagDraft.splice(i, 1)">×</span>
            </span>
            <span v-if="!tagDraft.length" class="no-act">{{ t('users.noTags') }}</span>
          </div>
          <div class="form-row">
            <label>{{ t('users.addTagLabel') }}</label>
            <input v-model="tagInput" class="ipt full" :placeholder="t('users.addTagPlaceholder')" @keydown.enter="addTag" />
          </div>
        </div>
        <div class="modal-ft">
          <button class="btn ghost" @click="tagModal = null">{{ t('common.cancel') }}</button>
          <button class="btn" :disabled="savingTags" @click="saveTags">{{ savingTags ? t('common.saving') : t('common.save') }}</button>
        </div>
      </div>
    </div>

    <!-- 金币调账弹框 -->
    <div v-if="coinModal" class="modal-mask" @click.self="coinModal = null">
      <div class="modal">
        <div class="modal-hd">{{ t('users.coinModalTitle', { name: coinModal.nickname || coinModal.user_id }) }}</div>
        <div class="modal-body">
          <div class="target-info">
            {{ t('users.coinCurrentBalance') }}<strong class="coin-bal">{{ coinModal.balance || 0 }}</strong>
          </div>
          <div class="form-row">
            <label>{{ t('users.coinModeLabel') }}</label>
            <div class="seg">
              <button class="btn sm" :class="coinForm.mode === 'add' ? '' : 'ghost'" @click="coinForm.mode = 'add'">{{ t('users.coinAdd') }}</button>
              <button class="btn sm" :class="coinForm.mode === 'deduct' ? 'danger' : 'ghost'" @click="coinForm.mode = 'deduct'">{{ t('users.coinDeduct') }}</button>
            </div>
          </div>
          <div class="form-row">
            <label>{{ t('users.coinAmountLabel') }}</label>
            <input v-model.number="coinForm.amount" type="number" min="1" step="1" class="ipt full" :placeholder="t('users.coinAmountPlaceholder')" />
          </div>
          <div class="form-row">
            <label>{{ t('users.coinRemarkLabel') }}</label>
            <input v-model="coinForm.remark" class="ipt full" maxlength="64" :placeholder="t('users.coinRemarkPlaceholder')" />
          </div>
          <div class="coin-preview" :class="coinForm.mode === 'deduct' ? 'minus' : 'plus'">
            {{ coinPreview }}
          </div>
        </div>
        <div class="modal-ft">
          <button class="btn ghost" @click="coinModal = null">{{ t('common.cancel') }}</button>
          <button class="btn" :class="coinForm.mode === 'deduct' ? 'danger' : ''" :disabled="!coinValid || adjustingCoins" @click="doAdjustCoins">
            {{ adjustingCoins ? t('users.coinAdjusting') : t('users.doCoinAdjust') }}
          </button>
        </div>
      </div>
    </div>

    <!-- 推送弹框 -->
    <div v-if="pushModal" class="modal-mask" @click.self="pushModal = null">
      <div class="modal">
        <div class="modal-hd">{{ t('users.pushModalTitle', { name: pushModal.nickname }) }}</div>
        <div class="modal-body">
          <div class="form-row">
            <label>{{ t('users.sceneLabel') }}</label>
            <select v-model="pushForm.scene" class="sel full">
              <option value="work_recommend">{{ t('users.sceneWorkRecommend') }}</option>
              <option value="activity">{{ t('users.sceneActivity') }}</option>
              <option value="reply">{{ t('users.sceneReply') }}</option>
              <option value="chat">{{ t('users.sceneChat') }}</option>
              <option value="checkin">{{ t('users.sceneCheckin') }}</option>
            </select>
          </div>
          <div class="form-row">
            <label>{{ t('users.field1') }}</label>
            <input v-model="pushForm.f1" class="ipt full" :placeholder="t('users.field1Placeholder')" />
          </div>
          <div class="form-row">
            <label>{{ t('users.field2') }}</label>
            <input v-model="pushForm.f2" class="ipt full" :placeholder="t('users.field2Placeholder')" />
          </div>
          <div class="form-row">
            <label>{{ pushForm.scene === 'checkin' ? t('users.field3Time') : t('users.field3') }}</label>
            <input
              v-model="pushForm.f3"
              class="ipt full"
              :placeholder="pushForm.scene === 'checkin' ? t('users.field3TimePlaceholder') : t('users.field3Placeholder')"
            />
          </div>
          <div class="form-row">
            <label>{{ t('users.pageLabel') }}</label>
            <input v-model="pushForm.page" class="ipt full" placeholder="/pages/ocean/ocean" />
          </div>
        </div>
        <div class="modal-ft">
          <button class="btn ghost" @click="pushModal = null">{{ t('common.cancel') }}</button>
          <button class="btn" :disabled="!pushForm.f1 || pushing" @click="doPush">
            {{ pushing ? t('users.pushingNow') : t('users.doPush') }}
          </button>
        </div>
      </div>
    </div>

    <!-- 发起对话弹框 -->
    <div v-if="chatModal" class="modal-mask" @click.self="chatModal = null">
      <div class="modal">
        <div class="modal-hd">{{ t('users.chatModalTitle') }}</div>
        <div class="modal-body">
          <i18n-t keypath="users.chatTarget" tag="div" class="target-info">
            <template #name><strong>{{ chatModal.nickname }}</strong></template>
          </i18n-t>
          <select v-model="selectedBot" class="sel full">
            <option value="">{{ t('users.selectBot') }}</option>
            <option v-for="r in robots" :key="r.user_id" :value="String(r.user_id)">
              {{ r.nickname }}{{ r.city ? '（' + r.city + '）' : '' }}
            </option>
          </select>
        </div>
        <div class="modal-ft">
          <button class="btn ghost" @click="chatModal = null">{{ t('common.cancel') }}</button>
          <button class="btn" :disabled="!selectedBot || startingChat" @click="doStartChat">
            {{ startingChat ? t('users.creating') : t('users.doStartChat') }}
          </button>
        </div>
      </div>
    </div>

    <div v-if="toast" class="gtoast" :class="toastOk ? 'ok' : 'err'">{{ toast }}</div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api'

const { t } = useI18n()
const list = ref([])
const total = ref(0)
const page = ref(1)
const size = ref(20)
const loading = ref(false)
const totalPages = computed(() => Math.max(1, Math.ceil(total.value / size.value)))
const keyword = ref('')
const statusFilter = ref('')
const robotFilter = ref('')
const actLoading = ref(null)
const tagFilter = ref('')
const googleFilter = ref('')

const chatModal = ref(null)
const selectedBot = ref('')
const robots = ref([])
const startingChat = ref(false)

const pushModal = ref(null)
const pushing = ref(false)
const pushForm = ref({ scene: 'work_recommend', f1: '', f2: '', f3: '', page: '' })

const coinModal = ref(null)
const coinForm = ref({ mode: 'add', amount: null, remark: '' })
const adjustingCoins = ref(false)
// 金额必须是正整数;扣减不能超过当前余额(后端也会拦,这里先把按钮灰掉)
const coinValid = computed(() => {
  const n = coinForm.value.amount
  if (!Number.isInteger(n) || n <= 0) return false
  if (coinForm.value.mode === 'deduct' && coinModal.value && n > (coinModal.value.balance || 0)) return false
  return true
})
const coinPreview = computed(() => {
  if (!coinModal.value) return ''
  const cur = coinModal.value.balance || 0
  const n = Number.isInteger(coinForm.value.amount) && coinForm.value.amount > 0 ? coinForm.value.amount : 0
  const next = coinForm.value.mode === 'deduct' ? cur - n : cur + n
  return t('users.coinPreview', { from: cur, to: next })
})

const tagModal = ref(null)
const tagDraft = ref([])
const tagInput = ref('')
const savingTags = ref(false)

const toast = ref('')
const toastOk = ref(true)

onMounted(() => { load(); loadRobots() })

async function load() {
  loading.value = true
  try {
    // 租户维度走全局选择器(api.js 自动注入 X-Tenant-ID),此处不再传 tenant_id
    const params = { page: page.value, size: size.value, keyword: keyword.value, status: statusFilter.value, robot: robotFilter.value, tag: tagFilter.value, google: googleFilter.value }
    const res = await api.listUsers(params)
    list.value = res.list || []
    total.value = res.total || 0
  } catch (e) { flash(e.message, false) }
  loading.value = false
}

async function loadRobots() {
  try {
    const res = await api.listRobotProfiles()
    robots.value = (res || []).filter(r => r.user_id)
  } catch (e) {}
}

function doSearch() { page.value = 1; load() }
function reset() { keyword.value = ''; statusFilter.value = ''; robotFilter.value = ''; tagFilter.value = ''; googleFilter.value = ''; doSearch() }
function goPage(p) { page.value = p; load() }

async function toggleBan(u) {
  const ban = u.status !== 'banned'
  actLoading.value = u.user_id
  try {
    await api.banUser(u.user_id, ban)
    u.status = ban ? 'banned' : 'active'
    flash(ban ? t('users.banned') : t('users.unbanned'))
  } catch (e) { flash(e.message, false) }
  actLoading.value = null
}

async function toggleMute(u) {
  const mute = !u.is_muted
  actLoading.value = u.user_id
  try {
    await api.muteUser(u.user_id, mute)
    u.is_muted = mute
    flash(mute ? t('users.muted') : t('users.unmuted'))
  } catch (e) { flash(e.message, false) }
  actLoading.value = null
}

function openChat(u) { chatModal.value = u; selectedBot.value = '' }

function tagList(tags) { return (tags || '').split(',').filter(Boolean) }

function openTags(u) {
  tagModal.value = u
  tagDraft.value = tagList(u.tags)
  tagInput.value = ''
}

function addTag() {
  // 局部变量避开 t() 翻译函数的命名
  const tag = tagInput.value.trim()
  if (!tag) return
  if (tag.includes(',') || tag.includes('，')) { flash(t('users.tagNoComma'), false); return }
  if ([...tag].length > 16) { flash(t('users.tagTooLong'), false); return }
  if (!tagDraft.value.includes(tag)) tagDraft.value.push(tag)
  tagInput.value = ''
}

async function saveTags() {
  if (!tagModal.value) return
  // 收编输入框残留:输入了但没按回车的标签也算数;校验未通过(addTag 已提示)则中断保存
  if (tagInput.value.trim()) {
    addTag()
    if (tagInput.value.trim()) return
  }
  savingTags.value = true
  try {
    await api.updateUserTags(tagModal.value.user_id, tagDraft.value.join(','))
    tagModal.value.tags = tagDraft.value.join(',')
    flash(t('users.tagsSaved'))
    tagModal.value = null
  } catch (e) { flash(e.message, false) }
  savingTags.value = false
}

function openCoins(u) {
  coinModal.value = u
  coinForm.value = { mode: 'add', amount: null, remark: '' }
}

async function doAdjustCoins() {
  if (!coinModal.value || !coinValid.value) return
  const n = coinForm.value.amount
  const delta = coinForm.value.mode === 'deduct' ? -n : n
  adjustingCoins.value = true
  try {
    const res = await api.adjustUserCoins(coinModal.value.user_id, delta, coinForm.value.remark.trim())
    coinModal.value.balance = res.balance
    flash(t('users.coinAdjusted', { n: Math.abs(delta), balance: res.balance }))
    coinModal.value = null
  } catch (e) { flash(e.message, false) }
  adjustingCoins.value = false
}

function openPush(u) {
  pushModal.value = u
  pushForm.value = { scene: 'work_recommend', f1: '', f2: '', f3: '', page: '' }
}

async function doPush() {
  if (!pushModal.value || !pushForm.value.f1) return
  pushing.value = true
  try {
    await api.pushUser(pushModal.value.user_id, pushForm.value)
    flash(t('users.pushOk'))
    pushModal.value = null
  } catch (e) { flash(e.message, false) }
  pushing.value = false
}

async function doStartChat() {
  if (!selectedBot.value || !chatModal.value) return
  startingChat.value = true
  try {
    await api.startRobotChat(chatModal.value.user_id, selectedBot.value)
    flash(t('users.chatStarted'))
    chatModal.value = null
  } catch (e) { flash(e.message, false) }
  startingChat.value = false
}

function isTest(u) { return typeof u.openid === 'string' && u.openid.startsWith('wxdev') }
function genderLabel(g) { return t(g === 1 ? 'users.male' : g === 2 ? 'users.female' : 'users.unknown') }
function shortTime(ts) { return ts ? String(ts).slice(0, 16).replace('T', ' ') : '—' }
function flash(msg, ok = true) { toast.value = msg; toastOk.value = ok; setTimeout(() => { toast.value = '' }, 2500) }
</script>

<style scoped>
/* 组件样式统一来自 styles/base.css，这里只留本页特有的部分。
   原本这一页有 7 种徽章色和 3 种实心按钮色（danger/warn/push），
   其中 .danger 与 .warn 在模板里根本没用到——是死代码。 */
.ipt { width: 200px; }
.ipt.full { width: 100%; }
.sel.full { width: 100%; }

/* 机器人行：原本是淡紫底 + 紫色条。紫在这套系统里没有语义，
   改用一档更浅的面色 + 中性竖条，区分度够且不占色相。 */
.tbl tr.row-bot td { background: var(--panel-2); }
.tbl tr.row-bot td:first-child { box-shadow: inset 2px 0 0 var(--ink-3); }

.user-cell { display: flex; align-items: center; gap: var(--s-2); }
.av { width: 36px; height: 36px; flex-shrink: 0; border-radius: 50%; object-fit: cover; }

/* 头像占位不用实心品牌色：品牌色只表示「可以点」，头像不可点 */
.av-ph {
  width: 36px;
  height: 36px;
  flex-shrink: 0;
  display: grid;
  place-items: center;
  border-radius: 50%;
  background: var(--brand-weak);
  color: var(--brand-text);
  font-size: var(--t-h2);
  font-weight: 600;
}

.av-ph.bot { background: var(--panel-3); color: var(--ink-2); font-size: var(--t-h1); }

.nick { font-weight: 600; }
.uid { font-size: var(--t-tag); color: var(--ink-3); }

.openid {
  display: inline-block;
  max-width: 200px;
  font-family: var(--f-mono);
  font-size: var(--t-tag);
  color: var(--ink-3);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  vertical-align: middle;
}

.tag-ipt { width: 140px; }
.tag-editor { display: flex; flex-wrap: wrap; align-items: center; gap: var(--s-1); min-height: 30px; }
.tag-x { margin-left: 2px; font-weight: 700; cursor: pointer; }
.tag-x:hover { color: var(--danger); }

.acts { display: flex; align-items: center; gap: var(--s-1); flex-wrap: wrap; }
.no-act { font-size: var(--t-label); color: var(--ink-3); }

/* 推送弹层（这一页用的是 modal-* 命名，不是全局的 .dlg） */
.modal-mask {
  position: fixed;
  inset: 0;
  z-index: 100;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: var(--s-4);
  background: var(--scrim);
}

.modal {
  width: 420px;
  max-width: 95vw;
  background: var(--panel);
  border: 1px solid var(--line);
  border-radius: var(--radius);
  box-shadow: var(--shadow-2);
}

.modal-hd {
  padding: var(--s-5) var(--s-6) var(--s-3);
  border-bottom: 1px solid var(--line-2);
  font-size: var(--t-h1);
  font-weight: 600;
}

.modal-body {
  display: flex;
  flex-direction: column;
  gap: var(--s-3);
  padding: var(--s-5) var(--s-6);
}

.target-info { font-size: var(--t-value); color: var(--ink-2); }

.modal-ft {
  display: flex;
  justify-content: flex-end;
  gap: var(--s-2);
  padding: var(--s-4) var(--s-6);
  border-top: 1px solid var(--line-2);
}

.form-row { display: flex; flex-direction: column; gap: var(--s-1); }
.form-row label { font-size: var(--t-label); color: var(--ink-2); }

.gtoast.err {
  border-color: var(--danger-line);
  background: var(--danger-bg);
  color: var(--danger);
}
.coin-bal { margin-left: 6px; font-size: 18px; }
.seg { display: flex; gap: 6px; }
.coin-preview { margin-top: 8px; font-size: 13px; color: var(--ink-3); }
.coin-preview.plus { color: #15803d; }
.coin-preview.minus { color: #b91c1c; }
</style>
