<!-- JNexus Ops Platform — By JJ Zhang, Version 1.0 -->
<template>
  <el-row :gutter="12" class="shell-row">
    <el-col :span="6" v-show="!sideCollapsed">
      <el-card v-loading="loading" class="side-card">
        <template #header>
          <div style="display:flex; align-items:center; justify-content:space-between">
            <span>{{ $t('shell.assetTree') }}</span>
            <el-button size="small" text :loading="loading" @click="loadHosts">{{ $t('common.refresh') }}</el-button>
          </div>
        </template>
        <el-tree :data="treeData" node-key="key" highlight-current :default-expand-all="false" :expand-on-click-node="false"
                 @node-click="onTreeNode">
          <template #default="{ data }">
            <span class="tree-node" :style="data.rdpOnly ? 'opacity:.5' : ''">
              <el-icon v-if="data.type === 'group'"><Folder /></el-icon>
              <el-icon v-else :color="data.host.status === 'online' ? '#67c23a' : '#c0c4cc'"><Monitor /></el-icon>
              <span>{{ data.label }}</span>
              <el-tag v-if="data.rdpOnly" size="small" style="margin-left:6px">RDP</el-tag>
            </span>
          </template>
        </el-tree>
        <div style="color:#909399; font-size:12px; margin-top:8px">{{ $t('shell.tip') }}</div>
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
        <div v-if="sessions.length" class="term-tabs">
          <div v-for="s in sessions" :key="s.id" class="term-tab" :class="{ active: s.id === activeId }"
               @click="activate(s.id)">
            <el-icon :color="s.connected ? '#67c23a' : '#f56c6c'"><Connection /></el-icon>
            <span class="term-tab-label">{{ s.label }}</span>
            <el-icon class="term-tab-close" @click.stop="closeSession(s.id)"><Close /></el-icon>
          </div>
        </div>
        <div v-for="s in sessions" :key="s.id" v-show="s.id === activeId"
             :ref="el => setTermEl(s.id, el)" class="term-container"></div>
      </el-card>
    </el-col>

    <!-- RDP account picker (Windows hosts launched from the asset tree) -->
    <el-dialog v-model="rdpPickVisible" :title="$t('hosts.rdpPickTitle')" width="420px" append-to-body>
      <el-form label-width="100px">
        <el-form-item :label="$t('hosts.credOsAccount')">
          <el-select v-model="rdpPickCred" style="width:100%">
            <el-option v-for="a in rdpPickAccounts" :key="a.id"
                       :label="`${a.username}${a.label ? '（' + a.label + '）' : ''}${a.is_default ? ' · ' + $t('hosts.rdpDefault') : ''}`" :value="a.id" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="rdpPickVisible = false">{{ $t('common.cancel') }}</el-button>
        <el-button type="primary" @click="connectRdp(rdpPickHost, rdpPickCred)">{{ $t('hosts.rdp') }}</el-button>
      </template>
    </el-dialog>
  </el-row>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import api from '../api'
import i18n from '../i18n'
import '@xterm/xterm/css/xterm.css'

const { t } = i18n.global
const route = useRoute()
const router = useRouter()
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

