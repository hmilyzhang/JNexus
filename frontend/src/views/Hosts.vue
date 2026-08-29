<template>
  <div>
    <el-card>
      <div style="display:flex; gap:8px; margin-bottom:12px; flex-wrap:wrap">
        <el-input v-model="keyword" placeholder="搜索主机名/IP" style="width:200px" clearable @change="load" />
        <el-select v-model="groupFilter" placeholder="全部分组" style="width:160px" clearable @change="load">
          <el-option v-for="g in groups" :key="g.id" :label="g.name" :value="g.id" />
        </el-select>
        <el-button type="success" @click="probeAll" :loading="probing">探测连通性</el-button>
        <el-button type="primary" @click="dlgHost()">新增主机</el-button>
        <el-button @click="dlgImport">批量导入</el-button>
        <el-button @click="dlgGroup">分组管理</el-button>
        <el-button type="warning" plain @click="showKeys = true">SSH 密钥管理</el-button>
      </div>

      <el-table :data="hosts" v-loading="loading" size="small" border>
        <el-table-column prop="id" label="ID" width="60" />
        <el-table-column prop="name" label="名称" min-width="120" />
        <el-table-column prop="ip" label="IP" width="140" />
        <el-table-column prop="port" label="端口" width="70" />
        <el-table-column prop="username" label="用户" width="100" />
        <el-table-column label="认证" width="80">
          <template #default="{ row }">{{ row.auth_type === 'key' ? '密钥' : '密码' }}</template>
        </el-table-column>
        <el-table-column label="分组" width="120">
          <template #default="{ row }">{{ row.group?.name || '-' }}</template>
        </el-table-column>
        <el-table-column label="状态" width="90">
          <template #default="{ row }">
            <el-tag :type="row.status === 'online' ? 'success' : row.status === 'offline' ? 'danger' : 'info'" size="small">
              {{ row.status === 'online' ? '在线' : row.status === 'offline' ? '离线' : '未知' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="220" fixed="right">
          <template #default="{ row }">
            <el-button size="small" type="primary" link @click="$router.push(`/terminal/${row.id}`)">终端</el-button>
            <el-button size="small" link @click="dlgHost(row)">编辑</el-button>
            <el-popconfirm title="确认删除该主机?" @confirm="delHost(row)">
              <template #reference><el-button size="small" type="danger" link>删除</el-button></template>
            </el-popconfirm>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 新增/编辑主机 -->
    <el-dialog v-model="hostVisible" :title="hostForm.id ? '编辑主机' : '新增主机'" width="460px">
      <el-form label-width="90px">
        <el-form-item label="名称"><el-input v-model="hostForm.name" placeholder="默认取 IP" /></el-form-item>
        <el-form-item label="IP"><el-input v-model="hostForm.ip" /></el-form-item>
        <el-form-item label="端口"><el-input-number v-model="hostForm.port" :min="1" :max="65535" /></el-form-item>
        <el-form-item label="用户名"><el-input v-model="hostForm.username" /></el-form-item>
        <el-form-item label="认证方式">
          <el-radio-group v-model="hostForm.auth_type">
            <el-radio value="key">SSH 密钥</el-radio>
            <el-radio value="password">密码</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="密钥" v-if="hostForm.auth_type === 'key'">
          <el-select v-model="hostForm.ssh_key_id" placeholder="选择密钥" style="width:100%">
            <el-option v-for="k in keys" :key="k.id" :label="k.name" :value="k.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="密码" v-else>
          <el-input v-model="hostForm.password" type="password" show-password :placeholder="hostForm.id ? '留空则不修改' : ''" />
        </el-form-item>
        <el-form-item label="分组">
          <el-select v-model="hostForm.group_id" placeholder="未分组" style="width:100%" clearable>
            <el-option v-for="g in groups" :key="g.id" :label="g.name" :value="g.id" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="hostVisible = false">取消</el-button>
        <el-button type="primary" @click="saveHost">保存</el-button>
      </template>
    </el-dialog>

    <!-- 批量导入 -->
    <el-dialog v-model="importVisible" title="批量导入主机" width="520px">
      <el-form label-width="110px">
        <el-form-item label="默认用户名"><el-input v-model="importForm.username" placeholder="root" /></el-form-item>
        <el-form-item label="认证密钥">
          <el-select v-model="importForm.ssh_key_id" style="width:100%">
            <el-option v-for="k in keys" :key="k.id" :label="k.name" :value="k.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="主机列表">
          <el-input v-model="importForm.content" type="textarea" :rows="8"
            placeholder="每行一台主机，格式（逗号分隔）：&#10;IP,端口,用户名,分组名&#10;端口/用户名/分组可省略，分组不存在会自动创建&#10;10.0.0.1&#10;10.0.0.2,22,root,生产环境" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="importVisible = false">取消</el-button>
        <el-button type="primary" @click="doImport">导入</el-button>
      </template>
    </el-dialog>

    <!-- 分组管理 -->
    <el-dialog v-model="groupVisible" title="分组管理" width="520px">
      <div style="display:flex; gap:8px; margin-bottom:12px">
        <el-input v-model="newGroup" placeholder="新分组名称" style="width:200px" />
        <el-button type="primary" @click="addGroup">添加分组</el-button>
      </div>
      <el-table :data="groups" size="small" border>
        <el-table-column prop="name" label="分组名" />
        <el-table-column prop="host_count" label="主机数" width="80" />
        <el-table-column label="操作" width="140">
          <template #default="{ row }">
            <el-popconfirm title="确认删除分组?" @confirm="delGroup(row)">
              <template #reference><el-button size="small" type="danger" link>删除</el-button></template>
            </el-popconfirm>
          </template>
        </el-table-column>
      </el-table>
    </el-dialog>

    <!-- SSH 密钥管理 -->
    <el-drawer v-model="showKeys" title="SSH 密钥管理" size="480px">
      <div style="margin-bottom:12px">
        <el-button type="primary" size="small" @click="keyDlgVisible = true">导入密钥</el-button>
      </div>
      <el-table :data="keys" size="small" border>
        <el-table-column prop="name" label="名称" />
        <el-table-column prop="public_key" label="公钥" show-overflow-tooltip />
        <el-table-column label="操作" width="80">
          <template #default="{ row }">
            <el-popconfirm title="确认删除密钥?" @confirm="delKey(row)">
              <template #reference><el-button size="small" type="danger" link>删除</el-button></template>
            </el-popconfirm>
          </template>
        </el-table-column>
      </el-table>
      <div style="margin-top:12px; color:#909399; font-size:12px">
        提示：将平台公钥（导入私钥后可由对应公钥）写入目标机 ~/.ssh/authorized_keys 即可免密登录。
      </div>
    </el-drawer>

    <el-dialog v-model="keyDlgVisible" title="导入 SSH 密钥" width="520px">
      <el-form label-width="80px">
        <el-form-item label="名称"><el-input v-model="keyForm.name" /></el-form-item>
        <el-form-item label="公钥"><el-input v-model="keyForm.public_key" type="textarea" :rows="2" placeholder="ssh-rsa AAAA...（可选，用于展示）" /></el-form-item>
        <el-form-item label="私钥"><el-input v-model="keyForm.private_key" type="textarea" :rows="6" placeholder="-----BEGIN OPENSSH PRIVATE KEY-----" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="keyDlgVisible = false">取消</el-button>
        <el-button type="primary" @click="saveKey">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import api from '../api'
import { ElMessage } from 'element-plus'

const hosts = ref([])
const groups = ref([])
const keys = ref([])
const keyword = ref('')
const groupFilter = ref('')
const loading = ref(false)
const probing = ref(false)

const hostVisible = ref(false)
const hostForm = ref({})
const importVisible = ref(false)
const importForm = ref({ content: '', ssh_key_id: null, username: 'root' })
const groupVisible = ref(false)
const newGroup = ref('')
const showKeys = ref(false)
const keyDlgVisible = ref(false)
const keyForm = ref({ name: '', public_key: '', private_key: '' })

const load = async () => {
  loading.value = true
  try {
    const params = {}
    if (keyword.value) params.keyword = keyword.value
    if (groupFilter.value) params.group_id = groupFilter.value
    hosts.value = await api.get('/hosts', { params })
  } finally { loading.value = false }
  groups.value = await api.get('/host_groups')
}
const loadKeys = async () => { keys.value = await api.get('/ssh_keys') }

onMounted(() => { load(); loadKeys() })

const dlgHost = row => {
  hostForm.value = row ? { ...row, password: '' } : { name: '', ip: '', port: 22, username: 'root', auth_type: 'key', ssh_key_id: keys.value[0]?.id, group_id: null }
  hostVisible.value = true
}
const saveHost = async () => {
  if (!hostForm.value.ip || !hostForm.value.username) { ElMessage.warning('IP 与用户名必填'); return }
  if (hostForm.value.auth_type === 'key' && !hostForm.value.ssh_key_id) { ElMessage.warning('请选择密钥'); return }
  if (hostForm.value.id) {
    await api.put(`/hosts/${hostForm.value.id}`, hostForm.value)
  } else {
    await api.post('/hosts', hostForm.value)
  }
  ElMessage.success('已保存')
  hostVisible.value = false
  load()
}
const delHost = async row => { await api.delete(`/hosts/${row.id}`); ElMessage.success('已删除'); load() }

const probeAll = async () => {
  probing.value = true
  try {
    await api.post('/probe', { host_ids: hosts.value.map(h => h.id) })
    await load()
    ElMessage.success('探测完成')
  } finally { probing.value = false }
}

const dlgImport = () => { importVisible.value = true }
const doImport = async () => {
  const res = await api.post('/hosts/import', importForm.value)
  ElMessage.success(`新增 ${res.created} 台，跳过 ${res.skipped} 台${res.errors?.length ? '，' + res.errors.length + ' 台失败' : ''}`)
  importVisible.value = false
  load()
}

const dlgGroup = () => { groupVisible.value = true; load() }
const addGroup = async () => {
  if (!newGroup.value) return
  await api.post('/host_groups', { name: newGroup.value })
  newGroup.value = ''
  load()
}
const delGroup = async row => {
  await api.delete(`/host_groups/${row.id}`)
  ElMessage.success('已删除')
  load()
}

const saveKey = async () => {
  if (!keyForm.value.name || !keyForm.value.private_key) { ElMessage.warning('名称与私钥必填'); return }
  await api.post('/ssh_keys', keyForm.value)
  ElMessage.success('密钥已保存')
  keyDlgVisible.value = false
  keyForm.value = { name: '', public_key: '', private_key: '' }
  loadKeys()
}
const delKey = async row => {
  await api.delete(`/ssh_keys/${row.id}`)
  ElMessage.success('已删除')
  loadKeys()
}
</script>
