// 一次性脚本：SystemConfig AI tab（执行后删除）
const fs = require('fs');
const f = 'D:/AutoOps/frontend/src/views/SystemConfig.vue';
let s = fs.readFileSync(f, 'utf8');

// ---- 1) form 增加 ai 键 ----
s = s.replace(
  `  rotation_enabled: 'false'
})`,
  `  rotation_enabled: 'false',
  ai_enabled: 'false', ai_base_url: '', ai_api_key: '', ai_model: '', ai_timeout_sec: '120'
})`);

// ---- 2) save 透传 ai 字段 ----
s = s.replace(
  `      rotation_days: String(rotationDays.value) }`,
  `      rotation_days: String(rotationDays.value),
      ai_enabled: String(form.ai_enabled),
      ai_base_url: form.ai_base_url,
      ai_api_key: form.ai_api_key === '******' ? '' : form.ai_api_key,
      ai_model: form.ai_model,
      ai_timeout_sec: String(form.ai_timeout_sec || '120') }`);

// ---- 3) AI tab（插在 Monthly Report 之前） ----
const anchor = `    <el-tab-pane :label="$t('mreport.tab')" name="report">`;
if (!s.includes(anchor)) { console.log('REPORT TAB NOT FOUND'); process.exit(1); }
const aiTab = `    <el-tab-pane :label="$t('ai.tab')" name="ai">
    <el-card>
      <el-form label-width="150px">
        <el-form-item :label="$t('ai.enabled')">
          <el-switch v-model="form.ai_enabled" active-value="true" inactive-value="false" />
          <div style="color:var(--el-text-color-secondary); font-size:12px; margin-top:4px">{{ $t('ai.tip') }}</div>
        </el-form-item>
        <template v-if="form.ai_enabled === 'true'">
          <el-form-item :label="$t('ai.baseUrl')">
            <el-input v-model="form.ai_base_url" class="mono" placeholder="http://127.0.0.1:11434/v1" />
            <div style="color:var(--el-text-color-secondary); font-size:12px; width:100%">{{ $t('ai.baseUrlTip') }}</div>
          </el-form-item>
          <el-form-item :label="$t('ai.apiKey')">
            <el-input v-model="form.ai_api_key" type="password" show-password class="mono" :placeholder="$t('ai.apiKeyTip')" />
          </el-form-item>
          <el-form-item :label="$t('ai.model')">
            <el-input v-model="form.ai_model" class="mono" placeholder="qwen2.5:7b" />
          </el-form-item>
          <el-form-item :label="$t('ai.timeout')">
            <el-input-number v-model="aiTimeout" :min="5" :max="600" />
            <span style="margin-left:8px; color:var(--el-text-color-secondary); font-size:12px">{{ $t('ai.timeoutTip') }}</span>
          </el-form-item>
        </template>
        <el-form-item>
          <el-button type="primary" :loading="saving" @click="save">{{ $t('common.save') }}</el-button>
          <el-button :loading="aiTesting" @click="testAi">{{ $t('ai.testConn') }}</el-button>
        </el-form-item>
      </el-form>

      <el-divider content-position="left">{{ $t('ai.chatTest') }}</el-divider>
      <div style="display:flex; gap:8px">
        <el-input v-model="aiPrompt" :placeholder="$t('ai.promptTip')" @keyup.enter="sendAi" />
        <el-button type="primary" :loading="aiSending" @click="sendAi">{{ $t('ai.send') }}</el-button>
      </div>
      <pre v-if="aiReply" class="mono" style="margin-top:12px; white-space:pre-wrap; background:var(--el-fill-color-light); padding:12px; border-radius:6px">{{ aiReply }}</pre>
    </el-card>
    </el-tab-pane>

`;
s = s.replace(anchor, aiTab + anchor);

// ---- 4) 脚本：ai 状态/方法 ----
const scAnchor = `const saveRoles = async () => {`;
if (!s.includes(scAnchor)) { console.log('SCRIPT ANCHOR NOT FOUND'); process.exit(1); }
const scAdd = `// ---- AI 模块：配置 + 测试对话 ----
const aiTesting = ref(false)
const aiSending = ref(false)
const aiPrompt = ref('')
const aiReply = ref('')
const aiTimeout = ref(120)

const loadAiConfig = async () => {
  try {
    const cfg = await api.get('/system/config')
    form.ai_enabled = cfg.ai_enabled || 'false'
    form.ai_base_url = cfg.ai_base_url || ''
    form.ai_model = cfg.ai_model || ''
    form.ai_timeout_sec = cfg.ai_timeout_sec || '120'
    form.ai_api_key = cfg.ai_api_key === '******' ? '******' : (cfg.ai_api_key || '')
  } catch { /* ignore */ }
}

const testAi = async () => {
  await save()
  aiTesting.value = true
  try {
    const r = await api.post('/system/ai/test')
    aiReply.value = r.reply || ('OK · ' + (r.elapsed_ms || 0) + 'ms')
    ElMessage.success(t('system.saved'))
  } catch { /* interceptor shows the error */ } finally { aiTesting.value = false }
}

const sendAi = async () => {
  if (!aiPrompt.value.trim()) { ElMessage.warning(t('ai.promptTip')); return }
  aiSending.value = true
  aiReply.value = ''
  try {
    const r = await api.post('/ai/chat', { prompt: aiPrompt.value })
    aiReply.value = r.reply || ''
  } catch { /* interceptor shows the error */ } finally { aiSending.value = false }
}

const saveRoles = async () => {`;
s = s.replace(scAnchor, scAdd);

// ---- 5) onMounted 时加载 AI 配置 ----
s = s.replace(
  `    rotationDays.value = Number(form.rotation_days) || 90`,
  `    rotationDays.value = Number(form.rotation_days) || 90
    form.ai_enabled = cfg.ai_enabled || 'false'
    form.ai_base_url = cfg.ai_base_url || ''
    form.ai_model = cfg.ai_model || ''
    form.ai_timeout_sec = cfg.ai_timeout_sec || '120'
    if (cfg.ai_api_key) form.ai_api_key = '******'`);

fs.writeFileSync(f, s);
console.log('SystemConfig AI tab patched');
