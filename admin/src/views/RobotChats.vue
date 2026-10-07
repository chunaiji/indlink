<template>
  <div class="wrap">
    <!-- 左侧对话列表 -->
    <div class="side">
      <div class="side-hd">{{ t('robotChats.sideTitle') }} <span class="total">({{ total }})</span></div>
      <div class="chat-list">
        <div
          v-for="c in list"
          :key="c.chat_id"
          class="chat-item"
          :class="{ active: selected && selected.chat_id === c.chat_id }"
          @click="select(c)"
        >
          <div class="ci-top">
            <span class="bot-name">🤖 {{ c.bot_nickname }}</span>
            <span class="time">{{ shortTime(c.updated_at) }}</span>
          </div>
          <div class="ci-user">
            <span class="ws-dot" :class="onlineSet.has(String(c.user_id)) ? 'online' : 'offline'"
                  :title="onlineSet.has(String(c.user_id)) ? t('robotChats.wsConnected') : t('robotChats.wsDisconnected')"></span>
            👤 {{ c.user_nickname }}
          </div>
          <div class="ci-last">{{ c.last_message || '—' }}</div>
        </div>
        <div v-if="!list.length && !loading" class="empty">{{ t('robotChats.empty') }}</div>
      </div>
      <div class="pager">
        <button class="btn ghost sm" :disabled="page <= 1" @click="goPage(page-1)">{{ t('common.prev') }}</button>
        <span class="pg">{{ page }}/{{ totalPages }}</span>
        <button class="btn ghost sm" :disabled="page >= totalPages" @click="goPage(page+1)">{{ t('common.next') }}</button>
        <select v-model="size" @change="page = 1; load()" class="sel-pg">
          <option :value="20">20</option>
          <option :value="50">50</option>
          <option :value="100">100</option>
        </select>
      </div>
    </div>

    <!-- 右侧消息面板 -->
    <div class="panel" v-if="selected">
      <div class="panel-hd">
        <span>🤖 {{ selected.bot_nickname }}</span>
        <span class="arr">↔</span>
        <span>
          <span class="ws-dot" :class="onlineSet.has(String(selected.user_id)) ? 'online' : 'offline'"></span>
          👤 {{ selected.user_nickname }}
        </span>
        <span class="ws-label" :class="onlineSet.has(String(selected.user_id)) ? 'online' : 'offline'">
          {{ onlineSet.has(String(selected.user_id)) ? t('robotChats.wsOnline') : t('robotChats.wsOffline') }}
        </span>
        <button class="btn ghost sm refresh" @click="loadMessages">{{ t('robotChats.refresh') }}</button>
      </div>
      <div class="msgs" ref="msgsEl">
        <div
          v-for="m in messages"
          :key="m.message_id"
          class="msg"
          :class="m.sender_id === selected.bot_user_id ? 'msg-bot' : 'msg-user'"
        >
          <div class="bubble">{{ m.content }}</div>
          <div class="mtime">{{ shortTime(m.created_at) }}</div>
        </div>
        <div v-if="!messages.length" class="empty">{{ t('robotChats.emptyMessages') }}</div>
      </div>
      <div class="input-row">
        <textarea
          v-model="replyText"
          class="ta"
          rows="3"
          :placeholder="t('robotChats.replyPlaceholder')"
          @keydown.ctrl.enter="send"
        />
        <button class="btn send" :disabled="sending || !replyText.trim()" @click="send">
          {{ sending ? t('robotChats.sending') : t('robotChats.send') }}
        </button>
      </div>
    </div>
    <div class="panel empty-panel" v-else>
      <div class="empty">{{ t('robotChats.pickChat') }}</div>
    </div>

    <div v-if="toast" class="gtoast">{{ toast }}</div>
  </div>
</template>

