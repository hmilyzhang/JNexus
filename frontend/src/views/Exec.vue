<!-- JNexus Ops Platform — By JJ Zhang, Version 1.0 -->
<template>
  <div>
    <el-card>
      <el-form label-width="110px" style="max-width:900px">
        <el-form-item :label="$t('exec.targetHosts')">
          <div style="width:100%">
            <el-tree-select v-model="selectedNodes" :data="treeData" multiple check-strictly=false
                            :render-after-expand="false" :default-expand-all="false" :placeholder="$t('exec.targetHosts')"
                            style="width:100%" node-key="value" :max-collapse-tags="3" collapse-tags />
            <el-input v-model="ipInput" :placeholder="$t('exec.ipInputPlaceholder')" style="margin-top:8px" clearable>
              <template #prepend>{{ $t('exec.ipInput') }}</template>
            </el-input>
          </div>
        </el-form-item>
        <el-form-item :label="$t('hosts.credOsAccount')">
          <el-select v-model="credentialId" style="width:100%" clearable filterable
                       :placeholder="$t('hosts.credSelectPlaceholder')" @visible-change="credDdlVisible = $event">
            <template v-if="credDdlVisible">
              <el-option v-for="c in usableCreds" :key="c.id"
                         :label="`${c.host_name} · ${c.host_ip} — ${c.username}${c.label ? '（' + c.label + '）' : ''}`" :value="c.id" />
            </template>
            <el-option v-else-if="selectedCred" :key="'sel-' + selectedCred.id"
                       :label="`${selectedCred.host_name} · ${selectedCred.host_ip} — ${selectedCred.username}`" :value="selectedCred.id" />
          </el-select>
        </el-form-item>
        <el-form-item :label="$t('exec.mode')">
          <el-radio-group v-model="form.mode">
            <el-radio value="command">{{ $t('exec.modeCommand') }}</el-radio>
            <el-radio value="script">{{ $t('exec.modeScript') }}</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item :label="$t('exec.command')" v-if="form.mode === 'command'">
          <el-input v-model="form.command" type="textarea" :rows="4" :placeholder="$t('exec.commandPlaceholder')" class="mono" />
        </el-form-item>
        <el-form-item :label="$t('exec.script')" v-else>
          <el-select v-model="form.script_id" :placeholder="$t('exec.script')" style="width:100%">
            <el-option v-for="s in scripts" :key="s.id" :label="s.name" :value="s.id" />
          </el-select>
        </el-form-item>
        <el-form-item :label="$t('exec.scriptArgs')" v-if="form.mode === 'script'">
          <el-input v-model="form.script_args" :placeholder="$t('exec.scriptArgsPlaceholder')" class="mono" />
        </el-form-item>
        <el-form-item :label="$t('exec.timeout')"><el-input-number v-model="form.timeout_sec" :min="5" :max="3600" /></el-form-item>
        <el-form-item :label="$t('exec.concurrency')"><el-input-number v-model="form.concurrency" :min="1" :max="100" /></el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="running" @click="run">{{ $t('common.execute') }}</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <el-card style="margin-top:16px" v-if="taskId">
      <template #header>
        <span>{{ $t('exec.realtime') }} · {{ $t('exec.task') }} #{{ taskId }}
          <el-tag size="small" style="margin-left:8px" :type="taskDone ? (taskFailed ? 'danger' : 'success') : 'warning'">
            {{ taskDone ? (taskFailed ? $t('exec.taskDoneFailed') : $t('exec.taskDone')) : $t('exec.taskRunning') }}
          </el-tag>
          <el-tag size="small" type="success" style="margin-left:4px">{{ $t('common.success') }} {{ liveDone.success }}</el-tag>
          <el-tag v-if="liveDone.failed" size="small" type="danger" style="margin-left:4px">{{ $t('common.failed') }} {{ liveDone.failed }}</el-tag>
          <el-progress v-if="!taskDone && liveTotal" :percentage="Math.round(liveDone.total / liveTotal * 100)" style="width:120px; margin-left:8px" />
          <el-button size="small" link style="float:right" @click="$router.push(`/tasks?detail=${taskId}`)">{{ $t('exec.viewDetail') }}</el-button>
        </span>
      </template>
      <div v-for="r in liveResults" :key="r.result_id" style="margin-bottom:12px">
        <div style="font-size:13px; font-weight:600">
          {{ r.host_name }}（{{ r.host_ip }}）
          <el-tag size="small" :type="r.status === 'success' ? 'success' : r.status === 'failed' ? 'danger' : r.status === 'running' ? 'warning' : 'info'">
            {{ r.status }}
          </el-tag>
          <span v-if="r.exit_code !== undefined" style="color:#909399; font-size:12px"> {{ $t('exec.exitCode') }} {{ r.exit_code }}</span>
        </div>
        <div class="log-box">{{ r.text || $t('exec.noOutput') }}</div>
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import api from '../api'
import i18n from '../i18n'
import { ElMessage, ElMessageBox } from 'element-plus'

const { t } = i18n.global
const hosts = ref([])
const scripts = ref([])
const running = ref(false)
const taskId = ref(null)
const taskDone = ref(false)
const taskFailed = ref(false)
const liveResults = ref([])
const selectedNodes = ref([])
const ipInput = ref('')
const credentialId = ref(null)
const usableCreds = ref([])
const credDdlVisible = ref(false)
const selectedCred = computed(() => usableCreds.value.find(c => c.id === credentialId.value))
let ws = null

