<!-- JNexus 运维平台 — By JJ Zhang, Version 1.0 -->
<!-- 可复用悬浮 AI 对话组件：任何页面引入即可使用 -->
<template>
  <div>
    <!-- 对话面板 -->
    <transition name="ai-slide">
      <div v-if="open" class="ai-chat-panel">
        <div class="ai-chat-head">
          <span class="ai-chat-title">{{ $t('ai.assistantTitle') }}</span>
          <el-button text size="small" style="color:inherit" @click="open = false">
            <el-icon :size="16"><Close /></el-icon>
          </el-button>
        </div>
        <div class="ai-chat-body" ref="msgBox">
          <div v-for="(msg, i) in messages" :key="i" class="ai-msg-row" :class="msg.role">
            <div class="ai-msg-bubble" :class="msg.role">{{ msg.text }}</div>
          </div>
          <div v-if="thinking" class="ai-msg-row bot"><div class="ai-msg-bubble" style="opacity:.5">…</div></div>
        </div>
        <div class="ai-chat-input">
          <el-input v-model="input" :placeholder="$t('ai.promptTip')" size="default"
                    @keyup.enter="send" :disabled="busy" clearable />
          <el-button type="primary" size="default" :loading="busy" @click="send">{{ $t('ai.send') }}</el-button>
        </div>
      </div>
    </transition>
    <!-- 浮动按钮 -->
    <transition name="ai-fab-pop">
      <div v-if="!open" class="ai-float-btn" @click="open = true" :title="$t('ai.assistantTitle')">
        <svg viewBox="0 0 24 24" width="26" height="26" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
          <path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1-2-2h14a2 2 0 0 1 2 2z"/>
          <circle cx="9" cy="10" r="0.5" fill="currentColor"/>
          <circle cx="13" cy="10" r="0.5" fill="currentColor"/>
          <circle cx="17" cy="10" r="0.5" fill="currentColor"/>
        </svg>
      </div>
    </transition>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import api from '../api'

const open = ref(false)
const busy = ref(false)
const input = ref('')
const messages = ref([])
const msgBox = ref(null)

const send = async () => {
  const text = input.value.trim()
  if (!text || busy.value) return
  messages.value.push({ role: 'user', text })
  busy.value = true
  try {
    const r = await api.post('/ai/chat', { prompt: text })
    messages.value.push({ role: 'bot', text: r.reply || '…' })
  } catch {
    messages.value.push({ role: 'bot', text: '…' })
  } finally {
    input.value = ''
    busy.value = false
    scrollToBottom()
  }
}

const scrollToBottom = () => {
  setTimeout(() => { if (msgBox.value) msgBox.value.scrollTop = msgBox.value.scrollHeight }, 50)
}
</script>

<style scoped>
.ai-float-btn {
  position: fixed; bottom: 22px; right: 22px; z-index: 2000;
  width: 48px; height: 48px; border-radius: 50%;
  background: linear-gradient(135deg, #67c23a, #4fc3a1); color: #fff;
  display: flex; align-items: center; justify-content: center;
  cursor: pointer; border: none;
  box-shadow: 0 4px 14px rgba(103,194,58,.4);
  transition: transform .2s, box-shadow .2s;
}
.ai-float-btn:hover { transform: scale(1.1); box-shadow: 0 6px 20px rgba(103,194,58,.5); }
.ai-chat-panel {
  position: fixed; bottom: 80px; right: 22px; z-index: 2001;
  width: 380px; height: 480px; display: flex; flex-direction: column;
  background: var(--el-bg-color); border: 1px solid var(--el-border-color-lighter);
  border-radius: 14px; box-shadow: 0 12px 40px rgba(0,0,0,.18);
  overflow: hidden;
}
.ai-chat-head {
  display: flex; align-items: center; gap: 8px; padding: 12px 14px 8px;
  font-size: 14px; font-weight: 700; color: var(--el-text-color-primary);
  border-bottom: 1px solid var(--el-border-color-lighter);
}
.ai-chat-title { flex: 1; }
.ai-chat-body { flex: 1; overflow-y: auto; padding: 12px 14px; }
.ai-msg-row { margin-bottom: 10px; display: flex; }
.ai-msg-row.user { justify-content: flex-end; }
.ai-msg-bubble {
  max-width: 82%; padding: 8px 12px; border-radius: 10px;
  font-size: 13.5px; line-height: 1.55; word-break: break-word;
  background: var(--el-fill-color); color: var(--el-text-color-primary);
}
.ai-msg-bubble.user { background: var(--el-color-primary-light-8); }
.ai-chat-input { display: flex; gap: 8px; padding: 8px 14px 12px; }
.ai-slide-enter-active, .ai-slide-leave-active { transition: opacity .25s, transform .25s; }
.ai-slide-enter-from, .ai-slide-leave-to { opacity: 0; transform: translateY(12px) scale(.97); }
.ai-fab-pop-enter-active, .ai-fab-pop-leave-active { transition: opacity .25s, transform .25s; }
.ai-fab-pop-enter-from, .ai-fab-pop-leave-to { opacity: 0; transform: scale(.6); }
</style>
