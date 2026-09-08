<!-- JNexus 运维平台 — By JJ Zhang, Version 1.0 -->
<template>
  <div class="rdp-page">
    <div class="rdp-bar">
      <span class="mono">{{ hostName }}（{{ hostIp }}）</span>
      <span class="muted">{{ status }}</span>
      <span style="flex:1"></span>
      <el-button size="small" @click="reconnect">Reconnect</el-button>
      <el-button size="small" type="danger" plain @click="disconnect">Close</el-button>
    </div>
    <div id="rdp-display" class="rdp-display"></div>
  </div>
</template>

<script setup>
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import Guacamole from 'guacamole-common-js'

const route = useRoute()
const status = ref('connecting')
const hostName = ref(route.query.host || '')
const hostIp = ref(route.query.ip || '')

let client = null
let tunnel = null

const setStatus = s => { status.value = s }

const connect = () => {
  const gw = route.query.gw || `ws://${location.hostname}:4823`
  const q = route.query.q || ''
  if (!q) { setStatus('missing query'); return }

  const wsUrl = `${gw.replace(/^http/, 'ws')}/?${q}`
  tunnel = new Guacamole.WebSocketTunnel(wsUrl)
  client = new Guacamole.Client(tunnel)

  const el = document.getElementById('rdp-display')
  el.appendChild(client.getDisplay().getElement())

  client.onerror = e => { setStatus(`error: ${e.message || 'unknown'}`) }
  tunnel.onstatechange = s => {
    if (s === Guacamole.Tunnel.State.OPEN) setStatus('connected')
    else if (s === Guacamole.Tunnel.State.CLOSED) setStatus('closed')
  }
  client.onsync = () => { /* 帧同步 */ }

  // 鼠标 / 键盘 / 剪贴板
  const mouse = new Guacamole.Mouse(client.getDisplay().getElement())
  mouse.onmousedown = mouse.onmouseup = mouse.onmousemove = m => client.sendMouseState(m)
  const keyboard = new Guacamole.Keyboard(window)
  keyboard.onkeydown = k => client.sendKeyEvent(1, k)
  keyboard.onkeyup = k => client.sendKeyEvent(0, k)
  window.addEventListener('beforeunload', disconnect)

  client.connect()
}

const disconnect = () => {
  try { client && client.disconnect() } catch { /* ignore */ }
  window.close()
}
const reconnect = () => {
  try { client && client.disconnect() } catch { /* ignore */ }
  setTimeout(connect, 300)
}

onMounted(connect)
onBeforeUnmount(() => { try { client && client.disconnect() } catch { /* ignore */ } })
</script>

<style scoped>
.rdp-page { height: 100vh; display: flex; flex-direction: column; background: #1a1a1a; }
.rdp-bar {
  display: flex; align-items: center; gap: 10px; padding: 8px 14px;
  background: #222; color: #ddd; flex-shrink: 0;
}
.mono { font-family: Consolas, Menlo, monospace; font-size: 13px; }
.muted { font-size: 12px; color: #909399; }
.rdp-display { flex: 1; overflow: auto; background: #000; }
.rdp-display :deep(div) { margin: 0 auto; }
</style>
