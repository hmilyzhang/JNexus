<template>
  <div>
    <el-card>
      <div style="margin-bottom:12px; display:flex; gap:8px">
        <el-select v-model="typeFilter" placeholder="全部类型" clearable style="width:140px" @change="load">
          <el-option label="命令" value="command" />
          <el-option label="脚本" value="script" />
          <el-option label="文件" value="file" />
          <el-option label="发布" value="release" />
        </el-select>
        <el-button @click="load">刷新</el-button>
      </div>
      <el-table :data="tasks" v-loading="loading" size="small" border @row-click="openDetail">
        <el-table-column prop="id" label="ID" width="70" />
        <el-table-column label="类型" width="90">
          <template #default="{ row }">
            <el-tag size="small">{{ { command: '命令', script: '脚本', file: '文件', release: '发布' }[row.type] }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="operator" label="操作人" width="120" />
        <el-table-column prop="params" label="参数" min-width="240" show-overflow-tooltip />
        <el-table-column label="状态" width="110">
          <template #default="{ row }">
            <el-tag size="small" :type="row.status === 'done' ? 'success' : row.status === 'failed' ? 'danger' : row.status === 'blocked' ? 'warning' : 'primary'">
              {{ row.status }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="发起时间" width="170" />
        <el-table-column prop="finished_at" label="结束时间" width="170" />
      </el-table>
    </el-card>

    <el-drawer v-model="detailVisible" :title="`任务 #${detail?.task?.id} 详情`" size="640px">
      <template v-if="detail">
        <div v-for="r in detail.results" :key="r.id" style="margin-bottom:14px">
          <div style="font-size:13px; font-weight:600">
            {{ r.host_name }}（{{ r.host_ip }}）
            <el-tag size="small" :type="r.status === 'success' ? 'success' : r.status === 'failed' ? 'danger' : 'info'">{{ r.status }}</el-tag>
            <span v-if="r.status !== 'pending'" style="color:#909399; font-size:12px"> 退出码 {{ r.exit_code }}</span>
          </div>
          <div class="log-box">{{ r.output || '(无输出)' }}</div>
        </div>
      </template>
    </el-drawer>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import api from '../api'

const route = useRoute()
const tasks = ref([])
const typeFilter = ref('')
const loading = ref(false)
const detailVisible = ref(false)
const detail = ref(null)

const load = async () => {
  loading.value = true
  try {
    tasks.value = await api.get('/tasks', { params: typeFilter.value ? { type: typeFilter.value } : {} })
  } finally { loading.value = false }
}

const openDetail = async row => {
  detail.value = await api.get(`/tasks/${row.id}`)
  detailVisible.value = true
}

onMounted(async () => {
  await load()
  if (route.query.detail) {
    const d = await api.get(`/tasks/${route.query.detail}`)
    detail.value = d
    detailVisible.value = true
  }
})
</script>
