<!-- JNexus 运维平台 — By JJ Zhang, Version 1.0 -->
<template>
  <div>
    <el-card>
      <template #header>
        <div style="display:flex; align-items:center; gap:10px">
          <span style="flex:1">{{ $t('logtail.title') }}</span>
          <el-tag size="small" :type="ws && ws.readyState === 1 ? 'success' : 'info'">{{ status }}</el-tag>
        </div>
      </template>
      <el-form inline style="margin-bottom:6px">
        <el-form-item :label="$t('logtail.host')">
          <el-select v-model="hostId" filterable style="width:220px" :disabled="connected">
            <el-option v-for="h in hosts" :key="h.id" :value="h.id"
                       :label="`${h.name} · ${h.ip}`" :disabled="h.status !== 'online'" />
          </el-select>
        </el-form-item>
        <el-form-item :label="$t('logtail.path')">
          <el-input v-model="path" class="mono" style="width:360px" :disabled="connected"
                    :placeholder="$t('logtail.pathTip')" @keyup.enter="connect" />
        </el-form-item>
        <el-form-item :label="$t('logtail.lines')">
          <el-input-number v-model="lines" :min="0" :max="5000" :step="100" :disabled="connected" style="width:120px" />
        </el-form-item>
        <el-form-item>
          <el-checkbox v-model="follow">{{ $t('logtail.follow') }}</el-checkbox>
        </el-form-item>
        <el-form-item>
          <el-button v-if="!connected" type="primary" :loading="connecting" @click="connect">{{ $t('logtail.connect') }}</el-button>
          <el-button v-else type="danger" @click="disconnect">{{ $t('logtail.disconnect') }}</el-button>
          <el-button :disabled="!connected" @click="clear">{{ $t('logtail.clear') }}</el-button>
        </el-form-item>
      </el-form>

      <div class="term-shell">
        <div ref="termEl" class="term-box"></div>
      </div>
      <div style="color:#909399; font-size:12px; margin-top:8px">{{ $t('logtail.tip') }}</div>
    </el-card>
  </div>
</template>

<script setup>
import { nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import '@xterm/xterm/css/xterm.css'
import { ElMessage } from 'element-plus'
import api from '../api'
import i18n from '../i18n'

const { t } = i18n.global
const hosts = ref([])
const hostId = ref(null)
const path = ref('/var/log/syslog')
const lines = ref(200)
const follow = ref(true)
const status = ref(t('logtail.idle'))
const connected = ref(false)
const connecting = ref(false)
const termEl = ref(null)

let ws = null
let term = null, fit = null

const theme = {
  background: '#101418', foreground: '#cfe3ff',
  cursor: '#22d3ee', selectionBackground: '#264f78',
}

function initTerm() {
  if (term) return
  term = new Terminal({
    fontFamily: 'Consolas, Menlo, monospace', fontSize: 13,
    theme, cursorBlink: false, convertEol: true, disableStdin: true,
  })
  fit = new FitAddon()
  term.loadAddon(fit)
  term.open(termEl.value)
  fit.fit()
  window.addEventListener('resize', onResize)
}

function onResize() { try { fit && fit.fit() } catch { /* ignore */ } }

const writeLine = (text, color) => {
  const c = color ? `\x1b[${color}m` : ''
  term.write(`${c}${text}\x1b[0m\r\n`)
}

const connect = async () => {
  if (!hostId.value || !path.value.trim()) {
    ElMessage.warning(t('logtail.needHostPath'))
    return
  }
  connecting.value = true
  initTerm()
  term.reset()
  await nextTick()
  onResize()

  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  const q = new URLSearchParams({
    token: localStorage.getItem('token') || '',
    host_id: hostId.value,
    path: path.value.trim(),
    lines: lines.value,
  })
  ws = new WebSocket(`${proto}://${location.host}/api/ws/tail?` + q)
  ws.binaryType = 'arraybuffer'

  ws.onmessage = ev => {
    if (ev.data instanceof ArrayBuffer) {
      term.write(new Uint8Array(ev.data))
    } else {
      writeLine(ev.data, '33')
    }
    if (follow.value) term.scrollToBottom()
  }
  ws.onclose = () => {
    connected.value = false
    status.value = t('logtail.disconnected')
    writeLine(`[${t('logtail.disconnected')}]`, '31')
  }
  ws.onerror = () => { status.value = t('logtail.error') }
  ws.onopen = () => {
    connected.value = true
    status.value = t('logtail.connected')
    if (follow.value) term.scrollToBottom()
  }
  setTimeout(() => { connecting.value = false }, 1500)
}

const disconnect = () => {
  if (ws) { ws.onclose = null; ws.close(); ws = null }
  connected.value = false
  status.value = t('logtail.idle')
  writeLine(`[${t('logtail.stopped')}]`, '31')
}

const clear = () => { term && term.reset() }

onMounted(async () => {
  hosts.value = await api.get('/hosts').catch(() => [])
  if (hosts.value.length && !hostId.value) hostId.value = hosts.value[0].id
  initTerm()
  writeLine(t('logtail.hint'), '90')
})
onBeforeUnmount(() => {
  if (ws) { ws.onclose = null; ws.close() }
  window.removeEventListener('resize', onResize)
  term && term.dispose()
})
</script>

<style scoped>
.term-shell { height: 62vh; background: #101418; border-radius: 6px; padding: 6px; }
.term-box { height: 100%; }
</style>
