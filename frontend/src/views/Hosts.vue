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
          <el-button type="info" plain @click="showTemplates = true">{{ $t('hosts.tplMgmt') }}</el-button>
          <el-button v-if="store.isAdmin || canManageCreds" type="danger" plain :disabled="!selHosts.length"
                     @click="batchDelHosts">{{ $t('k8s.batchDelete') }}{{ selHosts.length ? ` (${selHosts.length})` : '' }}</el-button>
        </div>

        <el-table :data="hosts" v-loading="loading" size="small" border
                  @selection-change="s => (selHosts = s)">
          <el-table-column type="selection" width="38" :selectable="() => store.isAdmin || canManageCreds" />
          <el-table-column prop="id" label="ID" width="60" />
          <el-table-column prop="name" :label="$t('hosts.name')" min-width="120" />
          <el-table-column prop="ip" :label="$t('hosts.ip')" width="140" />
          <el-table-column prop="port" :label="$t('hosts.port')" width="70" />
          <el-table-column :label="'OS'" width="90" align="center">
            <template #default="{ row }">
              <el-tag size="small" :type="row.os_type === 'windows' ? 'warning' : 'info'">
                {{ row.os_type === 'windows' ? 'Windows' : 'Linux' }}
              </el-tag>
            </template>
          </el-table-column>
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
                    <el-dropdown-item v-if="row.os_type === 'windows'" command="rdp">RDP</el-dropdown-item>
                    <el-dropdown-item command="capacity">{{ $t('k8s.capacity') }}</el-dropdown-item>
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
      <el-form-item :label="$t('hosts.port')">
        <el-input-number v-model="hostForm.port" :min="1" :max="65535" />
        <el-radio-group v-model="hostForm.os_type" style="margin-left:16px" @change="onOsChange">
          <el-radio value="linux">Linux</el-radio>
          <el-radio value="windows">Windows</el-radio>
        </el-radio-group>
      </el-form-item>
      <el-form-item v-if="hostForm.os_type === 'windows'" :label="'WinRM'">
        <el-input-number v-model="hostForm.winrm_port" :min="1" :max="65535" />
        <span style="margin-left:16px">RDP</span>
        <el-input-number v-model="hostForm.rdp_port" :min="1" :max="65535" />
      </el-form-item>
      <el-form-item :label="$t('hosts.user')"><el-input v-model="hostForm.username" /></el-form-item>
      <el-form-item :label="$t('hosts.authType')">
        <el-radio-group v-model="hostForm.auth_type">
          <el-radio value="key" :disabled="hostForm.os_type === 'windows'">{{ $t('hosts.key') }}</el-radio>
          <el-radio value="password">{{ $t('hosts.password') }}</el-radio>
        </el-radio-group>
      </el-form-item>
      <el-form-item :label="$t('hosts.key')" v-if="hostForm.auth_type === 'key'">
        <el-select v-model="hostForm.ssh_key_id" :placeholder="$t('hosts.keyPlaceholder')" style="width:100%">
          <el-option v-for="k in keys" :key="k.id" :label="k.name" :value="k.id" />
        </el-select>
      </el-form-item>
      <el-form-item v-if="!hostForm.id" :label="$t('hosts.tplPick')">
        <el-select v-model="hostForm.template_id" clearable style="width:100%" :placeholder="$t('hosts.tplPickTip')"
                   @change="onTplPick">
          <el-option v-for="tp in templates" :key="tp.id" :value="tp.id"
                     :label="`${tp.username}${tp.is_ldap ? ' (AD)' : ''} · ${tp.label}`" />
        </el-select>
      </el-form-item>
      <el-form-item :label="$t('hosts.password')" v-else>
        <el-input v-model="hostForm.password" type="password" show-password :disabled="!!hostForm.template_id"
                  :placeholder="hostForm.id ? $t('hosts.passwordKeep') : (hostForm.template_id ? $t('hosts.tplInUse') : '')" />
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
      <el-button type="primary" :loading="savingHost" @click="saveHost">{{ $t('common.save') }}</el-button>
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
      <el-form-item :label="$t('hosts.tplPick')">
        <el-select v-model="importForm.template_id" clearable style="width:100%" :placeholder="$t('hosts.tplPickTip')">
          <el-option v-for="tp in templates" :key="tp.id" :value="tp.id"
                     :label="`${tp.username}${tp.is_ldap ? ' (AD)' : ''} · ${tp.label}`" />
        </el-select>
      </el-form-item>
      <el-form-item :label="$t('hosts.commonPassword')">
        <el-input v-model="importForm.password" type="password" show-password autocomplete="new-password"
                  :disabled="!!importForm.template_id"
                  :placeholder="importForm.template_id ? $t('hosts.tplInUse') : $t('hosts.commonPasswordPlaceholder')" />
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

  <!-- 账号模板管理 -->
  <el-drawer v-model="showTemplates" :title="$t('hosts.tplMgmt')" size="480px">
    <div style="display:flex; gap:8px; margin-bottom:12px; flex-wrap:wrap">
      <el-input v-model="tplForm.username" :placeholder="$t('users.username')" style="width:140px" />
      <el-input v-model="tplForm.password" type="password" show-password :placeholder="$t('hosts.password')" style="width:150px" />
      <el-input v-model="tplForm.label" :placeholder="$t('hosts.tplLabel')" style="width:110px" />
      <el-checkbox v-model="tplForm.is_ldap" style="margin:0">AD/LDAP</el-checkbox>
      <el-button type="primary" size="small" :loading="tplSaving" @click="saveTpl">{{ $t('common.add') }}</el-button>
    </div>
    <el-table :data="templates" size="small" border>
      <el-table-column prop="username" label="User" min-width="120" />
      <el-table-column prop="label" :label="$t('hosts.tplLabel')" min-width="90" />
      <el-table-column :label="'AD/LDAP'" width="90" align="center">
        <template #default="{ row }">
          <el-tag size="small" :type="row.is_ldap ? 'warning' : 'info'">{{ row.is_ldap ? 'AD' : '-' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column :label="$t('common.operation')" width="80">
        <template #default="{ row }">
          <el-popconfirm :title="$t('monitor.configDelConfirm')" @confirm="delTpl(row)">
            <template #reference><el-button size="small" type="danger" link>{{ $t('common.delete') }}</el-button></template>
          </el-popconfirm>
        </template>
      </el-table-column>
    </el-table>
    <div style="margin-top:12px; color:#909399; font-size:12px">{{ $t('hosts.tplTip') }}</div>
  </el-drawer>

  <!-- 容量规划抽屉 -->
  <el-drawer v-model="capVisible" size="56%" :title="`${$t('k8s.capacity')} · ${capHost?.name || ''}`" destroy-on-close>
    <div style="display:flex; gap:8px; align-items:center; margin-bottom:10px">
      <el-radio-group v-model="capDays" size="small" @change="loadCap">
        <el-radio-button :value="30">30d</el-radio-button>
        <el-radio-button :value="180">180d</el-radio-button>
        <el-radio-button :value="365">1y</el-radio-button>
      </el-radio-group>
      <span style="flex:1"></span>
      <span style="display:flex; gap:14px" class="km-usage-sub">
        <span><span class="cap-dot" style="background:#409eff"></span>CPU</span>
        <span><span class="cap-dot" style="background:#67c23a"></span>{{ $t('monitor.mem') }}</span>
        <span><span class="cap-dot" style="background:#e6a23c"></span>{{ $t('monitor.disk') }}</span>
        <span><span class="cap-dot" style="background:#f56c6c"></span>{{ $t('k8s.capTrend') }}</span>
        <span><span class="cap-dot" style="background:#909399"></span>90%</span>
      </span>
    </div>
    <el-alert v-if="cap?.degraded" :title="$t('k8s.capDegraded')" type="warning" :closable="false" style="margin-bottom:10px" />
    <div v-for="m in ['cpu', 'mem', 'disk']" :key="m" class="km-usage-card" style="margin-bottom:10px">
      <div class="km-usage-head">
        {{ m === 'cpu' ? 'CPU' : m === 'mem' ? $t('monitor.mem') : $t('monitor.disk') }}
        <span style="float:right; font-weight:400" class="mono">{{ capLast(m) }}%</span>
      </div>
      <svg viewBox="0 0 600 110" class="cap-chart">
        <line x1="30" x2="590" :y1="pctY(90)" :y2="pctY(90)" stroke="#909399" stroke-dasharray="4 3" stroke-width="1" />
        <polyline :points="pctPoly(m)" fill="none" :stroke="m === 'cpu' ? '#409eff' : m === 'mem' ? '#67c23a' : '#e6a23c'" stroke-width="2" />
        <polyline v-if="pctForecastLine(m).length" :points="pctForecastLine(m)" fill="none" stroke="#f56c6c"
                  stroke-width="1.5" stroke-dasharray="5 4" />
      </svg>
      <div class="km-usage-sub" :style="{ color: capDaysColor(cap?.forecast?.[capField(m)]) }">
        {{ capDaysText(cap?.forecast?.[capField(m)]) }}
      </div>
    </div>
  </el-drawer>

</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import api from '../api'
import i18n from '../i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useUserStore } from '../store'

const { t } = i18n.global

// ---- 容量规划抽屉（30d raw / 180d·1y 小时聚合，线性预测到 90% 水位） ----
const capVisible = ref(false)
const capHost = ref(null)
const capDays = ref(30)
const cap = ref(null)
const capField = m => ({ cpu: 'cpu_days_to_90', mem: 'mem_days_to_90', disk: 'disk_days_to_90' })[m]
const openCapacity = row => {
  capHost.value = row
  capDays.value = 30
  capVisible.value = true
  loadCap()
}
// ---- RDP 远程桌面（guacamole-lite 网关，加密连接串 5 分钟有效） ----
const rdpWin = ref(null)
const openRDP = async row => {
  try {
    const r = await api.post(`/hosts/${row.id}/rdp-token`, {})
    const w = 1280, h = 720
    const left = (screen.width - w) / 2
    const top = Math.max(0, (screen.height - h) / 2 - 40)
    window.open(`${r.gateway}/?width=${w}&height=${h}&dpi=${r.dpi || 96}`, '_blank',
      `width=${w + 16},height=${h + 60},left=${left},top=${top}`)
  } catch { /* 拦截器提示 */ }
}

const loadCap = async () => {
  cap.value = await api.get(`/monitoring/hosts/${capHost.value.id}/capacity`, { params: { days: capDays.value } }).catch(() => null)
}
const CH = { x0: 30, x1: 590, y0: 6, y1: 104 }
const capSpanMs = computed(() => (capDays.value + 90) * 86400000)
const pctY = v => {
  const y = CH.y1 - ((Number(v) || 0) / 100) * (CH.y1 - CH.y0)
  return Math.max(CH.y0, Math.min(CH.y1, y)).toFixed(1)
}
const pctX = t => {
  const start = Date.now() - capDays.value * 86400000
  const x = CH.x0 + ((new Date(t).getTime() - start) / capSpanMs.value) * (CH.x1 - CH.x0)
  return Math.max(CH.x0, Math.min(CH.x1, x)).toFixed(1)
}
const pctPoly = m => {
  const pts = cap.value?.points || []
  if (pts.length < 2) return ''
  return pts.map(p => `${pctX(p.t)},${pctY(p[m + '_percent'])}`).join(' ')
}
const pctForecastLine = m => {
  const fc = cap.value?.forecast
  const pts = cap.value?.points || []
  if (!fc || !pts.length) return ''
  const slope = m === 'cpu' ? fc.cpu_slope_pct_per_day : m === 'mem' ? fc.mem_slope_pct_per_day : fc.disk_slope_pct_per_day
  if (!(slope > 0)) return ''
  const last = pts[pts.length - 1]
  const cur = m === 'cpu' ? last.cpu_percent : m === 'mem' ? last.mem_percent : last.disk_percent
  const p1 = `${pctX(last.t)},${pctY(cur)}`
  const future = new Date(Date.now() + 90 * 86400000).toISOString()
  const p2 = `${pctX(future)},${pctY(Math.min(100, cur + slope * 90))}`
  return `${p1} ${p2}`
}
const capLast = m => {
  const pts = cap.value?.points || []
  if (!pts.length) return '0'
  const last = pts[pts.length - 1]
  return (m === 'cpu' ? last.cpu_percent : m === 'mem' ? last.mem_percent : last.disk_percent).toFixed(1)
}
const capDaysText = d => {
  if (d == null) return t('k8s.capNoExhaust')
  if (d <= 0) return t('k8s.capExhausted')
  return d < 60 ? `${t('k8s.cap90In')} ~${Math.round(d)} ${t('k8s.capDays')}` : `${t('k8s.cap90In')} ~${(d / 30).toFixed(1)} ${t('k8s.capMonths')}`
}
const capDaysColor = d => (d == null ? '#67c23a' : d < 90 ? '#f56c6c' : d < 180 ? '#e6a23c' : '#67c23a')
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
const importForm = ref({ content: '', ssh_key_id: null, username: 'root', password: '', credential_label: '', auto_pair: true, template_id: null })
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
  loadTemplates()
})

