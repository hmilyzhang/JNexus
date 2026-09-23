<!-- JNexus Ops Platform — By JJ Zhang, Version 1.0 -->
<template>
  <div>
    <el-card>
      <div style="margin-bottom:12px">
        <el-button v-if="canManage" type="primary" @click="dlg()">{{ $t('apps.create') }}</el-button>
        <el-button @click="load">{{ $t('common.refresh') }}</el-button>
      </div>
      <el-table :data="apps" v-loading="loading" size="small" border>
        <el-table-column prop="id" label="ID" width="70" />
        <el-table-column prop="name" :label="$t('apps.name')" width="160" />
        <el-table-column prop="description" :label="$t('apps.desc')" min-width="180" show-overflow-tooltip />
        <el-table-column :label="$t('apps.deployHosts')" min-width="300">
          <template #default="{ row }">
            <el-tag v-for="ah in row.app_hosts || []" :key="ah.id" size="small" style="margin-right:6px">
              {{ ah.host?.name || ah.host?.ip || ah.host_id }} : {{ ah.deploy_dir }}/{{ ah.jar_name }}
            </el-tag>
            <span v-if="!(row.app_hosts || []).length" style="color:#909399">{{ $t('apps.unbound') }}</span>
          </template>
        </el-table-column>
        <el-table-column :label="$t('common.operation')" width="140" fixed="right">
          <template #default="{ row }">
            <el-button v-if="canManage" size="small" link @click="dlg(row)">{{ $t('common.edit') }}</el-button>
            <el-popconfirm :title="$t('apps.delConfirm')" @confirm="del(row)">
              <template #reference><el-button v-if="canManage" size="small" type="danger" link>{{ $t('common.delete') }}</el-button></template>
            </el-popconfirm>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="visible" :title="form.id ? $t('apps.edit') : $t('apps.createTitle')" width="760px">
      <el-form label-width="110px">
        <el-form-item :label="$t('apps.name')"><el-input v-model="form.name" /></el-form-item>
        <el-form-item :label="$t('apps.desc')"><el-input v-model="form.description" /></el-form-item>
        <el-form-item :label="$t('apps.deployConfig')">
          <div style="width:100%">
            <div v-for="(ah, i) in form.app_hosts" :key="i" class="ah-row">
              <el-select v-model="ah.host_id" filterable :placeholder="$t('apps.host')" style="width:180px" size="small">
                <el-option v-for="h in hosts" :key="h.id" :label="`${h.name} · ${h.ip}`" :value="h.id" />
              </el-select>
              <el-select v-model="ah.credential_id" filterable clearable :placeholder="$t('hosts.credPublishAccount')" style="width:200px" size="small">
                <el-option v-for="c in usableCreds.filter(x => x.host_id === ah.host_id)" :key="c.id"
                           :label="`${c.username}${c.label ? '（' + c.label + '）' : ''}`" :value="c.id" />
              </el-select>
              <el-input v-model="ah.deploy_dir" :placeholder="$t('apps.deployDirPlaceholder')" class="mono" style="width:180px" size="small" />
              <el-input v-model="ah.jar_name" :placeholder="$t('apps.jarNamePlaceholder')" class="mono" style="width:140px" size="small" />
              <el-input v-model="ah.stop_cmd" :placeholder="$t('apps.stopCmdPlaceholder')" class="mono" style="width:200px" size="small" />
              <el-input v-model="ah.start_cmd" :placeholder="$t('apps.startCmdPlaceholder')" class="mono" style="width:200px" size="small" />
              <el-input v-model="ah.backup_dir" :placeholder="$t('apps.backupDirPlaceholder')" class="mono" style="width:180px" size="small" />
              <el-input v-model="ah.health_check_url" :placeholder="$t('apps.healthUrlPlaceholder')" class="mono" style="width:180px" size="small" />
              <el-button type="danger" link size="small" @click="form.app_hosts.splice(i, 1)">{{ $t('apps.remove') }}</el-button>
            </div>
            <div style="display:flex; gap:8px; align-items:center; width:100%; flex-wrap:wrap">
              <el-select v-model="bulkGroup" filterable clearable size="small" :placeholder="$t('apps.fromGroup')"
                         style="width:200px" @change="addFromGroup">
                <el-option v-for="g in hostGroups" :key="g.id" :label="groupLabel(hostGroups, g)" :value="g.id" />
              </el-select>
              <el-button size="small" @click="form.app_hosts.push({ host_id: null, credential_id: null, deploy_dir: '', jar_name: 'app.jar', stop_cmd: '', start_cmd: '', backup_dir: '', health_check_url: '' })">
                {{ $t('apps.addHostRow') }}
              </el-button>
              <span style="color:#909399; font-size:12px">{{ $t('apps.fromGroupTip') }}</span>
            </div>
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="visible = false">{{ $t('common.cancel') }}</el-button>
        <el-button type="primary" @click="save">{{ $t('common.save') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import api from '../api'
import { groupLabel } from '../utils/groupPath'
import { useUserStore } from '../store'
import i18n from '../i18n'
import { ElMessage } from 'element-plus'

const { t } = i18n.global
const store = useUserStore()
const canManage = computed(() => ['admin', 'ops', 'publisher'].includes(store.role))
const apps = ref([])
const hosts = ref([])
const hostGroups = ref([])
const bulkGroup = ref(null)
const usableCreds = ref([])
const loading = ref(false)
const visible = ref(false)
const form = ref({ name: '', description: '', app_hosts: [] })

const load = async () => {
  loading.value = true
  try { apps.value = await api.get('/apps') } finally { loading.value = false }
}
onMounted(async () => {
  load()
  hosts.value = await api.get('/hosts')
  hostGroups.value = await api.get('/host_groups')
  usableCreds.value = await api.get('/credentials/usable')
})

const dlg = row => {
  form.value = row ? {
    id: row.id, name: row.name, description: row.description,
    app_hosts: (row.app_hosts || []).map(ah => ({
      host_id: ah.host_id, credential_id: ah.credential_id || null, deploy_dir: ah.deploy_dir, jar_name: ah.jar_name,
      stop_cmd: ah.stop_cmd, start_cmd: ah.start_cmd, backup_dir: ah.backup_dir, health_check_url: ah.health_check_url
    }))
  } : { name: '', description: '', app_hosts: [] }
  visible.value = true
}
const addFromGroup = gid => {
  if (!gid) return
  const bound = new Set(form.value.app_hosts.map(x => x.host_id))
  const inGroup = hosts.value.filter(h => String(h.group_id) === String(gid) && !bound.has(h.id))
  if (!inGroup.length) return
  // clone the config of the first row (or a fresh default) so identical content
  // doesn't have to be re-typed for every host
  const base = form.value.app_hosts[0] || { deploy_dir: '', jar_name: 'app.jar', stop_cmd: '', start_cmd: '', backup_dir: '', health_check_url: '' }
  for (const h of inGroup) {
    form.value.app_hosts.push({
      host_id: h.id, credential_id: null,
      deploy_dir: base.deploy_dir, jar_name: base.jar_name,
      stop_cmd: base.stop_cmd, start_cmd: base.start_cmd,
      backup_dir: base.backup_dir, health_check_url: base.health_check_url,
    })
  }
  bulkGroup.value = null
}
const save = async () => {
  if (!form.value.name) { ElMessage.warning(t('apps.needName')); return }
  if (form.value.app_hosts.some(ah => !ah.host_id || !ah.deploy_dir || !ah.jar_name)) {
    ElMessage.warning(t('apps.needRow'))
    return
  }
  if (form.value.id) await api.put(`/apps/${form.value.id}`, form.value)
  else await api.post('/apps', form.value)
  ElMessage.success(t('hosts.saved'))
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
