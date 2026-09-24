<!-- JNexus Ops Platform — By JJ Zhang, Version 1.0 -->
<template>
  <div style="display:flex; gap:12px; height:100%">
    <!-- left: sources + accounts -->
    <el-card class="db-side" v-loading="loading">
      <template #header>
        <div style="display:flex; align-items:center; justify-content:space-between">
          <span style="font-weight:600">{{ $t('db.sources') }}</span>
          <div>
            <el-button v-if="store.isAdmin" size="small" type="primary" @click="srcDlg()">{{ $t('db.addSource') }}</el-button>
            <el-button size="small" text @click="load">{{ $t('common.refresh') }}</el-button>
          </div>
        </div>
      </template>
      <div v-for="s in sources" :key="s.id" class="db-src" :class="{ active: s.id === sourceId }" @click="pickSource(s)">
        <div style="display:flex; align-items:center; gap:6px">
          <el-tag size="small" type="info">{{ s.db_type }}</el-tag>
          <span style="font-weight:600">{{ s.name }}</span>
          <el-tag v-if="s.read_only" size="small" type="warning">{{ $t('db.readonly') }}</el-tag>
        </div>
        <div style="color:#909399; font-size:12px">{{ s.host }}:{{ s.port }} / {{ s.database }}</div>
      </div>
      <div v-if="!sources.length" style="color:#909399; font-size:13px">{{ $t('db.noSources') }}</div>

      <template v-if="sourceId">
        <el-divider style="margin:10px 0">{{ $t('db.accounts') }}</el-divider>
        <div v-for="a in accounts" :key="a.id" class="db-acc" :class="{ active: accountId === a.id }" @click="accountId = a.id">
          <el-radio :model-value="accountId" :label="a.id" style="margin-right:6px"><span></span></el-radio>
          <span style="flex:1">{{ a.username }}<span v-if="a.label" style="color:#909399">（{{ a.label }}）</span></span>
          <el-button v-if="store.isAdmin" size="small" link type="danger" @click.stop="delAccount(a)">{{ $t('common.delete') }}</el-button>
        </div>
        <el-button v-if="store.isAdmin" size="small" style="width:100%; margin-top:6px" @click="accDlg()">{{ $t('db.addAccount') }}</el-button>
        <div v-if="store.isAdmin" style="margin-top:8px">
          <el-button size="small" style="width:100%" @click="guardDlgVisible = true">{{ $t('db.guardrails') }}</el-button>
        </div>
      </template>
    </el-card>

    <!-- right: SQL editor + results -->
    <el-card class="db-main">
      <div ref="editorRef" class="sql-editor mono"></div>
      <div style="display:flex; align-items:center; gap:10px; margin:8px 0">
        <el-button type="primary" :loading="running" :disabled="!sourceId || !accountId" @click="run">
          {{ $t('db.run') }} (Ctrl+Enter)
        </el-button>
        <el-button :disabled="!result" @click="exportCsv">{{ $t('db.exportCsv') }}</el-button>
        <span v-if="result" style="color:#909399; font-size:12px">
          {{ result.elapsed_ms }} ms ·
          {{ result.truncated ? $t('db.truncated') : '' }}
          {{ result.affected ? $t('db.affected') + ': ' + result.affected : (result.rows ? $t('db.rows') + ': ' + result.rows.length : '') }}
        </span>
        <span style="flex:1"></span>
        <el-select v-model="histPick" size="small" :placeholder="$t('db.history')" style="width:220px" @change="h => setEditorText(h)" clearable>
          <el-option v-for="h in history" :key="h" :label="h.slice(0, 60)" :value="h" />
        </el-select>
      </div>
      <el-alert v-if="lastError" type="error" :closable="true" @close="lastError = ''"
                style="margin-bottom:10px">
        <div class="db-err">{{ lastError }}</div>
      </el-alert>
      <el-table v-if="result && result.columns" :data="tableRows" size="small" border stripe max-height="480">
        <el-table-column type="index" :index="i => i + 1" width="52" :label="'#'" />
        <el-table-column v-for="(c, i) in result.columns" :key="i" :prop="'c' + i"
                         :align="isNumericType(c.type) ? 'right' : 'left'" min-width="130" show-overflow-tooltip>
          <template #header>
            <span class="col-name">{{ c.name }}</span>
            <span class="col-type">{{ shortType(c.type) }}</span>
          </template>
          <template #default="{ row }">
            <span v-if="row['c' + i] === null" class="cell-null">NULL</span>
            <span v-else class="mono">{{ row['c' + i] }}</span>
          </template>
        </el-table-column>
      </el-table>
      <div v-else-if="result" style="color:#909399">{{ $t('db.noRows') }}</div>
    </el-card>

    <!-- account dialog (admin) -->
    <el-dialog v-model="accVisible" :title="accForm.id ? $t('common.edit') : $t('db.addAccount')" width="460px">
      <el-form label-width="100px">
        <el-form-item :label="$t('hosts.credUser')"><el-input v-model="accForm.username" class="mono" /></el-form-item>
        <el-form-item :label="$t('hosts.password')">
          <el-input v-model="accForm.password" type="password" show-password class="mono"
                    :placeholder="accForm.id ? $t('webapp.pwdKeep') : ''" />
        </el-form-item>
        <el-form-item :label="$t('webapp.name')"><el-input v-model="accForm.label" /></el-form-item>
        <el-form-item :label="$t('db.allowedGroups')">
          <el-select v-model="accGroupIds" multiple style="width:100%" :placeholder="$t('db.allGroups')">
            <el-option v-for="g in userGroups" :key="g.id" :label="g.name" :value="g.id" />
          </el-select>
          <div style="color:#909399; font-size:12px; margin-top:4px">{{ $t('db.allowedGroupsTip') }}</div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="accVisible = false">{{ $t('common.cancel') }}</el-button>
        <el-button type="primary" @click="saveAccount">{{ $t('common.save') }}</el-button>
      </template>
    </el-dialog>

    <!-- guardrails dialog (admin) -->
    <el-dialog v-model="guardDlgVisible" :title="$t('db.guardrails')" width="420px">
      <el-form label-width="130px">
        <el-form-item :label="$t('db.readonly')"><el-switch v-model="guardForm.read_only" /></el-form-item>
        <el-form-item :label="$t('db.timeoutSec')"><el-input-number v-model="guardForm.timeout_sec" :min="5" :max="600" /></el-form-item>
        <el-form-item :label="$t('db.maxRows')"><el-input-number v-model="guardForm.max_rows" :min="1" :max="100000" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="guardDlgVisible = false">{{ $t('common.cancel') }}</el-button>
        <el-button type="primary" @click="saveGuard">{{ $t('common.save') }}</el-button>
      </template>
    </el-dialog>

    <!-- new source dialog (admin) -->
    <el-dialog v-model="srcVisible" :title="$t('db.addSource')" width="460px">
      <el-form label-width="90px">
        <el-form-item :label="$t('webapp.name')"><el-input v-model="srcForm.name" /></el-form-item>
        <el-form-item :label="$t('db.type')">
          <el-select v-model="srcForm.db_type" style="width:100%" @change="onTypeChange">
            <el-option label="PostgreSQL" value="pgsql" />
            <el-option label="MySQL" value="mysql" />
            <el-option label="SQL Server" value="mssql" />
            <el-option label="Oracle" value="oracle" />
          </el-select>
        </el-form-item>
        <el-form-item :label="$t('hosts.ip')"><el-input v-model="srcForm.host" class="mono" /></el-form-item>
        <el-form-item :label="$t('hosts.port')"><el-input-number v-model="srcForm.port" :min="1" :max="65535" /></el-form-item>
        <el-form-item :label="$t('db.database')"><el-input v-model="srcForm.database" class="mono" /></el-form-item>
        <el-form-item :label="$t('db.readonly')"><el-switch v-model="srcForm.read_only" /></el-form-item>
      </el-form>
      <div style="color:#909399; font-size:12px">{{ $t('db.sourceTip') }}</div>
      <template #footer>
        <el-button @click="srcVisible = false">{{ $t('common.cancel') }}</el-button>
        <el-button type="primary" @click="saveSource">{{ $t('common.save') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, onBeforeUnmount, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { EditorView, keymap as cmKeymap, lineNumbers, foldGutter } from '@codemirror/view'
import { EditorState, Compartment } from '@codemirror/state'
import { sql as sqlLang, PostgreSQL, MySQL, MSSQL } from '@codemirror/lang-sql'
	import { HighlightStyle, syntaxHighlighting, foldGutter as foldGutterExt } from '@codemirror/language'
	import { tags as hlTags } from '@lezer/highlight'
	import { autocompletion } from '@codemirror/autocomplete'
import { defaultKeymap, history as cmHistory, historyKeymap, indentWithTab } from '@codemirror/commands'
import api from '../api'
import i18n from '../i18n'
import { useUserStore } from '../store'

const { t } = i18n.global
const store = useUserStore()
const sources = ref([])
const accounts = ref([])
const userGroups = ref([])
const sourceId = ref(null)
const accountId = ref(null)
const editorRef = ref(null)
let editorView = null
let langComp = null
const running = ref(false)
const result = ref(null)
const lastError = ref('')
const history = ref([])
const histPick = ref('')
const loading = ref(false)
const accVisible = ref(false)
const accForm = ref({})
const accGroupIds = ref([])
const guardDlgVisible = ref(false)
const guardForm = ref({ read_only: false, timeout_sec: 30, max_rows: 1000 })

const currentSource = computed(() => sources.value.find(s => s.id === sourceId.value))

const isNumericType = ty => /int|numeric|dec|float|real|double|number|serial|money/i.test(ty || '')
const shortType = ty => (ty || '').replace(/(varchar|character varying|timestamp with time zone|timestamp without time zone)/i, 'str').slice(0, 10)

const sqlDialectFor = t2 => ({ pgsql: PostgreSQL, mysql: MySQL, mssql: MSSQL }[t2] || (t2 === 'oracle' ? sqlLang() : PostgreSQL))

// switching type (user action) snaps the port to that engine's default -
// a watcher would also fire when the edit dialog opens and clobber stored ports
const dbDefaultPorts = { pgsql: 5432, mysql: 3306, mssql: 1433, oracle: 1521 }
const onTypeChange = t2 => { if (dbDefaultPorts[t2]) srcForm.value.port = dbDefaultPorts[t2] }

const buildLang = () => {
  const tables = {}
  for (const t2 of (schemaTables.value || [])) tables[t2.name] = t2.columns
  return sqlLang({ dialect: sqlDialectFor(currentDialect.value), upperCaseKeywords: true, schema: tables })
}
const reconfigureLang = () => {
  if (langComp && editorView) editorView.dispatch({ effects: langComp.reconfigure(buildLang()) })
}

const dbHighlight = HighlightStyle.define([
  { tag: [hlTags.keyword, hlTags.operatorKeyword], color: 'var(--el-color-primary)' },
  { tag: [hlTags.string, hlTags.special(hlTags.string)], color: 'var(--el-color-success)' },
  { tag: [hlTags.number], color: 'var(--el-color-warning)' },
  { tag: [hlTags.comment, hlTags.lineComment, hlTags.blockComment], color: 'var(--el-text-color-secondary)', fontStyle: 'italic' },
  { tag: [hlTags.operator], color: 'var(--el-text-color-regular)' },
  { tag: [hlTags.bool, hlTags.null], color: 'var(--el-color-danger)' },
])

const initEditor = () => {
  if (editorView) return
  try {
  langComp = new Compartment()
  const state = EditorState.create({
    doc: '',
    extensions: [
      lineNumbers(),
      foldGutterExt(),
      cmHistory(),
      cmKeymap.of([...historyKeymap, indentWithTab,
        { key: 'Mod-Enter', run: () => { run(); return true } },
        { key: 'Ctrl-Enter', run: () => { run(); return true } }]),
      cmKeymap.of(defaultKeymap),
      langComp.of(buildLang()),
      syntaxHighlighting(dbHighlight),
      autocompletion(),
      EditorView.lineWrapping,
      EditorView.theme({
        '&': { fontSize: '13px', backgroundColor: 'var(--el-bg-color)', color: 'var(--el-text-color-regular)' },
        '.cm-content': { caretColor: 'var(--el-color-primary)' },
        '.cm-gutters': { backgroundColor: 'var(--el-fill-color-light)', color: 'var(--el-text-color-secondary)', border: 'none' },
        '.cm-activeLine': { backgroundColor: 'var(--el-fill-color)' },
        '.cm-editor.cm-focused': { outline: 'none' },
        '.cm-tooltip': {
          backgroundColor: 'var(--el-bg-color-overlay)', border: '1px solid var(--el-border-color-light)',
          borderRadius: '6px', overflow: 'hidden' },
        '.cm-tooltip.cm-tooltip-autocomplete > ul': {
          fontFamily: 'Consolas, Monaco, monospace', maxHeight: '220px',
          backgroundColor: 'var(--el-bg-color-overlay)', color: 'var(--el-text-color-regular)' },
        '.cm-tooltip.cm-tooltip-autocomplete > ul > li': {
          color: 'var(--el-text-color-regular)', padding: '3px 8px' },
        '.cm-tooltip.cm-tooltip-autocomplete > ul > li[aria-selected]': {
          backgroundColor: 'var(--el-color-primary-light-8)', color: 'var(--el-color-primary)' },
        '.cm-completionLabel': { color: 'var(--el-text-color-regular)' },
        '.cm-completionIcon': { color: 'var(--el-text-color-secondary)', paddingRight: '4px' },
      }),
    ],
  })
  editorView = new EditorView({ state, parent: editorRef.value })
  } catch (e) { window.__cmerr = String((e && e.stack) || e) }
}

watch(accountId, () => loadSchema())
const load = async () => {
  loading.value = true
  try {
    sources.value = await api.get('/databases/sources')
    userGroups.value = await api.get('/user_groups').catch(() => [])
  } finally { loading.value = false }
}
onMounted(() => { load(); initEditor() })

const schemaTables = ref([])
const currentDialect = ref('pgsql')
const schemaLoading = ref(false)
const loadSchema = async () => {
  if (!sourceId.value || !accountId.value) { schemaTables.value = []; reconfigureLang(); return }
  schemaLoading.value = true
  try {
    const r = await api.get(`/databases/${sourceId.value}/schema`, { params: { account_id: accountId.value } })
    schemaTables.value = (r.tables || []).map(x2 => ({ name: x2.name, columns: x2.columns }))
    currentDialect.value = (sources.value.find(x2 => x2.id === sourceId.value) || {}).db_type || 'pgsql'
    reconfigureLang()
  } finally { schemaLoading.value = false }
}

const pickSource = async s2 => {
  sourceId.value = s2.id
  accountId.value = null
  result.value = null
  accounts.value = await api.get(`/databases/${s2.id}/accounts`)
  if (accounts.value.length) accountId.value = accounts.value[0].id
  loadSchema()
  guardForm.value = { read_only: !!s2.read_only, timeout_sec: s2.timeout_sec || 30, max_rows: s2.max_rows || 1000 }
}

const accDlg = () => {
  accForm.value = { username: '', password: '', label: '', allowed_groups: '' }
  accGroupIds.value = []
  accVisible.value = true
}
const saveAccount = async () => {
  if (!accForm.value.username || (!accForm.value.id && !accForm.value.password)) {
    ElMessage.warning(t('db.needUserPwd')); return
  }
  const payload = { ...accForm.value, allowed_groups: accGroupIds.value.join(',') }
  if (accForm.value.id) await api.put(`/databases/accounts/${accForm.value.id}`, payload)
  else await api.post(`/databases/${sourceId.value}/accounts`, payload)
  ElMessage.success(t('common.success'))
  accVisible.value = false
  accounts.value = await api.get(`/databases/${sourceId.value}/accounts`)
}
onBeforeUnmount(() => { try { editorView?.destroy() } catch { /* ignore */ } })
const delAccount = async a => {
  await api.delete(`/databases/accounts/${a.id}`)
  accounts.value = accounts.value.filter(x => x.id !== a.id)
}
const saveGuard = async () => {
  await api.put(`/databases/sources/${sourceId.value}/guardrails`, guardForm.value)
  ElMessage.success(t('common.success'))
  guardDlgVisible.value = false
  await load()
  const s = sources.value.find(x => x.id === sourceId.value)
  if (s) guardForm.value = { read_only: !!s.read_only, timeout_sec: s.timeout_sec || 30, max_rows: s.max_rows || 1000 }
}

// new source (admin)
const srcVisible = ref(false)
const srcForm = ref({})
const srcDlg = () => {
  srcForm.value = { name: '', db_type: 'pgsql', host: '', port: 5432, database: '', read_only: false }
  srcVisible.value = true
}
const saveSource = async () => {
  const f = srcForm.value
  if (!f.name || !f.host || !f.database) { ElMessage.warning(t('db.needNameHost')); return }
  const r = await api.post('/databases/sources', f)
  ElMessage.success(t('common.success'))
  srcVisible.value = false
  await load()
  pickSource(sources.value.find(x => x.id === r.id) || { id: r.id })
}

const editorText = () => (editorView ? editorView.state.doc.toString() : '')
const setEditorText = t2 => {
  if (!editorView) return
  editorView.dispatch({ changes: { from: 0, to: editorView.state.doc.length, insert: t2 || '' } })
}

const run = async () => {
  if (!sourceId.value || !accountId.value || !editorText().trim()) return
  running.value = true
  lastError.value = ''
  try {
    const r = await api.post(`/databases/${sourceId.value}/query`, { account_id: accountId.value, sql: editorText() })
    result.value = r
    lastError.value = ''
    const cur = editorText()
    if (!history.value.includes(cur)) history.value.unshift(cur)
  } catch (e) {
    // the interceptor still toasts briefly; this panel keeps the full text readable
    lastError.value = (e && e.response && e.response.data && e.response.data.error) || (e && e.message) || String(e)
  }
  finally { running.value = false }
}

const tableRows = computed(() => {
  if (!result.value || !result.value.columns) return []
  return result.value.rows.map(r => {
    const o = {}
    result.value.columns.forEach((c, i) => { o['c' + i] = r[i] })
    return o
  })
})
const exportCsv = () => {
  if (!result.value) return
  const esc = v => '"' + String(v ?? '').replaceAll('"', '""') + '"'
  const lines = [result.value.columns.map(esc).join(',')]
  for (const r of result.value.rows) lines.push(r.map(esc).join(','))
  const blob = new Blob(['\ufeff' + lines.join('\n')], { type: 'text/csv;charset=utf-8' })
  const a = document.createElement('a')
  a.href = URL.createObjectURL(blob)
  a.download = 'query-result.csv'
  a.click()
  URL.revokeObjectURL(a.href)
}

defineExpose({ openCreate: () => srcDlg() })
</script>

<style scoped>
.db-err { white-space: pre-wrap; word-break: break-word; max-height: 200px;
  overflow: auto; font-size: 12px; line-height: 1.6; }

.sql-editor { border: 1px solid var(--el-border-color-lighter); border-radius: 6px; overflow: hidden; }
.sql-editor :deep(.cm-editor) { max-height: 260px; }
.col-name { font-weight: 600; }
.col-type { color: var(--el-text-color-secondary); font-size: 11px; margin-left: 4px; font-family: Consolas, Monaco, monospace; }
.cell-null { color: var(--el-text-color-secondary); font-style: italic; font-size: 12px; }

.db-side { width: 280px; flex: 0 0 280px; overflow: auto; }
.db-main { flex: 1; overflow: auto; }
.db-src, .db-acc { padding: 8px 10px; border-radius: 6px; cursor: pointer; margin-bottom: 4px; }
.db-src:hover, .db-acc:hover { background: var(--el-fill-color-light); }
.db-src.active, .db-acc.active { background: var(--el-color-primary-light-8); }
</style>
