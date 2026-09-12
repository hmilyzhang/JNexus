// 一次性脚本：SystemConfig.vue 轮换 tab 加适用账号表格 + 立即轮换（执行后删除）
const fs = require('fs');
const f = 'D:/AutoOps/frontend/src/views/SystemConfig.vue';
let s = fs.readFileSync(f, 'utf8');

// ---- 1) 模板：表单卡片后插入适用账号面板 ----
const anchor = `      <el-button type="primary" :loading="saving" @click="save">{{ $t('common.save') }}</el-button>
    </el-card>
    </el-tab-pane>`;
if (!s.includes(anchor)) { console.log('TPL ANCHOR NOT FOUND'); process.exit(1); }
const tplAdd = `      <el-button type="primary" :loading="saving" @click="save">{{ $t('common.save') }}</el-button>
    </el-card>

    <el-card style="margin-top:16px">
      <template #header>
        <div style="display:flex; align-items:center; gap:10px">
          <span style="flex:1">{{ $t('rot.applicableAccts') }}（{{ rotAccounts.length }}）</span>
          <el-button size="small" @click="loadRotAccounts">{{ $t('common.refresh') }}</el-button>
          <el-button size="small" type="warning" :loading="rotNowLoading" :disabled="!rotAccounts.length"
                     @click="runAllNow">{{ $t('rot.runNow') }}</el-button>
        </div>
      </template>
      <el-table :data="rotAccounts" size="small" border max-height="420">
        <el-table-column prop="host" :label="$t('menu.hosts')" min-width="150" show-overflow-tooltip />
        <el-table-column prop="username" :label="$t('hosts.credUser')" min-width="110" />
        <el-table-column :label="$t('rot.enable')" width="80" align="center">
          <template #default="{ row }">
            <el-tag v-if="row.rotate_enabled" size="small">{{ row.days }}{{ $t('rot.daysUnit') }}</el-tag>
            <span v-else style="color:var(--el-text-color-secondary)">—</span>
          </template>
        </el-table-column>
        <el-table-column :label="$t('rot.due')" width="80" align="center">
          <template #default="{ row }">
            <el-tag size="small" :type="row.due ? 'danger' : 'success'">{{ row.due ? $t('rot.dueYes') : $t('rot.dueNo') }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="$t('rot.lastRot')" min-width="150">
          <template #default="{ row }">
            <span :style="{ color: (row.last_rotation_result || '').includes('失败') ? 'var(--el-color-danger)' : 'inherit' }">
              {{ row.last_rotation_result || '—' }}
            </span>
          </template>
        </el-table-column>
      </el-table>
    </el-card>
    </el-tab-pane>`;
s = s.replace(anchor, tplAdd);

// ---- 2) 脚本：状态 + 加载 + run-now + 进度轮询 ----
const scriptAnchor = `const loadMaintAudit = async () => {`;
if (!s.includes(scriptAnchor)) { console.log('SCRIPT ANCHOR NOT FOUND'); process.exit(1); }
const scriptAdd = `// ---- 密码轮换：适用账号 + 立即全部轮换（复用异步批次） ----
const rotAccounts = ref([])
const rotAccountsLoading = ref(false)
const rotBatchId = ref('')
const rotBatchProg = ref({ done: 0, total: 0, ok: 0, failed: 0, running: false, results: [] })
const rotNowLoading = ref(false)
let rotBatchTimer = null

const loadRotAccounts = async () => {
  rotAccountsLoading.value = true
  try {
    const r = await api.get('/system/rotation/accounts')
    rotAccounts.value = r.accounts || []
  } finally { rotAccountsLoading.value = false }
}

const runAllNow = async () => {
  try {
    await ElMessageBox.confirm(t('rot.runNowConfirm'), t('rot.tab'), { type: 'warning' })
  } catch { return }
  rotNowLoading.value = true
  try {
    const r = await api.post('/system/rotation/run-now')
    rotBatchId.value = r.batch
    if (!r.batch) { ElMessage.info(t('rot.noAccounts')); return }
    rotBatchPollStop()
    rotBatchTimer = setInterval(async () => {
      try {
        const st = await api.get('/credentials/rotate-batch/' + rotBatchId.value)
        rotBatchProg.value = st
        if (!st.running) {
          rotBatchPollStop()
          rotNowLoading.value = false
          loadRotAccounts()
        }
      } catch { rotBatchPollStop() }
    }, 1500)
  } finally { rotNowLoading.value = false }
}

const loadMaintAudit = async () => {`;
s = s.replace(scriptAnchor, scriptAdd);

// ---- 3) onMounted 加载 + 清理 ----
const omAnchor = s.match(/onMounted\(\(\) => \{[\s\S]{0,300}?\}\)/);
if (!omAnchor) { console.log('onMounted NOT FOUND'); process.exit(1); }
s = s.replace(omAnchor[0], omAnchor[0].slice(0, -1) + '\n  loadRotAccounts()\n})');
s = s.replace(`import { computed, onMounted, reactive, ref } from 'vue'`, `import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'`);

fs.writeFileSync(f, s);
console.log('SystemConfig patched');
