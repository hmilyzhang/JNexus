<!-- JNexus 运维平台 — By JJ Zhang, Version 1.0 -->
<template>
  <div>
    <el-card :header="$t('report.generate')">
      <div class="tpl-cards">
        <div v-for="tpl in templates" :key="tpl.key" class="tpl-card"
             :class="{ active: form.template === tpl.key }" @click="form.template = tpl.key">
          <div class="tpl-name">{{ tplName(tpl) }}</div>
          <div class="tpl-desc">{{ tplDesc(tpl) }}</div>
        </div>
      </div>
      <el-form label-width="90px" style="margin-top:14px">
        <el-form-item :label="$t('cron.target')">
          <el-select v-model="form.host_ids" multiple filterable style="width:100%"
                     :max-collapse-tags="2" collapse-tags :placeholder="$t('cron.target')">
            <el-option v-for="h in hosts" :key="h.id" :label="`${h.name} · ${h.ip}`" :value="h.id" />
          </el-select>
          <div style="color:#909399; font-size:12px; margin-top:4px">{{ $t('report.targetTip') }}</div>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="generating" :disabled="!form.template"
                     @click="generate">{{ $t('report.generate') }}</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <el-card :header="$t('report.listTitle')" style="margin-top:16px">
      <el-table :data="reports" v-loading="loading" size="small" border @row-click="openDetail">
        <el-table-column prop="id" label="ID" width="70" />
        <el-table-column :label="$t('report.name')" min-width="200">
          <template #default="{ row }">{{ reportLabel(row) }}</template>
        </el-table-column>
        <el-table-column prop="operator" :label="$t('tasks.operator')" width="110" />
        <el-table-column prop="host_count" :label="$t('cron.target')" width="90" />
        <el-table-column :label="$t('tasks.status')" width="100">
          <template #default="{ row }">
            <el-tag size="small" :type="row.status === 'done' ? 'success' : 'warning'">
              {{ row.status === 'done' ? $t('exec.taskDone') : $t('exec.taskRunning') }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" :label="$t('tasks.createdAt')" width="170" />
        <el-table-column :label="$t('common.operation')" width="80" fixed="right">
          <template #default="{ row }">
            <el-popconfirm v-if="store.isAdmin" :title="$t('report.delConfirm')" @confirm="del(row)">
              <template #reference><el-button size="small" type="danger" link>{{ $t('common.delete') }}</el-button></template>
            </el-popconfirm>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 报告详情 -->
    <el-drawer v-model="detailVisible" :title="detail ? reportLabel(detail.report) : $t('report.detail')" size="780px">
      <template v-if="detail">
        <div style="display:flex; gap:8px; align-items:center; margin-bottom:10px">
          <el-tag size="small" type="success">{{ okCount }} {{ $t('report.okShort') }}</el-tag>
          <el-tag size="small" :type="failCount ? 'danger' : 'info'">{{ failCount }} {{ $t('report.failShort') }}</el-tag>
          <el-checkbox v-model="onlyFailed" style="margin-left:8px">{{ $t('report.onlyFailedHosts') }}</el-checkbox>
          <span style="flex:1"></span>
          <el-button size="small" @click="download('log')">{{ $t('tasks.exportLog') }}</el-button>
          <el-button size="small" @click="download('csv')">{{ $t('tasks.exportCsv') }}</el-button>
        </div>
        <el-collapse v-model="expandedHosts">
          <el-collapse-item v-for="it in shownItems" :key="it.id" :name="it.id">
            <template #title>
              <el-tag size="small" :type="it.status === 'success' ? 'success' : 'danger'" style="margin-right:8px">
                {{ it.status === 'success' ? 'OK' : 'FAIL' }}
              </el-tag>
              <span style="font-weight:600">{{ it.host_name }}</span>
              <span class="mono" style="color:#909399; margin-left:8px">{{ it.host_ip }}</span>
              <span v-if="it.error" style="color:#f56c6c; margin-left:8px; font-size:12px; overflow:hidden; text-overflow:ellipsis; white-space:nowrap">{{ it.error }}</span>
            </template>
            <div class="log-box" style="max-height:320px">{{ it.content || $t('report.noOutput') }}</div>
          </el-collapse-item>
        </el-collapse>
        <el-empty v-if="!shownItems.length" :description="$t('report.allOk')" :image-size="60" />
      </template>
    </el-drawer>
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import api from '../api'
import i18n from '../i18n'
import { ElMessage } from 'element-plus'
import { useUserStore } from '../store'

const { t } = i18n.global
const route = useRoute()
const store = useUserStore()
const templates = ref([])
const reports = ref([])
const hosts = ref([])
const loading = ref(false)
const generating = ref(false)
const detailVisible = ref(false)
const detail = ref(null)
const onlyFailed = ref(false)
const expandedHosts = ref([])
const okCount = computed(() => (detail.value?.items || []).filter(i => i.status === 'success').length)
const failCount = computed(() => (detail.value?.items || []).length - okCount.value)
const shownItems = computed(() => {
  const items = detail.value?.items || []
  return onlyFailed.value ? items.filter(i => i.status !== 'success') : items
})
const autoExpandFailures = () => {
  const items = detail.value?.items || []
  expandedHosts.value = items.filter(i => i.status !== 'success').map(i => i.id)
}
let pollTimer = null

const generate = async () => {
  generating.value = true
  try {
    const res = await api.post('/reports', { template: form.value.template, host_ids: form.value.host_ids })
    ElMessage.success(t('report.generated'))
    await load()
    openDetail({ id: res.report_id })
  } finally { generating.value = false }
}

const form = ref({ template: 'accounts', host_ids: [] })
// 模板名/描述优先取语言包（report.tpl_<key>），无对应键时回退后端返回值
const tplName = tpl => i18n.global.te(`report.tpl_${tpl.key}`)
  ? i18n.global.t(`report.tpl_${tpl.key}`) : tpl.name
const tplDesc = tpl => i18n.global.te(`report.tpl_${tpl.key}_desc`)
  ? i18n.global.t(`report.tpl_${tpl.key}_desc`) : tpl.desc
// 报告存库名称含中文模板名，展示时按 template 键重新本地化
const fmtStamp = iso => {
  const d = iso ? new Date(iso) : null
  if (!d || isNaN(d)) return ''
  const p = n => String(n).padStart(2, '0')
  return `${d.getFullYear()}${p(d.getMonth() + 1)}${p(d.getDate())}-${p(d.getHours())}${p(d.getMinutes())}${p(d.getSeconds())}`
}
const reportLabel = r => {
  if (r.template && i18n.global.te(`report.tpl_${r.template}`))
    return `${i18n.global.t(`report.tpl_${r.template}`)} ${fmtStamp(r.created_at)}`.trim()
  return r.name
}
const openDetail = async row => {
  detail.value = await api.get(`/reports/${row.id}`)
  autoExpandFailures()
  detailVisible.value = true
  pollDetail(row.id)
}
const pollDetail = id => {
  clearInterval(pollTimer)
  pollTimer = setInterval(async () => {
    try {
      const d = await api.get(`/reports/${id}`)
      detail.value = d
      autoExpandFailures()
      if (d.report.status === 'done') clearInterval(pollTimer)
    } catch { clearInterval(pollTimer) }
  }, 2000)
}
onUnmounted(() => clearInterval(pollTimer))

const download = async format => {
  const data = await api.get(`/reports/${detail.value.report.id}/export`, { params: { format }, responseType: 'blob' })
  const url = URL.createObjectURL(new Blob([data]))
  const a = document.createElement('a')
  a.href = url
  a.download = `report-${detail.value.report.id}.${format}`
  a.click()
  URL.revokeObjectURL(url)
}

const load = async () => {
  loading.value = true
  try {
    reports.value = await api.get('/reports')
    templates.value = await api.get('/reports/templates')
  } finally { loading.value = false }
}

const del = async row => { await api.delete(`/reports/${row.id}`); load() }

onMounted(async () => {
  await load()
  hosts.value = await api.get('/hosts')
  if (route.query.tpl) form.value.template = route.query.tpl
})
</script>

<style scoped>
.tpl-cards { display: grid; grid-template-columns: repeat(auto-fill, minmax(200px, 1fr)); gap: 10px; }
.tpl-card {
  border: 1px solid #dcdfe6; border-radius: 6px; padding: 12px; cursor: pointer;
  transition: border-color .2s;
}
.tpl-card:hover { border-color: #409eff; }
.tpl-card.active { border-color: #409eff; background: #ecf5ff; }
.tpl-name { font-weight: 600; margin-bottom: 4px; }
.tpl-desc { color: #909399; font-size: 12px; line-height: 1.5; }
</style>
