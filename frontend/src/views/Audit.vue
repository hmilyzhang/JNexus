<!-- AutoOps 运维平台 — By JJ Zhang, Version 1.0 -->
<template>
  <el-card>
    <div style="margin-bottom:12px; display:flex; gap:8px; flex-wrap:wrap">
      <el-input v-model="username" :placeholder="$t('audit.operator')" style="width:140px" clearable @change="load" />
      <el-select v-model="action" :placeholder="$t('audit.action')" clearable style="width:120px" @change="load">
        <el-option label="POST" value="POST" />
        <el-option label="PUT" value="PUT" />
        <el-option label="DELETE" value="DELETE" />
      </el-select>
      <el-input v-model="keyword" :placeholder="$t('audit.keywordPlaceholder')" style="width:200px" clearable @change="load" />
      <el-button @click="load">{{ $t('common.search') }}</el-button>
    </div>
    <el-table :data="logs" v-loading="loading" size="small" border>
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column prop="username" :label="$t('audit.operator')" width="110" />
      <el-table-column :label="$t('audit.action')" width="80">
        <template #default="{ row }">
          <el-tag size="small" :type="row.action === 'DELETE' ? 'danger' : row.action === 'PUT' ? 'warning' : 'primary'">{{ row.action }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="resource" :label="$t('audit.resource')" min-width="180" class-name="mono" />
      <el-table-column prop="detail" :label="$t('audit.detail')" min-width="240" show-overflow-tooltip class-name="mono" />
      <el-table-column prop="ip" :label="$t('audit.sourceIp')" width="130" />
      <el-table-column prop="status" :label="$t('audit.statusCode')" width="80" />
      <el-table-column prop="created_at" :label="$t('audit.time')" width="170" />
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
