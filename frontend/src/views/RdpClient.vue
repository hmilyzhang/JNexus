<!-- JNexus Ops Platform — By JJ Zhang, Version 1.0 -->
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
  // Gateway address: backend default is the same-origin path /rdp-gw (proxied by
  // JNexus itself); legacy values pointing at localhost are rewritten to the page
  // hostname for remote access
  let gw = route.query.gw || `ws://${location.hostname}:4823`
  try {
    if (gw.startsWith('/')) {
      gw = `${location.protocol === 'https:' ? 'wss' : 'ws'}://${location.host}${gw}`
    } else {
      const u = new URL(gw.replace(/^ws/, 'http'))
      if (['localhost', '127.0.0.1'].includes(u.hostname)) u.hostname = location.hostname
      gw = u.href.replace(/^http/, 'ws').replace(/\/$/, '')
    }
  } catch { /* keep the original value */ }
  const q = route.query.q || ''
  if (!q) { setStatus('missing query'); return }

  // guacamole-lite reads the encrypted connection string from the `token` query param
  const wsUrl = gw.startsWith('/') ? `${gw}?token=${encodeURIComponent(q)}` : `${gw}/?token=${encodeURIComponent(q)}`
  tunnel = new Guacamole.WebSocketTunnel(wsUrl)
  client = new Guacamole.Client(tunnel)

  const el = document.getElementById('rdp-display')
  el.appendChild(client.getDisplay().getElement())

  client.onerror = e => { setStatus(`error: ${e.message || 'unknown'}`) }
  tunnel.onstatechange = s => {
    if (s === Guacamole.Tunnel.State.OPEN) setStatus('connected')
    else if (s === Guacamole.Tunnel.State.CLOSED) setStatus('closed')
  }
  client.onsync = () => { /* frame sync */ }

  // Mouse / keyboard / clipboard
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
