<!-- JNexus 运维平台 — By JJ Zhang, Version 1.0 -->
<template>
  <div class="km-page">
    <!-- 左侧集群菜单 -->
    <aside class="km-side">
      <div class="km-brand">JNexus <span>K8S</span></div>
      <div class="km-cluster mono" v-if="cluster">{{ cluster.name }}</div>
      <el-menu :default-active="active" class="km-menu" background-color="transparent"
               text-color="#a7b1c2" active-text-color="#ffffff" unique-opened :collapse-transition="false"
               @select="k => (active = k)">
        <el-menu-item index="overview">
          <el-icon><Odometer /></el-icon><span>{{ $t('k8s.navOverview') }}</span>
        </el-menu-item>
        <el-sub-menu index="grp-cluster">
          <template #title><el-icon><OfficeBuilding /></el-icon><span>{{ $t('k8s.navCluster') }}</span></template>
          <el-menu-item index="capacity">{{ $t('k8s.capacity') }}</el-menu-item>
          <el-menu-item index="nodes">Nodes</el-menu-item>
          <el-menu-item index="namespaces">{{ $t('k8s.namespaces') }}</el-menu-item>
        </el-sub-menu>
        <el-sub-menu index="grp-workloads">
          <template #title><el-icon><Box /></el-icon><span>{{ $t('k8s.navWorkloads') }}</span></template>
          <el-menu-item index="pods">Pods</el-menu-item>
          <el-menu-item index="deployments">Deployments</el-menu-item>
          <el-menu-item index="daemonsets">DaemonSets</el-menu-item>
          <el-menu-item index="statefulsets">StatefulSets</el-menu-item>
          <el-menu-item index="jobs">Jobs</el-menu-item>
          <el-menu-item index="cronjobs">CronJobs</el-menu-item>
        </el-sub-menu>
        <el-sub-menu index="grp-discovery">
          <template #title><el-icon><Connection /></el-icon><span>{{ $t('k8s.navDiscovery') }}</span></template>
          <el-menu-item index="services">Services</el-menu-item>
          <el-menu-item index="ingresses">Ingresses</el-menu-item>
        </el-sub-menu>
        <el-sub-menu index="grp-config">
          <template #title><el-icon><Setting /></el-icon><span>{{ $t('k8s.navConfig') }}</span></template>
          <el-menu-item index="configmaps">ConfigMaps</el-menu-item>
          <el-menu-item index="secrets">Secrets</el-menu-item>
        </el-sub-menu>
        <el-sub-menu index="grp-storage">
          <template #title><el-icon><Coin /></el-icon><span>{{ $t('k8s.navStorage') }}</span></template>
          <el-menu-item index="pvcs">PVCs</el-menu-item>
          <el-menu-item index="pvs">PVs</el-menu-item>
          <el-menu-item index="storageclasses">StorageClasses</el-menu-item>
        </el-sub-menu>
        <el-sub-menu index="grp-access">
          <template #title><el-icon><Key /></el-icon><span>{{ $t('k8s.navAccess') }}</span></template>
          <el-menu-item index="serviceaccounts">ServiceAccounts</el-menu-item>
        </el-sub-menu>
        <el-sub-menu index="grp-apps">
          <template #title><el-icon><ShoppingBag /></el-icon><span>{{ $t('k8s.navApps') }}</span></template>
          <el-menu-item index="helmreleases">Helm Releases</el-menu-item>
        </el-sub-menu>
      </el-menu>
    </aside>

    <!-- 主区域 -->
    <main class="km-main">
      <header class="km-top">
        <el-button size="small" @click="$router.push('/k8s')">
          <el-icon style="margin-right:4px"><Back /></el-icon>{{ $t('k8s.backToList') }}
        </el-button>
        <b style="font-size:15px">{{ cluster?.name || '...' }}</b>
        <el-tag v-if="summary?.version" size="small" type="info" class="mono">{{ summary.version }}</el-tag>
        <el-tag v-if="cluster" size="small" :type="cluster.status === 'online' ? 'success' : 'danger'">
          {{ cluster.status === 'online' ? $t('k8s.online') : $t('k8s.offline') }}
        </el-tag>
        <span style="flex:1"></span>
        <el-select v-model="ns" size="small" clearable style="width:190px" :placeholder="$t('k8s.allNamespaces')"
                   :disabled="!namespacedActive">
          <el-option v-for="n in namespaces" :key="n" :label="n" :value="n" />
        </el-select>
        <el-tag v-if="cluster" size="small" :type="cluster.my_role === 'admin' ? 'warning' : cluster.my_role === 'user' ? 'success' : 'info'">
          {{ roleText(cluster.my_role) }}
        </el-tag>
        <el-button size="small" :loading="loading" @click="load">{{ $t('common.refresh') }}</el-button>
      </header>

      <section class="km-body">
        <!-- 概览 -->
        <template v-if="active === 'overview'">
          <div class="km-cards">
            <div v-for="card in cards" :key="card.key" class="km-card" :style="{ '--c': card.color }" @click="active = card.go">
              <div class="km-card-num">{{ card.text }}</div>
              <div class="km-card-label">{{ card.label }}</div>
            </div>
          </div>
          <h4 class="km-h4">{{ $t('k8s.clusterResources') }}</h4>
          <div class="km-usage">
            <div class="km-usage-card">
              <div class="km-usage-head">{{ $t('k8s.clusterCPU') }}
                <el-link type="primary" style="float:right; font-size:12px" @click="active = 'capacity'; loadCapacity()">{{ $t('k8s.capViewTrend') }} →</el-link>
              </div>
              <el-progress :percentage="usagePercent.cpu" :stroke-width="14" :color="usageColor(usagePercent.cpu)" />
              <div class="km-usage-sub mono">
                {{ fmtCores(usage?.cpu_used_m) }} {{ $t('k8s.used') }} · {{ $t('k8s.podReq') }} {{ fmtCores(usage?.pod_req_cpu_m) }} · {{ fmtCores(usage?.cpu_capacity_m) }}
              </div>
            </div>
            <div class="km-usage-card">
              <div class="km-usage-head">{{ $t('k8s.clusterMem') }}
                <el-link type="primary" style="float:right; font-size:12px" @click="active = 'capacity'; loadCapacity()">{{ $t('k8s.capViewTrend') }} →</el-link>
              </div>
              <el-progress :percentage="usagePercent.mem" :stroke-width="14" :color="usageColor(usagePercent.mem)" />
              <div class="km-usage-sub mono">
                {{ usage?.mem_used_mi || 0 }}Mi {{ $t('k8s.used') }} · {{ $t('k8s.podReq') }} {{ usage?.pod_req_mem_mi || 0 }}Mi · {{ fmtGi(usage?.mem_capacity_mi) }}
              </div>
            </div>
          </div>

          <h4 class="km-h4">{{ $t('k8s.podUsage') }}</h4>
          <el-table :data="usagePods" size="small" border v-loading="loading">
            <el-table-column prop="namespace" label="Namespace" width="120" sortable />
            <el-table-column prop="name" label="Pod" min-width="180" sortable />
            <el-table-column prop="phase" :label="$t('k8s.status')" width="100" align="center">
              <template #default="{ row }">
                <el-tag size="small" :type="row.phase === 'Running' ? 'success' : 'warning'">{{ row.phase }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column :label="$t('k8s.cpu')" min-width="220" align="center">
              <template #header>{{ $t('k8s.cpu') }}（{{ $t('k8s.reqLimit') }} / {{ $t('k8s.used') }}）</template>
              <template #default="{ row }">
                <span class="mono">{{ fmtReqLimC(row.cpu_req_m, row.cpu_lim_m) }} / <b>{{ fmtCoresV(row.cpu_m) }}</b></span>
              </template>
            </el-table-column>
            <el-table-column :label="$t('k8s.memory')" min-width="230" align="center">
              <template #header>{{ $t('k8s.memory') }}（{{ $t('k8s.reqLimit') }} / {{ $t('k8s.used') }}）</template>
              <template #default="{ row }">
                <span class="mono">{{ fmtReqLim(row.mem_req_mi, row.mem_lim_mi) }} / <b>{{ fmtMem(row.mem_mi) }}</b></span>
              </template>
            </el-table-column>
          </el-table>

          <h4 class="km-h4" style="margin-top:18px">{{ $t('k8s.recentEvents') }}</h4>
          <el-table :data="events" size="small" border v-loading="loading">
            <el-table-column prop="namespace" label="Namespace" width="110" />
            <el-table-column :label="$t('monitor.evType')" width="90" align="center">
              <template #default="{ row }">
                <el-tag size="small" :type="row.type === 'Warning' ? 'warning' : 'info'">{{ row.type }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="reason" label="Reason" width="140" />
            <el-table-column prop="object" label="Object" min-width="150" />
            <el-table-column prop="message" :label="$t('audit.detail')" min-width="240" show-overflow-tooltip />
            <el-table-column prop="count" label="#" width="60" align="center" />
            <el-table-column :label="$t('audit.time')" width="160">
              <template #default="{ row }">{{ fmtTime(row.last_seen) }}</template>
            </el-table-column>
          </el-table>
        </template>

        <!-- 容量规划 -->
        <template v-else-if="active === 'capacity'">
          <div class="km-toolbar">
            <h4 class="km-h4" style="margin:0">{{ $t('k8s.capacity') }}</h4>
            <span style="flex:1"></span>
            <el-radio-group v-model="capDays" size="small" @change="loadCapacity">
              <el-radio-button :value="30">30d</el-radio-button>
              <el-radio-button :value="180">180d</el-radio-button>
              <el-radio-button :value="365">1y</el-radio-button>
            </el-radio-group>
          </div>
          <el-alert v-if="cap?.degraded" :title="$t('k8s.capDegraded')" type="warning" :closable="false" style="margin-bottom:12px" />

          <div class="km-usage">
            <div class="km-usage-card">
              <div class="km-usage-head">
                {{ $t('k8s.clusterCPU') }} · {{ $t('k8s.capForecast') }}
                <span style="float:right; font-weight:400" class="mono">{{ capLastPct('cpu') }}%</span>
              </div>
              <MetricChart :points="capPctPoints('cpu')" :range="capRange" :empty-text="$t('monitor.noData')"
                           color="#409eff" unit="%" :y-max="100" />
              <div class="km-usage-sub mono">
                {{ fmtCores(cap?.forecast?.cpu_current_m) }} / {{ fmtCores(cap?.forecast?.cpu_capacity_m) }} ·
                {{ $t('k8s.capSlope') }} {{ capSlopeText(cap?.forecast?.cpu_slope_m_per_day, 'cpu') }}
              </div>
              <div class="km-usage-sub" :style="{ color: capDaysLeftColor(cap?.forecast?.cpu_days_left) }">
                {{ capDaysLeftText(cap?.forecast?.cpu_days_left, 'cpu') }}
              </div>
            </div>
            <div class="km-usage-card">
              <div class="km-usage-head">
                {{ $t('k8s.clusterMem') }} · {{ $t('k8s.capForecast') }}
                <span style="float:right; font-weight:400" class="mono">{{ capLastPct('mem') }}%</span>
              </div>
              <MetricChart :points="capPctPoints('mem')" :range="capRange" :empty-text="$t('monitor.noData')"
                           color="#67c23a" unit="%" :y-max="100" />
              <div class="km-usage-sub mono">
                {{ fmtMem(cap?.forecast?.mem_current_mi) }} / {{ fmtMem(cap?.forecast?.mem_capacity_mi) }} ·
                {{ $t('k8s.capSlope') }} {{ capSlopeText(cap?.forecast?.mem_slope_mi_per_day, 'mem') }}
              </div>
              <div class="km-usage-sub" :style="{ color: capDaysLeftColor(cap?.forecast?.mem_days_left) }">
                {{ capDaysLeftText(cap?.forecast?.mem_days_left, 'mem') }}
              </div>
            </div>
          </div>

          <div class="km-usage-sub" style="display:flex; gap:16px; margin-bottom:12px">
            <span><span class="cap-dot" style="background:#409eff"></span>CPU</span>
            <span><span class="cap-dot" style="background:#67c23a"></span>{{ $t('k8s.memory') }}</span>
            <span>100% = {{ $t('k8s.capCapacityLine') }}</span>
          </div>

          <h4 class="km-h4" style="margin-top:6px">{{ $t('k8s.podCapTitle') }}</h4>
          <div class="km-toolbar" style="margin-bottom:8px">
            <el-select v-model="capPodSel" size="small" style="width:280px" :placeholder="$t('k8s.podCapPick')">
              <el-option v-for="p in capPods" :key="p.namespace + '/' + p.name"
                         :value="p.namespace + '/' + p.name"
                         :label="p.namespace + '/' + p.name" />
            </el-select>
            <span v-if="capPodTrend" class="km-usage-sub mono" style="margin:0">
              {{ $t('k8s.capSlope') }} {{ capPodSlopeText }}
            </span>
          </div>
          <div class="km-usage-card" style="max-width:720px">
            <div class="km-usage-sub mono" style="margin:0 0 2px">
              <span><span class="cap-dot" style="background:#409eff"></span>CPU</span>
            </div>
            <MetricChart :points="podPoints('cpu_m')" :range="capRange" :empty-text="$t('monitor.noData')"
                         color="#409eff" unit="m" :y-max="podYMax('cpu_m')" />
            <div class="km-usage-sub mono" style="margin:10px 0 2px">
              <span><span class="cap-dot" style="background:#67c23a"></span>{{ $t('k8s.memory') }}</span>
            </div>
            <MetricChart :points="podPoints('mem_mi')" :range="capRange" :empty-text="$t('monitor.noData')"
                         color="#67c23a" unit="Mi" :y-max="podYMax('mem_mi')" />
          </div>
        </template>

        <!-- 资源列表（通用表格） -->
        <template v-else>
          <div class="km-toolbar">
            <h4 class="km-h4" style="margin:0">{{ sectionTitle }}</h4>
            <span style="flex:1"></span>
            <el-input v-model="search" size="small" clearable style="width:190px"
                      :prefix-icon="SearchIcon" :placeholder="$t('k8s.searchTip')" />
            <template v-if="canOp">
              <el-button v-if="creatable.has(active)" size="small" type="primary" @click="openCreateYAML">
                {{ $t('k8s.yamlCreate') }}
              </el-button>
              <el-button v-if="deletable.has(active)" size="small" type="danger" plain
                         :disabled="!selectedRows.length" @click="batchDelete">
                {{ $t('k8s.batchDelete') }}{{ selectedRows.length ? ` (${selectedRows.length})` : '' }}
              </el-button>
            </template>
            <el-button size="small" @click="exportCSV">{{ $t('k8s.exportCsv') }}</el-button>
          </div>
          <el-table :data="filteredRows" size="small" border v-loading="loading"
                    @selection-change="s => (selectedRows = s)" :row-key="r => (r.namespace || '') + '/' + r.name">
            <el-table-column v-if="canOp && deletable.has(active)" type="selection" width="38" />
            <el-table-column v-for="col in cols" :key="col.prop" :min-width="col.w || 120" :align="col.align">
              <template #header><span v-if="col.headerHtml" v-html="col.headerHtml"></span><span v-else>{{ colLabel(col) }}</span></template>
              <template #default="{ row }">
                <el-tag v-if="col.tag" size="small" :type="col.tag(row)">
                  {{ col.text ? col.text(row) : row[col.prop] }}
                </el-tag>
                <template v-else>{{ col.text ? col.text(row) : row[col.prop] }}</template>
              </template>
            </el-table-column>
            <el-table-column :label="$t('common.operation')" :width="opWidth" fixed="right">
              <template #default="{ row }">
                <!-- YAML 查看（Helm 行除外） -->
                <el-button v-if="kindOf[active]" size="small" link type="info" @click="showYAML(row)">{{ $t('k8s.yaml') }}</el-button>
                <!-- Pods -->
                <template v-if="active === 'pods'">
                  <el-button size="small" link type="primary" @click="showLog(row)">{{ $t('k8s.logs') }}</el-button>
                  <el-button size="small" link type="primary" @click="openFollow(row)">{{ $t('k8s.logFollow') }}</el-button>
                  <el-button size="small" link type="success" @click="openShell(row)">Shell</el-button>
                  <el-popconfirm v-if="canOp" :title="$t('k8s.podDelConfirm')" @confirm="deletePod(row)">
                    <template #reference><el-button size="small" type="danger" link>{{ $t('common.delete') }}</el-button></template>
                  </el-popconfirm>
                </template>
                <!-- Deployments / StatefulSets -->
                <template v-if="(active === 'deployments' || active === 'statefulsets') && canOp">
                  <el-button size="small" link type="primary" @click="scaleWorkload(row)">{{ $t('k8s.scale') }}</el-button>
                  <el-button v-if="active === 'deployments'" size="small" link type="warning" @click="restartDeployment(row)">{{ $t('k8s.restart') }}</el-button>
                </template>
                <!-- CronJobs -->
                <template v-if="active === 'cronjobs' && canOp">
                  <el-button size="small" link type="warning" @click="toggleCron(row)">
                    {{ row.suspend ? $t('k8s.cronResume') : $t('k8s.cronSuspendBtn') }}
                  </el-button>
                  <el-popconfirm :title="$t('k8s.cronDelConfirm')" @confirm="deleteCron(row)">
                    <template #reference><el-button size="small" type="danger" link>{{ $t('common.delete') }}</el-button></template>
                  </el-popconfirm>
                </template>
                <!-- ConfigMaps / Secrets / ServiceAccounts -->
                <el-popconfirm v-if="delConfirmKey && canOp" :title="$t(delConfirmKey)" @confirm="deleteResource(row)">
                  <template #reference><el-button size="small" type="danger" link>{{ $t('common.delete') }}</el-button></template>
                </el-popconfirm>
              </template>
            </el-table-column>
          </el-table>
        </template>
      </section>
    </main>

    <!-- YAML 查看 / 编辑 -->
    <el-dialog v-model="yamlVisible" :title="yamlTitle" width="780px" top="5vh">
      <pre v-if="!yamlEditing" class="mono" style="background:#1e2a35; color:#d8e4f0; padding:14px; border-radius:6px; max-height:520px; overflow:auto; font-size:12px; line-height:1.6">{{ yamlText }}</pre>
      <el-input v-else v-model="yamlEdit" type="textarea" :rows="24" class="mono"
                style="font-size:12px" spellcheck="false" />
      <template #footer>
        <div style="display:flex; gap:8px">
          <span style="flex:1"></span>
          <el-button v-if="!yamlEditing" size="small" @click="downloadYAML">{{ $t('k8s.yamlDownload') }}</el-button>
          <template v-if="yamlEditing">
            <el-button size="small" @click="yamlEditing = false">{{ $t('common.cancel') }}</el-button>
            <el-button size="small" type="primary" :loading="yamlSaving" @click="saveYAML">{{ $t('common.save') }}</el-button>
          </template>
          <el-button v-else-if="canOp" size="small" type="primary" @click="startYAMLEdit">{{ $t('common.edit') }}</el-button>
        </div>
      </template>
    </el-dialog>

    <!-- YAML 创建 -->
    <el-dialog v-model="createVisible" :title="`${$t('k8s.yamlCreate')} · ${createKind}`" width="780px" top="5vh">
      <el-input v-model="createText" type="textarea" :rows="22" class="mono" spellcheck="false" />
      <template #footer>
        <el-button @click="createVisible = false">{{ $t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="createSaving" @click="submitCreate">{{ $t('common.save') }}</el-button>
      </template>
    </el-dialog>

    <!-- 实时日志抽屉 -->
    <el-drawer v-model="followVisible" size="58%" destroy-on-close :title="followTitle">
      <div style="display:flex; gap:8px; align-items:center; margin-bottom:8px">
        <el-tag size="small" :type="followStatus === 'connected' ? 'success' : followStatus === 'closed' ? 'info' : 'danger'">
          {{ followStatus }}
        </el-tag>
        <span style="flex:1"></span>
        <el-button size="small" @click="clearFollow">{{ $t('common.refresh') }}</el-button>
      </div>
      <div ref="followBox" class="follow-box mono"></div>
    </el-drawer>

    <!-- Pod Shell（页内抽屉，不再新开窗口） -->
    <el-drawer v-model="shellDrawer" size="62%" :with-header="false" destroy-on-close>
      <K8sShell v-if="shellDrawer" :cluster-id="id" :namespace="shellPod.namespace" :pod="shellPod.name" />
    </el-drawer>

    <!-- Pod 日志 -->
    <el-dialog v-model="logVisible" :title="logTitle" width="820px">
      <pre class="mono" style="background:#1e2a35; color:#d8e4f0; padding:14px; border-radius:6px; max-height:480px; overflow:auto; font-size:12px; line-height:1.6">{{ logText }}</pre>
    </el-dialog>

    <!-- CronJob 创建 -->
    <el-dialog v-model="cronDlgVisible" :title="$t('k8s.cronAdd')" width="520px">
      <el-form label-width="110px">
        <el-form-item :label="$t('k8s.cronName')"><el-input v-model="cronForm.name" /></el-form-item>
        <el-form-item label="Namespace"><el-input v-model="cronForm.namespace" placeholder="default" /></el-form-item>
        <el-form-item label="Schedule"><el-input v-model="cronForm.schedule" class="mono" placeholder="*/5 * * * *" /></el-form-item>
        <el-form-item :label="$t('k8s.cronCommand')">
          <el-input v-model="cronForm.command" type="textarea" :rows="2" class="mono" placeholder="echo hello" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="cronDlgVisible = false">{{ $t('common.cancel') }}</el-button>
        <el-button type="primary" @click="createCron">{{ $t('common.save') }}</el-button>
      </template>
    </el-dialog>

    <!-- SA 创建 -->
    <el-dialog v-model="saDlgVisible" :title="$t('k8s.saAdd')" width="460px">
      <el-form label-width="110px">
        <el-form-item :label="$t('k8s.saName')"><el-input v-model="saForm.name" /></el-form-item>
        <el-form-item label="Namespace"><el-input v-model="saForm.namespace" placeholder="default" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="saDlgVisible = false">{{ $t('common.cancel') }}</el-button>
        <el-button type="primary" @click="createSa">{{ $t('common.save') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { Odometer, OfficeBuilding, Box, Connection, Setting, Coin, Key, Back, ShoppingBag, Search as SearchIcon, TrendCharts } from '@element-plus/icons-vue'
import api from '../api'
import i18n from '../i18n'
import K8sShell from '../components/K8sShell.vue'
import MetricChart from '../components/MetricChart.vue'
import { ElMessage, ElMessageBox } from 'element-plus'

const { t } = i18n.global
const route = useRoute()
const id = route.params.id
const P = `/k8s/clusters/${id}`

const cluster = ref(null)
const summary = ref(null)
const usage = ref(null)
const usagePods = ref([])
const events = ref([])
const rows = ref([])
const namespaces = ref([])
const ns = ref('')
const active = ref('overview')
const loading = ref(false)

const fmtTime = v => (v ? String(v).replace('T', ' ').slice(0, 19) : '-')
const roleText = r => ({ admin: t('k8s.roleAdmin'), user: t('k8s.roleUser'), viewer: t('k8s.roleViewer') }[r] || '-')
const canOp = computed(() => !!cluster.value && cluster.value.my_role !== 'viewer')
const namespacedActive = computed(() => !['nodes', 'pvs', 'storageclasses', 'overview'].includes(active.value))
const nsParams = () => (ns.value ? { params: { namespace: ns.value } } : {})

// ---- 各区块表格列定义（label 为 i18n key 或字面量） ----
const statusTag = v => (v === 'Ready' || v === 'Bound' || v === 'Available' || v === 'Active' || v === 'Running' ? 'success' : v === 'Pending' ? 'warning' : 'danger')
const colDefs = {
  nodes: [
    { prop: 'name', w: 160 }, { prop: 'roles', w: 140 }, { prop: 'internal_ip', label: 'IP', w: 130 },
    { prop: 'status', tag: row => statusTag(row.status), w: 100, align: 'center' }, { prop: 'version', w: 110 },
    { prop: 'cpu', label: 'k8s.cpu', w: 90, align: 'center' }, { prop: 'memory', label: 'k8s.memory', w: 100, align: 'center' },
  ],
  namespaces: [{ prop: 'name', w: 200 }],
  pods: [
    { prop: 'name', w: 180 }, { prop: 'namespace', label: 'Namespace', w: 110 },
    { prop: 'status', tag: row => statusTag(row.status), w: 130 },
    { prop: 'node', w: 130 }, { prop: 'ip', label: 'IP', w: 120 },
    { prop: 'cpu', label: 'k8s.cpu', w: 80, align: 'center' }, { prop: 'memory', label: 'k8s.memory', w: 90, align: 'center' },
    { prop: 'restarts', label: 'k8s.restarts', w: 90, align: 'center' },
  ],
  helmreleases: [
    { prop: 'name', w: 150 }, { prop: 'namespace', label: 'Namespace', w: 110 },
    { prop: 'chart', label: 'Chart', w: 150 }, { prop: 'version', label: 'k8s.chartVer', w: 100 },
    { prop: 'revision', label: 'k8s.revision', w: 90, align: 'center' },
    { prop: 'status', tag: row => (row.status === 'deployed' ? 'success' : row.status === 'failed' ? 'danger' : 'warning'), w: 130, align: 'center' },
    { prop: 'updated_at', label: 'k8s.age', w: 160 },
  ],
  deployments: [
    { prop: 'name', w: 180 }, { prop: 'namespace', label: 'Namespace', w: 110 },
    { prop: 'replicas', label: 'k8s.replicas', w: 100, align: 'center', text: row => `${row.ready}/${row.replicas}` },
  ],
  daemonsets: [
    { prop: 'name', w: 180 }, { prop: 'namespace', label: 'Namespace', w: 110 },
    { prop: 'desired', label: 'k8s.readyDesired', w: 100, align: 'center', text: row => `${row.ready}/${row.desired}` },
    { prop: 'age', label: 'k8s.age', w: 160 },
  ],
  statefulsets: [
    { prop: 'name', w: 180 }, { prop: 'namespace', label: 'Namespace', w: 110 },
    { prop: 'desired', label: 'k8s.readyDesired', w: 100, align: 'center', text: row => `${row.ready}/${row.desired}` },
    { prop: 'age', label: 'k8s.age', w: 160 },
  ],
  jobs: [
    { prop: 'name', w: 180 }, { prop: 'namespace', label: 'Namespace', w: 110 },
    { prop: 'status', tag: row => (row.status === 'Running' ? 'warning' : row.status === 'Succeeded' ? 'success' : 'danger'), w: 110, align: 'center' },
    { prop: 'desired', label: 'k8s.readyDesired', w: 100, align: 'center', text: row => `${row.ready}/${row.desired}` },
    { prop: 'age', label: 'k8s.age', w: 160 },
  ],
  cronjobs: [
    { prop: 'name', w: 160 }, { prop: 'namespace', label: 'Namespace', w: 110 },
    { prop: 'schedule', w: 130 }, { prop: 'active', label: 'k8s.cronActive', w: 70, align: 'center' },
    { prop: 'suspend', label: 'k8s.cronSuspend', w: 100, align: 'center', tag: row => (row.suspend ? 'warning' : 'success'), text: row => (row.suspend ? 'Suspended' : 'Active') },
  ],
  services: [
    { prop: 'name', w: 170 }, { prop: 'namespace', label: 'Namespace', w: 110 },
    { prop: 'type', label: 'k8s.svcType', w: 110 },
    { prop: 'cluster_ip', label: 'Cluster IP', w: 130 }, { prop: 'ports', label: 'k8s.ports', w: 120 },
    { prop: 'age', label: 'k8s.age', w: 160 },
  ],
  ingresses: [
    { prop: 'name', w: 170 }, { prop: 'namespace', label: 'Namespace', w: 110 },
    { prop: 'hosts', label: 'k8s.hosts', w: 220 }, { prop: 'age', label: 'k8s.age', w: 160 },
  ],
  configmaps: [
    { prop: 'name', w: 200 }, { prop: 'namespace', label: 'Namespace', w: 130 },
    { prop: 'data_keys', label: 'monitor.dataKeys', w: 100, align: 'center' },
    { prop: 'age', label: 'k8s.age', w: 160 },
  ],
  secrets: [
    { prop: 'name', w: 200 }, { prop: 'namespace', label: 'Namespace', w: 130 },
    { prop: 'age', label: 'k8s.age', w: 160 },
  ],
  pvcs: [
    { prop: 'name', w: 170 }, { prop: 'namespace', label: 'Namespace', w: 110 },
    { prop: 'phase', tag: row => statusTag(row.phase), w: 100, align: 'center' },
    { prop: 'capacity', label: 'k8s.capacity', w: 100 }, { prop: 'storage_class', label: 'StorageClass', w: 140 },
    { prop: 'age', label: 'k8s.age', w: 160 },
  ],
  pvs: [
    { prop: 'name', w: 180 },
    { prop: 'phase', tag: row => statusTag(row.phase), w: 100, align: 'center' },
    { prop: 'capacity', label: 'k8s.capacity', w: 100 }, { prop: 'claim', label: 'k8s.claim', w: 180 },
    { prop: 'storage_class', label: 'StorageClass', w: 140 }, { prop: 'age', label: 'k8s.age', w: 160 },
  ],
  storageclasses: [
    { prop: 'name', w: 200 }, { prop: 'provisioner', label: 'k8s.provisioner', w: 240 },
    { prop: 'age', label: 'k8s.age', w: 160 },
  ],
  serviceaccounts: [
    { prop: 'name', w: 200 }, { prop: 'namespace', label: 'Namespace', w: 160 },
    { prop: 'secrets', label: 'Secrets', w: 100, align: 'center' },
    { prop: 'age', label: 'k8s.age', w: 160 },
  ],
}
const cols = computed(() => (colDefs[active.value] || []).map(c => ({ ...c, headerHtml: c.headerHtml })))
const colLabel = col => (col.label ? (col.label.includes('.') ? t(col.label) : col.label) : col.prop.replace(/_/g, ' ').toUpperCase())
const sectionTitle = computed(() => {
  const names = {
    nodes: 'Nodes', namespaces: t('k8s.namespaces'), pods: 'Pods', deployments: 'Deployments',
    daemonsets: 'DaemonSets', statefulsets: 'StatefulSets', jobs: 'Jobs', cronjobs: 'CronJobs',
    services: 'Services', ingresses: 'Ingresses', configmaps: 'ConfigMaps', secrets: 'Secrets',
    pvcs: 'PVCs', pvs: 'PVs', storageclasses: 'StorageClasses', serviceaccounts: 'ServiceAccounts',
    helmreleases: 'Helm Releases',
  }
  return names[active.value] || active.value
})
// 各区块对应的 YAML 查看资源类型（helmreleases 无对应单体路径，不提供）
const kindOf = {
  nodes: 'node', pods: 'pod', deployments: 'deployment', daemonsets: 'daemonset',
  statefulsets: 'statefulset', jobs: 'job', cronjobs: 'cronjob', services: 'service',
  ingresses: 'ingress', configmaps: 'configmap', secrets: 'secret', pvcs: 'pvc', pvs: 'pv',
  storageclasses: 'storageclass', serviceaccounts: 'serviceaccount',
}
const opWidth = computed(() => (active.value === 'pods' ? 270 : active.value === 'deployments' || active.value === 'statefulsets' ? 160 : active.value === 'cronjobs' ? 150 : 90))
const delConfirmKey = computed(() => ({
  configmaps: 'monitor.configDelConfirm', secrets: 'monitor.configDelConfirm', serviceaccounts: 'k8s.podDelConfirm',
}[active.value] || ''))

// ---- 工具栏：全文搜索 / 批量选择 / 导出 ----
const search = ref('')
const selectedRows = ref([])
const creatable = new Set(['deployments', 'daemonsets', 'statefulsets', 'jobs', 'cronjobs',
  'services', 'ingresses', 'pvcs', 'configmaps', 'secrets', 'serviceaccounts'])
const deletable = new Set(['pods', 'deployments', 'daemonsets', 'statefulsets', 'jobs', 'cronjobs',
  'services', 'ingresses', 'configmaps', 'secrets', 'pvcs', 'serviceaccounts'])
const filteredRows = computed(() => {
  const q = search.value.trim().toLowerCase()
  if (!q) return rows.value
  return rows.value.filter(r => Object.values(r).some(v => v != null && String(v).toLowerCase().includes(q)))
})

const exportCSV = () => {
  const esc = v => `"${String(v ?? '').replace(/"/g, '""')}"`
  const lines = [cols.value.map(c => esc(colLabel(c))).join(',')]
  for (const r of filteredRows.value) {
    lines.push(cols.value.map(c => esc(c.text ? c.text(r) : r[c.prop])).join(','))
  }
  const blob = new Blob(['\ufeff' + lines.join('\n')], { type: 'text/csv;charset=utf-8' })
  const a = document.createElement('a')
  a.href = URL.createObjectURL(blob)
  a.download = `${cluster.value?.name || 'cluster'}-${active.value}.csv`
  a.click()
  URL.revokeObjectURL(a.href)
}

const batchDelete = async () => {
  try {
    await ElMessageBox.confirm(t('k8s.batchDelConfirm', { n: selectedRows.value.length }), t('k8s.batchDelete'), { type: 'warning' })
  } catch { return }
  await Promise.allSettled(selectedRows.value.map(r =>
    api.post(`${P}/delete`, { kind: kindOf[active.value], namespace: r.namespace || '', name: r.name })))
  ElMessage.success(t('common.success'))
  selectedRows.value = []
  load()
}

// ---- YAML 创建 ----
const createVisible = ref(false)
const createKind = ref('')
const createText = ref('')
const createSaving = ref(false)
const yamlTpl = (kind, ns) => `apiVersion: apps/v1
kind: Deployment
metadata:
  name: example-app
  namespace: ${ns || 'default'}
spec:
  replicas: 1
  selector:
    matchLabels:
      app: example-app
  template:
    metadata:
      labels:
        app: example-app
    spec:
      containers:
        - name: example-app
          image: nginx:1.27
          ports:
            - containerPort: 80
`
const YAML_TEMPLATES = {
  deployments: yamlTpl,
  daemonsets: ns => `apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: example-agent
  namespace: ${ns || 'default'}
spec:
  selector:
    matchLabels:
      app: example-agent
  template:
    metadata:
      labels:
        app: example-agent
    spec:
      containers:
        - name: example-agent
          image: busybox:1.36
          command: ["sleep", "3600"]
`,
  statefulsets: ns => `apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: example-db
  namespace: ${ns || 'default'}
spec:
  serviceName: example-db
  replicas: 1
  selector:
    matchLabels:
      app: example-db
  template:
    metadata:
      labels:
        app: example-db
    spec:
      containers:
        - name: example-db
          image: busybox:1.36
          command: ["sleep", "3600"]
`,
  jobs: ns => `apiVersion: batch/v1
kind: Job
metadata:
  name: example-job
  namespace: ${ns || 'default'}
spec:
  template:
    spec:
      restartPolicy: Never
      containers:
        - name: example-job
          image: busybox:1.36
          command: ["echo", "hello"]
`,
  cronjobs: ns => `apiVersion: batch/v1
kind: CronJob
metadata:
  name: example-cron
  namespace: ${ns || 'default'}
spec:
  schedule: "*/5 * * * *"
  jobTemplate:
    spec:
      template:
        spec:
          restartPolicy: OnFailure
          containers:
            - name: example-cron
              image: busybox:1.36
              command: ["echo", "hello"]
`,
  services: ns => `apiVersion: v1
kind: Service
metadata:
  name: example-svc
  namespace: ${ns || 'default'}
spec:
  type: ClusterIP
  selector:
    app: example-app
  ports:
    - port: 80
      targetPort: 8080
`,
  ingresses: ns => `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: example-ing
  namespace: ${ns || 'default'}
spec:
  rules:
    - host: app.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: example-svc
                port:
                  number: 80
`,
  pvcs: ns => `apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: example-pvc
  namespace: ${ns || 'default'}
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
`,
  configmaps: ns => `apiVersion: v1
kind: ConfigMap
metadata:
  name: example-cm
  namespace: ${ns || 'default'}
data:
  key1: value1
`,
  secrets: ns => `apiVersion: v1
kind: Secret
metadata:
  name: example-secret
  namespace: ${ns || 'default'}
type: Opaque
stringData:
  username: admin
`,
  serviceaccounts: ns => `apiVersion: v1
kind: ServiceAccount
metadata:
  name: example-sa
  namespace: ${ns || 'default'}
`,
}
const openCreateYAML = () => {
  createKind.value = active.value
  createText.value = YAML_TEMPLATES[active.value](ns.value)
  createVisible.value = true
}
const submitCreate = async () => {
  createSaving.value = true
  try {
    await api.post(`${P}/yaml`, { kind: kindOf[active.value], namespace: ns.value || '', yaml: createText.value })
    ElMessage.success(t('common.success'))
    createVisible.value = false
    load()
  } finally { createSaving.value = false }
}

// ---- 集群资源概况 ----
const usagePercent = computed(() => {
  const u = usage.value
  if (!u || !u.cpu_capacity_m || !u.mem_capacity_mi) return { cpu: 0, mem: 0 }
  return {
    cpu: Math.min(100, Math.round((u.cpu_used_m / u.cpu_capacity_m) * 100)),
    mem: Math.min(100, Math.round((u.mem_used_mi / u.mem_capacity_mi) * 100)),
  }
})
const usageColor = p => (p >= 90 ? '#f56c6c' : p >= 70 ? '#e6a23c' : '#67c23a')
const fmtCoresV = m => (m ? (m / 1000).toFixed(m % 1000 === 0 ? 1 : 2) : '0')
const fmtCores = m => (m ? `${fmtCoresV(m)} Core` : '-')
const fmtReqLimC = (req, lim) => (lim ? `${fmtCoresV(req)} / ${fmtCoresV(lim)}` : fmtCoresV(req))
const fmtMem = mi => { const v = Number(mi) || 0; return v >= 1024 ? `${(v / 1024).toFixed(1)}Gi` : `${Math.round(v)}Mi` }
const fmtReqLim = (req, lim) => (lim ? `${fmtMem(req)} / ${fmtMem(lim)}` : fmtMem(req))

// ---- 容量规划 ----
const cap = ref(null)
const capDays = ref(30)
const capRange = computed(() => [Date.now() - capDays.value * 86400000, Date.now()])

// 图表统一使用 MetricChart 组件：集群曲线以最新集群容量为 100% 的百分比量纲
const capPctPoints = kind => {
  const pts = cap.value?.points || []
  const denom = kind === 'cpu' ? cap.value?.forecast?.cpu_capacity_m : cap.value?.forecast?.mem_capacity_mi
  if (!denom) return []
  return pts.map(p => ({
    t: p.t,
    v: Math.min(100, ((kind === 'cpu' ? p.cpu_used_m : p.mem_used_mi) / denom) * 100),
  }))
}
const capLastPct = kind => {
  const arr = capPctPoints(kind)
  return arr.length ? arr[arr.length - 1].v.toFixed(1) : '0.0'
}
const capSlopeText = (slope, kind) => {
  if (slope == null || slope <= 0) return t('k8s.capStable')
  const perMonth = slope * 30
  return kind === 'cpu' ? `+${(perMonth / 1000).toFixed(2)} Core/${t('k8s.capMonth')}` : `+${fmtMem(perMonth)}/${t('k8s.capMonth')}`
}
const capDaysLeftText = (days, kind) => {
  if (days == null) return t('k8s.capNoExhaust')
  if (days <= 0) return t('k8s.capExhausted')
  const label = kind === 'cpu' ? 'CPU' : t('k8s.clusterMem')
  return `${label} ~${days < 60 ? Math.round(days) + ' ' + t('k8s.capDays') : (days / 30).toFixed(1) + ' ' + t('k8s.capMonths')} ${t('k8s.capExhaustIn')}`
}
const capDaysLeftColor = d => (d == null ? '#67c23a' : d < 90 ? '#f56c6c' : d < 180 ? '#e6a23c' : '#67c23a')

// ---- Pod 级容量趋势 ----
const capPods = ref([])
const capPodSel = ref('')
const capPodTrend = computed(() => capPods.value.find(p => p.namespace + '/' + p.name === capPodSel.value) || null)
const loadCapacity = async () => {
  cap.value = await api.get(`${P}/capacity/history`, { params: { days: capDays.value } }).catch(() => null)
  const r = await api.get(`${P}/capacity/pods`, { params: { days: capDays.value } }).catch(() => null)
  capPods.value = r?.pods || []
  if (capPods.value.length && !capPods.value.find(p => p.namespace + '/' + p.name === capPodSel.value)) {
    capPodSel.value = capPods.value[0].namespace + '/' + capPods.value[0].name
  }
}
// Pod 级趋势图（绝对量纲：CPU millicore / 内存 Mi，Y 轴按自身峰值缩放）
const podPoints = field => (capPodTrend.value?.points || []).map(p => ({ t: p.t, v: p[field] }))
const podYMax = field => {
  let m = 0
  for (const p of podPoints(field)) m = Math.max(m, Number(p.v) || 0)
  return m * 1.1 || 1
}
const capPodSlopeText = computed(() => {
  const t = capPodTrend.value
  if (!t) return ''
  const c = t.slope_cpu_m_per_day > 0 ? `CPU +${(t.slope_cpu_m_per_day * 30 / 1000).toFixed(2)} Core/${t('k8s.capMonth')}` : `CPU ${t('k8s.capStable')}`
  const m = t.slope_mem_mi_per_day > 0 ? `${t('k8s.memory')} +${fmtMem(t.slope_mem_mi_per_day * 30)}/${t('k8s.capMonth')}` : `${t('k8s.memory')} ${t('k8s.capStable')}`
  return `${c} · ${m}`
})
const fmtGi = mi => (mi ? `${(mi / 1024).toFixed(1)} Gi` : '-')

// ---- 概览卡片 ----
const cards = computed(() => {
  const s = summary.value
  if (!s) return []
  return [
    { key: 'nodes', label: `${t('k8s.nodes')} ${t('k8s.online')}`, text: `${s.nodes_ready}/${s.nodes}`, color: '#e6a23c', go: 'nodes' },
    { key: 'pods', label: 'Pods', text: `${s.running_pods}/${s.pods}`, color: '#409eff', go: 'pods' },
    { key: 'namespaces', label: t('k8s.namespaces'), text: s.namespaces, color: '#67c23a', go: 'namespaces' },
    { key: 'deployments', label: 'Deployments', text: s.deployments, color: '#f56c6c', go: 'deployments' },
    { key: 'daemonsets', label: 'DaemonSets', text: s.daemonsets, color: '#9c27b0', go: 'daemonsets' },
    { key: 'statefulsets', label: 'StatefulSets', text: s.statefulsets, color: '#ff9800', go: 'statefulsets' },
    { key: 'jobs', label: 'Jobs', text: s.jobs, color: '#00bcd4', go: 'jobs' },
    { key: 'cronjobs', label: 'CronJobs', text: s.cronjobs, color: '#3f51b5', go: 'cronjobs' },
    { key: 'services', label: 'Services', text: s.services, color: '#8bc34a', go: 'services' },
    { key: 'ingresses', label: 'Ingresses', text: s.ingresses, color: '#e91e63', go: 'ingresses' },
    { key: 'pvs', label: 'PVs', text: s.pvs, color: '#795548', go: 'pvs' },
    { key: 'pvcs', label: 'PVCs', text: s.pvcs, color: '#607d8b', go: 'pvcs' },
    { key: 'configmaps', label: 'ConfigMaps', text: s.configmaps, color: '#cddc39', go: 'configmaps' },
    { key: 'secrets', label: 'Secrets', text: s.secrets, color: '#f44336', go: 'secrets' },
    { key: 'serviceaccounts', label: 'ServiceAccounts', text: s.serviceaccounts, color: '#009688', go: 'serviceaccounts' },
  ]
})

// ---- 数据加载 ----
// metrics-server 用量合并进列表；未安装 metrics-server 时列为 '-'
const mergeMetrics = (list, metrics, keyFn) => {
  const m = Object.fromEntries((metrics || []).map(x => [keyFn(x), x]))
  return list.map(r => ({
    ...r,
    cpu: m[keyFn(r)] ? `${fmtCoresV(m[keyFn(r)].cpu_m)} Core` : '-',
    memory: m[keyFn(r)] ? `${m[keyFn(r)].mem_mi}Mi` : '-',
  }))
}
const loaders = {
  nodes: async () => {
    const [r, metrics] = await Promise.all([
      api.get(`${P}/nodes`), api.get(`${P}/nodemetrics`).catch(() => []),
    ])
    // 后端返回 {cluster, nodes} 包裹结构
    return mergeMetrics(r.nodes || [], metrics, x => x.name)
  },
  namespaces: async () => (await api.get(`${P}/namespaces`)).map(n => ({ name: n })),
  pods: async () => {
    const [list, metrics] = await Promise.all([
      api.get(`${P}/pods`, nsParams()),
      api.get(`${P}/podmetrics`, nsParams()).catch(() => []),
    ])
    return mergeMetrics(list, metrics, x => `${x.namespace}/${x.name}`)
  },
  deployments: () => api.get(`${P}/deployments`, nsParams()),
  daemonsets: () => api.get(`${P}/daemonsets`, nsParams()),
  statefulsets: () => api.get(`${P}/statefulsets`, nsParams()),
  jobs: () => api.get(`${P}/jobs`, nsParams()),
  cronjobs: () => api.get(`${P}/cronjobs`, nsParams()),
  services: () => api.get(`${P}/services`, nsParams()),
  ingresses: () => api.get(`${P}/ingresses`, nsParams()),
  configmaps: () => api.get(`${P}/configmaps`, nsParams()),
  secrets: () => api.get(`${P}/secrets`, nsParams()),
  pvcs: () => api.get(`${P}/pvcs`, nsParams()),
  pvs: () => api.get(`${P}/pvs`),
  storageclasses: () => api.get(`${P}/storageclasses`),
  serviceaccounts: () => api.get(`${P}/serviceaccounts`, nsParams()),
  helmreleases: () => api.get(`${P}/helmreleases`, nsParams()),
}

const load = async () => {
  loading.value = true
  try {
    if (active.value === 'capacity') { await loadCapacity(); return }
    if (active.value === 'overview') {
      const [r, usageData] = await Promise.all([
        api.get(`${P}/summary`),
        api.get(`${P}/usage`).catch(() => null),
      ])
      summary.value = r.summary
      usage.value = usageData
      usagePods.value = (usageData?.pods || []).slice().sort((a, b) => b.cpu_m - a.cpu_m)
      events.value = await api.get(`${P}/events`)
    } else {
      rows.value = await loaders[active.value]()
    }
  } finally { loading.value = false }
}

watch(active, () => load())
watch(ns, () => { if (active.value !== 'overview') load() })

// ---- Pods 操作 ----
const logVisible = ref(false)
const logTitle = ref('')
const logText = ref('')
const showLog = async row => {
  const r = await api.get(`${P}/podlog`, { params: { namespace: row.namespace, pod: row.name } })
  logTitle.value = `${row.namespace}/${row.name}`
  logText.value = r.log || t('k8s.noLog')
  logVisible.value = true
}
const shellDrawer = ref(false)
const shellPod = reactive({ namespace: '', name: '' })
const openShell = row => {
  shellPod.namespace = row.namespace
  shellPod.name = row.name
  shellDrawer.value = true
}
const deletePod = async row => {
  await api.delete(`${P}/pods/${row.namespace}/${row.name}`)
  ElMessage.success(t('common.success'))
  load()
}

// ---- YAML 查看 / 编辑 / 下载 ----
const yamlVisible = ref(false)
const yamlTitle = ref('')
const yamlText = ref('')
const yamlEditing = ref(false)
const yamlEdit = ref('')
const yamlSaving = ref(false)
const yamlYamlRow = ref({})
const showYAML = async row => {
  yamlYamlRow.value = row
  const r = await api.get(`${P}/yaml`, { params: { kind: kindOf[active.value], namespace: row.namespace || '', name: row.name } })
  yamlTitle.value = `YAML · ${row.namespace ? row.namespace + '/' : ''}${row.name}`
  yamlText.value = r.yaml || ''
  yamlEditing.value = false
  yamlVisible.value = true
}
const startYAMLEdit = () => {
  yamlEdit.value = yamlText.value
  yamlEditing.value = true
}
const saveYAML = async () => {
  yamlSaving.value = true
  try {
    await api.put(`${P}/yaml`, {
      kind: kindOf[active.value], namespace: yamlYamlRow.value.namespace || '', name: yamlYamlRow.value.name, yaml: yamlEdit.value,
    })
    ElMessage.success(t('common.success'))
    yamlEditing.value = false
    yamlVisible.value = false
    load()
  } finally { yamlSaving.value = false }
}
const downloadYAML = () => {
  const blob = new Blob([yamlText.value], { type: 'text/yaml' })
  const a = document.createElement('a')
  a.href = URL.createObjectURL(blob)
  a.download = (yamlYamlRow.value.name || 'resource') + '.yaml'
  a.click()
  URL.revokeObjectURL(a.href)
}

// ---- 实时日志（WS 流式跟随） ----
const followVisible = ref(false)
const followTitle = ref('')
const followStatus = ref('')
const followBox = ref(null)
let followWs = null
let followRow = null
const openFollow = row => {
  followRow = row
  followTitle.value = `Logs · ${row.namespace}/${row.name}`
  followStatus.value = t('k8s.shellConnecting')
  followVisible.value = true
  nextTick(() => connectFollow())
}
const connectFollow = () => {
  const box = followBox.value
  if (!box || !followRow) return
  if (followWs) { followWs.onclose = null; followWs.close() }
  box.textContent = ''
  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  const q = new URLSearchParams({
    namespace: followRow.namespace, pod: followRow.name,
    container: followRow.container || '', tail: '100',
    token: localStorage.getItem('token') || '',
  })
  followWs = new WebSocket(`${proto}://${location.host}/api/ws/k8s/logs/${id}?` + q)
  followWs.onopen = () => { followStatus.value = t('k8s.shellConnected') }
  followWs.onmessage = ev => {
    const data = typeof ev.data === 'string' ? ev.data : new TextDecoder().decode(new Uint8Array(ev.data))
    box.appendChild(document.createTextNode(data))
    // 长时间跟随会无限堆积 DOM 节点：超过 500 个文本块时丢弃最早的
    while (box.childNodes.length > 500) box.removeChild(box.firstChild)
    box.scrollTop = box.scrollHeight
  }
  followWs.onclose = () => { followStatus.value = t('k8s.shellClosed') }
  followWs.onerror = () => { followStatus.value = t('k8s.shellError') }
}
const clearFollow = () => connectFollow()
onBeforeUnmount(() => { if (followWs) followWs.close() })

// ---- Deployment / StatefulSet 伸缩 ----
const scaleWorkload = async row => {
  try {
    const { value } = await ElMessageBox.prompt(t('k8s.scaleTip'), `${t('k8s.scale')} · ${row.namespace}/${row.name}`, {
      inputValue: String(row.replicas), inputPattern: /^\d+$/, inputErrorMessage: t('k8s.scaleTip'),
      confirmButtonText: t('common.save'), cancelButtonText: t('common.cancel'),
    })
    await api.post(`${P}/${active.value}/${row.namespace}/${row.name}/scale`, { replicas: parseInt(value, 10) })
    ElMessage.success(t('common.success'))
    load()
  } catch { /* 取消 */ }
}

// ---- Deployments ----
const restartDeployment = async row => {
  await api.post(`${P}/deployments/${row.namespace}/${row.name}/restart`)
  ElMessage.success(t('common.success'))
  load()
}

// ---- CronJobs / SA / Config 删除与创建 ----
const deleteCron = async row => {
  await api.delete(`${P}/cronjobs/${row.namespace}/${row.name}`)
  ElMessage.success(t('common.success'))
  load()
}
const toggleCron = async row => {
  await api.put(`${P}/cronjobs/${row.namespace}/${row.name}/suspend`, { suspend: !row.suspend })
  ElMessage.success(t('common.success'))
  load()
}
const cronDlgVisible = ref(false)
const cronForm = reactive({ name: '', namespace: '', schedule: '', command: '' })
const openCronDlg = () => {
  Object.assign(cronForm, { name: '', namespace: ns.value || '', schedule: '', command: '' })
  cronDlgVisible.value = true
}
const createCron = async () => {
  if (!cronForm.name || !cronForm.schedule || !cronForm.command) { ElMessage.warning(t('k8s.nameServerRequired')); return }
  await api.post(`${P}/cronjobs`, cronForm)
  ElMessage.success(t('common.success'))
  cronDlgVisible.value = false
  load()
}

const saDlgVisible = ref(false)
const saForm = reactive({ name: '', namespace: '' })
const openSaDlg = () => {
  Object.assign(saForm, { name: '', namespace: ns.value || '' })
  saDlgVisible.value = true
}
const createSa = async () => {
  if (!saForm.name) { ElMessage.warning(t('k8s.saName')); return }
  await api.post(`${P}/serviceaccounts/${saForm.namespace || 'default'}`, { name: saForm.name })
  ElMessage.success(t('common.success'))
  saDlgVisible.value = false
  load()
}

const deleteResource = async row => {
  const kind = { configmaps: 'configmaps', secrets: 'secrets', serviceaccounts: 'serviceaccounts' }[active.value]
  await api.delete(`${P}/${kind}/${row.namespace}/${row.name}`)
  ElMessage.success(t('common.success'))
  load()
}

onMounted(async () => {
  const clusters = await api.get('/k8s/clusters')
  cluster.value = clusters.find(c => String(c.id) === String(id)) || null
  namespaces.value = await api.get(`${P}/namespaces`).catch(() => [])
  load()
})
</script>

<style scoped>
.km-page { display: flex; height: 100vh; background: #f0f2f5; }
.km-side {
  width: 210px; flex-shrink: 0; background: #1d2935; color: #a7b1c2;
  display: flex; flex-direction: column; overflow-y: auto;
}
.km-brand { color: #fff; font-size: 17px; font-weight: 700; padding: 16px 18px 4px; }
.km-brand span { color: #409eff; }
.km-cluster { color: #6b7a8c; font-size: 12px; padding: 0 18px 10px; border-bottom: 1px solid #2a3947; }
.km-menu { border-right: none; flex: 1; }
.km-menu :deep(.el-menu-item.is-active) { background: #409eff !important; }
.km-main { flex: 1; display: flex; flex-direction: column; min-width: 0; }
.km-top {
  background: #fff; padding: 10px 16px; display: flex; gap: 10px; align-items: center;
  border-bottom: 1px solid #e4e7ed; box-shadow: 0 1px 4px rgba(0, 21, 41, .06);
}
.km-body { flex: 1; overflow: auto; padding: 16px; }
.km-h4 { margin: 0 0 10px; color: #303133; }
.km-toolbar { display: flex; align-items: center; gap: 8px; margin-bottom: 10px; }
.km-usage { display: grid; grid-template-columns: repeat(auto-fit, minmax(280px, 1fr)); gap: 12px; margin-bottom: 18px; }
.km-usage-card { background: #fff; border-radius: 6px; padding: 14px 16px; box-shadow: 0 1px 3px rgba(0,21,41,.08); }
.km-usage-head { font-size: 13px; font-weight: 600; color: #303133; margin-bottom: 10px; }
.km-usage-sub { font-size: 12px; color: #909399; margin-top: 6px; }
.cap-dot { display: inline-block; width: 10px; height: 3px; vertical-align: middle; margin-right: 4px; }
.km-cards { display: grid; grid-template-columns: repeat(auto-fill, minmax(150px, 1fr)); gap: 12px; margin-bottom: 18px; }
.km-card {
  background: #fff; border-radius: 6px; padding: 14px 16px; cursor: pointer;
  border-left: 3px solid var(--c); box-shadow: 0 1px 3px rgba(0, 21, 41, .08);
  transition: transform .15s;
}
.km-card:hover { transform: translateY(-2px); }
.km-card-num { font-size: 24px; font-weight: 700; color: var(--c); }
.km-card-label { font-size: 12px; color: #909399; margin-top: 2px; }
:deep(.el-drawer__body) { padding: 0; height: 100%; background: #1e2a35; }
.follow-box {
  background: #1e2a35; color: #d8e4f0; padding: 12px; border-radius: 6px;
  height: calc(100vh - 140px); overflow: auto; font-size: 12px; line-height: 1.6;
  white-space: pre-wrap; word-break: break-all;
}
</style>
