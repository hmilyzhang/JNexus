<!-- JNexus 运维平台 — By JJ Zhang, Version 1.0 -->
<template>
  <div class="k8s-exec-page">
    <div class="exec-header">
      <span class="mono">{{ namespace }} / {{ pod }}</span>
      <span style="flex:1"></span>
      <span style="color:#909399; font-size:12px">{{ $t('k8s.containers') }}:</span>
      <el-select v-model="container" size="small" style="width:150px" class="exec-select" @change="restartAll">
        <el-option v-for="c in containers" :key="c" :value="c" :label="c" />
      </el-select>
      <el-button size="small" :type="split ? 'primary' : 'default'" @click="toggleSplit">
        {{ split ? $t('k8s.singleScreen') : $t('k8s.splitScreen') }}
      </el-button>
      <span class="muted">{{ status }}</span>
    </div>
    <div class="exec-terms" :class="{ split }">
      <div ref="termEl" class="exec-term"></div>
      <div v-if="split" ref="termEl2" class="exec-term"></div>
    </div>
  </div>
</template>

<script setup>
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import '@xterm/xterm/css/xterm.css'
import api from '../api'
import i18n from '../i18n'

// Pod Shell 终端（可复用组件）：独立页 /k8s/exec 与 K8S 管理页抽屉共用
const props = defineProps({
  clusterId: { type: [String, Number], required: true },
  namespace: { type: String, default: '' },
  pod: { type: String, required: true },
  container: { type: String, default: '' },
})

const $t = i18n.global.t
const termEl = ref(null)
const termEl2 = ref(null)
const status = ref($t('k8s.shellConnecting'))
const containers = ref([])
const container = ref(props.container)
const split = ref(false)

let ws = null
let term = null
let fit = null
let ws2 = null
let term2 = null
let fit2 = null

function makeTerm(el) {
  const t = new Terminal({
    fontFamily: 'Consolas, Menlo, monospace', fontSize: 14,
    theme: { background: '#1e2a35', foreground: '#d8e4f0' }, cursorBlink: true,
  })
  const f = new FitAddon()
  t.loadAddon(f)
  t.open(el)
  f.fit()
  return { term: t, fit: f }
}

function connect(containerName, onMsg) {
  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  const q = new URLSearchParams({
    namespace: props.namespace, pod: props.pod, container: containerName || '',
    command: 'sh', token: localStorage.getItem('token') || '',
  })
  const sock = new WebSocket(`${proto}://${location.host}/api/ws/k8s/${props.clusterId}?` + q)
  sock.binaryType = 'arraybuffer'
  sock.onopen = () => { status.value = $t('k8s.shellConnected') || 'connected'; if (onMsg) onMsg(); else term.focus() }
  sock.onmessage = ev => {
    const data = new Uint8Array(ev.data)
    if (data.length > 1) onMsg ? onMsg().write(data.slice(1)) : term.write(data.slice(1))
  }
  sock.onclose = () => { status.value = $t('k8s.shellClosed'); if (onMsg) onMsg().writeln('\r\n\x1b[31m[' + $t('k8s.shellClosed') + ']') }
  sock.onerror = () => { if (onMsg) onMsg().writeln('\r\n\x1b[31m[' + $t('k8s.shellError') + ']') }
  return sock
}

function sendInput(d) {
  if (ws && ws.readyState === 1) {
    const frame = new Uint8Array(d.length + 1)
    frame[0] = 0
    frame.set(new TextEncoder().encode(d), 1)
    ws.send(frame)
  }
}

function restartAll() {
  if (ws) { ws.onclose = null; ws.close() }
  if (ws2) { ws2.onclose = null; ws2.close() }
  if (term) term.reset()
  if (term2) term2.reset()
  ws = connect(container.value, null)
  if (split.value) {
    ws2 = connect(container.value, () => term2)
  }
}

function toggleSplit() {
  split.value = !split.value
  setTimeout(() => {
    if (split.value && !term2) {
      const inst = makeTerm(termEl2.value)
      term2 = inst.term
      fit2 = inst.fit
      term2.onData(d => {
        if (ws2 && ws2.readyState === 1) {
          const frame = new Uint8Array(d.length + 1)
          frame[0] = 0
          frame.set(new TextEncoder().encode(d), 1)
          ws2.send(frame)
        }
      })
      ws2 = connect(container.value, () => term2)
    }
    if (fit) fit.fit()
    if (fit2) fit2.fit()
  }, 100)
}

onMounted(async () => {
  const inst = makeTerm(termEl.value)
  term = inst.term
  fit = inst.fit
  term.onData(sendInput)
  // 拉取容器列表（多容器选择）
  try {
    const pods = await api.get(`/k8s/clusters/${props.clusterId}/pods`, { params: { namespace: props.namespace } })
    const target = pods.find(p => p.name === props.pod)
    containers.value = target?.containers || []
    if (containers.value.length && !container.value) container.value = containers.value[0]
  } catch { /* 忽略：无容器列表也可用默认容器 */ }
  connect(container.value, null)
  window.addEventListener('resize', onResize)
  // 宿主容器（如抽屉）展开动画完成后重新适配尺寸
  setTimeout(() => { if (fit) fit.fit(); if (fit2) fit2.fit() }, 250)
})
onBeforeUnmount(() => {
  window.removeEventListener('resize', onResize)
  if (ws) ws.close()
  if (ws2) ws2.close()
  if (term) term.dispose()
  if (term2) term2.dispose()
})
const onResize = () => { if (fit) fit.fit(); if (fit2) fit2.fit() }
</script>

<style scoped>
.k8s-exec-page { height: 100%; display: flex; flex-direction: column; background: #1e2a35; }
.exec-header { padding: 8px 16px; color: #d8e4f0; display: flex; align-items: center; gap: 10px; }
.exec-select :deep(.el-input__inner) { color: #d8e4f0; }
.exec-terms { flex: 1; display: flex; padding: 0 8px 8px; gap: 8px; }
.exec-term { flex: 1; min-width: 0; }
.exec-terms.split .exec-term { border: 1px solid #34495e; border-radius: 4px; }
.muted { color: #909399; font-size: 12px; }
</style>
