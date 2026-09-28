<!-- JNexus Ops Platform — By JJ Zhang, Version 1.0 -->
<template>
  <div>
    <div style="display:flex; align-items:center; gap:10px; margin-bottom:10px">
      <span style="flex:1"></span>
      <el-button size="small" type="warning" plain @click="$router.push('/screen')">{{ $t('monitor.bigScreen') }} →</el-button>
    </div>
    <el-tabs v-model="activeTab">
      <!-- Tab 1: CMD monitoring (CPU / memory / disk) -->
      <el-tab-pane v-if="mcan.host" :label="$t('monitor.tabCmd')" name="cmd">
        <el-card>
          <template #header>
            <div style="display:flex; align-items:center; gap:10px">
              <span style="flex:1">{{ $t('monitor.hostRes') }}</span>
              <el-select v-model="groupFilter" size="small" style="width:170px" clearable :placeholder="$t('monitor.allGroups')">
                <el-option v-for="g in groupOptions" :key="g" :label="g" :value="g" />
              </el-select>
              <el-input v-model="hostKw" size="small" style="width:200px" clearable :placeholder="$t('tasks.searchOutput')" />
              <el-select v-model="timeRange" size="small" style="width:130px" clearable :placeholder="$t('monitor.allTime')">
                <el-option :label="$t('monitor.range24h')" value="24" />
                <el-option :label="$t('monitor.range7d')" value="168" />
                <el-option :label="$t('monitor.range1m')" value="720" />
                <el-option :label="$t('monitor.range6m')" value="4320" />
              </el-select>
            </div>
          </template>
          <el-table :data="pagedHosts" v-loading="hostsLoading" size="small" border
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
          <div class="list-pager">
            <el-pagination v-model:current-page="hostPage" v-model:page-size="hostPageSize"
                           :total="filteredHosts.length" :page-sizes="[20, 50, 100]"
                           layout="total, sizes, prev, pager, next" small background />
          </div>
        </el-card>
      </el-tab-pane>

      <!-- Tab 2: application monitoring -->
      <el-tab-pane v-if="mcan.app" :label="$t('monitor.tabApp')" name="app">
        <el-card>
          <template #header>
            <div style="display:flex; align-items:center; gap:10px">
              <span style="flex:1">{{ $t('monitor.appMon') }}</span>
              <span v-if="!mcan.manage" style="color:#909399; font-size:12px">{{ $t('monitor.appSelfTip') }}</span>
              <el-button size="small" :loading="loading" @click="load">{{ $t('common.refresh') }}</el-button>
              <el-button size="small" type="primary" @click="openDlg()">{{ $t('monitor.addMon') }}</el-button>
            </div>
          </template>
          <div v-if="!monitors.length" style="color:#909399; padding:12px 0">{{ $t('monitor.noData') }}</div>
          <div v-else class="mon-split">
            <div class="mon-nav">
              <div v-for="g in monNav" :key="g.key" class="mon-nav-item" :class="{ active: selGroup === g.key }" @click="selGroup = g.key">
                <span class="mon-nav-name" :title="g.name">{{ g.name }}</span>
                <span class="mon-nav-count">{{ g.total }}</span>
                <span v-if="g.down" class="mon-nav-down" :title="$t('monitor.monOffline') + ' ' + g.down">{{ g.down }}</span>
              </div>
            </div>
            <div class="mon-list">
              <div class="mon-list-bar">
                <el-radio-group v-model="statusFilter" size="small">
                  <el-radio-button value="all">{{ $t('monitor.monAll') }}</el-radio-button>
                  <el-radio-button value="up">{{ $t('monitor.monOnline') }}</el-radio-button>
                  <el-radio-button value="down">{{ $t('monitor.monOffline') }}</el-radio-button>
                </el-radio-group>
              </div>
              <div v-if="!selRows.length" class="mon-empty">{{ $t('monitor.monNoMatch') }}</div>
              <div v-for="row in selRows" :key="row.monitor.id" class="mon-row">
          <span class="dot" :class="statusClass(row)"></span>
          <div style="flex:1; min-width:0">
            <div style="font-weight:600">
              {{ row.monitor.name }}
              <el-tag size="small" type="info" style="margin-left:6px">{{ typeLabel(row.monitor.type) }}</el-tag>
              <el-tag v-if="!row.monitor.enabled" size="small" type="warning" style="margin-left:6px">{{ $t('monitor.paused') }}</el-tag>
            <el-tag v-if="row.monitor.cert_not_after" size="small" :type="certBadge(row.monitor.cert_not_after).type"
                      style="margin-left:6px">{{ $t('monitor.certDaysLeft') }} {{ certDaysLeft(row.monitor.cert_not_after) }}{{ $t('monitor.days') }}</el-tag></div>
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
          </div>
          </div>
        </el-card>
      </el-tab-pane>

      <!-- Tab: security logs (security department): Windows Security events /
           DB audit / alert events over whitelisted streams -->
      <el-tab-pane v-if="mcan.sec" :label="$t('monitor.tabSec')" name="sec">
        <el-card>
          <div style="display:flex; gap:8px; flex-wrap:wrap; align-items:center; margin-bottom:10px">
            <el-select v-model="sec.stream" style="width:190px" @change="searchSec">
              <el-option value="windows_events" :label="$t('monitor.secWinEvents')" />
              <el-option value="linux_events" :label="$t('monitor.secLinuxEvents')" />
              <el-option value="db_audit" :label="$t('monitor.secDbAudit')" />
              <el-option value="alert_events" :label="$t('monitor.secAlertEvents')" />
            </el-select>
            <el-select v-model="sec.hours" style="width:110px" @change="searchSec">
              <el-option value="1" label="1h" />
              <el-option value="24" label="24h" />
              <el-option value="168" label="7d" />
              <el-option value="720" label="30d" />
            </el-select>
            <el-input v-model="sec.host" :placeholder="$t('monitor.secHost')" clearable style="width:150px" />
            <el-input v-if="sec.stream === 'windows_events'" v-model="sec.eventId"
                      :placeholder="$t('monitor.secEventId')" clearable style="width:110px" />
            <el-input v-model="sec.keyword" :placeholder="$t('monitor.secKeyword')" clearable style="width:150px" />
            <el-button type="primary" :loading="sec.busy" @click="searchSec">{{ $t('common.search') }}</el-button>
            <span style="flex:1"></span>
            <el-button :disabled="!secRows.length" @click="exportSecCsv">{{ $t('oo.exportCsv') }}</el-button>
          </div>
          <el-alert v-if="sec.err" type="warning" :title="sec.err" :closable="false" show-icon style="margin-bottom:10px" />
          <el-table v-if="secCols.length" :data="secRows" size="small" border max-height="520">
            <el-table-column :label="$t('oo.time')" width="165">
              <template #default="{ row }">{{ secTime(row) }}</template>
            </el-table-column>
            <el-table-column v-for="col in secCols" :key="col" :prop="col" :label="col" min-width="140" show-overflow-tooltip>
              <template #default="{ row }">{{ fmtSecCell(row[col]) }}</template>
            </el-table-column>
          </el-table>
          <el-empty v-else-if="!sec.busy" :description="$t('oo.empty')" :image-size="80" />
        </el-card>
      </el-tab-pane>

      <!-- Tab 3: alert settings (notification channels) -->
      <el-tab-pane v-if="mcan.manage" :label="$t('monitor.tabAlert')" name="alert">
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

        <!-- security-event push channels (system config sec_alert_channels) -->
        <el-card style="margin-top:16px">
          <template #header>
            <div style="display:flex; align-items:center; gap:10px">
              <span style="flex:1">{{ $t('monitor.secChannels') }}</span>
              <el-button size="small" type="primary" :loading="secChSaving" @click="saveSecChannels">{{ $t('common.save') }}</el-button>
            </div>
          </template>
          <el-select v-model="secChannelIds" multiple style="width:100%">
            <el-option v-for="ch in channels" :key="ch.id" :label="`${ch.name}（${chTypeLabel(ch.type)}）`" :value="ch.id" />
          </el-select>
          <div style="color:#909399; font-size:12px; margin-top:8px">{{ $t('monitor.secChannelsTip') }}</div>
        </el-card>
      </el-tab-pane>

      <!-- Tab 4: alert rules (global) -->
      <el-tab-pane v-if="mcan.manage" :label="$t('monitor.tabRules')" name="rules">
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
            <el-form-item :label="$t('monitor.certWarnDays')">
              <el-input-number v-model="alertRule.cert_warn_days" :min="1" :max="365" />
              <div style="color:#909399; font-size:12px; margin-left:10px">{{ $t('monitor.certWarnTip') }}</div>
            </el-form-item>
            <el-form-item :label="$t('monitor.certCritDays')">
              <el-input-number v-model="alertRule.cert_crit_days" :min="1" :max="365" />
              <div style="color:#909399; font-size:12px; margin-left:10px">{{ $t('monitor.certCritTip') }}</div>
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

      <!-- Tab: AI alert diagnostics (admin only): gate config + controlled cleanup catalog.
           Lives next to the alert rules it extends (levels/metrics/channels) so the whole
           alert pipeline — threshold → level → channel → AI diagnosis — sits in one place. -->
      <el-tab-pane v-if="isAdmin" :label="$t('monitor.tabAiDiag')" name="aidiag">
        <el-card>
          <div style="font-weight:600; font-size:15px; margin-bottom:6px">{{ $t('ai.diagTitle') }}</div>
          <div style="color:var(--el-text-color-secondary); font-size:12px; margin-bottom:14px">{{ $t('ai.diagTip') }}</div>
          <el-alert type="info" :title="$t('monitor.aiDiagDepTip')" :closable="false" style="margin-bottom:14px" />
          <el-form label-width="130px" style="max-width:560px">
            <el-form-item :label="$t('ai.diagEnabled')"><el-switch v-model="diagCfg.ai_diag_enabled" active-value="true" inactive-value="false" /></el-form-item>
            <el-form-item :label="$t('ai.diagLevels')">
              <el-checkbox-group v-model="diagLevels">
                <el-checkbox value="P1">P1</el-checkbox>
                <el-checkbox value="P2">P2</el-checkbox>
                <el-checkbox value="P3">P3</el-checkbox>
                <el-checkbox value="P4">P4</el-checkbox>
              </el-checkbox-group>
            </el-form-item>
            <el-form-item :label="$t('ai.diagMetrics')">
              <el-checkbox-group v-model="diagMetrics">
                <el-checkbox value="disk">{{ $t('monitor.disk') }}</el-checkbox>
                <el-checkbox value="cpu">CPU</el-checkbox>
                <el-checkbox value="mem">{{ $t('monitor.mem') }}</el-checkbox>
              </el-checkbox-group>
            </el-form-item>
            <el-form-item :label="$t('ai.diagCooldown')">
              <el-input-number v-model="diagCooldown" :min="0" :max="1440" :step="5" />
              <span style="margin-left:8px; color:var(--el-text-color-secondary); font-size:12px">{{ $t('ai.diagCooldownUnit') }}</span>
            </el-form-item>
            <el-form-item :label="$t('ai.diagPrompt')">
              <el-input v-model="diagPrompt" type="textarea" :rows="5" class="mono"
                        :placeholder="$t('ai.diagPromptPlaceholder')" />
              <div style="color:var(--el-text-color-secondary); font-size:12px; margin-top:4px">{{ $t('ai.diagPromptTip') }}</div>
            </el-form-item>
            <el-form-item>
              <el-button type="primary" :loading="diagSaving" @click="saveDiagCfg">{{ $t('common.save') }}</el-button>
            </el-form-item>
          </el-form>

          <el-divider style="margin:14px 0" />
          <div style="display:flex; align-items:center; gap:10px; margin-bottom:10px">
            <span style="font-weight:600">{{ $t('ai.cleanupTitle') }}</span>
            <span style="flex:1"></span>
            <el-button size="small" @click="cleanupDlg()">{{ $t('ai.cleanupAdd') }}</el-button>
          </div>
          <el-table :data="cleanupItems" size="small" border>
            <el-table-column prop="name" :label="$t('ai.cleanupName')" width="180" />
            <el-table-column prop="command" :label="$t('ai.cleanupCommand')" min-width="240">
              <template #default="{ row }"><span class="mono">{{ row.command }}</span></template>
            </el-table-column>
            <el-table-column :label="$t('oa.enabledCol')" width="80" align="center">
              <template #default="{ row }"><el-switch v-model="row.enabled" /></template>
            </el-table-column>
            <el-table-column :label="$t('common.actions')" width="120" fixed="right">
              <template #default="{ row }">
                <el-button size="small" link type="primary" @click="cleanupDlg(row)">{{ $t('common.edit') }}</el-button>
                <el-button size="small" link type="danger" @click="cleanupDel(row.id)">{{ $t('common.delete') }}</el-button>
              </template>
            </el-table-column>
          </el-table>
          <div style="color:var(--el-text-color-secondary); font-size:12px; margin-top:8px">{{ $t('ai.cleanupTip') }}</div>
        </el-card>
      </el-tab-pane>

      <!-- Tab: maintenance windows (global, calendar date range) -->
      <el-tab-pane v-if="mcan.manage" :label="$t('monitor.tabMaint')" name="maint">
        <el-card>
          <template #header>
            <div style="display:flex; align-items:center; gap:10px">
              <span style="flex:1">{{ $t('monitor.maintTitle') }}</span>
              <el-button size="small" :loading="maintAuditLoading" @click="loadMaintAudit">{{ $t('common.refresh') }}</el-button>
              <el-button size="small" type="primary" :loading="maintSaving" @click="saveMaintWindows">{{ $t('common.save') }}</el-button>
            </div>
          </template>
          <div style="color:#909399; font-size:12px; margin-bottom:6px">{{ $t('monitor.maintTip') }}</div>
          <el-alert :type="maintStatus.in_window ? 'warning' : 'info'"
                    :title="maintStatus.in_window ? $t('monitor.maintNowActive') : $t('monitor.maintNowIdle')"
                    :closable="false" show-icon style="margin-bottom:10px" />
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
              <el-button v-if="isAdmin" size="small" type="danger" link @click="clearMaintLogs">{{ $t('monitor.maintLogClear') }}</el-button>
              <el-button size="small" :loading="maintAuditLoading" @click="loadMaintAudit">{{ $t('common.refresh') }}</el-button>
            </div>
          </template>
          <div style="color:#909399; font-size:12px; margin-bottom:10px">{{ $t('monitor.maintAuditTip') }}</div>
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
                <el-tag size="small" :type="row.active ? 'success' : 'info'">{{ row.active ? $t('monitor.maintTagCur') : $t('monitor.maintTagHist') }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column :label="$t('audit.time')" width="170">
              <template #default="{ row }">{{ fmtTime(row.created_at) }}</template>
            </el-table-column>
            <el-table-column v-if="isAdmin" :label="$t('common.actions')" width="80" align="center">
              <template #default="{ row }">
                <el-button size="small" link type="danger" @click="delMaintLog(row.id)">{{ $t('common.delete') }}</el-button>
              </template>
            </el-table-column>
          </el-table>
          <div v-if="!maintAudit.length" style="color:#909399; padding:8px 0">{{ $t('monitor.maintAuditEmpty') }}</div>
        </el-card>
      </el-tab-pane>

      <!-- Tab 5: template settings (list on the left, editor on the right) -->
      <el-tab-pane v-if="mcan.manage" :label="$t('monitor.tabTpl')" name="templates">
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
      <el-tab-pane v-if="mcan.manage" :label="$t('mreport.tab')" name="report">
        <el-card>
          <div class="rep-toolbar" style="display:flex; gap:10px; align-items:center; margin-bottom:14px">
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
        <el-form-item :label="$t('monitor.monGroup')">
          <el-select v-model="form.mon_group" filterable allow-create clearable :placeholder="$t('monitor.monGroupPh')" style="width:100%">
            <el-option v-for="g in monGroupOptions" :key="g" :label="g" :value="g" />
          </el-select>
        </el-form-item>
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

    <!-- AI diag cleanup command dialog (admin) -->
    <el-dialog v-model="cleanupDlgVisible" :title="cleanupForm.id ? $t('ai.cleanupEdit') : $t('ai.cleanupAdd')" width="520px">
      <el-form label-width="90px">
        <el-form-item :label="$t('ai.cleanupName')" required>
          <el-input v-model="cleanupForm.name" placeholder="清理 /tmp 7 天未访问文件" />
        </el-form-item>
        <el-form-item :label="$t('ai.cleanupCommand')" required>
          <el-input v-model="cleanupForm.command" type="textarea" :rows="2" class="mono"
                    placeholder="find /tmp -xdev -type f -atime +7 -delete" />
          <div style="color:var(--el-text-color-secondary); font-size:12px; margin-top:4px">{{ $t('ai.cleanupCommandTip') }}</div>
        </el-form-item>
        <el-form-item :label="$t('oa.enabledCol')">
          <el-switch v-model="cleanupForm.enabled" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="cleanupDlgVisible = false">{{ $t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="cleanupSaving" @click="cleanupSave">{{ $t('common.save') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import api from '../api'
import { useRouter } from 'vue-router'
import i18n from '../i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import MetricChart from '../components/MetricChart.vue'
import { useUserStore } from '../store'

const { t } = i18n.global
const $router = useRouter()
const store = useUserStore()
const isAdmin = computed(() => store.role === 'admin')

// department view: split monitor permissions from the role capability matrix
const roleConf = ref(null)
api.get('/system/roles').then(rs => {
  roleConf.value = rs
  // permission-scoped loads: only fetch what this role can actually see
  if (mcan.value.host) loadHosts()
  if (mcan.value.manage) {
    loadAlertRule()
    loadAlertTemplates()
    loadCmdLevels()
    loadMaintWindows()
    loadMaintAudit()
    loadMaintStatus()
    loadSecChannels()
  }
}).catch(() => {})
const mcan = computed(() => {
  if (store.isAdmin) return { host: true, app: true, sec: true, manage: true }
  const acts = roleConf.value?.[store.role]?.perms?.monitor || []
  const has = a => acts.includes(a)
  const manage = has('manage')
  return {
    host: has('view_host') || manage,
    app: has('view_app') || manage,
    sec: has('view_sec') || manage,
    manage,
  }
})
const activeTab = ref('cmd')
watch(mcan, m => {
  if (!m.host && activeTab.value === 'cmd') activeTab.value = m.app ? 'app' : (m.sec ? 'sec' : (m.manage ? 'alert' : 'report'))
}, { immediate: true })

// Auto-load the monthly report the first time the tab is opened (was: empty until Generate clicked);
// AI diag config loads once when its tab is first opened
watch(activeTab, tab => {
  if (tab === 'report' && !rep.value && !repLoading.value) loadReport()
  if (tab === 'aidiag' && !diagLoaded.value) loadDiagCfg()
})

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
const groupedMonitors = computed(() => {
  const order = []
  const map = {}
  for (const row of monitors.value) {
    const g = row.monitor.mon_group || t('monitor.defGroup')
    if (!map[g]) { map[g] = []; order.push(g) }
    map[g].push(row)
  }
  return order.map(name => ({ name, rows: map[name] }))
})
// left nav (split pane): "all" entry + one entry per group with total / offline counts
const ALL_GROUP = '__all__'
const selGroup = ref(ALL_GROUP)
const statusFilter = ref('all')
const offlineOf = rows => rows.filter(r => r.monitor.enabled && r.monitor.last_status === 'down').length
const monNav = computed(() => [
  { key: ALL_GROUP, name: t('monitor.monAll'), total: monitors.value.length, down: offlineOf(monitors.value) },
  ...groupedMonitors.value.map(g => ({ key: g.name, name: g.name, total: g.rows.length, down: offlineOf(g.rows) })),
])
const selRows = computed(() => {
  const rows = selGroup.value === ALL_GROUP
    ? monitors.value
    : (groupedMonitors.value.find(g => g.name === selGroup.value)?.rows || [])
  if (statusFilter.value === 'up') return rows.filter(r => r.monitor.enabled && r.monitor.last_status === 'up')
  if (statusFilter.value === 'down') return rows.filter(r => r.monitor.enabled && r.monitor.last_status === 'down')
  return rows
})
const monGroupOptions = computed(() => {
  const set = new Set()
  for (const row of monitors.value) if (row.monitor.mon_group) set.add(row.monitor.mon_group)
  return [...set]
})
const timeRange = ref('')
const hostPage = ref(1)
const hostPageSize = ref(20)
const pagedHosts = computed(() => {
  const start = (hostPage.value - 1) * hostPageSize.value
  return filteredHosts.value.slice(start, start + hostPageSize.value)
})
watch(() => filteredHosts.value.length, n => {
  const maxPage = Math.max(1, Math.ceil(n / hostPageSize.value))
  if (hostPage.value > maxPage) hostPage.value = maxPage
})
const filteredHosts = computed(() => {
  let list = hostRows.value
  if (groupFilter.value) list = list.filter(h => h.group === groupFilter.value)
  const kw = hostKw.value.trim().toLowerCase()
  if (kw) list = list.filter(h => `${h.name} ${h.ip} ${h.group || ''}`.toLowerCase().includes(kw))
  if (timeRange.value) {
    const cut = Date.now() - Number(timeRange.value) * 3600000
    list = list.filter(h => h.collected_at && new Date(h.collected_at).getTime() >= cut)
  }
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
  hostsLoading.value = mcan.value.host
  try {
    const reqs = [api.get('/monitors'), api.get('/alert_channels')]
    if (mcan.value.host) reqs.push(api.get('/monitoring/hosts'))
    const [ms, chs, hs] = await Promise.all(reqs)
    monitors.value = ms
    channels.value = chs
    if (mcan.value.host) hostRows.value = hs
  } finally {
    hostsLoading.value = false
    loading.value = false
  }
}
const loadChannels = async () => { channels.value = await api.get('/alert_channels') }
const loadHosts = async () => {
  try { hostRows.value = await api.get('/monitoring/hosts') } catch { /* view_host required */ }
}

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
  // start from a blank record: reactive form persists between dialogs, and a
  // leftover id (or any stale field) would turn "new" into "overwrite the
  // previously edited monitor"
  for (const k of Object.keys(form)) delete form[k]
  Object.assign(form, {
    type: m?.type || 'http', method: m?.method || 'GET',
    accepted_status: m?.accepted_status || '200-299',
    keyword_type: m?.keyword_type || 'contain',
    interval_sec: m?.interval_sec || 60, timeout_sec: m?.timeout_sec || 10,
    enabled: m ? !!m.enabled : true, port: m?.port || 80,
    mon_group: m?.mon_group || '',
    channel_ids: [...(channelIds || [])],
  })
  if (m) {
    // only copy the editable fields of an existing monitor - never its id
    const { id, created_at, last_status, last_error, last_resp_ms, last_checked_at,
            down_since, alert_fired, next_run_at, cert_not_after, ...rest } = m
    Object.assign(form, rest)
    form.enabled = !!m.enabled
  }
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
// live "inside a window right now?" indicator (distinct from the audit rows' current-config tag)
const maintStatus = ref({ in_window: false })
const loadMaintStatus = async () => {
  try { maintStatus.value = await api.get('/maintenance_windows/status') } catch { /* ignore */ }
}
const delMaintLog = async id => {
  try {
    await ElMessageBox.confirm(t('monitor.maintLogDelConfirm'), t('common.tip'), { type: 'warning' })
  } catch { return }
  try {
    await api.delete(`/maintenance_windows/logs/${id}`)
    loadMaintAudit()
  } catch { /* interceptor shows the error */ }
}
const clearMaintLogs = async () => {
  try {
    await ElMessageBox.confirm(t('monitor.maintLogClearConfirm'), t('common.tip'), { type: 'warning' })
  } catch { return }
  try {
    await api.delete('/maintenance_windows/logs')
    loadMaintAudit()
  } catch { /* interceptor shows the error */ }
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
const certDaysLeft = na => Math.ceil((new Date(na).getTime() - Date.now()) / 86400000)
const certBadge = na => {
  const d = certDaysLeft(na)
  if (d < 0) return { type: 'danger' }
  if (d <= 7) return { type: 'danger' }
  if (d <= 30) return { type: 'warning' }
  return { type: 'success' }
}

const alertRule = reactive({ mode: 'grace', grace_sec: 60, notify_recovery: true, cert_warn_days: 30, cert_crit_days: 7 })
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

// 打印：克隆月报到独立打印根节点（与页面 DOM 完全隔离，避免嵌套容器裁剪与样式污染）
let printRoot = null
const buildPrintClone = () => {
  const report = document.getElementById('monthly-report')
  if (!report) return
  printRoot = document.createElement('div')
  printRoot.id = 'print-root'
  printRoot.appendChild(report.cloneNode(true))
  document.body.appendChild(printRoot)
  document.documentElement.classList.add('printing-report')
}
const teardownPrintClone = () => {
  document.documentElement.classList.remove('printing-report')
  printRoot?.remove()
  printRoot = null
}
window.addEventListener('beforeprint', buildPrintClone)
window.addEventListener('afterprint', teardownPrintClone)

// ---- security logs (security department view) ----
const sec = reactive({ stream: 'windows_events', hours: '24', host: '', eventId: '', keyword: '', busy: false, err: '' })
const secRows = ref([])
const secCols = ref([])
const searchSec = async () => {
  sec.busy = true
  sec.err = ''
  try {
    const end = Date.now(), start = end - Number(sec.hours) * 3600 * 1000
    const r = await api.post('/monitors/sec/logs', {
      stream: sec.stream, start_ms: start, end_ms: end,
      host: sec.host.trim(), event_id: Number(sec.eventId) || 0,
      keyword: sec.keyword.trim(), size: 300,
    })
    const hits = r.hits || []
    secRows.value = hits
    const cols = new Set()
    for (const h of hits.slice(0, 50)) Object.keys(h).forEach(k => { if (k !== '_timestamp') cols.add(k) })
    secCols.value = [...cols]
    if (!hits.length) sec.err = t('oo.emptyResult')
  } catch (e) {
    secRows.value = []
    secCols.value = []
    sec.err = e?.response?.data?.error || t('oo.notEnabled')
  } finally { sec.busy = false }
}
const secTime = row => {
  const n = Number(row._timestamp)
  return isNaN(n) || n <= 0 ? '-' : new Date(n / 1000).toLocaleString()
}
const fmtSecCell = v => v === null || v === undefined || v === '' ? '-' : (typeof v === 'object' ? JSON.stringify(v) : String(v))
const exportSecCsv = () => {
  const csvCell = s => /[",\n]/.test(s) ? '"' + s.replace(/"/g, '""') + '"' : s
  const head = secCols.value.map(csvCell).join(',')
  const body = secRows.value.map(r => secCols.value.map(col => csvCell(fmtSecCell(r[col]))).join(',')).join('\n')
  const blob = new Blob(['\ufeff' + head + '\n' + body], { type: 'text/csv;charset=utf-8' })
  const a = document.createElement('a')
  a.href = URL.createObjectURL(blob)
  a.download = `security-logs-${sec.stream}.csv`
  a.click()
  URL.revokeObjectURL(a.href)
}

// ---- security-event push channels (system config) ----
const secChannelIds = ref([])
const secChSaving = ref(false)
const loadSecChannels = async () => {
  try {
    const cfg = await api.get('/system/config')
    secChannelIds.value = (cfg.sec_alert_channels || '').split(',').map(x => Number(x.trim())).filter(Boolean)
  } catch { secChannelIds.value = [] }
}
const saveSecChannels = async () => {
  secChSaving.value = true
  try {
    await api.put('/system/config', { sec_alert_channels: secChannelIds.value.join(',') })
    ElMessage.success(t('common.success'))
  } catch { /* interceptor shows the error */ } finally { secChSaving.value = false }
}

onMounted(() => {
  load()
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

onUnmounted(() => {
  clearInterval(timer)
  window.removeEventListener('beforeprint', buildPrintClone)
  window.removeEventListener('afterprint', teardownPrintClone)
  teardownPrintClone()
})

// ---- AI alert diagnostics (admin): gate config + controlled cleanup catalog ----
const diagLoaded = ref(false)
const diagCfg = ref({ ai_diag_enabled: 'false', ai_diag_levels: 'P1,P2', ai_diag_metrics: 'disk', ai_diag_cooldown_min: '30' })
const diagLevels = ref(['P1', 'P2'])
const diagMetrics = ref(['disk'])
const diagCooldown = ref(30)
const diagSaving = ref(false)
const diagPrompt = ref('')
const cleanupItems = ref([])
const cleanupDlgVisible = ref(false)
const cleanupSaving = ref(false)
const cleanupForm = ref({})

const loadDiagCfg = async () => {
  try {
    const cfg = await api.get('/system/ai/diag/config')
    diagCfg.value = { ...diagCfg.value, ...cfg }
    diagLevels.value = (cfg.ai_diag_levels || '').split(',').map(x => x.trim()).filter(Boolean)
    diagMetrics.value = (cfg.ai_diag_metrics || '').split(',').map(x => x.trim()).filter(Boolean)
    diagCooldown.value = Number(cfg.ai_diag_cooldown_min) || 30
    diagPrompt.value = cfg.ai_diag_prompt || ''
    cleanupItems.value = await api.get('/system/ai/diag/cleanup') || []
    diagLoaded.value = true
  } catch { /* non-admin or backend error: tab is admin-gated anyway */ }
}

const saveDiagCfg = async () => {
  diagSaving.value = true
  try {
    await api.put('/system/ai/diag/config', {
      ai_diag_enabled: diagCfg.value.ai_diag_enabled,
      ai_diag_levels: diagLevels.value.join(','),
      ai_diag_metrics: diagMetrics.value.join(','),
      ai_diag_cooldown_min: String(diagCooldown.value),
      ai_diag_prompt: diagPrompt.value,
    })
    ElMessage.success(t('system.saved'))
  } catch { /* interceptor shows the error */ } finally { diagSaving.value = false }
}

const cleanupDlg = row => {
  cleanupForm.value = row ? { ...row } : { id: '', name: '', command: '', enabled: true }
  cleanupDlgVisible.value = true
}

const cleanupSave = async () => {
  cleanupSaving.value = true
  try {
    const list = cleanupItems.value.filter(x => x.id !== cleanupForm.value.id)
    list.push({ ...cleanupForm.value })
    await api.post('/system/ai/diag/cleanup', list)
    ElMessage.success(t('system.saved'))
    cleanupDlgVisible.value = false
    cleanupItems.value = await api.get('/system/ai/diag/cleanup') || []
  } catch { /* interceptor shows the error */ } finally { cleanupSaving.value = false }
}

const cleanupDel = async id => {
  try {
    await api.delete(`/system/ai/diag/cleanup/${encodeURIComponent(id)}`)
    cleanupItems.value = cleanupItems.value.filter(x => x.id !== id)
  } catch { /* interceptor shows the error */ }
}
</script>

<style scoped>
.list-pager { display: flex; justify-content: flex-end; margin-top: 10px; }
.mon-split { display: flex; gap: 12px; align-items: flex-start; }
.mon-nav { flex: 0 0 210px; border: 1px solid var(--el-border-color-lighter); border-radius: 6px; padding: 6px; max-height: calc(100vh - 300px); min-height: 260px; overflow: auto; }
.mon-nav-item { display: flex; align-items: center; gap: 6px; padding: 7px 10px; border-radius: 4px; cursor: pointer; font-size: 13px; }
.mon-nav-item:hover { background: var(--el-fill-color-light); }
.mon-nav-item.active { background: var(--el-color-primary-light-9); color: var(--el-color-primary); font-weight: 600; }
.mon-nav-name { flex: 1; min-width: 0; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.mon-nav-count { color: var(--el-text-color-secondary); font-size: 12px; flex-shrink: 0; }
.mon-nav-down { background: var(--el-color-danger); color: #fff; font-size: 11px; border-radius: 8px; padding: 0 6px; line-height: 16px; min-width: 16px; text-align: center; flex-shrink: 0; }
.mon-list { flex: 1; min-width: 0; max-height: calc(100vh - 300px); min-height: 260px; overflow: auto; border: 1px solid var(--el-border-color-lighter); border-radius: 6px; padding: 0 12px; }
.mon-list-bar { position: sticky; top: 0; z-index: 2; background: var(--el-bg-color, #fff); padding: 10px 0 8px; }
.mon-empty { color: var(--el-text-color-secondary); text-align: center; padding: 32px 0; }

.mon-row {
  display: flex; align-items: center; gap: 14px; padding: 10px 4px;
  border-bottom: 1px solid var(--el-border-color-lighter); min-height: 56px;
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
.stat-lbl { font-size: 11px; color: var(--el-text-color-secondary); }
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
  /* 打印方案：beforeprint 时克隆月报到独立 #print-root（body 直属），
     隐藏应用本体 —— 与页面 DOM 完全隔离，样式 100% 保留、无容器裁剪 */
  @page { size: A4 portrait; margin: 12mm; }
  html.printing-report #app { display: none !important; }
  html.printing-report, html.printing-report body {
    background: #ffffff !important;
    height: auto !important;
    overflow: visible !important;
  }
  #print-root { display: block !important; }

  /* 月报强制浅色版式：作用域级变量让所有 EP 组件在打印时呈现浅色样式 */
  #print-root #monthly-report {
    --el-bg-color: #ffffff; --el-bg-color-page: #ffffff; --el-bg-color-overlay: #ffffff;
    --el-fill-color: #f0f2f5; --el-fill-color-light: #f5f7fa; --el-fill-color-lighter: #fafafa; --el-fill-color-blank: #ffffff;
    --el-text-color-primary: #1d2935; --el-text-color-regular: #606266; --el-text-color-secondary: #909399; --el-text-color-placeholder: #a8abb2;
    --el-border-color: #dcdfe6; --el-border-color-light: #e4e7ed; --el-border-color-lighter: #ebeef5; --el-border-color-extra-light: #f2f6fc;
    --el-table-border-color: #dcdfe6; --el-table-header-bg-color: #f5f7fa; --el-table-header-text-color: #1d2935; --el-table-tr-bg-color: #ffffff;
    background: #ffffff !important;
    max-width: none !important;
    padding: 0 !important;
    box-shadow: none !important;
    border: none !important;
  }
  #print-root #monthly-report h2 { color: #1d2935 !important; }
  #print-root #monthly-report .rep-stat { background: #f5f7fa !important; border: 1px solid #e4e7ed !important; }
  #print-root #monthly-report .rep-stat b { color: #409eff !important; }
  #print-root #monthly-report .rep-stat span { color: #606266 !important; }
  #print-root #monthly-report .rep-foot { color: #909399 !important; }
  #print-root #monthly-report .el-table { font-size: 11px; break-inside: avoid; }
  #print-root #monthly-report .el-table th.el-table__cell { background: #f5f7fa !important; color: #1d2935 !important; }
  #print-root #monthly-report h3 { break-after: avoid-page; color: #1d2935 !important; }
  * { -webkit-print-color-adjust: exact; print-color-adjust: exact; }
}
</style>