const form = reactive({ mode: 'command', command: '', script_id: null, script_args: '', timeout_sec: 300, concurrency: 10 })

// Tree select data: group nodes use value=g-<id>, host nodes use value=<id>
// Multi-level group tree: groups nest by parent_id, hosts attach to their group node (ungrouped hosts attach to root)
const buildTree = (hosts, groups, leafOf) => {
  const byId = new Map(groups.map(g => [g.id, { value: `g-${g.id}`, label: g.name, children: [] }]))
  const roots = []
  for (const g of groups) {
    const node = byId.get(g.id)
    if (g.parent_id && byId.has(g.parent_id)) byId.get(g.parent_id).children.push(node)
    else roots.push(node)
  }
  for (const h of hosts) {
    const leaf = leafOf(h)
    if (h.group_id && byId.has(h.group_id)) byId.get(h.group_id).children.push(leaf)
    else roots.push(leaf)
  }
  return roots
}

const treeData = computed(() => buildTree(hosts.value, groups.value, h => ({ value: h.id, label: `${h.name} · ${h.ip}` })))

// Expand selection: map group nodes to the host IDs beneath them
const resolveSelected = () => {
  const ids = new Set()
  const descendants = gid => {
    const out = new Set([gid])
    let changed = true
    while (changed) {
      changed = false
      for (const g of groups.value) {
        if (g.parent_id && out.has(g.parent_id) && !out.has(g.id)) { out.add(g.id); changed = true }
      }
    }
    return out
  }
  for (const v of selectedNodes.value) {
    if (typeof v === 'number') { ids.add(v); continue }
    if (v === 'g-none') continue
    const gset = descendants(Number(String(v).slice(2)))
    for (const h of hosts.value) {
      if (h.group_id && gset.has(h.group_id)) ids.add(h.id)
    }
  }
  return [...ids]
}

const groups = ref([])
const loadHosts = async () => {
  hosts.value = await api.get('/hosts')
  scripts.value = await api.get('/scripts')
  usableCreds.value = await api.get('/credentials/usable')
  groups.value = await api.get('/host_groups')
}

onMounted(loadHosts)
onUnmounted(() => ws?.close())

const ensureWS = () => new Promise(resolve => {
  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  ws = new WebSocket(`${proto}://${location.host}/api/ws/task/${taskId.value}?token=${localStorage.getItem('token')}`)
  ws.onmessage = ev => {
    const msg = JSON.parse(ev.data)
    if (msg.type === 'task_done') {
      taskDone.value = true
      taskFailed.value = msg.status === 'failed'
      ws.close()
      return
    }
    if (msg.type === 'status' || msg.type === 'output') {
      let r = liveResults.value.find(x => x.result_id === msg.result_id)
      if (!r) {
        r = { result_id: msg.result_id, host_name: '', host_ip: '', status: 'running', text: '' }
        liveResults.value.push(r)
      }
      if (msg.type === 'output') r.text += msg.text
      if (msg.type === 'status') {
        r.status = msg.status
        if (msg.status === 'success' || msg.status === 'failed') fetchExitCode(r)
      }
    }
  }
  ws.onopen = resolve
})

const liveTotal = computed(() => {
  const ids = resolveSelected().length
  return ids || liveResults.value.length
})
const liveDone = computed(() => {
  const done = liveResults.value.filter(r => r.status === 'success' || r.status === 'failed')
  return { total: done.length, success: done.filter(r => r.status === 'success').length, failed: done.filter(r => r.status === 'failed').length }
})

const fetchExitCode = async r => {
  try {
    const data = await api.get(`/tasks/${taskId.value}`)
    const res = data.results.find(x => x.id === r.result_id)
    if (res) {
      r.exit_code = res.exit_code
      r.host_name = res.host_name
      r.host_ip = res.host_ip
      if (r.text.length < (res.output || '').length && res.output) r.text = res.output
    }
  } catch { /* ignore */ }
}

const run = async () => {
  const ids = resolveSelected()
  const ips = ipInput.value.split(',').map(s => s.trim()).filter(Boolean)
  if (!ids.length && !ips.length) { ElMessage.warning(t('exec.needHosts')); return }
  if (form.mode === 'command' && !form.command.trim()) { ElMessage.warning(t('exec.needCommand')); return }
  if (form.mode === 'script' && !form.script_id) { ElMessage.warning(t('exec.needScript')); return }

  const type = form.mode === 'command' ? t('exec.typeCommand') : t('exec.typeScript')
  try {
    await ElMessageBox.confirm(t('exec.confirmMsg', { n: ids.length || ips.length, type }), t('common.tip'), { type: 'warning' })
  } catch { return }

  const payload = { timeout_sec: form.timeout_sec, concurrency: form.concurrency }
  if (ids.length) payload.host_ids = ids
  if (ips.length) payload.ips = ips
  if (credentialId.value) payload.credential_id = credentialId.value
  if (form.mode === 'command') payload.command = form.command
  else { payload.script_id = form.script_id; payload.script_args = form.script_args }

  try {
    const res = await api.post('/exec', payload)
    taskId.value = res.task_id
    taskDone.value = false
    taskFailed.value = false
    liveResults.value = []
    await ensureWS()
  } catch { /* interception and other errors are surfaced by the interceptor */ }
}
</script>