<script setup>
import { ref, computed, nextTick, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api'

const { t } = useI18n()
const list = ref([])
const total = ref(0)
const page = ref(1)
const size = ref(20)
const loading = ref(false)
const totalPages = computed(() => Math.max(1, Math.ceil(total.value / size.value)))
const onlineSet = ref(new Set()) // 当前 WS 在线用户 ID 集合（字符串）
let onlineTimer = null

const selected = ref(null)
const messages = ref([])
const replyText = ref('')
const sending = ref(false)
const toast = ref('')
const msgsEl = ref(null)
let msgTimer = null

onMounted(() => {
  load()
  loadOnline()
  onlineTimer = setInterval(loadOnline, 8000)
})
onUnmounted(() => {
  if (onlineTimer) clearInterval(onlineTimer)
  if (msgTimer) clearInterval(msgTimer)
})

async function loadOnline() {
  try {
    const ids = await api.getOnlineUsers() || []
    onlineSet.value = new Set(ids.map(String))
  } catch (e) {}
}

async function load() {
  loading.value = true
  try {
    const res = await api.listRobotChats(page.value, size.value)
    list.value = res.list || []
    total.value = res.total || 0
  } catch (e) { flash(e.message) }
  loading.value = false
}

function goPage(p) { page.value = p; load() }

async function select(c) {
  selected.value = c
  messages.value = []
  replyText.value = ''
  if (msgTimer) clearInterval(msgTimer)
  await loadMessages()
  // 用户在小程序端的回复不经过 admin，需轮询才能实时显示
  msgTimer = setInterval(loadMessages, 5000)
}

async function loadMessages() {
  if (!selected.value) return
  try {
    const msgs = await api.robotChatMessages(selected.value.chat_id, 50) || []
    const hadNew = msgs.length > messages.value.length
    messages.value = msgs
    if (hadNew) {
      await nextTick()
      if (msgsEl.value) msgsEl.value.scrollTop = msgsEl.value.scrollHeight
    }
  } catch (e) { flash(e.message) }
}

async function send() {
  const text = replyText.value.trim()
  if (!text || !selected.value) return
  sending.value = true
  try {
    await api.sendAsRobot(selected.value.chat_id, text)
    replyText.value = ''
    flash(t('robotChats.sendOk'))
    await loadMessages()
    // 刷新对话列表里的最后消息
    load()
  } catch (e) { flash(e.message) }
  sending.value = false
}

function shortTime(t) {
  if (!t) return ''
  const s = String(t).slice(0, 16).replace('T', ' ')
  return s
}

function flash(t) { toast.value = t; setTimeout(() => { toast.value = '' }, 2500) }
</script>

<style scoped>
/* 按钮 / 输入 / toast / 空态来自 styles/base.css，这里只留本页特有的部分 */
.wrap {
  display: flex;
  height: calc(100vh - 116px);
  min-height: 400px;
  overflow: hidden;
  background: var(--panel);
  border: 1px solid var(--line);
  border-radius: var(--radius);
  box-shadow: var(--shadow-1);
}

.side {
  width: 280px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  border-right: 1px solid var(--line);
}

.side-hd {
  padding: var(--s-4);
  border-bottom: 1px solid var(--line);
  background: var(--panel-2);
  font-size: var(--t-h2);
  font-weight: 600;
}

.side-hd .total { margin-left: 0; font-weight: 400; }
.chat-list { flex: 1; overflow-y: auto; }

.chat-item {
  padding: var(--s-3) var(--s-4);
  border-bottom: 1px solid var(--line-2);
  cursor: pointer;
  transition: background-color var(--dur-1) var(--ease);
}

.chat-item:hover { background: var(--panel-2); }

/* 选中行：--brand-weak 底 + 左侧 2px 竖条（§7.3） */
.chat-item.active {
  background: var(--brand-weak);
  box-shadow: inset 2px 0 0 var(--brand);
}

.ci-top { display: flex; justify-content: space-between; gap: var(--s-2); margin-bottom: var(--s-1); }
.bot-name { font-size: var(--t-value); font-weight: 600; }
.time { font-size: var(--t-tag); color: var(--ink-3); }
.ci-user { margin-bottom: var(--s-1); font-size: var(--t-label); color: var(--ink-2); }

.ci-last {
  font-size: var(--t-label);
  color: var(--ink-3);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.pager {
  display: flex;
  align-items: center;
  gap: var(--s-1);
  margin-top: 0;
  padding: var(--s-2) var(--s-4);
  border-top: 1px solid var(--line-2);
  flex-wrap: wrap;
}

.pager .pg { flex: 1; text-align: center; }
.sel-pg { width: 58px; margin-left: 0; padding: 0 var(--s-1); }

.panel { flex: 1; display: flex; flex-direction: column; min-width: 0; }

.panel-hd {
  display: flex;
  align-items: center;
  gap: var(--s-2);
  padding: var(--s-3) var(--s-5);
  background: var(--panel-2);
  border-bottom: 1px solid var(--line);
  font-weight: 600;
}

.arr { color: var(--ink-3); }
.refresh { margin-left: auto; }

.msgs {
  flex: 1;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: var(--s-3);
  padding: var(--s-4);
  background: var(--panel-2);
}

.msg { display: flex; flex-direction: column; max-width: 70%; }
.msg-bot { align-self: flex-start; align-items: flex-start; }
.msg-user { align-self: flex-end; align-items: flex-end; }

.bubble {
  padding: var(--s-2) var(--s-3);
  border: 1px solid var(--line);
  border-radius: var(--radius-sm);
  font-size: var(--t-value);
  line-height: 1.5;
  word-break: break-all;
}

.msg-bot .bubble { background: var(--panel); border-bottom-left-radius: var(--radius-xs); }

/* 用户侧气泡原本是实心品牌蓝 —— 品牌色只用于「可以点」，
   气泡不可点。改用 brand-weak 一档，仍然区分得开。 */
.msg-user .bubble {
  background: var(--brand-weak);
  border-color: var(--brand-line);
  border-bottom-right-radius: var(--radius-xs);
}

.mtime { margin-top: var(--s-1); padding: 0 var(--s-1); font-size: var(--t-tag); color: var(--ink-3); }

.input-row {
  display: flex;
  align-items: flex-end;
  gap: var(--s-2);
  padding: var(--s-3) var(--s-4);
  border-top: 1px solid var(--line);
  background: var(--panel);
}

.ta { flex: 1; resize: none; }
.btn.send { height: auto; min-height: var(--d-ctl-h); align-self: stretch; }
.empty-panel { align-items: center; justify-content: center; }

/* WS 在线状态：绿=连着，离线用中性而不是红——没连上不是错误 */
.ws-dot {
  display: inline-block;
  width: 8px;
  height: 8px;
  flex-shrink: 0;
  margin-right: var(--s-1);
  border-radius: 50%;
  vertical-align: middle;
}

.ws-dot.online { background: var(--ok); }
.ws-dot.offline { background: var(--ink-4); }

.ws-label {
  padding: 2px var(--s-2);
  border: 1px solid var(--line);
  border-radius: var(--radius-xs);
  background: var(--panel-3);
  color: var(--ink-2);
  font-size: var(--t-tag);
  font-weight: 500;
}

.ws-label.online { border-color: var(--ok-line); background: var(--ok-bg); color: var(--ok); }

/* 移动端:左右两栏改上下堆叠(参考 Messages.vue),否则窄屏右侧面板被挤没看不到聊天内容 */
@media (max-width: 768px) {
  .wrap { flex-direction: column; height: auto; overflow: visible; }
  .side { width: 100%; border-right: none; border-bottom: 1px solid var(--line); max-height: 42vh; }
  .panel { min-height: 480px; }
  .panel-hd { flex-wrap: wrap; }
  .msg { max-width: 85%; }
}
</style>
