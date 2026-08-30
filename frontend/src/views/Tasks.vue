<!-- AutoOps 运维平台 — By JJ Zhang, Version 1.0 -->
<template>
  <div>
    <el-card>
      <div style="margin-bottom:12px; display:flex; gap:8px">
        <el-select v-model="typeFilter" :placeholder="$t('tasks.allTypes')" clearable style="width:140px" @change="load">
          <el-option :label="$t('tasks.typeCommand')" value="command" />
          <el-option :label="$t('tasks.typeScript')" value="script" />
          <el-option :label="$t('tasks.typeFile')" value="file" />
          <el-option :label="$t('tasks.typeRelease')" value="release" />
        </el-select>
        <el-button @click="load">{{ $t('common.refresh') }}</el-button>
        <el-tag v-if="mineOnly" size="small" type="info" style="align-self:center">{{ $t('tasks.mine') }}</el-tag>
      </div>
      <el-table :data="tasks" v-loading="loading" size="small" border @row-click="openDetail">
        <el-table-column prop="id" label="ID" width="70" />
        <el-table-column :label="$t('tasks.detail')" width="90">
          <template #default="{ row }">
            <el-tag size="small">{{ { command: $t('tasks.typeCommand'), script: $t('tasks.typeScript'), file: $t('tasks.typeFile'), release: $t('tasks.typeRelease') }[row.type] }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="operator" :label="$t('tasks.operator')" width="120" />
        <el-table-column prop="params" :label="$t('tasks.params')" min-width="240" show-overflow-tooltip />
        <el-table-column :label="$t('tasks.status')" width="110">
          <template #default="{ row }">
            <el-tag size="small" :type="row.status === 'done' ? 'success' : row.status === 'failed' ? 'danger' : row.status === 'blocked' ? 'warning' : 'primary'">
              {{ row.status }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" :label="$t('tasks.createdAt')" width="170" />
        <el-table-column prop="finished_at" :label="$t('tasks.finishedAt')" width="170" />
      </el-table>
    </el-card>

    <!-- 任务控制台（聚合视图） -->
    <el-drawer v-model="detailVisible" :title="`${$t('tasks.detail')} #${detail?.task?.id}`" size="820px">
      <template v-if="detail">
        <!-- 汇总统计 -->
        <div class="sum-line">
          <el-tag size="small">{{ typeText(detail.task.type) }}</el-tag>
          <span class="sum-op">{{ detail.task.operator }}</span>
          <span class="sum-time">{{ fmtTime(detail.task.created_at) }}</span>
          <span style="margin-left:auto; font-size:13px">
            <el-tag size="small" type="success">{{ $t('common.success') }} {{ cnt.success }}</el-tag>
            <el-tag size="small" type="danger" style="margin-left:4px">{{ $t('common.failed') }} {{ cnt.failed }}</el-tag>
            <el-tag size="small" type="info" style="margin-left:4px">{{ $t('tasks.total') }} {{ detail.results.length }}</el-tag>
          </span>
        </div>
        <div class="sum-params mono">{{ detail.task.params }}</div>

        <!-- 工具栏 -->
        <div class="toolbar">
          <el-switch v-model="onlyFailed" style="margin-right:4px" />
          <span style="font-size:13px; margin-right:10px">{{ $t('tasks.onlyFailed') }}</span>
          <el-input v-model="keyword" :placeholder="$t('tasks.searchOutput')" size="small" style="width:200px" clearable />
          <span style="flex:1"></span>
          <el-button size="small" @click="download('log')">{{ $t('tasks.exportLog') }}</el-button>
          <el-button size="small" @click="download('csv')">{{ $t('tasks.exportCsv') }}</el-button>
        </div>

        <!-- 主机汇总表格 -->
        <el-table :data="filteredResults" size="small" border highlight-current-row
                  :row-class-name="rowClass" @row-click="selectRow" style="margin-top:10px">
          <el-table-column :label="$t('menu.hosts')" min-width="180">
            <template #default="{ row }">{{ row.host_name }}（{{ row.host_ip }}）</template>
          </el-table-column>
          <el-table-column prop="os_user" :label="$t('hosts.credUser')" width="100">
            <template #default="{ row }">{{ row.os_user || '-' }}</template>
          </el-table-column>
          <el-table-column :label="$t('tasks.status')" width="90">
            <template #default="{ row }">
              <el-tag size="small" :type="row.status === 'success' ? 'success' : row.status === 'failed' ? 'danger' : 'info'">{{ statusText(row.status) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column :label="$t('tasks.exitCode')" width="80">
            <template #default="{ row }">{{ row.status === 'pending' ? '-' : row.exit_code }}</template>
          </el-table-column>
          <el-table-column :label="$t('tasks.duration')" width="90">
            <template #default="{ row }">{{ duration(row) }}</template>
          </el-table-column>
        </el-table>

        <!-- 输出面板 -->
        <template v-if="selected">
          <div class="out-title">
            {{ selected.host_name }}（{{ selected.host_ip }}）
            <el-tag size="small" :type="selected.status === 'success' ? 'success' : 'danger'" style="margin:0 6px">{{ statusText(selected.status) }}</el-tag>
            <span style="color:#909399; font-size:12px">{{ $t('tasks.exitCode') }} {{ selected.exit_code }}</span>
          </div>
          <div class="log-box">{{ selected.output || $t('tasks.noOutput') }}</div>
        </template>
        <div v-else class="out-title" style="color:#909399">{{ $t('tasks.clickRow') }}</div>
      </template>
    </el-drawer>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import api from '../api'
import i18n from '../i18n'
import { useUserStore } from '../store'

const { t } = i18n.global
const route = useRoute()
const store = useUserStore()
const tasks = ref([])
const typeFilter = ref('')
const loading = ref(false)
const detailVisible = ref(false)
const detail = ref(null)
const onlyFailed = ref(false)
const keyword = ref('')
const selected = ref(null)
const detailTaskId = ref(null)

const mineOnly = computed(() => !['admin', 'auditor'].includes(store.role))

const cnt = computed(() => {
  const r = detail.value?.results || []
  return {
    success: r.filter(x => x.status === 'success').length,
    failed: r.filter(x => x.status === 'failed').length
  }
})

// 失败优先排序 + 只看失败 + 关键字过滤（主机名/IP/输出）
const filteredResults = computed(() => {
  let list = [...(detail.value?.results || [])]
  const weight = s => (s === 'failed' ? 0 : s === 'running' ? 1 : 2)
  list.sort((a, b) => weight(a.status) - weight(b.status) || a.id - b.id)
  if (onlyFailed.value) list = list.filter(x => x.status === 'failed')
  const kw = keyword.value.trim().toLowerCase()
  if (kw) {
    list = list.filter(x =>
      (x.output || '').toLowerCase().includes(kw) ||
      (x.host_name || '').toLowerCase().includes(kw) ||
      (x.host_ip || '').toLowerCase().includes(kw))
  }
  return list
})

const typeText = ty => ({ command: t('tasks.typeCommand'), script: t('tasks.typeScript'), file: t('tasks.typeFile'), release: t('tasks.typeRelease') }[ty] || ty)
const statusText = st => ({ success: t('common.success'), failed: t('common.failed'), running: t('common.running'), pending: t('common.unknown') }[st] || st)
const fmtTime = v => (v ? String(v).replace('T', ' ').slice(0, 19) : '-')
const duration = r => (r.finished_at ? (new Date(r.finished_at) - new Date(r.created_at)) / 1000 + 's' : '-')
const selectRow = row => { selected.value = row }
const rowClass = ({ row }) => (selected.value && row.id === selected.value.id ? 'cur-row' : '')

const load = async () => {
  loading.value = true
  try {
    tasks.value = await api.get('/tasks', { params: typeFilter.value ? { type: typeFilter.value } : {} })
  } finally { loading.value = false }
}

const openDetail = async row => {
  detailTaskId.value = row.id
  detail.value = await api.get(`/tasks/${row.id}`)
  selected.value = detail.value.results.length ? detail.value.results[0] : null
  onlyFailed.value = false
  keyword.value = ''
  detailVisible.value = true
}

// blob 下载（带认证头）
const download = async format => {
  const data = await api.get(`/tasks/${detailTaskId.value}/export`, { params: { format }, responseType: 'blob' })
  const url = URL.createObjectURL(new Blob([data]))
  const a = document.createElement('a')
  a.href = url
  a.download = `task-${detailTaskId.value}.${format}`
  a.click()
  URL.revokeObjectURL(url)
}

onMounted(async () => {
  await load()
  if (route.query.detail) {
    openDetail({ id: Number(route.query.detail) })
  }
})
</script>

<style scoped>
.sum-line { display: flex; align-items: center; gap: 8px; }
.sum-op { font-weight: 600; }
.sum-time { color: #909399; font-size: 12px; }
.sum-params {
  margin: 8px 0; padding: 6px 10px; background: #f5f7fa; border-radius: 4px;
  font-size: 12px; color: #606266; word-break: break-all;
}
.toolbar { display: flex; align-items: center; margin-top: 6px; }
.out-title { margin: 12px 0 6px; font-size: 13px; font-weight: 600; }
.log-box { max-height: 300px; }
:deep(.cur-row) { background: #ecf5ff; }
</style>
