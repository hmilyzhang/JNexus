<template>
  <div>
    <el-card>
      <div style="margin-bottom:12px">
        <el-button type="primary" @click="dlg()">新建应用</el-button>
        <el-button @click="load">刷新</el-button>
      </div>
      <el-table :data="apps" v-loading="loading" size="small" border>
        <el-table-column prop="id" label="ID" width="70" />
        <el-table-column prop="name" label="应用名" width="160" />
        <el-table-column prop="description" label="描述" min-width="180" show-overflow-tooltip />
        <el-table-column label="部署主机" min-width="300">
          <template #default="{ row }">
            <el-tag v-for="ah in row.app_hosts || []" :key="ah.id" size="small" style="margin-right:6px">
              {{ ah.host?.name || ah.host?.ip || ah.host_id }} : {{ ah.deploy_dir }}/{{ ah.jar_name }}
            </el-tag>
            <span v-if="!(row.app_hosts || []).length" style="color:#909399">未绑定</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="140" fixed="right">
          <template #default="{ row }">
            <el-button size="small" link @click="dlg(row)">编辑</el-button>
            <el-popconfirm title="确认删除应用?" @confirm="del(row)">
              <template #reference><el-button size="small" type="danger" link>删除</el-button></template>
            </el-popconfirm>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="visible" :title="form.id ? '编辑应用' : '新建应用'" width="760px">
      <el-form label-width="90px">
        <el-form-item label="应用名"><el-input v-model="form.name" /></el-form-item>
        <el-form-item label="描述"><el-input v-model="form.description" /></el-form-item>
        <el-form-item label="部署配置">
          <div style="width:100%">
            <div v-for="(ah, i) in form.app_hosts" :key="i" class="ah-row">
              <el-select v-model="ah.host_id" filterable placeholder="主机" style="width:180px" size="small">
                <el-option v-for="h in hosts" :key="h.id" :label="`${h.name} · ${h.ip}`" :value="h.id" />
              </el-select>
              <el-input v-model="ah.deploy_dir" placeholder="部署目录 如 /app/myapp" class="mono" style="width:180px" size="small" />
              <el-input v-model="ah.jar_name" placeholder="jar名 如 app.jar" class="mono" style="width:140px" size="small" />
              <el-input v-model="ah.stop_cmd" placeholder="停止命令(空=按进程名kill)" class="mono" style="width:200px" size="small" />
              <el-input v-model="ah.start_cmd" placeholder="启动命令(空=默认java -jar)" class="mono" style="width:200px" size="small" />
              <el-input v-model="ah.backup_dir" placeholder="备份目录(空=部署目录/backup)" class="mono" style="width:180px" size="small" />
              <el-input v-model="ah.health_check_url" placeholder="健康检查URL(可选)" class="mono" style="width:180px" size="small" />
              <el-button type="danger" link size="small" @click="form.app_hosts.splice(i, 1)">移除</el-button>
            </div>
            <el-button size="small" @click="form.app_hosts.push({ host_id: null, deploy_dir: '', jar_name: 'app.jar', stop_cmd: '', start_cmd: '', backup_dir: '', health_check_url: '' })">
              + 添加主机
            </el-button>
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="visible = false">取消</el-button>
        <el-button type="primary" @click="save">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import api from '../api'
import { ElMessage } from 'element-plus'

const apps = ref([])
const hosts = ref([])
const loading = ref(false)
const visible = ref(false)
const form = ref({ name: '', description: '', app_hosts: [] })

const load = async () => {
  loading.value = true
  try { apps.value = await api.get('/apps') } finally { loading.value = false }
}
onMounted(async () => { load(); hosts.value = await api.get('/hosts') })

const dlg = row => {
  form.value = row ? {
    id: row.id, name: row.name, description: row.description,
    app_hosts: (row.app_hosts || []).map(ah => ({
      host_id: ah.host_id, deploy_dir: ah.deploy_dir, jar_name: ah.jar_name,
      stop_cmd: ah.stop_cmd, start_cmd: ah.start_cmd, backup_dir: ah.backup_dir, health_check_url: ah.health_check_url
    }))
  } : { name: '', description: '', app_hosts: [] }
  visible.value = true
}
const save = async () => {
  if (!form.value.name) { ElMessage.warning('应用名必填'); return }
  if (form.value.app_hosts.some(ah => !ah.host_id || !ah.deploy_dir || !ah.jar_name)) {
    ElMessage.warning('每条部署配置需要主机、目录与 jar 名')
    return
  }
  if (form.value.id) await api.put(`/apps/${form.value.id}`, form.value)
  else await api.post('/apps', form.value)
  ElMessage.success('已保存')
  visible.value = false
  load()
}
const del = async row => { await api.delete(`/apps/${row.id}`); load() }
</script>

<style scoped>
.ah-row {
  display: flex; gap: 6px; margin-bottom: 6px; flex-wrap: wrap;
  padding: 6px; border: 1px solid #ebeef5; border-radius: 4px;
}
</style>
