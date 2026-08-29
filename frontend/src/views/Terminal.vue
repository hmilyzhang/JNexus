<template>
  <el-card>
    <div style="display:flex; align-items:center; gap:10px; margin-bottom:8px">
      <span style="font-weight:600">Web 终端</span>
      <el-tag size="small" type="info">主机 #{{ route.params.hostId }}</el-tag>
      <el-tag size="small" :type="connected ? 'success' : 'danger'">{{ connected ? '已连接' : '未连接' }}</el-tag>
      <el-button size="small" link style="margin-left:auto" @click="$router.push('/hosts')">返回主机列表</el-button>
    </div>
    <div ref="termEl" style="width:100%; height:calc(100vh - 220px)"></div>
  </el-card>
</template>

<script setup>
import { onMounted, onUnmounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'

const route = useRoute()
const termEl = ref(null)
const connected = ref(false)
let term = null
let ws = null
let fitAddon = null

onMounted(() => {
  term = new Terminal({
    cursorBlink: true,
    fontSize: 14,
    fontFamily: 'Consolas, Monaco, monospace',
    theme: { background: '#1e1e1e' }
  })
  fitAddon = new FitAddon()
  term.loadAddon(fitAddon)
  term.open(termEl.value)
  fitAddon.fit()

  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  ws = new WebSocket(`${proto}://${location.host}/api/ws/term/${route.params.hostId}?token=${localStorage.getItem('token')}`)

  ws.onopen = () => {
    connected.value = true
    term.focus()
  }
  ws.onclose = () => {
    connected.value = false
    term.write('\r\n\x1b[31m[连接已断开]\x1b[0m\r\n')
  }
  ws.onmessage = ev => {
    const data = ev.data instanceof Blob ? ev.data : null
    if (data) {
      data.text().then(t => term.write(t))
    } else {
      term.write(ev.data)
    }
  }
  ws.onerror = () => {
    term.write('\r\n\x1b[31m[连接失败，请检查主机配置与凭证]\x1b[0m\r\n')
  }

  // 键盘输入
  term.onData(d => {
    if (ws.readyState === WebSocket.OPEN) ws.send(new TextEncoder().encode(d))
  })
  // 窗口变化
  const onResize = () => {
    fitAddon.fit()
    if (ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type: 'resize', cols: term.cols, rows: term.rows }))
    }
  }
  term.onResize(onResize)
  window.addEventListener('resize', onResize)
  setTimeout(() => { fitAddon.fit(); onResize() }, 100)
})

onUnmounted(() => {
  ws?.close()
  term?.dispose()
})
</script>
