<!-- JNexus Ops Platform — By JJ Zhang, Version 1.0 -->
<template>
  <el-card>
    <div style="margin-bottom:12px; display:flex; gap:8px; align-items:center">
      <el-button size="small" @click="load">{{ $t('common.refresh') }}</el-button>
      <span style="color:#909399; font-size:12px">{{ $t('paired.tip') }}</span>
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

    <el-card :header="$t('paired.platformKey')" style="margin-top:16px" v-loading="keyLoading">
      <div class="mono pk-line">{{ platformKey.name }}</div>
      <el-input type="textarea" :rows="4" readonly :model-value="platformKey.public_key" class="mono" />
      <div style="color:#909399; font-size:12px; margin-top:6px">{{ platformKey.hint }}</div>
    </el-card>
  </el-card>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import api from '../api'

const rows = ref([])
const platformKey = ref({})
const loading = ref(false)
const keyLoading = ref(true)

const load = async () => {
  loading.value = true
  try { rows.value = await api.get('/credentials/paired') } finally { loading.value = false }
}
onMounted(async () => {
  load()
  try { platformKey.value = await api.get('/system/platform_key') } finally { keyLoading.value = false }
})
</script>

<style scoped>
.pk-line { font-weight: 600; margin-bottom: 6px; }
</style>