// 终端：跳转到 Web Shell 终端工作台，可带主机直接连接
const openTerminal = row => {
  router.push(`/shell?host=${row.id}`)
}

// 行操作下拉分发
const onRowCmd = async (cmd, row) => {
  if (cmd === 'terminal') openTerminal(row)
  else if (cmd === 'rdp') openRDP(row)
  else if (cmd === 'capacity') openCapacity(row)
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
  hostForm.value = row ? { ...row, password: '', template_id: null } : { name: '', ip: '', port: 22, os_type: 'linux', winrm_port: 5985, rdp_port: 3389, username: 'root', auth_type: 'key', ssh_key_id: keys.value[0]?.id, group_id: null, auto_pair: true, template_id: null }
  hostVisible.value = true
}
// ---- OS 账号权限（下拉项显隐），管理功能在「OS 账号」页面 ----
const roleSettings = ref({})
const canManageCreds = computed(() => {
  if (store.isAdmin) return true
  return !!roleSettings.value[store.role]?.cred
})
api.get('/system/roles').then(rs => { roleSettings.value = rs }).catch(() => {})

// ---- 凭据模板（LDAP/域账号存一次，添加/导入引用） ----
const showTemplates = ref(false)
const templates = ref([])
const tplForm = ref({ username: '', password: '', label: '', is_ldap: false })
const tplSaving = ref(false)
const loadTemplates = async () => { templates.value = await api.get('/credentials/templates').catch(() => []) }
const saveTpl = async () => {
  if (!tplForm.value.username || !tplForm.value.password) { ElMessage.warning(t('hosts.needNameIpUser')); return }
  tplSaving.value = true
  try {
    await api.post('/credentials/templates', tplForm.value)
    ElMessage.success(t('common.success'))
    tplForm.value = { username: '', password: '', label: '', is_ldap: false }
    loadTemplates()
  } finally { tplSaving.value = false }
}
const delTpl = async row => { await api.delete(`/credentials/templates/${row.id}`); loadTemplates() }

