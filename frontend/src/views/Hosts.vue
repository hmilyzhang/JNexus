<!-- JNexus 运维平台 — By JJ Zhang, Version 1.0 -->
<template>
  <el-row :gutter="16">
    <el-col :span="6">
      <el-card :header="$t('hosts.treeView')" v-loading="loading">
        <el-tree ref="treeRef" :data="treeData" node-key="key" highlight-current default-expand-all
                 @node-click="onTreeNode">
          <template #default="{ data }">
            <span class="tree-node">
              <el-icon v-if="data.type === 'group'"><Folder /></el-icon>
              <el-icon v-else :color="data.host.status === 'online' ? '#67c23a' : '#c0c4cc'"><Monitor /></el-icon>
              <span>{{ data.label }}</span>
              <el-tag v-if="data.type === 'group'" size="small" type="info">{{ data.children.length }}</el-tag>
            </span>
          </template>
        </el-tree>
      </el-card>
    </el-col>

    <el-col :span="18">
      <el-card>
        <div style="display:flex; gap:8px; margin-bottom:12px; flex-wrap:wrap">
          <el-input v-model="keyword" :placeholder="$t('hosts.searchPlaceholder')" style="width:200px" clearable @change="load" />
          <el-select v-model="groupFilter" :placeholder="$t('hosts.allGroups')" style="width:160px" clearable @change="load">
            <el-option v-for="g in groups" :key="g.id" :label="g.name" :value="g.id" />
          </el-select>
          <el-button type="success" @click="probeAll" :loading="probing">{{ $t('hosts.probe') }}</el-button>
          <el-button type="primary" @click="dlgHost()">{{ $t('hosts.addHost') }}</el-button>
          <el-button @click="dlgImport">{{ $t('hosts.import') }}</el-button>
          <el-button type="info" plain @click="$router.push('/shell')">{{ $t('hosts.terminal') }}</el-button>
          <el-button @click="dlgGroup">{{ $t('hosts.groupMgmt') }}</el-button>
          <el-button type="warning" plain @click="showKeys = true">{{ $t('hosts.keyMgmt') }}</el-button>
        </div>

        <el-table :data="hosts" v-loading="loading" size="small" border>
          <el-table-column prop="id" label="ID" width="60" />
          <el-table-column prop="name" :label="$t('hosts.name')" min-width="120" />
          <el-table-column prop="ip" :label="$t('hosts.ip')" width="140" />
          <el-table-column prop="port" :label="$t('hosts.port')" width="70" />
          <el-table-column prop="username" :label="$t('hosts.user')" width="100" />
          <el-table-column :label="$t('hosts.auth')" width="80">
            <template #default="{ row }">{{ row.auth_type === 'key' ? $t('hosts.authKey') : $t('hosts.authPassword') }}</template>
          </el-table-column>
          <el-table-column :label="$t('hosts.group')" width="120">
            <template #default="{ row }">{{ row.group?.name || '-' }}</template>
          </el-table-column>
          <el-table-column :label="$t('common.status')" width="90">
            <template #default="{ row }">
              <el-tag :type="row.status === 'online' ? 'success' : row.status === 'offline' ? 'danger' : 'info'" size="small">
                {{ row.status === 'online' ? $t('common.online') : row.status === 'offline' ? $t('common.offline') : $t('common.unknown') }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column :label="$t('common.operation')" width="110" fixed="right">
            <template #default="{ row }">
              <el-dropdown trigger="click" @command="cmd => onRowCmd(cmd, row)">
                <el-button size="small" type="primary" plain>
                  {{ $t('common.operation') }}<el-icon style="margin-left:4px"><ArrowDown /></el-icon>
                </el-button>
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item command="terminal">{{ $t('hosts.terminal') }}</el-dropdown-item>
                    <el-dropdown-item v-if="canManageCreds" command="cred">{{ $t('hosts.credMgmt') }}</el-dropdown-item>
                    <el-dropdown-item command="edit" divided>{{ $t('common.edit') }}</el-dropdown-item>
                    <el-dropdown-item command="delete" style="color:#f56c6c">{{ $t('common.delete') }}</el-dropdown-item>
                  </el-dropdown-menu>
                </template>
              </el-dropdown>
            </template>
          </el-table-column>
        </el-table>
      </el-card>
    </el-col>
  </el-row>

  <!-- 新增/编辑主机 -->
  <el-dialog v-model="hostVisible" :title="hostForm.id ? $t('hosts.editHost') : $t('hosts.addHostTitle')" width="460px">
    <el-form label-width="100px">
      <el-form-item :label="$t('hosts.name')"><el-input v-model="hostForm.name" :placeholder="$t('hosts.namePlaceholder')" /></el-form-item>
      <el-form-item :label="$t('hosts.ip')"><el-input v-model="hostForm.ip" /></el-form-item>
      <el-form-item :label="$t('hosts.port')"><el-input-number v-model="hostForm.port" :min="1" :max="65535" /></el-form-item>
      <el-form-item :label="$t('hosts.user')"><el-input v-model="hostForm.username" /></el-form-item>
      <el-form-item :label="$t('hosts.authType')">
        <el-radio-group v-model="hostForm.auth_type">
          <el-radio value="key">{{ $t('hosts.key') }}</el-radio>
          <el-radio value="password">{{ $t('hosts.password') }}</el-radio>
        </el-radio-group>
      </el-form-item>
      <el-form-item :label="$t('hosts.key')" v-if="hostForm.auth_type === 'key'">
        <el-select v-model="hostForm.ssh_key_id" :placeholder="$t('hosts.keyPlaceholder')" style="width:100%">
          <el-option v-for="k in keys" :key="k.id" :label="k.name" :value="k.id" />
        </el-select>
      </el-form-item>
      <el-form-item :label="$t('hosts.password')" v-else>
        <el-input v-model="hostForm.password" type="password" show-password :placeholder="hostForm.id ? $t('hosts.passwordKeep') : ''" />
        <el-checkbox v-if="!hostForm.id" v-model="hostForm.auto_pair" style="margin-top:4px">
          {{ $t('hosts.autoPair') }}
        </el-checkbox>
        <div v-if="!hostForm.id && hostForm.auto_pair" style="color:#909399; font-size:12px; line-height:1.5">
          {{ $t('hosts.autoPairTip') }}
        </div>
      </el-form-item>
      <el-form-item :label="$t('hosts.group')">
        <el-select v-model="hostForm.group_id" :placeholder="$t('hosts.groupPlaceholder')" style="width:100%" clearable>
          <el-option v-for="g in groups" :key="g.id" :label="g.name" :value="g.id" />
        </el-select>
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="hostVisible = false">{{ $t('common.cancel') }}</el-button>
      <el-button type="primary" @click="saveHost">{{ $t('common.save') }}</el-button>
    </template>
  </el-dialog>

  <!-- 批量导入 -->
  <el-dialog v-model="importVisible" :title="$t('hosts.importTitle')" width="560px">
    <el-form label-width="130px">
      <el-form-item :label="$t('hosts.importFile')">
        <div>
          <input type="file" accept=".csv,.txt" @change="onImportFile" />
          <div style="color:#909399; font-size:12px; margin-top:2px">{{ $t('hosts.importFileTip') }}</div>
        </div>
      </el-form-item>
      <el-form-item :label="$t('hosts.commonPassword')">
        <el-input v-model="importForm.password" type="password" show-password autocomplete="new-password"
                  :placeholder="$t('hosts.commonPasswordPlaceholder')" />
      </el-form-item>
      <el-form-item :label="$t('hosts.credLabel')">
        <el-input v-model="importForm.credential_label" :placeholder="$t('hosts.credLabelPlaceholder')" />
      </el-form-item>
      <el-form-item :label="$t('hosts.defaultUser')"><el-input v-model="importForm.username" placeholder="root" /></el-form-item>
      <el-form-item :label="$t('hosts.importKey')" v-if="!importForm.auto_pair">
        <el-select v-model="importForm.ssh_key_id" style="width:100%">
          <el-option v-for="k in keys" :key="k.id" :label="k.name" :value="k.id" />
        </el-select>
      </el-form-item>
      <el-form-item :label="$t('hosts.autoPair')">
        <el-switch v-model="importForm.auto_pair" />
        <div style="color:#909399; font-size:12px; line-height:1.5; margin-top:4px">{{ $t('hosts.autoPairTip') }}</div>
      </el-form-item>
      <el-form-item :label="$t('hosts.hostList')">
        <el-input v-model="importForm.content" type="textarea" :rows="8" class="mono"
                  :placeholder="$t('hosts.hostListPlaceholder')" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="importVisible = false">{{ $t('common.cancel') }}</el-button>
      <el-button type="primary" :disabled="!importForm.content.trim()" :loading="importing" @click="doImport">{{ $t('hosts.import') }}</el-button>
    </template>
  </el-dialog>

  <!-- 分组管理 -->
  <el-dialog v-model="groupVisible" :title="$t('hosts.groupMgmt')" width="520px">
    <div style="display:flex; gap:8px; margin-bottom:12px; align-items:center; flex-wrap:wrap">
      <el-select v-model="newGroupParent" :placeholder="$t('hosts.parentGroup')" style="width:180px" clearable>
        <el-option v-for="g in groups" :key="g.id" :label="g.name" :value="g.id" />
      </el-select>
      <el-input v-model="newGroup" :placeholder="$t('hosts.groupName')" style="width:200px" />
      <el-button type="primary" @click="addGroup">{{ $t('hosts.addGroup') }}</el-button>
    </div>
    <el-table :data="groups" size="small" border row-key="id" default-expand-all>
      <el-table-column prop="name" :label="$t('hosts.groupName')" />
      <el-table-column prop="host_count" :label="$t('hosts.hostCountCol')" width="90" />
      <el-table-column :label="$t('common.operation')" width="150">
        <template #default="{ row }">
          <el-button size="small" link @click="renameGroupDlg(row)">{{ $t('common.edit') }}</el-button>
          <el-popconfirm :title="$t('hosts.delGroupConfirm')" @confirm="delGroup(row)">
            <template #reference><el-button size="small" type="danger" link>{{ $t('common.delete') }}</el-button></template>
          </el-popconfirm>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="renameVisible" :title="$t('common.edit')" width="380px" append-to-body>
      <el-form label-width="100px">
        <el-form-item :label="$t('hosts.groupName')"><el-input v-model="renameForm.name" /></el-form-item>
        <el-form-item :label="$t('hosts.parentGroup')">
          <el-select v-model="renameForm.parent_id" style="width:100%" clearable>
            <el-option v-for="g in groups.filter(x => x.id !== renameForm.id)" :key="g.id" :label="g.name" :value="g.id" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="renameVisible = false">{{ $t('common.cancel') }}</el-button>
        <el-button type="primary" @click="saveRename">{{ $t('common.save') }}</el-button>
      </template>
    </el-dialog>
  </el-dialog>

  <!-- SSH 密钥管理 -->
  <el-drawer v-model="showKeys" :title="$t('hosts.keyMgmt')" size="480px">
    <div style="margin-bottom:12px">
      <el-button type="primary" size="small" @click="keyDlgVisible = true">{{ $t('hosts.importKeyTitle') }}</el-button>
    </div>
    <el-table :data="keys" size="small" border>
      <el-table-column prop="name" :label="$t('hosts.name')" />
      <el-table-column prop="public_key" :label="$t('hosts.publicKey')" show-overflow-tooltip />
      <el-table-column :label="$t('common.operation')" width="80">
        <template #default="{ row }">
          <el-popconfirm :title="$t('hosts.delKeyConfirm')" @confirm="delKey(row)">
            <template #reference><el-button size="small" type="danger" link>{{ $t('common.delete') }}</el-button></template>
          </el-popconfirm>
        </template>
      </el-table-column>
    </el-table>
    <div style="margin-top:12px; color:#909399; font-size:12px">{{ $t('hosts.keyTip') }}</div>
  </el-drawer>

  <el-dialog v-model="keyDlgVisible" :title="$t('hosts.importKeyTitle')" width="520px">
    <el-form label-width="90px">
      <el-form-item :label="$t('hosts.name')"><el-input v-model="keyForm.name" /></el-form-item>
      <el-form-item :label="$t('hosts.publicKey')"><el-input v-model="keyForm.public_key" type="textarea" :rows="2" :placeholder="$t('hosts.publicKeyPlaceholder')" /></el-form-item>
      <el-form-item :label="$t('hosts.privateKey')"><el-input v-model="keyForm.private_key" type="textarea" :rows="6" :placeholder="$t('hosts.privateKeyPlaceholder')" /></el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="keyDlgVisible = false">{{ $t('common.cancel') }}</el-button>
      <el-button type="primary" @click="saveKey">{{ $t('common.save') }}</el-button>
    </template>
  </el-dialog>

</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import api from '../api'
import i18n from '../i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useUserStore } from '../store'

const { t } = i18n.global
const router = useRouter()
const store = useUserStore()
const hosts = ref([])
const allHosts = ref([])
const groups = ref([])
const keys = ref([])
const keyword = ref('')
const groupFilter = ref('')
const loading = ref(false)
const probing = ref(false)
const importing = ref(false)

const hostVisible = ref(false)
const hostForm = ref({})
const importVisible = ref(false)
const importForm = ref({ content: '', ssh_key_id: null, username: 'root', password: '', credential_label: '', auto_pair: true })
const groupVisible = ref(false)
const newGroup = ref('')
const showKeys = ref(false)
const keyDlgVisible = ref(false)
const keyForm = ref({ name: '', public_key: '', private_key: '' })

// 树状数据：分组 → 主机，未分组单独一层
// 多级分组树：按 parent_id 递归构建
const buildGroupTree = (groups, hosts) => {
  const byId = new Map(groups.map(g => [g.id, {
    key: 'g-' + g.id, type: 'group', groupId: g.id, label: g.name,
    children: groups.filter(c => c.parent_id === g.id)
      .map(c => buildGroupNode(c, groups, hosts))
  }]))
  const roots = groups.filter(g => !g.parent_id).map(g => byId.get(g.id))
  for (const g of groups) {
    if (g.parent_id) {
      const p = byId.get(g.parent_id)
      if (p) {
        const node = byId.get(g.id)
        node.children = node.children.concat(hosts.filter(h => h.group_id === g.id)
          .map(h => ({ key: 'h-' + h.id, type: 'host', label: `${h.name} · ${h.ip}`, host: h, children: [] })))
        continue
      }
    }
    if (!g.parent_id) {
      byId.get(g.id).children = byId.get(g.id).children.concat(hosts.filter(h => h.group_id === g.id)
        .map(h => ({ key: 'h-' + h.id, type: 'host', label: `${h.name} · ${h.ip}`, host: h, children: [] })))
    }
  }
  return roots
}
const buildGroupNode = (g, groups, hosts) => ({
  key: 'g-' + g.id, type: 'group', groupId: g.id, label: g.name,
  children: groups.filter(c => c.parent_id === g.id).map(c => buildGroupNode(c, groups, hosts))
    .concat(hosts.filter(h => h.group_id === g.id)
      .map(h => ({ key: 'h-' + h.id, type: 'host', label: `${h.name} · ${h.ip}`, host: h, children: [] })))
})

const treeData = computed(() => {
  const nodes = buildGroupTree(groups.value, allHosts.value)
  const orphan = allHosts.value.filter(h => !h.group_id)
    .map(h => ({ key: 'h-' + h.id, type: 'host', label: `${h.name} · ${h.ip}`, host: h, children: [] }))
  if (orphan.length) {
    nodes.push({ key: 'g-none', type: 'group', groupId: null, label: t('hosts.uncategorized'), children: orphan })
  }
  return nodes
})

const onTreeNode = node => {
  if (node.type === 'group') {
    groupFilter.value = node.groupId || undefined
  } else {
    keyword.value = node.host.ip
  }
  load()
}

const load = async () => {
  loading.value = true
  try {
    // 单次全量请求，表格筛选在前端完成（左侧树同样使用全量数据），避免双份 /hosts 载荷
    const all = await api.get('/hosts')
    allHosts.value = all
    let list = all
    if (groupFilter.value) list = list.filter(h => String(h.group_id) === String(groupFilter.value))
    if (keyword.value) {
      const kw = keyword.value.toLowerCase()
      list = list.filter(h => (h.name || '').toLowerCase().includes(kw) || (h.ip || '').toLowerCase().includes(kw))
    }
    hosts.value = list
  } finally { loading.value = false }
  groups.value = await api.get('/host_groups')
}
const loadKeys = async () => { keys.value = await api.get('/ssh_keys') }

onMounted(() => {
  load()
  loadKeys()
})

// 终端：跳转到 Web Shell 终端工作台，可带主机直接连接
const openTerminal = row => {
  router.push(`/shell?host=${row.id}`)
}

// 行操作下拉分发
const onRowCmd = async (cmd, row) => {
  if (cmd === 'terminal') openTerminal(row)
  else if (cmd === 'cred') router.push(`/os-accounts?host=${row.id}`)
  else if (cmd === 'edit') dlgHost(row)
  else if (cmd === 'delete') {
    try {
      await ElMessageBox.confirm(t('hosts.delHostConfirm'), t('common.tip'), { type: 'warning' })
    } catch { return }
    delHost(row)
  }
}

const dlgHost = row => {
  hostForm.value = row ? { ...row, password: '' } : { name: '', ip: '', port: 22, username: 'root', auth_type: 'key', ssh_key_id: keys.value[0]?.id, group_id: null, auto_pair: true }
  hostVisible.value = true
}
// ---- OS 账号权限（下拉项显隐），管理功能在「OS 账号」页面 ----
const roleSettings = ref({})
const canManageCreds = computed(() => {
  if (store.isAdmin) return true
  return !!roleSettings.value[store.role]?.cred
})
api.get('/system/roles').then(rs => { roleSettings.value = rs }).catch(() => {})

const saveHost = async () => {
  if (!hostForm.value.name || !hostForm.value.ip || !hostForm.value.username) { ElMessage.warning(t('hosts.needNameIpUser')); return }
  if (hostForm.value.auth_type === 'key' && !hostForm.value.ssh_key_id) { ElMessage.warning(t('hosts.needKey')); return }
  if (hostForm.value.id) {
    await api.put(`/hosts/${hostForm.value.id}`, hostForm.value)
    ElMessage.success(t('hosts.saved'))
  } else {
    const res = await api.post('/hosts', hostForm.value)
    // 密钥认证：同步创建默认 OS 账号；密码认证：后端已建账号并按 auto_pair 尝试配对
    if (hostForm.value.auth_type === 'key' && hostForm.value.ssh_key_id) {
      await api.post(`/hosts/${res.host.id}/credentials`, {
        username: hostForm.value.username, auth_type: 'key', ssh_key_id: hostForm.value.ssh_key_id, is_default: true
      })
      ElMessage.success(t('hosts.saved'))
    } else if (hostForm.value.auth_type === 'password' && hostForm.value.password) {
      if (res.paired) {
        ElMessage.success(t('hosts.saved') + ' · ' + t('hosts.pairOk'))
      } else {
        ElMessage.warning((res.pair_error || t('hosts.saved')) + '', { duration: 6000 })
      }
    }
  }
  hostVisible.value = false
  load()
}
const delHost = async row => { await api.delete(`/hosts/${row.id}`); ElMessage.success(t('hosts.deleted')); load() }

const probeAll = async () => {
  probing.value = true
  try {
    await api.post('/hosts/probe', { host_ids: hosts.value.map(h => h.id) })
    await load()
    ElMessage.success(t('hosts.probeDone'))
  } finally { probing.value = false }
}

const dlgImport = () => { importVisible.value = true }

// 读取 CSV/TXT 文件内容填入文本框（每行一台主机）
const onImportFile = ev => {
  const file = ev.target.files[0]
  if (!file) return
  const reader = new FileReader()
  reader.onload = () => {
    importForm.value.content = String(reader.result || '').replace(/^\uFEFF/, '')
    ElMessage.success(t('files.uploadOk'))
  }
  reader.readAsText(file, 'utf-8')
}
const doImport = async () => {
  importing.value = true
  try {
    const res = await api.post('/hosts/import', importForm.value)
    let msg = t('hosts.importResult', {
      created: res.created, skipped: res.skipped,
      errors: res.errors?.length ? `, ${res.errors.length} failed` : ''
    })
    if (res.auto_pair !== undefined || res.paired !== undefined) {
      msg += t('hosts.importPairResult', { paired: res.paired || 0, failed: res.pair_failed || 0 })
    }
    ElMessage({ message: msg, type: res.errors?.length ? 'warning' : 'success', duration: 6000 })
    importVisible.value = false
    load()
  } finally { importing.value = false }
}

const newGroupParent = ref(null)
const renameVisible = ref(false)
const renameForm = ref({})
const renameGroupDlg = row => {
  renameForm.value = { id: row.id, name: row.name, parent_id: row.parent_id || null }
  renameVisible.value = true
}
const saveRename = async () => {
  if (!renameForm.value.name) { ElMessage.warning(t('hosts.groupName')); return }
  await api.put(`/host_groups/${renameForm.value.id}`, { name: renameForm.value.name, description: '', parent_id: renameForm.value.parent_id })
  ElMessage.success(t('hosts.saved'))
  renameVisible.value = false
  load()
}
const addGroup = async () => {
  if (!newGroup.value) return
  await api.post('/host_groups', { name: newGroup.value, parent_id: newGroupParent.value })
  newGroup.value = ''
  load()
}
const dlgGroup = () => { groupVisible.value = true; load() }
const delGroup = async row => {
  await api.delete(`/host_groups/${row.id}`)
  ElMessage.success(t('hosts.deleted'))
  load()
}

const saveKey = async () => {
  if (!keyForm.value.name || !keyForm.value.private_key) { ElMessage.warning(t('hosts.needKey')); return }
  await api.post('/ssh_keys', keyForm.value)
  ElMessage.success(t('hosts.keySaved'))
  keyDlgVisible.value = false
  keyForm.value = { name: '', public_key: '', private_key: '' }
  loadKeys()
}
const delKey = async row => {
  await api.delete(`/ssh_keys/${row.id}`)
  ElMessage.success(t('hosts.deleted'))
  loadKeys()
}
</script>

<style scoped>
.tree-node { display: flex; align-items: center; gap: 6px; font-size: 13px; }
</style>
