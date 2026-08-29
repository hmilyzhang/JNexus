<template>
  <el-card>
    <div style="margin-bottom:12px; display:flex; gap:8px; flex-wrap:wrap">
      <el-input v-model="username" placeholder="操作人" style="width:140px" clearable @change="load" />
      <el-select v-model="action" placeholder="全部操作" clearable style="width:120px" @change="load">
        <el-option label="POST" value="POST" />
        <el-option label="PUT" value="PUT" />
        <el-option label="DELETE" value="DELETE" />
      </el-select>
      <el-input v-model="keyword" placeholder="资源/详情关键字" style="width:200px" clearable @change="load" />
      <el-button @click="load">查询</el-button>
    </div>
    <el-table :data="logs" v-loading="loading" size="small" border>
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column prop="username" label="操作人" width="110" />
      <el-table-column prop="action" label="操作" width="80">
        <template #default="{ row }">
          <el-tag size="small" :type="row.action === 'DELETE' ? 'danger' : row.action === 'PUT' ? 'warning' : 'primary'">{{ row.action }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="resource" label="资源" min-width="180" class-name="mono" />
      <el-table-column prop="detail" label="详情" min-width="240" show-overflow-tooltip class-name="mono" />
      <el-table-column prop="ip" label="来源IP" width="130" />
      <el-table-column prop="status" label="状态码" width="80" />
      <el-table-column prop="created_at" label="时间" width="170" />
    </el-table>
    <el-pagination style="margin-top:12px" layout="total, prev, pager, next" :total="total"
                   :page-size="size" :current-page="page" @current-change="p => { page = p; load() }" />
  </el-card>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import api from '../api'

const logs = ref([])
const total = ref(0)
const page = ref(1)
const size = 50
const username = ref('')
const action = ref('')
const keyword = ref('')
const loading = ref(false)

const load = async () => {
  loading.value = true
  try {
    const data = await api.get('/audit', {
      params: { page: page.value, size, username: username.value, action: action.value, keyword: keyword.value }
    })
    logs.value = data.items
    total.value = data.total
  } finally { loading.value = false }
}
onMounted(load)
</script>
