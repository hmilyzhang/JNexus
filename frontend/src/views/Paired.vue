<!-- JNexus Ops Platform — By JJ Zhang, Version 1.0 -->
<!-- Paired keys: platform key rotation config + paginated paired-credential list. -->
<template>
  <div>
    <!-- Rotation config -->
    <el-card v-loading="rotLoading">
      <template #header>
        <div style="display:flex; align-items:center; gap:10px">
          <span style="font-weight:600">{{ $t('paired.rotTitle') }}</span>
          <el-tag v-if="rot.running" size="small" type="warning">{{ $t('common.running') }}</el-tag>
        </div>
      </template>
      <div style="display:flex; gap:24px; align-items:center; flex-wrap:wrap">
        <div style="display:flex; align-items:center; gap:8px">
          <span>{{ $t('paired.rotEnabled') }}</span>
          <el-switch v-model="rot.enabled" />
        </div>
        <div style="display:flex; align-items:center; gap:8px">
          <span>{{ $t('paired.rotDays') }}</span>
          <el-input-number v-model="rot.days" :min="7" :max="365" :step="1" step-strictly style="width:110px" />
          <span style="color:#909399; font-size:12px">{{ $t('paired.rotDaysMin') }}</span>
        </div>
        <div style="display:flex; align-items:center; gap:8px">
          <span>{{ $t('paired.rotLast') }}</span>
          <span>{{ fmtTime(rot.lastRun) || $t('paired.never') }}</span>
        </div>
        <span style="flex:1"></span>
        <el-button :disabled="rot.running" :loading="rot.starting" type="warning" @click="rotateNow">
          {{ $t('paired.rotNow') }}
        </el-button>
        <el-button type="primary" :loading="rot.saving" @click="saveRotation">{{ $t('common.save') }}</el-button>
      </div>
      <div style="color:#909399; font-size:12px; margin-top:10px">{{ $t('paired.rotTip') }}</div>
    </el-card>

    <!-- Paired credential list -->
    <el-card style="margin-top:16px">
      <div style="margin-bottom:12px; display:flex; gap:8px; align-items:center; flex-wrap:wrap">
        <el-input v-model="keyword" :placeholder="$t('paired.searchPh')" clearable style="width:260px"
                  @keyup.enter="search" @clear="search" />
        <el-button size="small" @click="search">{{ $t('common.search') }}</el-button>
        <span style="flex:1"></span>
        <el-button size="small" @click="search">{{ $t('common.refresh') }}</el-button>
      </div>
      <el-table :data="rows" v-loading="loading" size="small" border>
        <el-table-column prop="name" :label="$t('paired.name')" min-width="180">
          <template #default="{ row }"><span class="mono">{{ row.name }}</span></template>
        </el-table-column>
        <el-table-column :label="$t('menu.hosts')" min-width="150">
          <template #default="{ row }">{{ row.host_name }}（{{ row.host_ip }}）</template>
        </el-table-column>
        <el-table-column prop="username" :label="$t('hosts.credUser')" min-width="110" />
        <el-table-column prop="label" :label="$t('hosts.credLabel')" min-width="120" />
        <el-table-column prop="key_name" :label="$t('paired.keyName')" min-width="200" show-overflow-tooltip />
        <el-table-column :label="$t('common.status')" min-width="100">
          <template #default="{ row }">
            <el-tag size="small" :type="row.is_default ? 'success' : 'info'">{{ row.is_default ? $t('hosts.credDefault') : '-' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" :label="$t('releases.time')" min-width="170" />
      </el-table>
      <div style="margin-top:12px; display:flex; justify-content:flex-end">
        <el-pagination v-model:current-page="page" v-model:page-size="pageSize"
                       :total="total" :page-sizes="[20, 50, 100, 200]" layout="total, sizes, prev, pager, next, jumper"
                       @current-change="load" @size-change="onSizeChange" />
      </div>
    </el-card>

    <el-card :header="$t('paired.platformKey')" style="margin-top:16px" v-loading="keyLoading">
      <div class="mono pk-line">{{ platformKey.name }}</div>
      <el-input type="textarea" :rows="4" readonly :model-value="platformKey.public_key" class="mono" />
      <div style="color:#909399; font-size:12px; margin-top:6px">{{ platformKey.hint }}</div>
    </el-card>
  </div>
</template>

<script setup>
import { onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import api from '../api'
import i18n from '../i18n'

const { t } = i18n.global

// ---- paired credentials (server-side pagination + keyword) ----
const rows = ref([])
const keyword = ref('')
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const loading = ref(false)

const load = async () => {
  loading.value = true
  try {
    const r = await api.get('/credentials/paired', {
      params: { page: page.value, page_size: pageSize.value, keyword: keyword.value.trim() },
    })
    rows.value = r.items || []
    total.value = r.total || 0
  } finally { loading.value = false }
}
const search = () => { page.value = 1; load() }
const onSizeChange = () => { page.value = 1; load() }

// ---- key rotation config ----
const rot = reactive({ enabled: false, days: 30, lastRun: null, running: false, saving: false, starting: false })
const rotLoading = ref(false)
let pollTimer = null

const loadRotation = async () => {
  rotLoading.value = true
  try {
    const r = await api.get('/system/key-rotation')
    rot.enabled = r.ssh_key_rotation_enabled === 'true'
    rot.days = r.days || 30
    rot.lastRun = r.last_run || null
    rot.running = !!r.running
  } finally { rotLoading.value = false }
}

const saveRotation = async () => {
  rot.saving.value = true
  try {
    await api.put('/system/key-rotation', {
      ssh_key_rotation_enabled: rot.enabled ? 'true' : 'false',
      ssh_key_rotation_days: String(rot.days),
    })
    ElMessage.success(t('common.success'))
  } finally { rot.saving.value = false }
}

const rotateNow = async () => {
  try {
    await ElMessageBox.confirm(t('paired.rotNowConfirm'), t('common.tip'), { type: 'warning' })
  } catch { return }
  rot.starting.value = true
  try {
    await api.post('/system/key-rotation/run')
    ElMessage.success(t('paired.rotStarted'))
    rot.running = true
    // poll until the background rotation finishes, then refresh last-run display
    stopPoll()
    pollTimer = setInterval(async () => {
      try {
        const r = await api.get('/system/key-rotation')
        rot.running = !!r.running
        if (!rot.running) {
          rot.lastRun = r.last_run || rot.lastRun
          stopPoll()
        }
      } catch { stopPoll() }
    }, 5000)
  } catch (e) {
    ElMessage.error(e?.response?.data?.error || t('common.failed'))
  } finally { rot.starting.value = false }
}
const stopPoll = () => { if (pollTimer) { clearInterval(pollTimer); pollTimer = null } }

const fmtTime = v => {
  if (!v) return ''
  const d = new Date(v)
  return isNaN(d) ? '' : d.toLocaleString()
}

onMounted(() => { load(); loadRotation() })
onBeforeUnmount(stopPoll)

const platformKey = ref({})
const keyLoading = ref(true)
onMounted(async () => {
  try { platformKey.value = await api.get('/system/platform_key') } finally { keyLoading.value = false }
})
</script>

<style scoped>
.pk-line { font-weight: 600; margin-bottom: 6px; }
</style>