// Multi-level group tree: groups nest by parent_id, hosts attach to their group, OS accounts attach under hosts.
// Mirrors the hosts page: ungrouped hosts collect under a synthetic "未分组" node; Windows hosts stay visible
// but greyed out with an RDP tag (this workspace is SSH-only, they open via the RDP page instead).
const treeData = computed(() => {
  const groups = hostGroups.value
  const byId = new Map(groups.map(g => [g.id, { key: `g-${g.id}`, type: 'group', label: g.name, children: [] }]))
  const roots = []
  for (const g of groups) {
    const node = byId.get(g.id)
    if (g.parent_id && byId.has(g.parent_id)) byId.get(g.parent_id).children.push(node)
    else roots.push(node)
  }
  const ungrouped = { key: 'g-ungrouped', type: 'group', label: t('shell.ungrouped'), children: [] }
  for (const h of hosts.value) {
    const win = h.os_type === 'windows'
    const hostNode = { key: 'h-' + h.id, type: 'host', rdpOnly: win, label: `${h.name} · ${h.ip}`, host: h, children: [] }
    if (!win) {
      // List every usable account (key accounts first) so the user can pick one per session
      const creds = (usableCreds.value || []).filter(c => c.host_id === h.id)
        .sort((a, b) => ((b.auth_type === 'key') - (a.auth_type === 'key')) || (a.id - b.id))
      for (const c of creds) {
        hostNode.children.push({
          key: 'c-' + c.id, type: 'credential', credentialId: c.id,
          label: `${c.username}${c.label ? '（' + c.label + '）' : ''}`, host: h, children: []
        })
      }
    }
    if (h.group_id && byId.has(h.group_id)) byId.get(h.group_id).children.push(hostNode)
    else ungrouped.children.push(hostNode)
  }
  if (ungrouped.children.length) roots.push(ungrouped)
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
    // Keep Windows hosts visible (greyed, RDP-only) so the tree matches the hosts page
    hosts.value = await api.get('/hosts')
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
  else if (node.type === 'host') {
    if (node.rdpOnly) { openRdp(node.host); return }
    openSession(node.host)
  }
}

// RDP from the shell tree: Windows hosts launch a remote desktop session here,
// with an account picker when the host has several usable accounts
const rdpPickVisible = ref(false)
const rdpPickHost = ref(null)
const rdpPickAccounts = ref([])
const rdpPickCred = ref(null)
const openRdp = async host => {
  let accounts = []
  try {
    accounts = (usableCreds.value || []).filter(c => c.host_id === host.id)
  } catch { /* fall through to the default account */ }
  if (accounts.length > 1) {
    rdpPickHost.value = host
    rdpPickAccounts.value = accounts
    rdpPickCred.value = (accounts.find(a => a.is_default) || accounts[0]).id
    rdpPickVisible.value = true
    return
  }
  await connectRdp(host, null)
}
const connectRdp = async (host, credentialId) => {
  try {
    const r = await api.post(`/hosts/${host.id}/rdp-token`, credentialId ? { credential_id: credentialId } : {})
    rdpPickVisible.value = false
    router.push({ path: '/rdp', query: { gw: r.gateway, q: r.query, host: host.name, ip: host.ip } })
  } catch { /* surfaced by the interceptor */ }
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
  // re-fit whenever the container size changes: sidebar collapse, fullscreen, window resize
  if (typeof ResizeObserver !== 'undefined' && !s.ro) {
    s.ro = new ResizeObserver(() => { if (s.id === activeId.value) fitActive() })
    s.ro.observe(el)
  }
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
.term-card :deep(.el-card__body) { flex: 1 1 0; min-height: 0; padding: 8px; display: flex; flex-direction: column; overflow: hidden; }
.term-container { position: relative; overflow: hidden; width: 100%; flex: 1 1 0; min-height: 0; background: #1e1e1e; border-radius: 6px; }
.term-tabs { display: flex; gap: 4px; overflow-x: auto; margin-bottom: 6px; flex: 0 0 auto; min-height: 0; }
.term-tab { display: flex; align-items: center; gap: 6px; padding: 4px 8px; border-radius: 6px; background: rgba(255,255,255,.04);
  cursor: pointer; white-space: nowrap; font-size: 12px; border: 1px solid transparent; }
.term-tab:hover { background: rgba(255,255,255,.08); }
.term-tab.active { background: rgba(64,158,255,.15); border-color: rgba(64,158,255,.45); }
.term-tab-label { max-width: 220px; overflow: hidden; text-overflow: ellipsis; }
.term-tab-close { cursor: pointer; opacity: .6; border-radius: 3px; }
.term-tab-close:hover { opacity: 1; background: rgba(255,255,255,.12); }
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
