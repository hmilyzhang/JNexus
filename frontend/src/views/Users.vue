<template>
  <div>
    <el-card>
      <div style="margin-bottom:12px; display:flex; gap:8px">
        <el-button type="primary" @click="dlg()">新增用户</el-button>
        <el-button @click="load">刷新</el-button>
      </div>
      <el-table :data="users" v-loading="loading" size="small" border>
        <el-table-column prop="id" label="ID" width="70" />
        <el-table-column prop="username" label="用户名" width="140" />
        <el-table-column label="角色" width="120">
          <template #default="{ row }">
            <el-tag size="small">{{ roleLabel(row.role) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="90">
          <template #default="{ row }">
            <el-tag size="small" :type="row.status === 1 ? 'success' : 'danger'">{{ row.status === 1 ? '启用' : '禁用' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="last_login_at" label="最后登录" width="170" />
        <el-table-column label="操作" width="180" fixed="right">
          <template #default="{ row }">
            <el-button size="small" link @click="grantDlg(row)">授权</el-button>
            <el-button size="small" link @click="dlg(row)">编辑</el-button>
            <el-popconfirm title="确认删除用户?" @confirm="del(row)">
              <template #reference><el-button size="small" type="danger" link>删除</el-button></template>
            </el-popconfirm>
          </template>
        </el-table-column>
      </el-table>
      <div style="margin-top:12px; color:#909399; font-size:12px">
        角色说明：管理员=全部权限；运维=主机/执行/文件/脚本/发布；发布员=执行/发布（需授权）；只读=仅查看。
        「授权」控制用户可执行的主机分组与可发布的应用。
      </div>
    </el-card>

    <el-dialog v-model="visible" :title="form.id ? '编辑用户' : '新增用户'" width="440px">
      <el-form label-width="90px">
        <el-form-item label="用户名"><el-input v-model="form.username" :disabled="!!form.id" /></el-form-item>
        <el-form-item label="密码" v-if="!form.id"><el-input v-model="form.password" type="password" show-password /></el-form-item>
        <el-form-item label="角色">
          <el-select v-model="form.role" style="width:100%">
            <el-option label="管理员" value="admin" />
            <el-option label="运维" value="ops" />
            <el-option label="发布员" value="publisher" />
            <el-option label="只读" value="viewer" />
          </el-select>
        </el-form-item>
        <el-form-item label="状态">
          <el-switch v-model="form.status" :active-value="1" :inactive-value="0" active-text="启用" inactive-text="禁用" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="visible = false">取消</el-button>
        <el-button type="primary" @click="save">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="grantVisible" :title="`授权：${grantUser?.username}`" width="560px">
      <div style="font-weight:600; margin-bottom:8px">可执行的主机分组</div>
      <el-table :data="grantForm.host_groups" size="small" border>
        <el-table-column prop="name" label="分组" />
        <el-table-column label="可执行" width="90">
          <template #default="{ row }"><el-checkbox v-model="row.can_exec" /></template>
        </el-table-column>
        <el-table-column label="可部署" width="90">
          <template #default="{ row }"><el-checkbox v-model="row.can_deploy" /></template>
        </el-table-column>
      </el-table>
      <div style="font-weight:600; margin:14px 0 8px">可发布的应用</div>
      <el-checkbox-group v-model="grantForm.apps">
        <el-checkbox v-for="a in options.apps" :key="a.id" :value="a.id">{{ a.name }}</el-checkbox>
      </el-checkbox-group>
      <template #footer>
        <el-button @click="grantVisible = false">取消</el-button>
        <el-button type="primary" @click="saveGrants">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import api from '../api'
import { ElMessage } from 'element-plus'

const users = ref([])
const options = ref({ host_groups: [], apps: [] })
const loading = ref(false)
const visible = ref(false)
const form = ref({})
const grantVisible = ref(false)
const grantUser = ref(null)
const grantForm = ref({ host_groups: [], apps: [] })

const roleLabel = r => ({ admin: '管理员', ops: '运维', publisher: '发布员', viewer: '只读' }[r] || r)

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
  if (form.value.id) {
    await api.put(`/users/${form.value.id}`, { role: form.value.role, status: form.value.status })
  } else {
    await api.post('/users', form.value)
  }
  ElMessage.success('已保存')
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
  ElMessage.success('授权已保存')
  grantVisible.value = false
}
</script>
