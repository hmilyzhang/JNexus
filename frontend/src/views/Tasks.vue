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

    <el-drawer v-model="detailVisible" :title="`${$t('tasks.detail')} #${detail?.task?.id}`" size="640px">
      <template v-if="detail">
        <div v-for="r in detail.results" :key="r.id" style="margin-bottom:14px">
          <div style="font-size:13px; font-weight:600">
            {{ r.host_name }}（{{ r.host_ip }}）
            <el-tag size="small" :type="r.status === 'success' ? 'success' : r.status === 'failed' ? 'danger' : 'info'">{{ r.status }}</el-tag>
            <span v-if="r.status !== 'pending'" style="color:#909399; font-size:12px"> {{ $t('tasks.exitCode') }} {{ r.exit_code }}</span>
          </div>
          <div class="log-box">{{ r.output || $t('tasks.noOutput') }}</div>
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
    detail.value = await api.get(`/tasks/${route.query.detail}`)
    detailVisible.value = true
  }
})
</script>
