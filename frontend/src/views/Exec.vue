<template>
  <div>
    <el-row :gutter="16">
      <el-col :span="8">
        <el-card header="目标主机" v-loading="loading">
          <div style="margin-bottom:8px; display:flex; gap:6px">
            <el-button size="small" @click="selectAll(true)">全选</el-button>
            <el-button size="small" @click="selectAll(false)">取消全选</el-button>
            <el-button size="small" @click="loadHosts">刷新</el-button>
          </div>
          <div v-for="g in groupedList" :key="g.name" style="margin-bottom:10px">
            <div style="font-weight:600; margin-bottom:4px; font-size:13px">{{ g.name }}（{{ g.list.length }}）</div>
            <el-checkbox v-for="h in g.list" :key="h.id" v-model="h.checked" style="display:block; margin-left:0">
              {{ h.name }} · {{ h.ip }} <el-tag size="small" :type="h.status==='online'?'success':'info'">{{ h.status }}</el-tag>
            </el-checkbox>
          </div>
        </el-card>
      </el-col>
      <el-col :span="16">
        <el-card>
          <el-form label-width="90px">
            <el-form-item label="执行方式">
              <el-radio-group v-model="form.mode">
                <el-radio value="command">直接命令</el-radio>
                <el-radio value="script">选择脚本</el-radio>
              </el-radio-group>
            </el-form-item>
            <el-form-item label="命令" v-if="form.mode === 'command'">
              <el-input v-model="form.command" type="textarea" :rows="4" placeholder="例如：df -h && free -m" class="mono" />
            </el-form-item>
            <el-form-item label="脚本" v-else>
              <el-select v-model="form.script_id" placeholder="选择脚本" style="width:100%">
                <el-option v-for="s in scripts" :key="s.id" :label="s.name" :value="s.id" />
              </el-select>
            </el-form-item>
            <el-form-item label="附加参数" v-if="form.mode === 'script'">
              <el-input v-model="form.script_args" placeholder="追加到脚本后的参数或额外命令行（可选）" class="mono" />
            </el-form-item>
            <el-form-item label="超时(秒)"><el-input-number v-model="form.timeout_sec" :min="5" :max="3600" /></el-form-item>
            <el-form-item label="并发数"><el-input-number v-model="form.concurrency" :min="1" :max="100" /></el-form-item>
            <el-form-item>
              <el-button type="primary" :loading="running" @click="run">执行</el-button>
            </el-form-item>
          </el-form>
        </el-card>
      </el-col>
    </el-row>

    <el-card header="实时输出" style="margin-top:16px" v-if="taskId">
      <template #header>
        <span>实时输出 · 任务 #{{ taskId }}
          <el-tag size="small" style="margin-left:8px" :type="taskDone ? (taskFailed ? 'danger' : 'success') : 'warning'">
            {{ taskDone ? (taskFailed ? '已完成(有失败)' : '已完成') : '执行中' }}
          </el-tag>
          <el-button size="small" link style="float:right" @click="$router.push(`/tasks?detail=${taskId}`)">查看任务详情</el-button>
        </span>
      </template>
      <div v-for="r in liveResults" :key="r.result_id" style="margin-bottom:12px">
        <div style="font-size:13px; font-weight:600">
          {{ r.host_name }}（{{ r.host_ip }}）
          <el-tag size="small" :type="r.status === 'success' ? 'success' : r.status === 'failed' ? 'danger' : r.status === 'running' ? 'warning' : 'info'">
            {{ r.status }}
          </el-tag>
          <span v-if="r.exit_code !== undefined" style="color:#909399; font-size:12px"> 退出码 {{ r.exit_code }}</span>
        </div>
        <div class="log-box">{{ r.text || '(无输出)' }}</div>
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import api from '../api'
import { ElMessage, ElMessageBox } from 'element-plus'

const hosts = ref([])
const scripts = ref([])
const loading = ref(false)
const running = ref(false)
const taskId = ref(null)
const taskDone = ref(false)
const taskFailed = ref(false)
const liveResults = ref([])
let ws = null

const form = reactive({ mode: 'command', command: '', script_id: null, script_args: '', timeout_sec: 300, concurrency: 10 })

const groupedList = computed(() => {
  const map = new Map()
  for (const h of hosts.value) {
    const name = h.group?.name || '未分组'
    if (!map.has(name)) map.set(name, [])
    map.get(name).push(h)
  }
  return [...map.entries()].map(([name, list]) => ({ name, list }))
})

const selected = () => hosts.value.filter(h => h.checked)

const loadHosts = async () => {
  loading.value = true
  try {
    hosts.value = (await api.get('/hosts')).map(h => ({
      ...h, checked: hosts.value.find(x => x.id === h.id)?.checked || false
    }))
  } finally { loading.value = false }
  scripts.value = await api.get('/scripts')
}

onMounted(loadHosts)
onUnmounted(() => ws?.close())

const selectAll = v => hosts.value.forEach(h => { h.checked = v })

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
        if (msg.status === 'success' || msg.status === 'failed') {
          // 拉取退出码
          fetchExitCode(r)
        }
      }
    }
  }
  ws.onopen = resolve
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
  const ids = selected().map(h => h.id)
  if (!ids.length) { ElMessage.warning('请选择目标主机'); return }
  if (form.mode === 'command' && !form.command.trim()) { ElMessage.warning('请输入命令'); return }
  if (form.mode === 'script' && !form.script_id) { ElMessage.warning('请选择脚本'); return }

  try {
    await ElMessageBox.confirm(
      `将在 ${ids.length} 台主机上执行${form.mode === 'command' ? '命令' : '脚本'}，是否继续？`,
      '执行确认', { type: 'warning' }
    )
  } catch { return }

  const payload = { host_ids: ids, timeout_sec: form.timeout_sec, concurrency: form.concurrency }
  if (form.mode === 'command') payload.command = form.command
  else { payload.script_id = form.script_id; payload.script_args = form.script_args }

  try {
    const res = await api.post('/exec', payload)
    taskId.value = res.task_id
    taskDone.value = false
    taskFailed.value = false
    liveResults.value = []
    await ensureWS()
  } catch (e) {
    // 拦截等错误由拦截器提示
  }
}
</script>
