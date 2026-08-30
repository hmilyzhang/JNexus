<!-- AutoOps 运维平台 — By JJ Zhang, Version 1.0 -->
<template>
  <div>
    <el-card>
      <div style="margin-bottom:12px">
        <el-button type="primary" @click="dlg()">{{ $t('scripts.create') }}</el-button>
        <el-button @click="load">{{ $t('common.refresh') }}</el-button>
      </div>
      <el-table :data="scripts" v-loading="loading" size="small" border>
        <el-table-column prop="id" label="ID" width="70" />
        <el-table-column prop="name" :label="$t('scripts.name')" min-width="160" />
        <el-table-column prop="description" :label="$t('scripts.desc')" min-width="200" show-overflow-tooltip />
        <el-table-column prop="creator" :label="$t('scripts.creator')" width="110" />
        <el-table-column prop="updated_at" :label="$t('scripts.updatedAt')" width="170" />
        <el-table-column :label="$t('common.operation')" width="220" fixed="right">
          <template #default="{ row }">
            <el-button size="small" type="primary" link @click="dlgExec(row)">{{ $t('common.execute') }}</el-button>
            <el-button size="small" link @click="dlg(row)">{{ $t('common.edit') }}</el-button>
            <el-popconfirm :title="$t('scripts.delConfirm')" @confirm="del(row)">
              <template #reference><el-button size="small" type="danger" link>{{ $t('common.delete') }}</el-button></template>
            </el-popconfirm>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="editVisible" :title="form.id ? $t('scripts.edit') : $t('scripts.createTitle')" width="640px">
      <el-form label-width="90px">
        <el-form-item :label="$t('scripts.name')"><el-input v-model="form.name" /></el-form-item>
        <el-form-item :label="$t('scripts.desc')"><el-input v-model="form.description" /></el-form-item>
        <el-form-item :label="$t('scripts.content')">
          <el-input v-model="form.content" type="textarea" :rows="14" class="mono" :placeholder="$t('scripts.contentPlaceholder')" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editVisible = false">{{ $t('common.cancel') }}</el-button>
        <el-button type="primary" @click="save">{{ $t('common.save') }}</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="execVisible" :title="`${$t('scripts.execTitle')}：${current?.name}`" width="560px">
      <el-form label-width="100px">
        <el-form-item :label="$t('scripts.targetHosts')">
          <el-select v-model="hostIds" multiple filterable :placeholder="$t('scripts.targetHosts')" style="width:100%">
            <el-option v-for="h in hosts" :key="h.id" :label="`${h.name} · ${h.ip}`" :value="h.id" />
          </el-select>
        </el-form-item>
        <el-form-item :label="$t('exec.scriptArgs')">
          <el-input v-model="scriptArgs" type="textarea" :rows="2" class="mono" :placeholder="$t('exec.scriptArgsPlaceholder')" />
        </el-form-item>
        <el-form-item :label="$t('exec.timeout')"><el-input-number v-model="timeoutSec" :min="5" :max="3600" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="execVisible = false">{{ $t('common.cancel') }}</el-button>
        <el-button type="primary" @click="doExec">{{ $t('common.execute') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import api from '../api'
import i18n from '../i18n'
import { ElMessage, ElMessageBox } from 'element-plus'

const { t } = i18n.global
const router = useRouter()
const scripts = ref([])
const hosts = ref([])
const loading = ref(false)
const editVisible = ref(false)
const execVisible = ref(false)
const current = ref(null)
const form = ref({})
const hostIds = ref([])
const scriptArgs = ref('')
const timeoutSec = ref(300)

const load = async () => {
  loading.value = true
  try {
    scripts.value = await api.get('/scripts')
  } finally { loading.value = false }
}

onMounted(async () => { load(); hosts.value = await api.get('/hosts') })

const dlg = row => {
  form.value = row ? { ...row } : { name: '', description: '', content: '#!/bin/bash\n' }
  editVisible.value = true
}
const save = async () => {
  if (!form.value.name) { ElMessage.warning(t('scripts.needName')); return }
  if (form.value.id) await api.put(`/scripts/${form.value.id}`, form.value)
  else await api.post('/scripts', form.value)
  ElMessage.success(t('hosts.saved'))
  editVisible.value = false
  load()
}
const del = async row => { await api.delete(`/scripts/${row.id}`); load() }

const dlgExec = async row => {
  current.value = row
  hostIds.value = []
  scriptArgs.value = ''
  execVisible.value = true
}
const doExec = async () => {
  if (!hostIds.value.length) { ElMessage.warning(t('exec.needHosts')); return }
  try {
    await ElMessageBox.confirm(t('scripts.confirmMsg', { name: current.value.name, n: hostIds.value.length }), t('common.tip'), { type: 'warning' })
  } catch { return }
  const res = await api.post(`/scripts/${current.value.id}/exec`, {
    host_ids: hostIds.value, script_args: scriptArgs.value, timeout_sec: timeoutSec.value
  })
  execVisible.value = false
  router.push(`/tasks?detail=${res.task_id}`)
}
</script>
