<!-- JNexus Ops Platform — By JJ Zhang, Version 1.0 -->
<template>
  <el-row :gutter="12" class="shell-row">
    <el-col :span="6" v-show="!sideCollapsed">
      <el-card :header="$t('shell.assetTree')" v-loading="loading" class="side-card">
        <el-tree :data="treeData" node-key="key" highlight-current :default-expand-all="false" :expand-on-click-node="false"
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

    <el-col :span="sideCollapsed ? 24 : 18">
      <el-card class="term-card" :class="{ 'term-fullscreen': fullScreen }" ref="termCardEl">
        <template #header>
          <div style="display:flex; align-items:center; gap:10px">
            <el-button text size="small" @click="sideCollapsed = !sideCollapsed" style="padding:4px">
              <el-icon :size="16"><component :is="sideCollapsed ? 'Expand' : 'Fold'" /></el-icon>
            </el-button>
            <span style="flex:1; font-weight:600">{{ activeLabel || $t('shell.title') }}</span>
            <el-select v-model="themeName" size="small" style="width:150px" @change="applyTheme">
              <el-option v-for="(t, name) in termThemes" :key="name" :label="name" :value="name" />
            </el-select>
            <el-button size="small" :icon="fullScreen ? 'Close' : 'FullScreen'" @click="toggleFull">
              {{ fullScreen ? $t('shell.exitFull') : $t('shell.fullScreen') }}
            </el-button>
          </div>
        </template>
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
import '@xterm/xterm/css/xterm.css'

const { t } = i18n.global
const route = useRoute()
const hosts = ref([])
const usableCreds = ref([])
const hostGroups = ref([])
const loading = ref(true)
const sessions = ref([])
const activeId = ref(null)
const termEls = {}

// ---- Terminal themes (popular palettes) and fullscreen ----
const termThemes = {
  '默认 Dark': { background: '#1e1e1e', foreground: '#cccccc', cursor: '#ffffff' },
  'Dracula': { background: '#282a36', foreground: '#f8f8f2', cursor: '#f8f8f0',
    selectionBackground: '#44475a', red: '#ff5555', green: '#50fa7b',
    yellow: '#f1fa8c', blue: '#bd93f9', magenta: '#ff79c6', cyan: '#8be9fd' },
  'One Dark': { background: '#282c34', foreground: '#abb2bf', cursor: '#528bff',
    selectionBackground: '#3e4451', red: '#e06c75', green: '#98c379', yellow: '#e5c07b',
    blue: '#61afef', magenta: '#c678dd', cyan: '#56b6c2' },
  'Solarized Dark': { background: '#002b36', foreground: '#839496', cursor: '#93a1a1',
    selectionBackground: '#073642', red: '#dc322f', green: '#859900', yellow: '#b58900',
    blue: '#268bd2', magenta: '#d33682', cyan: '#2aa198' },
  'Solarized Light': { background: '#fdf6e3', foreground: '#657b83', cursor: '#586e75',
    selectionBackground: '#eee8d5', red: '#dc322f', green: '#859900', yellow: '#b58900',
    blue: '#268bd2', magenta: '#d33682', cyan: '#2aa198' },
  'GitHub Light': { background: '#ffffff', foreground: '#24292f', cursor: '#0969da',
    selectionBackground: '#0969da33', red: '#cf222e', green: '#116329', yellow: '#bf8700',
    blue: '#0550ae', magenta: '#8250df', cyan: '#1b7c83' },
}
const sideCollapsed = ref(false)
const themeName = ref(localStorage.getItem('term_theme') || '默认 Dark')
const applyTheme = name => {
  const th = termThemes[name]
  if (!th) return
  localStorage.setItem('term_theme', name)
  sessions.value.forEach(sx => { if (sx.term) sx.term.options.theme = th })
}
const fullScreen = ref(false)
const termCardEl = ref(null)
const toggleFull = () => {
  fullScreen.value = !fullScreen.value
  // The el-card ref is a component instance; the actual DOM is on $el. If browser fullscreen is denied, term-fullscreen still fills the viewport
  const root = termCardEl.value?.$el || termCardEl.value
  if (fullScreen.value && root?.requestFullscreen) {
    root.requestFullscreen().catch(() => {})
  } else if (document.fullscreenElement) {
    document.exitFullscreen().catch(() => {})
  }
  setTimeout(() => sessions.value.forEach(sx => { if (sx.fit) sx.fit.fit() }), 150)
}
// Sync button state when ESC exits browser fullscreen
const onFsChange = () => { if (!document.fullscreenElement) fullScreen.value = false }
onMounted(() => {
  document.addEventListener('fullscreenchange', onFsChange)
  window.addEventListener('resize', onWinResize)
})
onBeforeUnmount(() => {
  document.removeEventListener('fullscreenchange', onFsChange)
  window.removeEventListener('resize', onWinResize)
})
let seq = 0
const encoder = new TextEncoder()