// 添加主机：选中模板后联动用户名/认证方式
const onTplPick = id => {
  const tp = templates.value.find(x => x.id === id)
  if (tp) {
    hostForm.value.username = tp.username
    hostForm.value.auth_type = 'password'
  }
}

const selHosts = ref([])
const batchDelHosts = async () => {
  if (!selHosts.value.length) return
  try {
    await ElMessageBox.confirm(t('hosts.batchDelConfirm', { n: selHosts.value.length }), t('common.delete'), { type: 'warning' })
  } catch { return }
  const r = await api.post('/hosts/batch-delete', { ids: selHosts.value.map(h => h.id) })
  const failed = Object.keys(r.failed || {}).length
  if (failed) ElMessage.warning(t('hosts.batchDelDone', { ok: r.deleted.length, fail: failed }))
  else ElMessage.success(t('common.success'))
  selHosts.value = []
  load()
}

const onOsChange = t => {
  if (t === 'windows') {
    if (!hostForm.value.winrm_port) hostForm.value.winrm_port = 5985
    if (!hostForm.value.rdp_port) hostForm.value.rdp_port = 3389
    if (hostForm.value.port === 22) hostForm.value.port = 5985
    hostForm.value.auth_type = 'password'
  } else {
    if (hostForm.value.port === 5985) hostForm.value.port = 22
  }
}

const savingHost = ref(false)
const saveHost = async () => {
  if (!hostForm.value.name || !hostForm.value.ip || !hostForm.value.username) { ElMessage.warning(t('hosts.needNameIpUser')); return }
  if (hostForm.value.auth_type === 'key' && !hostForm.value.ssh_key_id) { ElMessage.warning(t('hosts.needKey')); return }
  if (savingHost.value) return // 防连点：密码认证时后端要 SSH 配对，可能耗时数十秒
  savingHost.value = true
  try {
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
  } finally { savingHost.value = false }
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
.cap-chart { width: 100%; height: 110px; background: #fafbfc; border-radius: 4px; }
.cap-dot { display: inline-block; width: 10px; height: 3px; vertical-align: middle; margin-right: 4px; }
</style>
