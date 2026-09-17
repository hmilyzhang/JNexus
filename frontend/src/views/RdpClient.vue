<!-- JNexus Ops Platform — By JJ Zhang, Version 1.0 -->
<!-- RDP client (guacamole-lite): embedded in the main layout like the shell
     workspace. The remote desktop scales to fit the available area, and the
     session resolution follows the container size (adaptive). -->
<template>
  <div class="rdp-page">
    <div class="rdp-bar">
      <span class="mono">{{ hostName }}（{{ hostIp }}）</span>
      <span class="muted">{{ status }}</span>
      <span style="flex:1"></span>
      <el-button size="small" @click="reconnect">{{ $t('common.refresh') }}</el-button>
      <el-button size="small" type="danger" plain @click="closePage">{{ $t('hosts.rdpClose') }}</el-button>
    </div>
    <div ref="displayBox" class="rdp-display"></div>
  </div>
</template>

<script setup>
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import Guacamole from 'guacamole-common-js'

const route = useRoute()
const router = useRouter()
const status = ref('connecting')
const hostName = ref(route.query.host || '')
const hostIp = ref(route.query.ip || '')

const displayBox = ref(null)
let client = null
let tunnel = null
let keyboard = null
let curScale = 1
let resizeTimer = null
let ro = null

const setStatus = s => { status.value = s }

// fit the (possibly remote-resized) desktop into the container
const applyScale = () => {
  if (!client || !displayBox.value) return
  const display = client.getDisplay()
  const w = display.getWidth()
  const h = display.getHeight()
  if (!w || !h || !displayBox.value.clientWidth || !displayBox.value.clientHeight) return
  curScale = Math.min(displayBox.value.clientWidth / w, displayBox.value.clientHeight / h)
  display.scale(curScale)
}

// ask the RDP server to resize the session to the container (adaptive),
// debounced so dragging the window does not spam the tunnel
const scheduleSendSize = () => {
  clearTimeout(resizeTimer)
  resizeTimer = setTimeout(() => {
    if (!client || !displayBox.value) return
    const w = displayBox.value.clientWidth
    const h = displayBox.value.clientHeight
    if (w > 100 && h > 100) {
      try { client.sendSize(w, h) } catch { /* remote may not support resize */ }
    }
  }, 400)
}

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

  const box = displayBox.value
  box.innerHTML = ''
  box.appendChild(client.getDisplay().getElement())

  client.onerror = e => { setStatus(`error: ${e.message || 'unknown'}`) }
  tunnel.onstatechange = s => {
    if (s === Guacamole.Tunnel.State.OPEN) { setStatus('connected'); applyScale(); scheduleSendSize() }
    else if (s === Guacamole.Tunnel.State.CLOSED) setStatus('closed')
  }
  // remote resolution changed → re-fit
  client.getDisplay().onresize = applyScale

  // Mouse: divide by the display scale so clicks land on the right remote pixel
  const mouse = new Guacamole.Mouse(client.getDisplay().getElement())
  mouse.onmousedown = mouse.onmouseup = mouse.onmousemove = m => {
    m.x = m.x / curScale
    m.y = m.y / curScale
    client.sendMouseState(m)
  }
  keyboard = new Guacamole.Keyboard(window)
  keyboard.onkeydown = k => client.sendKeyEvent(1, k)
  keyboard.onkeyup = k => client.sendKeyEvent(0, k)

  // container size changes → refit + adaptive remote resolution
  ro = new ResizeObserver(() => { applyScale(); scheduleSendSize() })
  ro.observe(box)

  client.connect()
}

const disconnect = () => {
  clearTimeout(resizeTimer)
  try { ro && ro.disconnect() } catch { /* ignore */ }
  try { keyboard && (keyboard.onkeydown = keyboard.onkeyup = null) } catch { /* ignore */ }
  try { client && client.disconnect() } catch { /* ignore */ }
  client = null
}
const reconnect = () => {
  disconnect()
  setTimeout(connect, 300)
}
// same-tab close: disconnect and go back to the hosts page
const closePage = () => {
  disconnect()
  router.push('/hosts')
}

onMounted(connect)
onBeforeUnmount(disconnect)
</script>

<style scoped>
.rdp-page { height: calc(100vh - 92px); display: flex; flex-direction: column; background: #1a1a1a; border-radius: 8px; overflow: hidden; }
.rdp-bar {
  display: flex; align-items: center; gap: 10px; padding: 8px 14px;
  background: #222; color: #ddd; flex-shrink: 0;
}
.mono { font-family: Consolas, Menlo, monospace; font-size: 13px; }
.muted { font-size: 12px; color: #909399; }
.rdp-display { flex: 1; overflow: hidden; background: #000; display: flex; align-items: center; justify-content: center; }
</style>
