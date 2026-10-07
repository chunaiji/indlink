<template>
  <div class="chat-layout">
    <!-- 左:会话列表 -->
    <div class="conv-pane">
      <div class="conv-search">
        <input v-model.trim="keyword" class="inp" :placeholder="t('messages.searchPlaceholder')" @keyup.enter="reloadConvs" />
        <button class="btn sm" @click="reloadConvs">{{ t('messages.searchBtn') }}</button>
      </div>
      <div class="conv-list">
        <div
          v-for="c in convs" :key="c.chat_id"
          class="conv-item" :class="{ active: current && current.chat_id === c.chat_id }"
          @click="openChat(c)"
        >
          <div class="conv-title">
            <span :class="{ bot: c.robot_a }">{{ c.nick_a || t('messages.user') }}{{ c.robot_a ? ' 🤖' : '' }}</span>
            <span class="sep">↔</span>
            <span :class="{ bot: c.robot_b }">{{ c.nick_b || t('messages.user') }}{{ c.robot_b ? ' 🤖' : '' }}</span>
          </div>
          <div class="conv-last">{{ c.last_msg || t('messages.noMessage') }}</div>
          <div class="conv-time">{{ shortTime(c.updated_at) }}</div>
        </div>
        <div v-if="!convs.length && !loading" class="empty">{{ t('messages.emptyConvs') }}</div>
      </div>
      <div class="conv-pager">
        <button class="btn ghost sm" :disabled="page <= 1" @click="goPage(page - 1)">{{ t('messages.prev') }}</button>
        <span class="pg">{{ page }}/{{ totalPages }}</span>
        <button class="btn ghost sm" :disabled="page >= totalPages" @click="goPage(page + 1)">{{ t('messages.next') }}</button>
        <span class="total">{{ t('messages.totalShort', { n: total }) }}</span>
      </div>
    </div>

    <!-- 右:聊天记录 -->
    <div class="msg-pane">
      <template v-if="current">
        <div class="msg-hd">
          <span :class="{ bot: detail.robot_a }">{{ detail.nick_a || t('messages.userA') }}</span>
          <span class="sep">{{ t('messages.with') }}</span>
          <span :class="{ bot: detail.robot_b }">{{ detail.nick_b || t('messages.userB') }}</span>
          <span class="cid">{{ t('messages.chatId', { id: current.chat_id }) }}</span>
        </div>
        <div class="msg-body">
          <div v-if="msgLoading" class="empty">{{ t('common.loading') }}</div>
          <template v-else>
            <div
              v-for="m in detail.messages" :key="m.message_id"
              class="bubble-row" :class="side(m) === 'a' ? 'left' : 'right'"
            >
              <div class="bubble">
                <div class="bubble-name">{{ side(m) === 'a' ? detail.nick_a : detail.nick_b }}</div>
                <img
                  v-if="m.type === 'image' && m.content"
                  :src="m.content" class="bubble-img" loading="lazy"
                  @click="openImg(m.content)"
                />
                <div v-else class="bubble-text">{{ fmtContent(m) }}</div>
                <div class="bubble-time">{{ shortTime(m.created_at) }}</div>
              </div>
            </div>
            <div v-if="!detail.messages.length" class="empty">{{ t('messages.emptyMessages') }}</div>
          </template>
        </div>
      </template>
      <div v-else class="msg-placeholder">{{ t('messages.pickConv') }}</div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api'

const { t } = useI18n()
const convs = ref([])
const total = ref(0)
const page = ref(1)
const size = ref(20)
const loading = ref(false)
const keyword = ref('')
const totalPages = computed(() => Math.max(1, Math.ceil(total.value / size.value)))

const current = ref(null)
const detail = ref({ nick_a: '', nick_b: '', robot_a: false, robot_b: false, user_a: '', user_b: '', messages: [] })
const msgLoading = ref(false)

onMounted(loadConvs)

async function loadConvs() {
  loading.value = true
  try {
    const res = await api.listChats({ page: page.value, size: size.value, keyword: keyword.value })
    convs.value = res.list || []
    total.value = res.total || 0
  } catch (e) {}
  loading.value = false
}
function reloadConvs() { page.value = 1; loadConvs() }
function goPage(p) { page.value = p; loadConvs() }

async function openChat(c) {
  current.value = c
  msgLoading.value = true
  detail.value = { nick_a: c.nick_a, nick_b: c.nick_b, robot_a: c.robot_a, robot_b: c.robot_b, user_a: c.user_a, user_b: c.user_b, messages: [] }
  try {
    const d = await api.chatDetail(c.chat_id)
    if (d) detail.value = d
  } catch (e) {}
  msgLoading.value = false
}

// 发送方等于 user_a 排左侧,否则右侧(id 为字符串,直接比较)
function side(m) { return String(m.sender_id) === String(detail.value.user_a) ? 'a' : 'b' }

