<!-- AutoOps 运维平台 — By JJ Zhang, Version 1.0 -->
<template>
  <el-card>
    <el-tabs v-model="activeTab">
      <!-- Tab 1: 用户管理 -->
      <el-tab-pane :label="$t('menu.users')" name="users">
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
          <el-table-column :label="$t('users.userGroups')" min-width="140">
            <template #default="{ row }">
              <el-tag v-for="g in row.member_of || []" :key="g" size="small" type="warning" style="margin-right:4px">{{ groupName(g) }}</el-tag>
              <span v-if="!(row.member_of || []).length" style="color:#c0c4cc">-</span>
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
      </el-tab-pane>

      <!-- Tab 2: 组管理 -->
      <el-tab-pane :label="$t('users.groupTab')" name="groups">
        <div style="margin-bottom:12px; display:flex; gap:8px">
          <el-button type="primary" @click="gDlg()">{{ $t('users.groupCreate') }}</el-button>
          <el-button @click="load">{{ $t('common.refresh') }}</el-button>
        </div>
        <el-table :data="ugroups" v-loading="loading" size="small" border>
          <el-table-column prop="id" label="ID" width="70" />
          <el-table-column prop="name" :label="$t('users.groupName')" width="160" />
          <el-table-column prop="description" :label="$t('scripts.desc')" min-width="160" show-overflow-tooltip />
          <el-table-column :label="$t('users.members')" width="90" prop="members" />
          <el-table-column :label="$t('users.linkedHosts')" width="90" prop="hosts" />
          <el-table-column :label="$t('users.linkedGroups')" width="100" prop="host_groups" />
          <el-table-column prop="created_at" :label="$t('releases.time')" width="170" />
          <el-table-column :label="$t('common.operation')" width="150" fixed="right">
            <template #default="{ row }">
              <el-button size="small" type="primary" link @click="gDlg(row)">{{ $t('common.edit') }}</el-button>
              <el-popconfirm :title="$t('users.groupDelConfirm')" @confirm="gDel(row)">
                <template #reference><el-button size="small" type="danger" link>{{ $t('common.delete') }}</el-button></template>
              </el-popconfirm>
            </template>
          </el-table-column>
        </el-table>
        <div style="margin-top:12px; color:#909399; font-size:12px">{{ $t('users.groupTip') }}</div>
      </el-tab-pane>
    </el-tabs>
  </el-card>

  <!-- 用户 编辑/新增 -->
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
      <el-form-item :label="$t('users.userGroups')">
        <el-select v-model="form.user_group_ids" multiple style="width:100%" :placeholder="$t('users.groupTab')">
          <el-option v-for="g in ugroups" :key="g.id" :label="g.name" :value="g.id" />
        </el-select>
      </el-form-item>
      <el-form-item :label="$t('users.statusCol')">
        <el-switch v-model="form.status" :active-value="1" :inactive-value="0" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="visible = false">{{ $t('common.cancel') }}</el-button>
      <el-button type="primary" @click="save">{{ $t('common.save') }}</el-button>
    </template>
  </el-dialog>

  <!-- 用户组 编辑/新增 -->
  <el-dialog v-model="gVisible" :title="gform.id ? $t('users.groupEdit') : $t('users.groupCreate')" width="620px">
    <el-form label-width="130px">
      <el-form-item :label="$t('users.groupName')"><el-input v-model="gform.name" /></el-form-item>
      <el-form-item :label="$t('scripts.desc')"><el-input v-model="gform.description" /></el-form-item>
      <el-form-item :label="$t('users.members')">
        <el-select v-model="gform.member_ids" multiple filterable style="width:100%">
          <el-option v-for="u in users" :key="u.id" :label="u.username" :value="u.id" />
        </el-select>
      </el-form-item>
      <el-form-item :label="$t('users.linkedGroups')">
        <el-select v-model="gform.host_group_ids" multiple style="width:100%">
          <el-option v-for="g in hostGroups" :key="g.id" :label="`${g.name}（${g.host_count || 0}）`" :value="g.id" />
        </el-select>
      </el-form-item>
      <el-form-item :label="$t('users.linkedHosts')">
        <el-select v-model="gform.host_ids" multiple filterable style="width:100%">
          <el-option v-for="h in hosts" :key="h.id" :label="`${h.name} · ${h.ip}`" :value="h.id" />
        </el-select>
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="gVisible = false">{{ $t('common.cancel') }}</el-button>
      <el-button type="primary" @click="gSave">{{ $t('common.save') }}</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import api from '../api'
import i18n from '../i18n'
import { ElMessage } from 'element-plus'

const { t } = i18n.global
const activeTab = ref('users')
const users = ref([])
const ugroups = ref([])
const hostGroups = ref([])
const hosts = ref([])
const options = ref({ host_groups: [], apps: [] })
const loading = ref(false)
const visible = ref(false)
const form = ref({})
const grantVisible = ref(false)
const grantUser = ref(null)
const grantForm = ref({ host_groups: [], apps: [] })
const gVisible = ref(false)
const gform = ref({ name: '', description: '', member_ids: [], host_ids: [], host_group_ids: [] })

const roleLabel = r => ({
  admin: t('layout.roleAdmin'), ops: t('layout.roleOps'),
  publisher: t('layout.rolePublisher'), viewer: t('layout.roleViewer')
}[r] || r)
const groupName = id => (ugroups.value.find(g => g.id === id) || {}).name || id

const load = async () => {
  loading.value = true
  try {
    users.value = await api.get('/users')
    ugroups.value = await api.get('/user_groups')
    hosts.value = await api.get('/hosts')
    hostGroups.value = await api.get('/host_groups')
    options.value = await api.get('/grants/options')
  } finally { loading.value = false }
}
onMounted(load)

const dlg = row => {
  form.value = row
    ? { ...row, user_group_ids: [...(row.member_of || [])] }
    : { username: '', password: '', role: 'viewer', status: 1, user_group_ids: [] }
  visible.value = true
}
const save = async () => {
  if (form.value.id) {
    await api.put(`/users/${form.value.id}`, { role: form.value.role, status: form.value.status, user_group_ids: form.value.user_group_ids })
  } else {
    await api.post('/users', { ...form.value })
  }
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

// ---- 用户组 ----
const gDlg = async row => {
  if (row) {
    const d = await api.get(`/user_groups/${row.id}`)
    gform.value = {
      id: d.id, name: d.name, description: d.description,
      member_ids: d.member_ids || [], host_ids: d.host_ids || [], host_group_ids: d.host_group_ids || []
    }
  } else {
    gform.value = { name: '', description: '', member_ids: [], host_ids: [], host_group_ids: [] }
  }
  gVisible.value = true
}
const gSave = async () => {
  if (!gform.value.name) { ElMessage.warning(t('users.groupName') + ' required'); return }
  if (gform.value.id) {
    await api.put(`/user_groups/${gform.value.id}`, { name: gform.value.name, description: gform.value.description })
    await api.put(`/user_groups/${gform.value.id}/links`, {
      member_ids: gform.value.member_ids, host_ids: gform.value.host_ids, host_group_ids: gform.value.host_group_ids
    })
  } else {
    const created = await api.post('/user_groups', { name: gform.value.name, description: gform.value.description })
    await api.put(`/user_groups/${created.id}/links`, {
      member_ids: gform.value.member_ids, host_ids: gform.value.host_ids, host_group_ids: gform.value.host_group_ids
    })
  }
  ElMessage.success(t('hosts.saved'))
  gVisible.value = false
  load()
}
const gDel = async row => { await api.delete(`/user_groups/${row.id}`); load() }
</script>
