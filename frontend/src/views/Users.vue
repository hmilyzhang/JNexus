<!-- AutoOps 运维平台 — By JJ Zhang, Version 1.0 -->
<template>
  <div>
    <el-card>
      <div style="margin-bottom:12px; display:flex; gap:8px">
        <el-button type="primary" @click="dlg()">{{ $t('users.create') }}</el-button>
        <el-button @click="load">{{ $t('common.refresh') }}</el-button>
      </div>
      <el-table :data="users" v-loading="loading" size="small" border>
        <el-table-column prop="id" label="ID" width="70" />
        <el-table-column prop="username" :label="$t('users.username')" width="140" />
        <el-table-column :label="$t('users.role')" width="110">
          <template #default="{ row }">
            <el-tag size="small">{{ roleLabel(row.role) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="$t('users.authSource')" width="100">
          <template #default="{ row }">
            <el-tag size="small" :type="row.auth_source === 'ldap' ? 'warning' : 'info'">
              {{ row.auth_source === 'ldap' ? $t('users.authLdap') : $t('users.authLocal') }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="$t('users.statusCol')" width="90">
          <template #default="{ row }">
            <el-tag size="small" :type="row.status === 1 ? 'success' : 'danger'">{{ row.status === 1 ? $t('common.enabled') : $t('common.disabled') }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="last_login_at" :label="$t('users.lastLogin')" width="170" />
        <el-table-column :label="$t('common.operation')" width="180" fixed="right">
          <template #default="{ row }">
            <el-button size="small" link @click="grantDlg(row)">{{ $t('users.grant') }}</el-button>
            <el-button size="small" link @click="dlg(row)">{{ $t('common.edit') }}</el-button>
            <el-popconfirm :title="$t('users.delConfirm')" @confirm="del(row)">
              <template #reference><el-button size="small" type="danger" link>{{ $t('common.delete') }}</el-button></template>
            </el-popconfirm>
          </template>
        </el-table-column>
      </el-table>
      <div style="margin-top:12px; color:#909399; font-size:12px; white-space:pre-line">{{ $t('users.roleDesc') }}</div>
    </el-card>

    <el-dialog v-model="visible" :title="form.id ? $t('common.edit') : $t('users.create')" width="440px">
      <el-form label-width="110px">
        <el-form-item :label="$t('users.username')"><el-input v-model="form.username" :disabled="!!form.id" /></el-form-item>
        <el-form-item :label="$t('users.password')" v-if="!form.id"><el-input v-model="form.password" type="password" show-password /></el-form-item>
        <el-form-item :label="$t('users.role')">
          <el-select v-model="form.role" style="width:100%">
            <el-option :label="$t('layout.roleAdmin')" value="admin" />
            <el-option :label="$t('layout.roleOps')" value="ops" />
            <el-option :label="$t('layout.rolePublisher')" value="publisher" />
            <el-option :label="$t('layout.roleViewer')" value="viewer" />
          </el-select>
        </el-form-item>
        <el-form-item :label="$t('users.statusCol')">
          <el-switch v-model="form.status" :active-value="1" :inactive-value="0" active-text="" inactive-text="" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="visible = false">{{ $t('common.cancel') }}</el-button>
        <el-button type="primary" @click="save">{{ $t('common.save') }}</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="grantVisible" :title="`${$t('users.grant')}：${grantUser?.username}`" width="560px">
      <div style="font-weight:600; margin-bottom:8px">{{ $t('users.execGroups') }}</div>
      <el-table :data="grantForm.host_groups" size="small" border>
        <el-table-column prop="name" :label="$t('hosts.groupName')" />
        <el-table-column :label="$t('users.canExec')" width="90">
          <template #default="{ row }"><el-checkbox v-model="row.can_exec" /></template>
        </el-table-column>
        <el-table-column :label="$t('users.canDeploy')" width="90">
          <template #default="{ row }"><el-checkbox v-model="row.can_deploy" /></template>
        </el-table-column>
      </el-table>
      <div style="font-weight:600; margin:14px 0 8px">{{ $t('users.deployableApps') }}</div>
      <el-checkbox-group v-model="grantForm.apps">
        <el-checkbox v-for="a in options.apps" :key="a.id" :value="a.id">{{ a.name }}</el-checkbox>
      </el-checkbox-group>
      <template #footer>
        <el-button @click="grantVisible = false">{{ $t('common.cancel') }}</el-button>
        <el-button type="primary" @click="saveGrants">{{ $t('common.save') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import api from '../api'
import i18n from '../i18n'
import { ElMessage } from 'element-plus'

const { t } = i18n.global
const users = ref([])
const options = ref({ host_groups: [], apps: [] })
const loading = ref(false)
const visible = ref(false)
const form = ref({})
const grantVisible = ref(false)
const grantUser = ref(null)
const grantForm = ref({ host_groups: [], apps: [] })

const roleLabel = r => ({
  admin: t('layout.roleAdmin'), ops: t('layout.roleOps'),
  publisher: t('layout.rolePublisher'), viewer: t('layout.roleViewer')
}[r] || r)

const load = async () => {
  loading.value = true
  try { users.value = await api.get('/users') } finally { loading.value = false }
  options.value = await api.get('/grants/options')
}
onMounted(load)

const dlg = row => {
  form.value = row ? { ...row } : { username: '', password: '', role: 'viewer', status: 1 }
  visible.value = true
}
const save = async () => {
  if (form.value.id) await api.put(`/users/${form.value.id}`, { role: form.value.role, status: form.value.status })
  else await api.post('/users', form.value)
  ElMessage.success(t('hosts.saved'))
  visible.value = false
  load()
}
const del = async row => { await api.delete(`/users/${row.id}`); load() }

const grantDlg = async row => {
  grantUser.value = row
  const grants = await api.get(`/users/${row.id}/grants`)
  grantForm.value = {
    host_groups: options.value.host_groups.map(g => {
      const ex = (grants.host_groups || []).find(x => x.group_id === g.id)
      return { group_id: g.id, name: g.name, can_exec: ex?.can_exec || false, can_deploy: ex?.can_deploy || false }
    }),
    apps: (grants.apps || []).map(x => x.app_id)
  }
  grantVisible.value = true
}
const saveGrants = async () => {
  await api.put(`/users/${grantUser.value.id}/grants`, {
    host_groups: grantForm.value.host_groups.map(g => ({ group_id: g.group_id, can_exec: g.can_exec, can_deploy: g.can_deploy })),
    apps: grantForm.value.apps
  })
  ElMessage.success(t('users.grantSaved'))
  grantVisible.value = false
}
</script>