function fmtContent(m) {
  if (m.type === 'image') return t('messages.typeImage')
  if (m.type === 'audio') return t('messages.typeAudio')
  if (m.type === 'gift') return t('messages.typeGift')
  return m.content || ''
}
function openImg(url) { window.open(url, '_blank') }
function shortTime(ts) { return ts ? String(ts).slice(0, 16).replace('T', ' ') : '—' }
</script>

<style scoped>
.chat-layout { display: flex; gap: var(--s-4); height: calc(100vh - 140px); min-height: 480px; }

/* 按钮 / 输入 / 空态来自 styles/base.css，这里只留本页特有的部分。
   机器人标记原本是紫色（#7950f2）——紫在这套系统里没有语义，
   而色相只留给状态与交互，所以收敛为中性。 */

/* 左会话列表 */
.conv-pane {
  width: 320px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background: var(--panel);
  border: 1px solid var(--line);
  border-radius: var(--radius);
  box-shadow: var(--shadow-1);
}

.conv-search {
  display: flex;
  gap: var(--s-2);
  padding: var(--s-3);
  border-bottom: 1px solid var(--line-2);
}

.conv-search .inp { flex: 1; }
.conv-list { flex: 1; overflow-y: auto; }

.conv-item {
  padding: var(--s-3);
  border-bottom: 1px solid var(--line-2);
  cursor: pointer;
  transition: background-color var(--dur-1) var(--ease);
}

.conv-item:hover { background: var(--panel-2); }

/* 选中行：--brand-weak 底 + 左侧 2px 竖条（§7.3） */
.conv-item.active {
  background: var(--brand-weak);
  box-shadow: inset 2px 0 0 var(--brand);
}

.conv-title {
  display: flex;
  align-items: center;
  gap: var(--s-1);
  font-size: var(--t-value);
  font-weight: 600;
}

.conv-title .bot { color: var(--ink-2); }
.conv-title .sep { color: var(--ink-3); font-weight: 400; }

.conv-last {
  margin-top: var(--s-1);
  font-size: var(--t-label);
  color: var(--ink-2);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.conv-time { margin-top: var(--s-1); font-size: var(--t-tag); color: var(--ink-3); }

.conv-pager {
  display: flex;
  align-items: center;
  gap: var(--s-2);
  padding: var(--s-2) var(--s-3);
  border-top: 1px solid var(--line-2);
  font-size: var(--t-label);
  margin-top: 0;
}

.conv-pager .total { margin-left: auto; color: var(--ink-3); }

/* 右聊天记录 */
.msg-pane {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background: var(--panel);
  border: 1px solid var(--line);
  border-radius: var(--radius);
  box-shadow: var(--shadow-1);
}

.msg-hd {
  display: flex;
  align-items: center;
  gap: var(--s-2);
  padding: var(--s-3) var(--s-5);
  border-bottom: 1px solid var(--line-2);
  font-weight: 600;
}

.msg-hd .bot { color: var(--ink-2); }
.msg-hd .sep { color: var(--ink-3); font-weight: 400; }

.msg-hd .cid {
  margin-left: auto;
  font-family: var(--f-mono);
  font-size: var(--t-label);
  color: var(--ink-3);
}

.msg-body { flex: 1; overflow-y: auto; padding: var(--s-4) var(--s-5); background: var(--panel-2); }

.msg-placeholder {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  color: var(--ink-3);
}

.bubble-row { display: flex; margin-bottom: var(--s-3); }
.bubble-row.left { justify-content: flex-start; }
.bubble-row.right { justify-content: flex-end; }

.bubble {
  max-width: 62%;
  padding: var(--s-2) var(--s-3);
  border: 1px solid var(--line);
  border-radius: var(--radius-sm);
}

.bubble-row.left .bubble { background: var(--panel); border-top-left-radius: var(--radius-xs); }

.bubble-row.right .bubble {
  background: var(--brand-weak);
  border-color: var(--brand-line);
  border-top-right-radius: var(--radius-xs);
}

.bubble-name { margin-bottom: 3px; font-size: var(--t-tag); color: var(--ink-3); }
.bubble-text { font-size: var(--t-value); line-height: 1.5; white-space: pre-wrap; word-break: break-word; }

.bubble-img {
  display: block;
  max-width: 100%;
  max-height: 260px;
  border-radius: var(--radius-xs);
  cursor: zoom-in;
}

.bubble-time { margin-top: var(--s-1); font-size: var(--t-tag); color: var(--ink-3); text-align: right; }

/* 移动端:左右两栏改上下堆叠 */
@media (max-width: 768px) {
  .chat-layout { flex-direction: column; height: auto; gap: var(--s-3); }
  .conv-pane { width: 100%; max-height: 42vh; }
  .msg-pane { min-height: 420px; }
  .bubble { max-width: 82%; }
}
</style>