// Multi-level group tree: groups nest by parent_id, hosts attach to their group, OS accounts attach under hosts
const treeData = computed(() => {
  const groups = hostGroups.value
  const byId = new Map(groups.map(g => [g.id, { key: `g-${g.id}`, type: 'group', label: g.name, children: [] }]))
  const roots = []
  for (const g of groups) {
    const node = byId.get(g.id)
    if (g.parent_id && byId.has(g.parent_id)) byId.get(g.parent_id).children.push(node)
    else roots.push(node)
  }
  for (const h of hosts.value) {
    const hostNode = { key: 'h-' + h.id, type: 'host', label: `${h.name} · ${h.ip}`, host: h, children: [] }
    // Prefer key-based login: show only key-type usable accounts; fall back to all accounts when the host has none
    let creds = (usableCreds.value || []).filter(c => c.host_id === h.id && c.auth_type === 'key')
    if (!creds.length) creds = (usableCreds.value || []).filter(c => c.host_id === h.id)
    for (const c of creds) {
      hostNode.children.push({
        key: 'c-' + c.id, type: 'credential', credentialId: c.id,
        label: `${c.username}${c.label ? '（' + c.label + '）' : ''}`, host: h, children: []
      })
    }
    if (h.group_id && byId.has(h.group_id)) byId.get(h.group_id).children.push(hostNode)
    else roots.push(hostNode)
  }
  return roots
})

const activeLabel = computed(() => {
  const s = sessions.value.find(x => x.id === activeId.value)
  return s ? `${s.label}（${s.connected ? t('common.online') : t('common.offline')}）` : ''
})

const setTermEl = (id, el) => { if (el) termEls[id] = el }

const loadHosts = async () => {
  loading.value = true
  try {
    // Windows hosts are RDP-only — the shell workspace lists SSH (Linux) hosts
    hosts.value = (await api.get('/hosts')).filter(h => h.os_type !== 'windows')
    usableCreds.value = await api.get('/credentials/usable')
    hostGroups.value = await api.get('/host_groups')
  } finally { loading.value = false }
}

onMounted(async () => {
  await loadHosts()
  // Support /shell?host=ID to open a terminal for the given host directly
  const q = Number(route.query.host)
  const qc = Number(route.query.credential_id)
  if (q) {
    const h = hosts.value.find(x => x.id === q)
    if (h) await openSession(h, qc || undefined)
  }
})

onBeforeUnmount(() => {
  for (const s of sessions.value) { try { s.ws?.close() } catch { /* ignore */ } }
})

const onTreeNode = node => {
  if (node.type === 'credential') openSession(node.host, node.credentialId)
  else if (node.type === 'host') openSession(node.host)
}

const openSession = async (host, credentialId) => {
  const exist = sessions.value.find(s => s.hostId === host.id && s.credentialId === credentialId)
  if (exist) { activate(exist.id); return }

  const cred = (usableCreds.value || []).find(c => c.id === credentialId)
  const credSuffix = cred ? ` — ${cred.username}` : ''
  const id = ++seq
  sessions.value.push({ id, hostId: host.id, credentialId, label: `${host.name} · ${host.ip}${credSuffix}`, connected: false, term: null, ws: null, fit: null })
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
  const credQs = credentialId ? `&credential_id=${credentialId}` : ''
  // SSH only — Windows hosts are RDP-only and filtered out of the host list
  const ws = new WebSocket(`${proto}://${location.host}/api/ws/term/${host.id}?token=${localStorage.getItem('token')}${credQs}`)
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
  if (s && s.fit) {
    try { s.fit.fit(); sendResize(s) } catch { /* container too small mid-resize */ }
  }
}
// browser window resized: re-fit the active terminal so it always fills the area
const onWinResize = () => fitActive()

const closeSession = id => {
  const idx = sessions.value.findIndex(x => x.id === id)
  if (idx < 0) return
  const s = sessions.value[idx]
  try { s.ro?.disconnect() } catch { /* ignore */ }
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
.shell-row { height: calc(100vh - 92px); overflow: hidden; }
.side-card { overflow: auto; }
.term-card { height: 100%; display: flex; flex-direction: column; }
.term-card :deep(.el-card__header) { flex: 0 0 auto; }
.term-card.term-fullscreen { position: fixed; inset: 0; z-index: 2000; height: 100vh; border-radius: 0; }
.term-card :deep(.el-card__body) { flex: 1 1 0; min-height: 0; padding: 8px; }
.term-container { position: relative; overflow: hidden; width: 100%; height: 100%; background: #1e1e1e; border-radius: 6px; }
.term-container :deep(.xterm) { height: 100%; }
.term-empty { color: #909399; text-align: center; padding-top: 120px; }
.tree-node { display: flex; align-items: center; gap: 6px; font-size: 13px; }
.sess-item {
  display: flex; align-items: center; gap: 6px; padding: 6px 8px; border-radius: 4px;
  cursor: pointer; font-size: 13px; margin-bottom: 4px;
  color: var(--el-text-color-primary);
}
.sess-item:hover { background: var(--el-fill-color-light); }
.sess-item.active { background: var(--el-color-primary-light-9); color: var(--el-color-primary); }
.sess-label { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--el-text-color-primary); }
.sess-label { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
</style>
