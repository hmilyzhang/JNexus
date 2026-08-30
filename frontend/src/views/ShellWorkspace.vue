<!-- AutoOps 运维平台 — By JJ Zhang, Version 1.0 -->
<template>
  <el-row :gutter="12" class="shell-row">
    <el-col :span="6">
      <el-card :header="$t('shell.assetTree')" v-loading="loading" class="side-card">
        <el-tree :data="treeData" node-key="key" highlight-current default-expand-all :expand-on-click-node="false"
                 @node-click="onTreeNode">
          <template #default="{ data }">
            <span class="tree-node">
              <el-icon v-if="data.type === 'group'"><Folder /></el-icon>
              <el-icon v-else :color="data.host.status === 'online' ? '#67c23a' : '#c0c4cc'"><Monitor /></el-icon>
              <span>{{ data.label }}</span>
            </span>
          </template>
        </el-tree>
        <div style="color:#909399; font-size:12px; margin-top:8px">{{ $t('shell.tip') }}</div>
      </el-card>

      <el-card :header="$t('shell.openTerms')" class="side-card" style="margin-top:12px">
        <div v-if="!sessions.length" style="color:#909399; font-size:13px">{{ $t('shell.noTerm') }}</div>
        <div v-for="s in sessions" :key="s.id" class="sess-item" :class="{ active: s.id === activeId }"
             @click="activate(s.id)">
          <el-icon :color="s.connected ? '#67c23a' : '#f56c6c'"><Connection /></el-icon>
          <span class="sess-label">{{ s.label }}</span>
          <el-button link size="small" type="danger" @click.stop="closeSession(s.id)">
            <el-icon><Close /></el-icon>
          </el-button>
        </div>
      </el-card>
    </el-col>

    <el-col :span="18">
      <el-card class="term-card" :header="activeLabel || $t('shell.title')">
        <div v-if="!sessions.length" class="term-empty">{{ $t('shell.empty') }}</div>
        <div v-for="s in sessions" :key="s.id" v-show="s.id === activeId"
             :ref="el => setTermEl(s.id, el)" class="term-container"></div>
      </el-card>
    </el-col>
  </el-row>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import api from '../api'
import i18n from '../i18n'

const { t } = i18n.global
const route = useRoute()
const hosts = ref([])
const loading = ref(true)
const sessions = ref([])
const activeId = ref(null)
const termEls = {}
let seq = 0
const encoder = new TextEncoder()

const treeData = computed(() => {
  const byGroup = new Map()
  for (const h of hosts.value) {
    const key = h.group_id ? `g-${h.group_id}` : 'g-none'
    if (!byGroup.has(key)) byGroup.set(key, { key, type: 'group', label: h.group?.name || t('hosts.uncategorized'), children: [] })
    byGroup.get(key).children.push({ key: 'h-' + h.id, type: 'host', label: `${h.name} · ${h.ip}`, host: h, children: [] })
  }
  return [...byGroup.values()]
})

const activeLabel = computed(() => {
  const s = sessions.value.find(x => x.id === activeId.value)
  return s ? `${s.label}（${s.connected ? t('common.online') : t('common.offline')}）` : ''
})

const setTermEl = (id, el) => { if (el) termEls[id] = el }

const loadHosts = async () => {
  loading.value = true
  try { hosts.value = await api.get('/hosts') } finally { loading.value = false }
}

onMounted(async () => {
  await loadHosts()
  // 支持 /shell?host=ID 直接打开指定主机终端
  const q = Number(route.query.host)
  if (q) {
    const h = hosts.value.find(x => x.id === q)
    if (h) await openSession(h)
  }
})

onBeforeUnmount(() => {
  for (const s of sessions.value) { try { s.ws?.close() } catch { /* ignore */ } }
})

const onTreeNode = node => {
  if (node.type === 'host') openSession(node.host)
}

const openSession = async host => {
  const exist = sessions.value.find(s => s.hostId === host.id)
  if (exist) { activate(exist.id); return }

  const id = ++seq
  sessions.value.push({ id, hostId: host.id, label: `${host.name} · ${host.ip}`, connected: false, term: null, ws: null, fit: null })
  activeId.value = id
  await nextTick()

  const s = sessions.value.find(x => x.id === id)
  const el = termEls[id]
  if (!el) return

  const { Terminal } = await import('@xterm/xterm')
  const { FitAddon } = await import('@xterm/addon-fit')
  const term = new Terminal({
    cursorBlink: true, fontSize: 14,
    fontFamily: 'Consolas, Monaco, monospace',
    theme: { background: '#1e1e1e' }
  })
  const fit = new FitAddon()
  term.loadAddon(fit)
  term.open(el)
  fit.fit()

  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  const ws = new WebSocket(`${proto}://${location.host}/api/ws/term/${host.id}?token=${localStorage.getItem('token')}`)
  ws.onopen = () => {
    s.connected = true
    term.focus()
    sendResize(s)
  }
  ws.onclose = () => {
    s.connected = false
    term.write('\r\n\x1b[31m[disconnected]\x1b[0m\r\n')
  }
  ws.onerror = () => term.write('\r\n\x1b[31m[connection failed, check host config and credentials]\x1b[0m\r\n')
  ws.onmessage = ev => {
    if (ev.data instanceof Blob) ev.data.text().then(x => term.write(x))
    else term.write(ev.data)
  }
  term.onData(d => {
    if (ws.readyState === WebSocket.OPEN) ws.send(encoder.encode(d))
  })
  term.onResize(() => sendResize(s))
  window.addEventListener('resize', () => fitActive())

  s.term = term
  s.ws = ws
  s.fit = fit
}

const sendResize = s => {
  if (s.ws && s.ws.readyState === WebSocket.OPEN && s.term) {
    s.ws.send(JSON.stringify({ type: 'resize', cols: s.term.cols, rows: s.term.rows }))
  }
}

const activate = id => {
  activeId.value = id
  nextTick(() => {
    const s = sessions.value.find(x => x.id === id)
    if (s && s.fit) {
      s.fit.fit()
      sendResize(s)
      s.term?.focus()
    }
  })
}

const fitActive = () => {
  const s = sessions.value.find(x => x.id === activeId.value)
  if (s && s.fit) { s.fit.fit(); sendResize(s) }
}

const closeSession = id => {
  const idx = sessions.value.findIndex(x => x.id === id)
  if (idx < 0) return
  const s = sessions.value[idx]
  try { s.ws?.close() } catch { /* ignore */ }
  try { s.term?.dispose() } catch { /* ignore */ }
  delete termEls[id]
  sessions.value.splice(idx, 1)
  if (activeId.value === id) {
    activeId.value = sessions.value.length ? sessions.value[Math.max(0, idx - 1)].id : null
    if (activeId.value) activate(activeId.value)
  }
}
</script>

<style scoped>
.shell-row { height: calc(100vh - 110px); }
.side-card { overflow: auto; }
.term-card { height: 100%; }
.term-card :deep(.el-card__body) { height: calc(100% - 40px); padding: 8px; }
.term-container { width: 100%; height: 100%; background: #1e1e1e; border-radius: 6px; }
.term-empty { color: #909399; text-align: center; padding-top: 120px; }
.tree-node { display: flex; align-items: center; gap: 6px; font-size: 13px; }
.sess-item {
  display: flex; align-items: center; gap: 6px; padding: 6px 8px; border-radius: 4px;
  cursor: pointer; font-size: 13px; margin-bottom: 4px;
}
.sess-item:hover { background: #f5f7fa; }
.sess-item.active { background: #ecf5ff; }
.sess-label { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
</style>
