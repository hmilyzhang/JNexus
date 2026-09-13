<!-- JNexus Ops Platform — By JJ Zhang, Version 1.0 -->
<template>
  <div>
    <div style="display:flex; align-items:center; gap:10px; margin-bottom:10px">
      <span style="flex:1"></span>
      <el-button size="small" type="warning" plain @click="$router.push('/screen')">{{ $t('monitor.bigScreen') }} →</el-button>
    </div>
    <el-tabs v-model="activeTab">
      <!-- Tab 1: CMD monitoring (CPU / memory / disk) -->
      <el-tab-pane :label="$t('monitor.tabCmd')" name="cmd">
        <el-card>
          <template #header>
            <div style="display:flex; align-items:center; gap:10px">
              <span style="flex:1">{{ $t('monitor.hostRes') }}</span>
              <el-select v-model="groupFilter" size="small" style="width:170px" clearable :placeholder="$t('monitor.allGroups')">
                <el-option v-for="g in groupOptions" :key="g" :label="g" :value="g" />
              </el-select>
              <el-input v-model="hostKw" size="small" style="width:200px" clearable :placeholder="$t('tasks.searchOutput')" />
            </div>
          </template>
          <el-table :data="filteredHosts" v-loading="hostsLoading" size="small" border
                    @row-click="openHostTrend" style="cursor:pointer">
            <el-table-column prop="name" :label="$t('monitor.targetHost')" min-width="150">
              <template #default="{ row }">{{ row.name }}<div style="color:#909399; font-size:12px">{{ row.ip }}</div></template>
            </el-table-column>
            <el-table-column :label="$t('monitor.group')" min-width="110">
              <template #default="{ row }">{{ row.group || '-' }}</template>
            </el-table-column>
            <el-table-column :label="$t('monitor.cpu')" min-width="160" prop="cpu" sortable>
              <template #default="{ row }">
                <template v-if="row.collected_at">
                  <el-progress :percentage="row.cpu || 0" :color="barColor(row.cpu)" :stroke-width="10" />
                </template>
                <span v-else style="color:#c0c4cc">{{ $t('monitor.noData') }}</span>
              </template>
            </el-table-column>
            <el-table-column :label="$t('monitor.mem')" min-width="160" prop="mem" sortable>
              <template #default="{ row }">
                <template v-if="row.collected_at">
                  <el-progress :percentage="row.mem || 0" :color="barColor(row.mem)" :stroke-width="10" />
                </template>
                <span v-else style="color:#c0c4cc">{{ $t('monitor.noData') }}</span>
              </template>
            </el-table-column>
            <el-table-column :label="$t('monitor.disk')" min-width="160" prop="disk" sortable>
              <template #default="{ row }">
                <template v-if="row.collected_at">
                  <el-progress :percentage="row.disk || 0" :color="barColor(row.disk)" :stroke-width="10" />
                </template>
                <span v-else style="color:#c0c4cc">{{ $t('monitor.noData') }}</span>
              </template>
            </el-table-column>
            <el-table-column :label="$t('monitor.lastCheck')" width="150">
              <template #default="{ row }">{{ fmtTime(row.collected_at) }}</template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-tab-pane>

      <!-- Tab 2: application monitoring -->
      <el-tab-pane :label="$t('monitor.tabApp')" name="app">
        <el-card>
          <template #header>
            <div style="display:flex; align-items:center; gap:10px">
              <span style="flex:1">{{ $t('monitor.appMon') }}</span>
              <el-button size="small" :loading="loading" @click="load">{{ $t('common.refresh') }}</el-button>
              <el-button size="small" type="primary" @click="openDlg()">{{ $t('monitor.addMon') }}</el-button>
            </div>
          </template>
          <div v-if="!monitors.length" style="color:#909399; padding:12px 0">{{ $t('monitor.noData') }}</div>
          <div v-for="row in monitors" :key="row.monitor.id" class="mon-row">
            <span class="dot" :class="statusClass(row)"></span>
            <div style="flex:1; min-width:0">
              <div style="font-weight:600">
                {{ row.monitor.name }}
                <el-tag size="small" type="info" style="margin-left:6px">{{ typeLabel(row.monitor.type) }}</el-tag>
                <el-tag v-if="!row.monitor.enabled" size="small" type="warning" style="margin-left:6px">{{ $t('monitor.paused') }}</el-tag>
              </div>
              <div style="color:#909399; font-size:12px; white-space:nowrap; overflow:hidden; text-overflow:ellipsis"
                   :title="row.monitor.last_error || ''">
                {{ monitorTarget(row.monitor) }}<span v-if="row.monitor.last_error" style="color:#f56c6c"> — {{ row.monitor.last_error }}</span>
              </div>
            </div>
            <div class="hb">
              <el-tooltip v-for="(s, i) in row.recent || []" :key="i" placement="top"
                          :content="hbTip(s)" :show-after="80">
                <span class="hb-bar" :class="s.status === 'up' ? 'hb-up' : s.status === 'maint' ? 'hb-maint' : 'hb-down'"></span>
              </el-tooltip>
              <span v-if="!(row.recent || []).length" style="color:#c0c4cc; font-size:12px">{{ $t('monitor.notYet') }}</span>
            </div>
            <div class="mon-stats">
              <div class="stat">
                <div class="stat-val">{{ statusText(row) }}</div>
                <div class="stat-lbl">{{ row.monitor.last_resp_ms ? row.monitor.last_resp_ms + 'ms' : '—' }}</div>
              </div>
              <div class="stat">
                <div class="stat-val" :style="{ color: (row.uptime24h ?? 100) < 99 ? '#e6a23c' : '#67c23a' }">
                  {{ row.uptime24h != null ? row.uptime24h + '%' : '—' }}
                </div>
                <div class="stat-lbl">{{ $t('monitor.uptime24h') }}</div>
              </div>
              <div class="stat">
                <div class="stat-val" :style="{ color: (row.uptime_30d ?? 100) < 99 ? '#e6a23c' : '#67c23a' }">
                  {{ row.uptime_30d != null ? row.uptime_30d + '%' : '—' }}
                </div>
                <div class="stat-lbl">{{ $t('monitor.uptime30d') }}</div>
              </div>
            </div>
            <div style="display:flex; gap:4px; align-items:center">
              <el-button size="small" link type="primary" :disabled="!row.monitor.enabled" @click="testNow(row)">{{ $t('monitor.testNow') }}</el-button>
              <el-button size="small" link @click="openDlg(row.monitor, row.channel_ids)">{{ $t('common.edit') }}</el-button>
              <el-button size="small" link @click="togglePause(row)">{{ row.monitor.enabled ? $t('common.disabled') : $t('common.enabled') }}</el-button>
              <el-popconfirm :title="$t('monitor.delConfirm')" @confirm="del(row.monitor)">
                <template #reference><el-button size="small" type="danger" link>{{ $t('common.delete') }}</el-button></template>
              </el-popconfirm>
            </div>
          </div>
        </el-card>
      </el-tab-pane>

      <!-- Tab 3: alert settings (notification channels) -->
      <el-tab-pane :label="$t('monitor.tabAlert')" name="alert">
        <el-card>
          <template #header>
            <div style="display:flex; align-items:center; gap:10px">
              <span style="flex:1">{{ $t('monitor.chTitle') }}</span>
              <el-button size="small" :loading="chLoading" @click="loadChannels">{{ $t('common.refresh') }}</el-button>
              <el-button size="small" type="primary" @click="openChDlg()">{{ $t('monitor.chAdd') }}</el-button>
            </div>
          </template>
          <div style="color:#909399; font-size:12px; margin-bottom:10px">{{ $t('monitor.chTip') }}</div>
          <div v-if="!channels.length" style="color:#909399; padding:8px 0">{{ $t('monitor.chEmpty') }}</div>
          <el-table :data="channels" size="small" border>
            <el-table-column :label="$t('monitor.monName')" min-width="150">
              <template #default="{ row }">
                {{ row.name }}
                <el-tag v-if="!row.enabled" size="small" type="warning" style="margin-left:6px">{{ $t('monitor.paused') }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column :label="$t('monitor.chType')" width="130">
              <template #default="{ row }"><el-tag size="small">{{ chTypeLabel(row.type) }}</el-tag></template>
            </el-table-column>
            <el-table-column :label="$t('monitor.chConfig')" min-width="220">
              <template #default="{ row }"><span class="mono" style="font-size:12px; word-break:break-all">{{ chConfigSummary(row) }}</span></template>
            </el-table-column>
            <el-table-column :label="$t('common.operation')" width="220" fixed="right">
              <template #default="{ row }">
                <el-button size="small" link type="primary" @click="testChannel(row)">{{ $t('monitor.chTest') }}</el-button>
                <el-button size="small" link @click="openChDlg(row)">{{ $t('common.edit') }}</el-button>
                <el-button size="small" link @click="toggleCh(row)">{{ row.enabled ? $t('common.disabled') : $t('common.enabled') }}</el-button>
                <el-popconfirm :title="$t('monitor.chDelConfirm')" @confirm="delChannel(row)">
                  <template #reference><el-button size="small" type="danger" link>{{ $t('common.delete') }}</el-button></template>
                </el-popconfirm>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-tab-pane>

      <!-- Tab 4: alert rules (global) -->
      <el-tab-pane :label="$t('monitor.tabRules')" name="rules">
        <el-card>
          <template #header>
            <div style="display:flex; align-items:center; gap:10px">
              <span style="flex:1">{{ $t('monitor.ruleTitle') }}</span>
            </div>
          </template>
          <el-form label-width="150px" style="max-width:640px">

            <el-form-item :label="$t('monitor.ruleThreshold')">
              <el-input-number v-model="alertRule.grace_sec" :min="0" :max="86400" :disabled="alertRule.mode === 'immediate'" />
              <div style="color:#909399; font-size:12px">{{ $t('monitor.ruleThresholdTip') }}</div>
            </el-form-item>
            <el-form-item :label="$t('monitor.ruleRecovery')">
              <el-switch v-model="alertRule.notify_recovery" />
              <div style="color:#909399; font-size:12px">{{ $t('monitor.ruleRecoveryTip') }}</div>
            </el-form-item>
            <el-form-item>
              <el-button type="primary" :loading="ruleSaving" @click="saveAlertRule">{{ $t('common.save') }}</el-button>
            </el-form-item>
          </el-form>
        </el-card>
        <el-card style="margin-top:16px">
          <template #header>
            <div style="display:flex; align-items:center; gap:10px">
              <span style="flex:1">{{ $t('monitor.cmdLevels') }}</span>
              <el-button size="small" type="primary" :loading="cmdSaving" @click="saveCmdLevels">{{ $t('common.save') }}</el-button>
            </div>
          </template>
          <el-table :data="cmdLevels" size="small" border>
            <el-table-column :label="$t('monitor.level')" width="70" align="center">
              <template #default="{ row }">
                <el-tag size="small" :type="row.level === 'P1' ? 'danger' : row.level === 'P2' ? 'warning' : 'info'">{{ row.level }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column :label="$t('monitor.cpu')" min-width="120">
              <template #default="{ row }">
                <el-input-number v-model="row.cpu" size="small" :min="0" :max="100" />
              </template>
            </el-table-column>
            <el-table-column :label="$t('monitor.mem')" min-width="120">
              <template #default="{ row }">
                <el-input-number v-model="row.mem" size="small" :min="0" :max="100" />
              </template>
            </el-table-column>
            <el-table-column :label="$t('monitor.disk')" min-width="120">
              <template #default="{ row }">
                <el-input-number v-model="row.disk" size="small" :min="0" :max="100" />
              </template>
            </el-table-column>
            <el-table-column :label="$t('monitor.ruleDuration')" min-width="130">
              <template #default="{ row }">
                <el-input-number v-model="row.duration_sec" size="small" :min="0" :max="86400" />
              </template>
            </el-table-column>
            <el-table-column :label="$t('monitor.notif')" min-width="170">
              <template #default="{ row }">
                <el-select v-model="row.channel_ids" multiple size="small" style="width:100%" :placeholder="$t('monitor.chEmpty')">
                  <el-option v-for="ch in channels" :key="ch.id" :label="`${ch.name}（${chTypeLabel(ch.type)}）`" :value="ch.id" />
                </el-select>
              </template>
            </el-table-column>
          </el-table>
          <div style="color:#909399; font-size:12px; margin-top:8px">{{ $t('monitor.cmdLevelsTip') }}</div>
        </el-card>
      </el-tab-pane>

      <!-- Tab: maintenance windows (global, calendar date range) -->
      <el-tab-pane :label="$t('monitor.tabMaint')" name="maint">
        <el-card>
          <template #header>
            <div style="display:flex; align-items:center; gap:10px">
              <span style="flex:1">{{ $t('monitor.maintTitle') }}</span>
              <el-button size="small" :loading="maintAuditLoading" @click="loadMaintAudit">{{ $t('common.refresh') }}</el-button>
              <el-button size="small" type="primary" :loading="maintSaving" @click="saveMaintWindows">{{ $t('common.save') }}</el-button>
            </div>
          </template>
          <div style="color:#909399; font-size:12px; margin-bottom:10px">{{ $t('monitor.maintTip') }}</div>
          <div v-for="(w, i) in maintWins" :key="i" style="display:flex; gap:8px; align-items:center; margin-bottom:8px; flex-wrap:wrap">
            <el-select v-model="w.type" size="small" style="width:120px">
              <el-option value="once" :label="$t('monitor.maintOnce')" />
              <el-option value="daily" :label="$t('monitor.maintDaily')" />
              <el-option value="weekly" :label="$t('monitor.maintWeekly')" />
              <el-option value="monthly" :label="$t('monitor.maintMonthly')" />
            </el-select>
            <el-date-picker v-if="w.type === 'once'" v-model="w.dates" type="daterange" value-format="YYYY-MM-DD"
                            range-separator="→" start-placeholder="—" end-placeholder="—"
                            style="width:280px" :clearable="false" />
            <el-select v-if="w.type === 'weekly'" v-model="w.weekdays" multiple collapse-tags size="small"
                       style="width:200px" :placeholder="$t('monitor.maintWeekly')">
              <el-option v-for="(lbl, d) in maintWeekdays" :key="d" :value="d" :label="lbl" />
            </el-select>
            <el-time-select v-model="w.start" start="00:00" step="00:30" end="23:30" style="width:120px" placeholder="开始" />
            <span style="color:#909399">→</span>
            <el-time-select v-model="w.end" start="00:00" step="00:30" end="23:59" style="width:120px" placeholder="结束" />
            <el-button type="danger" link size="small" @click="maintWins.splice(i, 1)">{{ $t('apps.remove') }}</el-button>
          </div>
          <el-button size="small" @click="maintWins.push({ type: 'once', dates: [today(), today()], weekdays: [], start: '02:00', end: '04:00' })">{{ $t('monitor.maintAdd') }}</el-button>
        </el-card>
        <el-card style="margin-top:16px">
          <template #header>
            <div style="display:flex; align-items:center; gap:10px">
              <span style="flex:1">{{ $t('monitor.maintAudit') }}</span>
            </div>
          </template>
          <el-table :data="maintAudit" size="small" border>
            <el-table-column prop="username" :label="$t('audit.operator')" width="120" />
            <el-table-column :label="$t('monitor.maintLogWindows')" min-width="220">
              <template #default="{ row }">
                <div v-for="(w, i) in row.windows" :key="i" class="mono" style="font-size:12px">
                  {{ w.date_start }} → {{ w.date_end }} · {{ w.start }}-{{ w.end }}
                </div>
                <span v-if="!(row.windows || []).length" style="color:#c0c4cc">-</span>
              </template>
            </el-table-column>
            <el-table-column :label="$t('monitor.maintLogStatus')" width="100" align="center">
              <template #default="{ row }">
                <el-tag size="small" :type="row.active ? 'success' : 'info'">{{ row.active ? $t('monitor.maintActive') : $t('monitor.maintInactive') }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column :label="$t('audit.time')" width="170">
              <template #default="{ row }">{{ fmtTime(row.created_at) }}</template>
            </el-table-column>
          </el-table>
          <div v-if="!maintAudit.length" style="color:#909399; padding:8px 0">{{ $t('monitor.maintAuditEmpty') }}</div>
        </el-card>
      </el-tab-pane>

      <!-- Tab 5: template settings (list on the left, editor on the right) -->
      <el-tab-pane :label="$t('monitor.tabTpl')" name="templates">
        <el-card>
          <template #header>
            <div style="display:flex; align-items:center; gap:10px">
              <span style="flex:1">{{ $t('monitor.tplTitle') }}</span>
              <el-button size="small" type="primary" :loading="tplSaving" @click="saveAlertTemplates">{{ $t('common.save') }}</el-button>
            </div>
          </template>
          <div style="display:flex; gap:18px">
            <div style="width:220px; flex-shrink:0">
              <div v-for="sec in tplSections" :key="sec.key"
                   class="tpl-item" :class="{ active: selectedTpl === sec.key }"
                   @click="selectedTpl = sec.key">
                {{ secLabel(sec) }}
              </div>
            </div>
            <div style="flex:1; min-width:0">
              <div style="display:flex; align-items:center; gap:10px; margin-bottom:12px">
                <span style="font-weight:600">{{ secLabel(currentSection) }}</span>
                <el-button size="small" link type="primary" @click="previewSection(selectedTpl)">{{ $t('monitor.tplPreview') }}</el-button>
              </div>
              <el-form label-width="90px">
                <el-form-item :label="$t('monitor.chTplTitle')">
                  <el-input v-model="tplForm[currentSection.titleField]" class="mono" :placeholder="$t('monitor.chTplDefault')" />
                </el-form-item>
                <el-form-item :label="$t('monitor.chTplBody')">
                  <el-input v-model="tplForm[currentSection.bodyField]" type="textarea" :rows="6" class="mono"
                            :placeholder="$t('monitor.chTplDefault')" />
                </el-form-item>
              </el-form>
              <div style="color:#909399; font-size:12px; line-height:1.8">
                {{ $t('monitor.chTplVarsLabel') }}
                <span class="mono">{level} {host} {ip} {metric} {value} {threshold} {monitor} {type} {target} {status} {resp_ms} {error} {event} {time}</span>
              </div>
            </div>
          </div>
        </el-card>
      </el-tab-pane>

      <!-- Tab 7: monthly ops report -->
      <el-tab-pane :label="$t('mreport.tab')" name="report">
        <el-card>
          <div style="display:flex; gap:10px; align-items:center; margin-bottom:14px">
            <el-date-picker v-model="repMonth" type="month" :clearable="false"
                            :placeholder="$t('mreport.pickMonth')" style="width:160px" value-format="YYYY-MM" />
            <el-button type="primary" size="small" :loading="repLoading" @click="loadReport">{{ $t('mreport.generate') }}</el-button>
            <el-button size="small" :disabled="!rep" @click="printReport">{{ $t('mreport.print') }}</el-button>
            <el-button size="small" :disabled="!rep" @click="exportAlertCsv">{{ $t('k8s.exportCsv') }}</el-button>
          </div>

          <div v-if="rep" id="monthly-report" class="rep-page">
            <h2 style="text-align:center; margin:0 0 4px">{{ $t('mreport.title') }}</h2>
            <p style="text-align:center; color:#909399; margin:0 0 18px">{{ rep.period.from }} ~ {{ rep.period.to }}</p>

            <h3>{{ $t('mreport.secExec') }}</h3>
            <div class="rep-grid">
              <div class="rep-stat"><b>{{ rep.assets.hosts_total }}</b><span>{{ $t('mreport.hostsTotal') }}</span></div>
              <div class="rep-stat"><b>{{ rep.assets.hosts_online }}</b><span>{{ $t('mreport.hostsOnline') }}</span></div>
              <div class="rep-stat"><b>{{ rep.assets.k8s_clusters }}</b><span>{{ $t('mreport.k8sClusters') }}</span></div>
              <div class="rep-stat"><b>{{ rep.assets.monitors }}</b><span>{{ $t('mreport.monitors') }}</span></div>
              <div class="rep-stat"><b>{{ rep.alerts.total }}</b><span>{{ $t('mreport.alertsTotal') }}</span></div>
            </div>
            <h4>{{ $t('mreport.capRisk') }}</h4>
            <el-table :data="capRiskRows" size="small" border>
              <el-table-column :label="$t('mreport.target')" min-width="180">
                <template #default="{ row }">{{ row.target }}</template>
              </el-table-column>
              <el-table-column :label="$t('mreport.metric')" width="110">
                <template #default="{ row }">{{ row.metric }}</template>
              </el-table-column>
              <el-table-column :label="$t('mreport.currentUse')" width="150" align="center">
                <template #default="{ row }">{{ row.current }}</template>
              </el-table-column>
              <el-table-column :label="$t('mreport.capacity')" width="130" align="center">
                <template #default="{ row }">{{ row.capacity }}</template>
              </el-table-column>
              <el-table-column :label="$t('mreport.daysToLimit')" width="170" align="center">
                <template #default="{ row }">
                  <span :style="{ color: row.color, fontWeight: 600 }">{{ row.days }}</span>
                </template>
              </el-table-column>
              <el-table-column :label="$t('mreport.risk')" width="90" align="center">
                <template #default="{ row }">
                  <el-tag size="small" :type="row.risk === 'red' ? 'danger' : row.risk === 'yellow' ? 'warning' : 'success'">
                    {{ row.risk === 'red' ? $t('mreport.high') : row.risk === 'yellow' ? $t('mreport.medium') : $t('mreport.low') }}
                  </el-tag>
                </template>
              </el-table-column>
            </el-table>

            <h3>{{ $t('mreport.secHostCap') }}</h3>
            <el-table :data="rep.hosts" size="small" border>
              <el-table-column prop="name" :label="$t('hosts.name')" min-width="130" />
              <el-table-column prop="ip" :label="$t('hosts.ip')" width="130" />
              <el-table-column :label="$t('monitor.cpu') + ' ' + $t('mreport.avgPeak')" min-width="140" align="center">
                <template #default="{ row }">{{ row.avg_cpu }}% / {{ row.peak_cpu }}%</template>
              </el-table-column>
              <el-table-column :label="$t('monitor.mem') + ' ' + $t('mreport.avgPeak')" min-width="140" align="center">
                <template #default="{ row }">{{ row.avg_mem }}% / {{ row.peak_mem }}%</template>
              </el-table-column>
              <el-table-column :label="$t('monitor.disk')" width="100" align="center">
                <template #default="{ row }">{{ row.disk }}%</template>
              </el-table-column>
              <el-table-column :label="$t('mreport.forecast90')" min-width="200" align="center">
                <template #default="{ row }">{{ capDaysText(row.cpu_days_to_90) }}<br>{{ capDaysText(row.mem_days_to_90) }}</template>
              </el-table-column>
              <el-table-column :label="$t('mreport.risk')" width="80" align="center">
                <template #default="{ row }">
                  <el-tag size="small" :type="row.risk === 'red' ? 'danger' : row.risk === 'yellow' ? 'warning' : 'success'">
                    {{ row.risk === 'red' ? $t('mreport.high') : row.risk === 'yellow' ? $t('mreport.medium') : $t('mreport.low') }}
                  </el-tag>
                </template>
              </el-table-column>
            </el-table>

            <h3>{{ $t('mreport.secK8sCap') }}</h3>
            <el-table :data="rep.k8s" size="small" border>
              <el-table-column prop="cluster" :label="$t('k8s.cluster')" min-width="120" />
              <el-table-column prop="nodes" :label="$t('k8s.nodes')" width="70" align="center" />
              <el-table-column :label="$t('mreport.cpuUsedEnd')" min-width="150" align="center">
                <template #default="{ row }">{{ fmtCores2(row.cpu_used_end_m) }} / {{ fmtCores2(row.cpu_capacity_m) }}</template>
              </el-table-column>
              <el-table-column :label="$t('mreport.memUsedEnd')" min-width="150" align="center">
                <template #default="{ row }">{{ fmtMem(row.mem_used_end_mi) }} / {{ fmtMem(row.mem_capacity_mi) }}</template>
              </el-table-column>
              <el-table-column :label="$t('mreport.slopeCol')" min-width="150" align="center">
                <template #default="{ row }">+{{ (row.cpu_slope_m_day / 1000).toFixed(2) }} Core/{{ $t('k8s.capMonth') }}</template>
              </el-table-column>
              <el-table-column :label="$t('mreport.daysToLimit')" min-width="170" align="center">
                <template #default="{ row }">{{ capDaysText(row.cpu_days_to_full) }}<br>{{ capDaysText(row.mem_days_to_full) }}</template>
              </el-table-column>
            </el-table>
            <h4>{{ $t('mreport.secTopPods') }}</h4>
            <el-table :data="topPodRows" size="small" border>
              <el-table-column prop="cluster" :label="$t('k8s.cluster')" width="120" />
              <el-table-column prop="namespace" label="Namespace" width="120" />
              <el-table-column prop="pod" label="Pod" min-width="200" />
              <el-table-column :label="$t('mreport.avgCpuMonth')" width="150" align="center">
                <template #default="{ row }">{{ (row.avg_cpu_m / 1000).toFixed(2) }} Core</template>
              </el-table-column>
              <el-table-column :label="$t('mreport.avgMemMonth')" width="150" align="center">
                <template #default="{ row }">{{ fmtMem(row.avg_mem_mi) }}</template>
              </el-table-column>
            </el-table>

            <h3>{{ $t('mreport.secAlerts') }}</h3>
            <div class="rep-grid">
              <div class="rep-stat" v-for="(v, k) in rep.alerts.by_level" :key="k"><b>{{ v }}</b><span>{{ k }}</span></div>
            </div>
            <h4>{{ $t('mreport.secTopAlerts') }}</h4>
            <el-table :data="rep.alerts.top_targets" size="small" border>
              <el-table-column prop="target" :label="$t('mreport.target')" min-width="200" />
              <el-table-column prop="count" :label="$t('mreport.triggerCount')" width="140" align="center" />
            </el-table>
            <h4>{{ $t('mreport.secTimeline') }}</h4>
            <el-table :data="rep.alerts.timeline" size="small" border>
              <el-table-column :label="$t('audit.time')" width="160">
                <template #default="{ row }">{{ fmtTime(row.fired_at) }}</template>
              </el-table-column>
              <el-table-column prop="level" :label="$t('mreport.level')" width="80" align="center" />
              <el-table-column prop="target" :label="$t('mreport.target')" min-width="150" />
              <el-table-column prop="message" :label="$t('audit.detail')" min-width="280" show-overflow-tooltip />
              <el-table-column :label="$t('mreport.recoveryTime')" width="130" align="center">
                <template #default="{ row }">
                  <span v-if="row.duration_sec != null">{{ Math.round(row.duration_sec / 60) }} min</span>
                  <span v-else style="color:#c0c4cc">-</span>
                </template>
              </el-table-column>
            </el-table>

            <h3>{{ $t('mreport.secChanges') }}</h3>
            <div class="rep-grid">
              <div class="rep-stat"><b>{{ rep.changes.hosts_added }}</b><span>{{ $t('mreport.hostsAdded') }}</span></div>
              <div class="rep-stat"><b>{{ rep.changes.hosts_deleted }}</b><span>{{ $t('mreport.hostsDeleted') }}</span></div>
              <div class="rep-stat"><b>{{ rep.changes.credentials_added }}</b><span>{{ $t('mreport.credsAdded') }}</span></div>
            </div>
          </div>
          <el-empty v-else :description="$t('mreport.pickMonthFirst')" />
        </el-card>
      </el-tab-pane>

      <!-- Tab 8: OpenObserve log search (long-term storage / full-text) -->
      <el-tab-pane :label="$t('oo.searchTab')" name="oosearch">
        <el-card>
          <el-alert v-if="ooErr" type="warning" :title="ooErr" :closable="false" style="margin-bottom:12px" show-icon />
          <div style="display:flex; gap:8px; flex-wrap:wrap; align-items:center; margin-bottom:10px">
            <el-select v-model="ooStream" style="width:170px">
              <el-option value="host_metrics" :label="$t('oo.streamHostMetrics')" />
              <el-option value="task_logs" :label="$t('oo.streamTaskLogs')" />
              <el-option value="alert_events" :label="$t('oo.streamAlertEvents')" />
            </el-select>
            <el-select v-model="ooRange" style="width:130px" @change="ooSearch">
              <el-option value="1" :label="$t('oo.range1h')" />
              <el-option value="24" :label="$t('oo.range24h')" />
              <el-option value="168" :label="$t('oo.range7d')" />
              <el-option value="720" :label="$t('oo.range30d')" />
            </el-select>
            <el-button type="primary" :loading="ooBusy" @click="ooSearch">{{ $t('oo.run') }}</el-button>
            <span style="flex:1"></span>
            <span style="color:var(--el-text-color-secondary); font-size:12px">{{ $t('oo.sqlTip') }}</span>
          </div>
          <el-input v-model="ooSQL" type="textarea" :rows="3" class="mono"
                    :placeholder="'SELECT host, cpu_percent FROM host_metrics WHERE cpu_percent > 90'" />
          <el-table v-if="ooCols.length" :data="ooRows" size="small" border style="margin-top:12px" max-height="480">
            <el-table-column v-for="c in ooCols" :key="c" :prop="c" :label="c" min-width="140" show-overflow-tooltip>
              <template #default="{ row }">{{ formatOOCell(row[c]) }}</template>
            </el-table-column>
          </el-table>
          <el-empty v-else-if="!ooBusy && !ooErr" :description="$t('oo.empty')" />
        </el-card>
      </el-tab-pane>
    </el-tabs>

    <!-- Host resource trends -->
    <el-drawer v-model="trendVisible" :title="`${trendHost?.name} — ${$t('monitor.trend')}`" size="640px">
      <div style="display:flex; gap:6px; margin-bottom:14px">
        <el-radio-group v-model="trendHours" size="small" @change="loadTrend">
          <el-radio-button :value="6">6h</el-radio-button>
          <el-radio-button :value="24">24h</el-radio-button>
          <el-radio-button :value="168">7d</el-radio-button>
          <el-radio-button :value="720">30d</el-radio-button>
          <el-radio-button :value="4320">180d</el-radio-button>
          <el-radio-button :value="8760">1y</el-radio-button>
        </el-radio-group>
      </div>
      <div v-for="k in ['cpu', 'mem', 'disk']" :key="k" style="margin-bottom:18px">
        <div style="font-weight:600; margin-bottom:4px">{{ metricLabel(k) }}</div>
        <MetricChart :points="trendRows.map(r => ({ t: r.collected_at, v: r[k + '_percent'] }))"
                     :range="trendRange" :empty-text="$t('monitor.noData')"
                     :color="k === 'cpu' ? '#409eff' : k === 'mem' ? '#67c23a' : '#e6a23c'" unit="%" :y-max="100" />
      </div>
    </el-drawer>

    <!-- Create / edit monitor -->
    <el-dialog v-model="dlgVisible" :title="form.id ? $t('monitor.editMon') : $t('monitor.addMon')" width="520px">
      <el-form label-width="120px">
        <el-form-item :label="$t('monitor.monName')"><el-input v-model="form.name" /></el-form-item>
        <el-form-item :label="$t('monitor.monType')">
          <el-radio-group v-model="form.type">
            <el-radio-button value="http">{{ $t('monitor.typeHttp') }}</el-radio-button>
            <el-radio-button value="tcp">{{ $t('monitor.typeTcp') }}</el-radio-button>
            <el-radio-button value="ping">{{ $t('monitor.typePing') }}</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item :label="$t('monitor.target')">
          <el-input v-model="form.target" :placeholder="form.type === 'http' ? $t('monitor.urlTip') : $t('monitor.hostTip')" />
        </el-form-item>
        <el-form-item v-if="form.type === 'tcp'" :label="$t('monitor.port')">
          <el-input-number v-model="form.port" :min="1" :max="65535" />
        </el-form-item>
        <template v-if="form.type === 'http'">
          <el-form-item :label="$t('monitor.method')">
            <el-radio-group v-model="form.method">
              <el-radio value="GET">GET</el-radio>
              <el-radio value="HEAD">HEAD</el-radio>
            </el-radio-group>
          </el-form-item>
          <el-form-item :label="$t('monitor.acceptedStatus')">
            <el-input v-model="form.accepted_status" :placeholder="$t('monitor.acceptedTip')" class="mono" />
          </el-form-item>
          <el-form-item :label="$t('monitor.keyword')">
            <el-input v-model="form.keyword" :placeholder="$t('monitor.keywordTip')" />
          </el-form-item>
          <el-form-item v-if="form.keyword" :label="$t('monitor.keywordType')">
            <el-radio-group v-model="form.keyword_type">
              <el-radio value="contain">{{ $t('monitor.kwContain') }}</el-radio>
              <el-radio value="absent">{{ $t('monitor.kwAbsent') }}</el-radio>
            </el-radio-group>
          </el-form-item>
        </template>
        <el-form-item :label="$t('monitor.interval')"><el-input-number v-model="form.interval_sec" :min="15" :max="86400" /></el-form-item>
        <el-form-item :label="$t('monitor.timeout')"><el-input-number v-model="form.timeout_sec" :min="1" :max="120" /></el-form-item>
        <el-form-item :label="$t('monitor.notif')">
          <el-select v-model="form.channel_ids" multiple style="width:100%" :placeholder="$t('monitor.chEmpty')">
            <el-option v-for="ch in channels" :key="ch.id" :label="`${ch.name}（${chTypeLabel(ch.type)}）`" :value="ch.id" />
          </el-select>
        </el-form-item>
        <el-form-item :label="$t('monitor.enabled')"><el-switch v-model="form.enabled" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dlgVisible = false">{{ $t('common.cancel') }}</el-button>
        <el-button type="primary" @click="save">{{ $t('common.save') }}</el-button>
      </template>
    </el-dialog>

    <!-- Template sample-send preview -->
    <el-dialog v-model="previewVisible" :title="$t('monitor.tplPreviewTitle')" width="560px">
      <div style="color:#909399; font-size:12px; margin-bottom:10px">{{ $t('monitor.tplPreviewNote') }}</div>
      <el-descriptions :column="1" border size="small">
        <el-descriptions-item :label="$t('monitor.chTplTitle')"><span class="mono">{{ previewData.title }}</span></el-descriptions-item>
        <el-descriptions-item :label="$t('monitor.chTplBody')">
          <pre class="mono" style="margin:0; white-space:pre-wrap; font-size:12px">{{ previewData.body }}</pre>
        </el-descriptions-item>
      </el-descriptions>
    </el-dialog>

    <!-- Create / edit notification channel -->
    <el-dialog v-model="chDlgVisible" :title="chForm.id ? $t('monitor.chEdit') : $t('monitor.chAdd')" width="520px">
      <el-form label-width="120px">
        <el-form-item :label="$t('monitor.monName')"><el-input v-model="chForm.name" /></el-form-item>
        <el-form-item :label="$t('monitor.chType')">
          <el-select v-model="chForm.type" style="width:100%" @change="onChTypeChange">
            <el-option v-for="ty in channelTypes" :key="ty.value" :label="ty.label" :value="ty.value" />
          </el-select>
        </el-form-item>
        <template v-if="chForm.type">
          <el-form-item v-if="chForm.type === 'email'" :label="$t('monitor.chRecipients')">
            <el-input v-model="chForm.f_recipients" :placeholder="$t('monitor.chRecipientsTip')" />
          </el-form-item>
          <el-form-item v-if="chForm.type === 'telegram'" :label="$t('monitor.chBotToken')">
            <el-input v-model="chForm.f_bot_token" class="mono" />
          </el-form-item>
          <el-form-item v-if="chForm.type === 'telegram'" :label="$t('monitor.chChatId')">
            <el-input v-model="chForm.f_chat_id" class="mono" />
          </el-form-item>
          <el-form-item v-if="chForm.type !== 'email' && chForm.type !== 'telegram'" :label="$t('monitor.chUrl')">
            <el-input v-model="chForm.f_url" class="mono" :placeholder="$t('monitor.chUrlTip')" />
          </el-form-item>
        </template>
        <template v-if="chForm.type">
          <el-divider style="margin:8px 0 16px" />
          <el-form-item :label="$t('monitor.chTplTitle')" v-if="chForm.type === 'email'">
            <el-input v-model="chForm.f_title_tpl" :placeholder="$t('monitor.chTplDefault')" />
          </el-form-item>
          <el-form-item :label="chForm.type === 'email' ? $t('monitor.chTplBody') : $t('monitor.chTplMsg')">
            <el-input v-model="chForm.f_body_tpl" type="textarea" :rows="4"
                      :placeholder="$t('monitor.chTplDefault')" />
          </el-form-item>
          <div style="color:#909399; font-size:12px; line-height:1.8">
            {{ $t('monitor.chTplVarsLabel') }}
            <span class="mono">{level} {host} {ip} {metric} {value} {threshold} {monitor} {type} {target} {status} {resp_ms} {error} {event} {time}</span>
            <div>{{ $t('monitor.chTplTip') }}</div>
          </div>
        </template>
        <el-form-item :label="$t('monitor.enabled')"><el-switch v-model="chForm.enabled" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="chDlgVisible = false">{{ $t('common.cancel') }}</el-button>
        <el-button type="primary" @click="saveChannel">{{ $t('common.save') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import api from '../api'
import { useRouter } from 'vue-router'
import i18n from '../i18n'
import { ElMessage } from 'element-plus'
import MetricChart from '../components/MetricChart.vue'

const { t } = i18n.global
const $router = useRouter()
const activeTab = ref('cmd')

// ---- OpenObserve log search (queries via the backend proxy; disabled → friendly notice) ----
const ooStream = ref('host_metrics')
const ooRange = ref('24')
const ooSQL = ref('')
const ooBusy = ref(false)
const ooErr = ref('')
const ooRows = ref([])
const ooCols = ref([])

const ooDefaultSQL = stream => ({
  host_metrics: 'SELECT host, cpu_percent, mem_percent, disk_percent FROM host_metrics ORDER BY _timestamp DESC',
  task_logs: 'SELECT task_id, host, os_user, status, exit_code, output FROM task_logs ORDER BY _timestamp DESC',
  alert_events: 'SELECT kind, level, target, message FROM alert_events ORDER BY _timestamp DESC',
}[stream] || `SELECT * FROM ${stream} LIMIT 100`)

const formatOOCell = v => {
  if (v === null || v === undefined) return '-'
  if (typeof v === 'object') return JSON.stringify(v)
  const s = String(v)
  if (/^\d{16,}$/.test(s)) { // _timestamp is epoch microseconds
    const d = new Date(Number(s) / 1000)
    if (!isNaN(d)) return d.toLocaleString()
  }
  return s
}

const ooSearch = async () => {
  ooBusy.value = true
  ooErr.value = ''
  try {
    const sql = (ooSQL.value || ooDefaultSQL(ooStream.value)).trim()
    const end = Date.now()
    const start = end - Number(ooRange.value) * 3600 * 1000
    const r = await api.post('/monitors/oo/search', { sql, start_ms: start, end_ms: end, from: 0, size: 200 })
    const hits = r.hits || []
    const cols = new Set()
    for (const h of hits.slice(0, 50)) Object.keys(h).forEach(k => { if (k !== '_timestamp') cols.add(k) })
    ooCols.value = [...cols]
    ooRows.value = hits
    if (!hits.length) ooErr.value = t('oo.emptyResult')
  } catch (e) {
    ooRows.value = []
    ooCols.value = []
    ooErr.value = e?.response?.data?.error || t('oo.notEnabled')
  } finally { ooBusy.value = false }
}

// Seed the SQL placeholder and auto-run on first entry of the tab
watch(activeTab, tab => {
  if (tab === 'oosearch') {
    if (!ooSQL.value) ooSQL.value = ooDefaultSQL(ooStream.value)
    if (!ooRows.value.length && !ooErr.value) ooSearch()
  }
})
watch(ooStream, () => { ooSQL.value = ooDefaultSQL(ooStream.value) })

const hostsLoading = ref(false)
const hostRows = ref([])
const hostKw = ref('')
const groupFilter = ref('')
const monitors = ref([])
const channels = ref([])
const loading = ref(false)
const chLoading = ref(false)
const dlgVisible = ref(false)
const chDlgVisible = ref(false)
const form = reactive({})
const chForm = reactive({})
let timer = null

const groupOptions = computed(() => [...new Set(hostRows.value.map(h => h.group).filter(Boolean))])
const filteredHosts = computed(() => {
  let list = hostRows.value
  if (groupFilter.value) list = list.filter(h => h.group === groupFilter.value)
  const kw = hostKw.value.trim().toLowerCase()
  if (kw) list = list.filter(h => `${h.name} ${h.ip} ${h.group || ''}`.toLowerCase().includes(kw))
  return list
})

const fmtTime = v => (v ? String(v).replace('T', ' ').slice(0, 19) : '-')
const typeLabel = ty => ({ http: t('monitor.typeHttp'), tcp: t('monitor.typeTcp'), ping: t('monitor.typePing') }[ty] || ty)
const monitorTarget = m => (m.type === 'http' ? m.target : m.type === 'tcp' ? `${m.target}:${m.port}` : m.target)
const statusClass = row => (!row.monitor.enabled ? 'dot-paused' : row.monitor.last_status === 'up' ? 'dot-up' : row.monitor.last_status === 'down' ? 'dot-down' : 'dot-paused')
const hbTip = s => `${fmtTime(s.at)} · ${s.status === 'up' ? t('monitor.hbUp') : s.status === 'maint' ? t('monitor.hbMaint') : t('monitor.hbDown')} · ${s.resp_ms}ms`
const statusText = row => (!row.monitor.enabled ? t('monitor.paused') : row.monitor.last_status === 'up' ? t('monitor.up') : row.monitor.last_status === 'down' ? t('monitor.down') : t('monitor.notYet'))
const barColor = v => (v == null ? '#909399' : v >= 90 ? '#f56c6c' : v >= 75 ? '#e6a23c' : '#67c23a')

const channelTypes = [
  { value: 'email', label: 'Email' },
  { value: 'webhook', label: 'Webhook' },
  { value: 'wecom', label: 'WeCom' },
  { value: 'dingtalk', label: 'DingTalk' },
  { value: 'feishu', label: 'Feishu' },
  { value: 'telegram', label: 'Telegram' },
]
const chTypeLabel = v => (channelTypes.find(x => x.value === v) || {}).label || v
const chConfigSummary = row => {
  try {
    const cfg = JSON.parse(row.config || '{}')
    if (row.type === 'email') return cfg.recipients || '-'
    if (row.type === 'telegram') return cfg.chat_id || '-'
    return cfg.url || '-'
  } catch { return '-' }
}

const load = async () => {
  loading.value = true
  hostsLoading.value = true
  try {
    const [hs, ms, chs] = await Promise.all([
      api.get('/monitoring/hosts'), api.get('/monitors'), api.get('/alert_channels'),
    ])
    hostRows.value = hs
    monitors.value = ms
    channels.value = chs
  } finally {
    hostsLoading.value = false
    loading.value = false
  }
}
const loadChannels = async () => { channels.value = await api.get('/alert_channels') }

// ---- Host trends ----
const trendVisible = ref(false)
const trendHost = ref(null)
const trendRows = ref([])
const maxOf = arr => Math.max(10, ...arr.map(v => Math.ceil(v || 0)))
const metricLabel = k => ({ cpu: t('monitor.cpu'), mem: t('monitor.mem'), disk: t('monitor.disk') }[k])
const sparkPoints = arr => {
  const max = maxOf(arr)
  const n = arr.length
  if (!n) return ''
  return arr.map((v, i) => `${(i / Math.max(1, n - 1)) * 600},${80 - ((v || 0) / max) * 76}`).join(' ')
}
const trendHours = ref(6)
const trendRange = computed(() => [Date.now() - trendHours.value * 3600000, Date.now()])
const loadTrend = async () => {
  if (!trendHost.value) return
  trendRows.value = await api.get(`/monitoring/hosts/${trendHost.value.host_id}/history`, { params: { hours: trendHours.value } })
}
const openHostTrend = async row => {
  trendHost.value = row
  trendHours.value = 6
  await loadTrend()
  trendVisible.value = true
}

// ---- Monitor CRUD ----
const openDlg = (m, channelIds) => {
  Object.assign(form, m || {}, {
    type: m?.type || 'http', method: m?.method || 'GET',
    accepted_status: m?.accepted_status || '200-299',
    keyword_type: m?.keyword_type || 'contain',
    interval_sec: m?.interval_sec || 60, timeout_sec: m?.timeout_sec || 10,
    enabled: m ? !!m.enabled : true, port: m?.port || 80,
    channel_ids: [...(channelIds || [])],
  })
  dlgVisible.value = true
}
const save = async () => {
  const payload = { ...form }
  if (form.id) await api.put(`/monitors/${form.id}`, payload)
  else {
    await api.post('/monitors', payload)
    ElMessage.success(t('monitor.created'))
  }
  dlgVisible.value = false
  load()
}
const del = async m => { await api.delete(`/monitors/${m.id}`); load() }
const togglePause = async row => {
  await api.put(`/monitors/${row.monitor.id}`, { ...row.monitor, channel_ids: row.channel_ids || [], enabled: !row.monitor.enabled })
  load()
}
const testNow = async row => {
  const r = await api.post(`/monitors/${row.monitor.id}/test`)
  if (r.up) ElMessage.success(`${t('monitor.testUp')} · ${r.resp_ms}ms`)
  else ElMessage.error(`${t('monitor.testDown')}: ${r.error}`)
  load()
}

// ---- Template sample-send preview ----
const sampleVars = {
  level: 'P2', host: 'demo-01', ip: '10.0.0.8',
  metric: 'CPU', value: '91.5', threshold: '90',
  monitor: 'demo-monitor', type: 'http', target: 'http://10.0.0.8/health',
  status: 'DOWN', resp_ms: '233', error: '-',
  event: 'system rebooted (boot_id changed)',
  time: new Date().toLocaleString(),
}
const renderTplLocal = (tpl, vars) => {
  let out = tpl || ''
  for (const k of Object.keys(vars)) out = out.split('{' + k + '}').join(vars[k])
  return out
}
const tplSections = [
  { key: 'email', labelKey: 'tplEmail', titleField: 'email_title', bodyField: 'email_body' },
  { key: 'monitor_alert', labelKey: 'tplMonAlert', titleField: 'monitor_alert_title', bodyField: 'monitor_alert_body' },
  { key: 'monitor_recovery', labelKey: 'tplMonRecovery', titleField: 'monitor_recovery_title', bodyField: 'monitor_recovery_body' },
  { key: 'reboot', labelKey: 'tplReboot', titleField: 'reboot_title', bodyField: 'reboot_body' },
  { key: 'cmd_alert', labelKey: 'tplCmdAlert', titleField: 'cmd_alert_title', bodyField: 'cmd_alert_body' },
  { key: 'cmd_recovery', labelKey: 'tplCmdRecovery', titleField: 'cmd_recovery_title', bodyField: 'cmd_recovery_body' },
]
const selectedTpl = ref('monitor_alert')
const secLabel = sec => t('monitor.' + sec.labelKey)
const currentSection = computed(() => tplSections.find(x => x.key === selectedTpl.value) || tplSections[0])
const previewVisible = ref(false)
const previewData = reactive({ title: '', body: '' })
const previewSection = key => {
  const pick = {
    email: ['email_title', 'email_body'],
    monitor_alert: ['monitor_alert_title', 'monitor_alert_body'],
    monitor_recovery: ['monitor_recovery_title', 'monitor_recovery_body'],
    reboot: ['reboot_title', 'reboot_body'],
    cmd_alert: ['cmd_alert_title', 'cmd_alert_body'],
    cmd_recovery: ['cmd_recovery_title', 'cmd_recovery_body'],
  }[key] || ['', '']
  previewData.title = renderTplLocal(tplForm[pick[0]], sampleVars)
  previewData.body = renderTplLocal(tplForm[pick[1]], sampleVars)
  previewVisible.value = true
}

// ---- Global default templates ----
// Suggested email template content follows the UI language
const EMAIL_DFT = {
  'zh-CN': {
    title: 'JNexus 告警通知 - {host}',
    body: (
      '<h3>JNexus 告警通知</h3>\n'
      + '<p>主机：<b>{host}</b>（{ip}）</p>\n'
      + '<p>级别：{level}</p>\n'
      + '<p>指标：{metric} = <b>{value}%</b>（阈值 {threshold}%）</p>\n'
      + '<p>状态：{status}</p>\n'
      + '<p style="color:#c0392b">错误：{error}</p>\n'
      + '<p>时间：{time}</p>'
    )
  },
  'en-US': {
    title: 'JNexus Alert Notification - {host}',
    body: (
      '<h3>JNexus Alert Notification</h3>\n'
      + '<p>Host: <b>{host}</b> ({ip})</p>\n'
      + '<p>Level: {level}</p>\n'
      + '<p>Metric: {metric} = <b>{value}%</b> (threshold {threshold}%)</p>\n'
      + '<p>Status: {status}</p>\n'
      + '<p style="color:#c0392b">Error: {error}</p>\n'
      + '<p>Time: {time}</p>'
    )
  }
}
const tplForm = reactive({})
const tplSaving = ref(false)
// Prefill empty fields with suggested content for the current UI language
const prefillEmailTpl = () => {
  const dft = EMAIL_DFT[i18n.global.locale.value] || EMAIL_DFT['en-US']
  if (!tplForm.email_title) tplForm.email_title = dft.title
  if (!tplForm.email_body) tplForm.email_body = dft.body
}
const loadAlertTemplates = async () => {
  Object.assign(tplForm, await api.get('/alert_rules/templates'))
  prefillEmailTpl()
}
// When the UI language changes, unmodified prefilled content follows the switch
watch(() => i18n.global.locale.value, () => {
  const loc = i18n.global.locale.value
  const other = loc === 'zh-CN' ? 'en-US' : 'zh-CN'
  const d = EMAIL_DFT[loc] || EMAIL_DFT['en-US']
  const o = EMAIL_DFT[other] || EMAIL_DFT['en-US']
  if (tplForm.email_title === o.title) tplForm.email_title = d.title
  if (tplForm.email_body === o.body) tplForm.email_body = d.body
})
const saveAlertTemplates = async () => {
  tplSaving.value = true
  try {
    Object.assign(tplForm, await api.put('/alert_rules/templates', { ...tplForm }))
    ElMessage.success(t('common.success'))
  } finally { tplSaving.value = false }
}

// ---- Global maintenance windows (calendar date ranges) ----
const maintWins = ref([])
const maintSaving = ref(false)
const maintAudit = ref([])
const maintAuditLoading = ref(false)
const today = () => {
  const d = new Date()
  return d.getFullYear() + '-' + String(d.getMonth() + 1).padStart(2, '0') + '-' + String(d.getDate()).padStart(2, '0')
}
const loadMaintWindows = async () => {
  const wins = await api.get('/maintenance_windows')
  maintWins.value = (wins || []).map(w => ({
    type: w.type || 'once',
    dates: [w.date_start || today(), w.date_end || today()],
    weekdays: w.weekdays || [],
    start: w.start || '02:00',
    end: w.end || '04:00',
  }))
}
const maintWeekdays = computed(() => t('monitor.maintWd').split(','))
const saveMaintWindows = async () => {
  if (!maintWins.value.length) {
    ElMessage.warning(t('monitor.maintEmptyWarning'))
    return
  }
  // Validate by type: one-off windows need a date range, weekly windows need weekdays
  for (const w of maintWins.value) {
    if (w.type === 'once' && !(Array.isArray(w.dates) && w.dates[0] && w.dates[1])) {
      ElMessage.warning(t('monitor.maintFillDates'))
      return
    }
    if (w.type === 'weekly' && !(w.weekdays || []).length) {
      ElMessage.warning(t('monitor.maintPickDay'))
      return
    }
  }
  const rows = maintWins.value.map(w => ({
    type: w.type || 'once',
    date_start: w.type === 'once' ? (w.dates?.[0] || '') : '',
    date_end: w.type === 'once' ? (w.dates?.[1] || '') : '',
    weekdays: w.type === 'weekly' ? (w.weekdays || []) : [],
    start: w.start || '00:00', end: w.end || '00:00',
  }))
  maintSaving.value = true
  try {
    await api.put('/maintenance_windows', rows)
    ElMessage.success(t('common.success'))
    await loadMaintWindows()
    await loadMaintAudit()
  } finally { maintSaving.value = false }
}
const loadMaintAudit = async () => {
  maintAuditLoading.value = true
  try {
    maintAudit.value = await api.get('/maintenance_windows/logs')
  } finally { maintAuditLoading.value = false }
}

// ---- CMD tiered thresholds ----
const cmdLevels = ref([])
const cmdSaving = ref(false)
const loadCmdLevels = async () => { cmdLevels.value = await api.get('/alert_rules/cmd') }
const saveCmdLevels = async () => {
  cmdSaving.value = true
  try {
    cmdLevels.value = await api.put('/alert_rules/cmd', cmdLevels.value.map(l => ({ ...l })))
    ElMessage.success(t('common.success'))
  } finally { cmdSaving.value = false }
}

// ---- Alert rules (global) ----
const alertRule = reactive({ mode: 'grace', grace_sec: 60, notify_recovery: true })
const ruleSaving = ref(false)
const loadAlertRule = async () => {
  Object.assign(alertRule, await api.get('/alert_rules'))
}
const saveAlertRule = async () => {
  ruleSaving.value = true
  try {
    Object.assign(alertRule, await api.put('/alert_rules', { ...alertRule }))
    ElMessage.success(t('common.success'))
  } finally { ruleSaving.value = false }
}

const downFor = since => {
  const sec = Math.max(0, Math.floor((Date.now() - new Date(since).getTime()) / 1000))
  if (sec < 60) return sec + 's'
  return Math.floor(sec / 60) + 'm' + String(sec % 60).padStart(2, '0') + 's'
}

// ---- Notification channels ----
const onChTypeChange = () => { /* keep entered values on type change; users adjust them as needed */ }
const openChDlg = ch => {
  let cfg = {}
  try { cfg = JSON.parse(ch?.config || '{}') } catch { /* ignore */ }
  Object.assign(chForm, {
    id: ch?.id, name: ch?.name || '', type: ch?.type || '', enabled: ch ? !!ch.enabled : true,
    f_recipients: cfg.recipients || '', f_url: cfg.url || '',
    f_bot_token: cfg.bot_token || '', f_chat_id: cfg.chat_id || '',
    f_title_tpl: cfg.title_tpl || '', f_body_tpl: cfg.body_tpl || '',
  })
  chDlgVisible.value = true
}
const buildChConfig = () => {
  const cfg = {}
  if (chForm.type === 'email') cfg.recipients = chForm.f_recipients
  else if (chForm.type === 'telegram') { cfg.bot_token = chForm.f_bot_token; cfg.chat_id = chForm.f_chat_id }
  else cfg.url = chForm.f_url
  cfg.title_tpl = chForm.f_title_tpl || ''
  cfg.body_tpl = chForm.f_body_tpl || ''
  return JSON.stringify(cfg)
}
const saveChannel = async () => {
  if (!chForm.name || !chForm.type) { ElMessage.warning(t('monitor.chNameTypeRequired')); return }
  const payload = { name: chForm.name, type: chForm.type, config: buildChConfig(), enabled: chForm.enabled }
  if (chForm.id) await api.put(`/alert_channels/${chForm.id}`, payload)
  else await api.post('/alert_channels', payload)
  chDlgVisible.value = false
  loadChannels()
}
const delChannel = async ch => { await api.delete(`/alert_channels/${ch.id}`); loadChannels() }
const toggleCh = async ch => {
  await api.put(`/alert_channels/${ch.id}`, { ...ch, enabled: !ch.enabled })
  loadChannels()
}
const testChannel = async ch => {
  try {
    await api.post(`/alert_channels/${ch.id}/test`)
    ElMessage.success(t('monitor.chTestOk'))
  } catch { /* error toast shown by the interceptor */ }
}

onMounted(() => {
  load()
  loadAlertRule()
  loadAlertTemplates()
  loadCmdLevels()
  loadMaintWindows()
  loadMaintAudit()
  timer = setInterval(load, 30000)
})
// ---- Monthly ops report ----
const repMonth = ref(new Date().toISOString().slice(0, 7))
const rep = ref(null)
const repLoading = ref(false)
const loadReport = async () => {
  repLoading.value = true
  try {
    const [y, m] = (repMonth.value || '').split('-')
    rep.value = await api.get('/report/monthly', { params: { year: y, month: m } })
  } finally { repLoading.value = false }
}
const printReport = () => window.print()
const fmtCores2 = m => `${((Number(m) || 0) / 1000).toFixed(2)} Core`
const fmtMem = mi => { const v = Number(mi) || 0; return v >= 1024 ? `${(v / 1024).toFixed(1)}Gi` : `${Math.round(v)}Mi` }
const capDaysText = d => {
  if (d == null) return i18n.global.t('k8s.capNoExhaust')
  if (d <= 0) return i18n.global.t('k8s.capExhausted')
  return d < 60 ? `~${Math.round(d)} ${i18n.global.t('k8s.capDays')}` : `~${(d / 30).toFixed(1)} ${i18n.global.t('k8s.capMonths')}`
}
const capDaysColor = d => (d == null ? '#67c23a' : d < 90 ? '#f56c6c' : d < 180 ? '#e6a23c' : '#67c23a')
// Combined capacity-risk traffic-light view (hosts + K8s clusters)
const capRiskRows = computed(() => {
  const out = []
  for (const h of rep.value?.hosts || []) {
    for (const [metric, days, cur, cap, unit] of [
      ['CPU', h.cpu_days_to_90, h.peak_cpu, 100, '%'],
      ['MEM', h.mem_days_to_90, h.peak_mem, 100, '%'],
    ]) {
      if (days != null && days < 180) {
        out.push({ target: `${h.name}（${h.ip}）`, metric, current: `${cur}${unit}`, capacity: '100' + unit,
          days: `~${Math.round(days)}d`, color: capDaysColor(days), risk: days < 90 ? 'red' : 'yellow' })
      } else if (days == null && cur >= 90) {
        out.push({ target: `${h.name}（${h.ip}）`, metric, current: `${cur}${unit}`, capacity: '100' + unit,
          days: i18n.global.t('k8s.capExhausted'), color: '#f56c6c', risk: 'red' })
      }
    }
    if (h.disk >= 80) {
      out.push({ target: `${h.name}（${h.ip}）`, metric: 'DISK', current: `${h.disk}%`, capacity: '100%',
        days: h.disk >= 90 ? i18n.global.t('k8s.capExhausted') : `80%+`, color: h.disk >= 90 ? '#f56c6c' : '#e6a23c',
        risk: h.disk >= 90 ? 'red' : 'yellow' })
    }
  }
  for (const k of rep.value?.k8s || []) {
    for (const [metric, days, cur, capTxt, slope] of [
      ['CPU', k.cpu_days_to_full, k.cpu_used_end_m, k.cpu_capacity_m, k.cpu_slope_m_day],
      ['MEM', k.mem_days_to_full, k.mem_used_end_mi, k.mem_capacity_mi, k.mem_slope_m_day === 0 ? k.cpu_slope_m_day : k.cpu_slope_m_day],
    ]) {
      const realDays = metric === 'MEM' ? k.mem_days_to_full : days
      if (realDays != null && realDays < 180) {
        out.push({ target: `${k.cluster}（K8S）`, metric, current: metric === 'CPU' ? fmtCores2(cur) : fmtMem(cur),
          capacity: metric === 'CPU' ? fmtCores2(k.cpu_capacity_m) : fmtMem(k.mem_capacity_mi),
          days: `~${Math.round(realDays)}d`, color: capDaysColor(realDays), risk: realDays < 90 ? 'red' : 'yellow' })
      }
    }
  }
  return out
})
const topPodRows = computed(() => {
  const out = []
  for (const k of rep.value?.k8s || []) {
    for (const p of k.top_pods || []) out.push({ cluster: k.cluster, ...p })
  }
  return out
})
const exportAlertCsv = () => {
  const rows = rep.value?.alerts?.details || []
  const esc = v => `"${String(v ?? '').replace(/"/g, '""')}"`
  const lines = ['"time","level","kind","target","message","duration_sec"']
  for (const e of rows) {
    lines.push([esc(String(e.fired_at).replace('T', ' ').slice(0, 19)), esc(e.level), esc(e.kind),
      esc(e.target), esc(e.message), esc(e.duration_sec ?? '')].join(','))
  }
  const blob = new Blob(['\ufeff' + lines.join('\n')], { type: 'text/csv;charset=utf-8' })
  const a = document.createElement('a')
  a.href = URL.createObjectURL(blob)
  a.download = `alerts-${repMonth.value}.csv`
  a.click()
  URL.revokeObjectURL(a.href)
}

onUnmounted(() => clearInterval(timer))
</script>

<style scoped>
.mon-row {
  display: flex; align-items: center; gap: 14px; padding: 10px 4px;
  border-bottom: 1px solid #ebeef5; min-height: 56px;
}
.dot { width: 12px; height: 12px; border-radius: 50%; flex-shrink: 0; }
.dot-up { background: #67c23a; box-shadow: 0 0 0 3px rgba(103, 194, 58, .2); }
.dot-down { background: #f56c6c; box-shadow: 0 0 0 3px rgba(245, 108, 108, .2); }
.dot-paused { background: #c0c4cc; }
.hb { display: flex; gap: 2px; align-items: flex-end; height: 26px; flex-shrink: 0; width: 220px; }
.hb-bar { flex: 1; max-width: 5px; border-radius: 2px; display: inline-block; height: 100%; min-width: 2px; }
.hb-up { background: #67c23a; }
.hb-down { background: #f56c6c; }
.hb-maint { background: #909399; }
.mon-stats { display: flex; gap: 14px; text-align: center; flex-shrink: 0; }
.stat { width: 62px; }
.stat-val { font-weight: 600; }
.stat-lbl { font-size: 11px; color: #909399; }
/** Template settings left-side items */
.tpl-item {
  padding: 10px 14px; cursor: pointer; border-radius: 4px; font-size: 14px;
  border: 1px solid transparent;
}
.tpl-item:hover { background: var(--el-fill-color-light); }
.tpl-item.active {
  background: var(--el-color-primary-light-9); color: var(--el-color-primary);
  font-weight: 600; border-color: var(--el-color-primary-light-7);
}
@media (max-width: 1100px) { .hb { display: none; } }
</style>

<style>
/* ---- Monthly ops report layout (follows light/dark theme; forced light when printing) ---- */
#monthly-report {
  max-width: 980px; margin: 0 auto; background: var(--el-bg-color);
  padding: 36px 48px 40px; box-shadow: var(--el-box-shadow-light);
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 4px; color: var(--el-text-color-primary);
}
#monthly-report h2 { font-size: 22px; letter-spacing: 3px; color: var(--el-text-color-primary); }
#monthly-report h3 {
  font-size: 16px; margin: 26px 0 12px; padding-left: 10px;
  border-left: 4px solid #409eff; line-height: 1.3; color: var(--el-text-color-primary);
}
#monthly-report h4 { font-size: 13px; margin: 16px 0 8px; color: var(--el-text-color-regular); }
#monthly-report .el-table { margin-bottom: 10px; }
#monthly-report .el-table th.el-table__cell {
  background: var(--el-fill-color-light) !important; color: var(--el-text-color-primary); font-weight: 600;
}
.rep-grid { display: flex; flex-wrap: wrap; gap: 12px; margin: 0 0 14px; }
.rep-stat {
  flex: 1; min-width: 118px; background: var(--el-fill-color-light); border: 1px solid var(--el-border-color-lighter);
  border-radius: 6px; padding: 12px 8px; text-align: center;
}
.rep-stat b { display: block; font-size: 26px; color: #409eff; line-height: 1.2; }
.rep-stat span { font-size: 12px; color: #909399; }
.rep-foot { margin-top: 24px; text-align: center; color: #c0c4cc; font-size: 11px; }

@media print {
  body * { visibility: hidden; }
  #monthly-report, #monthly-report * {
    background: #ffffff !important; color: #1d2935 !important;
    border-color: #dcdfe6 !important; box-shadow: none !important;
  }
  #monthly-report .el-table th.el-table__cell { background: #f5f7fa !important; color: #1d2935 !important; }
  #monthly-report, #monthly-report * { visibility: visible; }
  #monthly-report {
    position: absolute; left: 0; top: 0; width: 100%;
    padding: 0; box-shadow: none; max-width: none;
  }
  * { -webkit-print-color-adjust: exact; print-color-adjust: exact; }
  #monthly-report h3 { break-after: avoid-page; }
  #monthly-report .el-table { break-inside: avoid; font-size: 11px; }
}
</style>
