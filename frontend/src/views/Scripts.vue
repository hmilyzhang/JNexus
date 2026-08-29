<template>
  <div>
    <el-card>
      <div style="margin-bottom:12px">
        <el-button type="primary" @click="dlg()">新建脚本</el-button>
        <el-button @click="load">刷新</el-button>
      </div>
      <el-table :data="scripts" v-loading="loading" size="small" border>
        <el-table-column prop="id" label="ID" width="70" />
        <el-table-column prop="name" label="名称" min-width="160" />
        <el-table-column prop="description" label="描述" min-width="200" show-overflow-tooltip />
        <el-table-column prop="creator" label="创建人" width="110" />
        <el-table-column prop="updated_at" label="更新时间" width="170" />
        <el-table-column label="操作" width="220" fixed="right">
          <template #default="{ row }">
            <el-button size="small" type="primary" link @click="dlgExec(row)">执行</el-button>
            <el-button size="small" link @click="dlg(row)">编辑</el-button>
            <el-popconfirm title="确认删除脚本?" @confirm="del(row)">
              <template #reference><el-button size="small" type="danger" link>删除</el-button></template>
            </el-popconfirm>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="editVisible" :title="form.id ? '编辑脚本' : '新建脚本'" width="640px">
      <el-form label-width="80px">
        <el-form-item label="名称"><el-input v-model="form.name" /></el-form-item>
        <el-form-item label="描述"><el-input v-model="form.description" /></el-form-item>
        <el-form-item label="内容">
          <el-input v-model="form.content" type="textarea" :rows="14" class="mono"
                    placeholder="#!/bin/bash&#10;echo hello $(hostname)" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editVisible = false">取消</el-button>
        <el-button type="primary" @click="save">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="execVisible" :title="`执行脚本：${current?.name}`" width="560px">
      <el-form label-width="100px">
        <el-form-item label="目标主机">
          <el-select v-model="hostIds" multiple filterable placeholder="选择主机" style="width:100%">
            <el-option v-for="h in hosts" :key="h.id" :label="`${h.name} · ${h.ip}`" :value="h.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="附加参数">
          <el-input v-model="scriptArgs" type="textarea" :rows="2" class="mono" placeholder="追加到脚本末尾的内容（可选）" />
        </el-form-item>
        <el-form-item label="超时(秒)"><el-input-number v-model="timeoutSec" :min="5" :max="3600" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="execVisible = false">取消</el-button>
        <el-button type="primary" @click="doExec">执行</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import api from '../api'
import { ElMessage, ElMessageBox } from 'element-plus'

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
  if (!form.value.name) { ElMessage.warning('名称必填'); return }
  if (form.value.id) await api.put(`/scripts/${form.value.id}`, form.value)
  else await api.post('/scripts', form.value)
  ElMessage.success('已保存')
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
  if (!hostIds.value.length) { ElMessage.warning('请选择目标主机'); return }
  try {
    await ElMessageBox.confirm(`将在 ${hostIds.value.length} 台主机上执行脚本「${current.value.name}」`, '执行确认', { type: 'warning' })
  } catch { return }
  const res = await api.post(`/scripts/${current.value.id}/exec`, {
    host_ids: hostIds.value, script_args: scriptArgs.value, timeout_sec: timeoutSec.value
  })
  execVisible.value = false
  router.push(`/tasks?detail=${res.task_id}`)
}
</script>
