<!-- AutoOps 运维平台 — By JJ Zhang, Version 1.0 -->
<template>
  <div class="k8s-exec-page">
    <div class="exec-header">
      <span class="mono">{{ pod.namespace }} / {{ pod.name }}</span>
      <span class="muted">sh · {{ status }}</span>
    </div>
    <div ref="termEl" class="exec-term"></div>
  </div>
</template>

<script setup>
import { onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { useRoute } from 'vue-router'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import '@xterm/xterm/css/xterm.css'

const route = useRoute()
const pod = reactive({
  cluster: route.query.clusterId || '', namespace: route.query.namespace || '',
  name: route.query.pod || '', container: route.query.container || '',
})
const termEl = ref(null)
const status = ref('connecting')

let ws = null
let term = null
let fit = null

onMounted(() => {
  term.value = new Terminal({
    fontFamily: 'Consolas, Menlo, monospace', fontSize: 14,
    theme: { background: '#1e2a35', foreground: '#d8e4f0' }, cursorBlink: true,
  })
  fit.value = new FitAddon()
  term.loadAddon(fit.value)
  term.open(termEl.value)
  fit.fit()
  term.onData(d => { if (ws && ws.readyState === 1) ws.send(d) })
  connect()
  window.addEventListener('resize', onResize)
})
onBeforeUnmount(() => {
  window.removeEventListener('resize', onResize)
  if (ws) ws.close()
  if (term) term.dispose()
})
const onResize = () => { if (fit) fit.fit() }

function connect() {
  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  const q = new URLSearchParams({
    namespace: pod.namespace, pod: pod.name, container: pod.container || '',
    command: 'sh', token: localStorage.getItem('token') || '',
  })
  ws = new WebSocket(`${proto}://${location.host}/api/ws/k8s/${route.query.clusterId}?` + q)
  ws.binaryType = 'arraybuffer'
  ws.onopen = () => { status.value = 'connected'; term.focus() }
  ws.onmessage = ev => {
    const data = new Uint8Array(ev.data)
    if (data.length > 1) term.write(data.slice(1))
  }
  ws.onclose = () => { status.value = 'closed'; term.writeln('\r\n\x1b[31m[连接已关闭]') }
  ws.onerror = () => { term.writeln('\r\n\x1b[31m[连接错误]') }
}
</script>

<style scoped>
.k8s-exec-page { height: 100vh; display: flex; flex-direction: column; background: #1e2a35; }
.exec-header { padding: 8px 16px; color: #d8e4f0; display: flex; justify-content: space-between; }
.exec-term { flex: 1; padding: 0 8px 8px; }
</style>
