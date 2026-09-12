// 一次性脚本：i18n 补齐所有 AI/轮换/报告相关键（执行后删除）
const fs = require('fs');
const p = 'D:/AutoOps/frontend/src/i18n';

// 读取当前键集合
function getKeys(f) {
  const s = fs.readFileSync(f, 'utf8');
  const keys = new Set();
  for (const m of s.matchAll(/'([a-zA-Z_.]+)':/g)) keys.add(m[1]);
  return keys;
}

// 需要的键和翻译
const additions = {
  'zh-CN.js': {
    // rot 段
    rot_applicableAccts: '适用账号（密码认证、非域账号）',
    rot_runNow: '立即全部轮换',
    rot_due: '轮换状态',
    rot_dueYes: '已到期',
    rot_dueNo: '未到期',
    rot_lastRot: '最近轮换',
    // ai 段
    ai_tab: 'AI 助手',
    ai_enabled: '启用 AI 助手',
    ai_tip: '兼容 OpenAI 协议的服务均可：OpenAI / Ollama / vLLM / LM Studio。',
    ai_baseUrl: '服务地址 (Base URL)',
    ai_baseUrlTip: 'OpenAI 兼容地址，含 /v1（Ollama 填 http://主机:11434/v1）',
    ai_apiKey: 'API Key',
    ai_apiKeyTip: '本地服务可留空',
    ai_model: '模型名称',
    ai_modelTip: '如 qwen2.5:7b、gpt-4o-mini',
    ai_timeout: '超时（秒）',
    ai_timeoutTip: '5-600 秒',
    ai_testConn: '测试连通',
    ai_chatTest: '测试对话',
    ai_promptTip: '输入内容，回车或点发送',
    ai_send: '发送',
    ai_stateOn: '已启用',
    ai_stateOff: '未启用',
    ai_noAccounts: '当前没有适用账号',
    ai_assistantTitle: 'AI 助手',
    ai_assistantPlaceholder: '输入内容…',
    ai_send: '发送',
    ai_error: '连接失败，请稍后再试',
    // osac 段
    osac_batchRotate: '批量轮换',
    osac_batchRotateTitle: '批量轮换 OS 账号密码',
    osac_batchRotateConfirm: '将对 {n} 个账号执行密码轮换：生成强随机新密码并在目标机生效，平台同步更新。密钥/LDAP 账号自动跳过。继续？',
    osac_batchRotateTip: '逐台串行执行（错峰 300ms）；新密码已加密入库，请通过「查看密码」或密码管理工具同步到需要的地方。',
    osac_batchRunning: '执行中',
    osac_unnamedHost: '未命名主机',
    osac_unbound: '未绑定主机',
    // menu
    menu_logtail: '日志输出',
    ai_greeting: '你好！我是 JNexus AI 助手，有什么可以帮你？',
    ai_error: '连接失败，请稍后再试',
  },
  'en-US.js': {
    rot_applicableAccts: 'Applicable accounts (password auth, non-LDAP)',
    rot_runNow: 'Rotate all now',
    rot_due: 'Due',
    rot_dueYes: 'Due',
    rot_dueNo: 'OK',
    rot_lastRot: 'Last rotation',
    ai_tab: 'AI Assistant',
    ai_enabled: 'Enable AI assistant',
    ai_tip: 'Works with any OpenAI-compatible service: OpenAI / Ollama / vLLM / LM Studio.',
    ai_baseUrl: 'Base URL',
    ai_baseUrlTip: 'OpenAI-compatible URL including /v1 (Ollama: http://host:11434/v1)',
    ai_apiKey: 'API Key',
    ai_apiKeyTip: 'Leave empty for local services',
    ai_model: 'Model name',
    ai_modelTip: 'e.g. qwen2.5:7b, gpt-4o-mini',
    ai_timeout: 'Timeout (sec)',
    ai_timeoutTip: '5-600 seconds',
    ai_testConn: 'Test connection',
    ai_chatTest: 'Test chat',
    ai_promptTip: 'Type a message…',
    ai_send: 'Send',
    ai_stateOn: 'Enabled',
    ai_stateOff: 'Disabled',
    ai_noAccounts: 'No applicable accounts',
    ai_assistantTitle: 'AI Assistant',
    ai_assistantPlaceholder: 'Type a message…',
    ai_send: 'Send',
    ai_error: 'Connection failed, please try again',
    ai_greeting: 'Hi! I am the JNexus AI assistant. How can I help?',
    osac_batchRotate: 'Batch rotate',
    osac_batchRotateTitle: 'Batch rotate OS account passwords',
    osac_batchRotateConfirm: 'Rotate passwords for {n} accounts: a strong random password is generated, applied on the target host and updated here. Key/LDAP accounts are skipped automatically. Continue?',
    osac_batchRotateTip: 'Executed sequentially (300ms stagger); new passwords are encrypted at rest - sync them via "View password" or your password manager.',
    osac_batchRunning: 'running',
    osac_unnamedHost: 'Unnamed host',
    osac_unbound: 'No host bound',
    menu_logtail: 'Log Viewer',
  },
};

for (const [fname, keys] of Object.entries(additions)) {
  const filepath = p + '/' + fname;
  let s = fs.readFileSync(filepath, 'utf8');
  let added = 0;
  for (const [fullKey, value] of Object.entries(keys)) {
    // fullKey 形如 "rot_applicableAccts" 或 "menu_logtail"
    const configKey = fullKey; // 用于检测是否已存在
    const checkKey = fullKey.replace(/_/g, '.'); // 不需要，直接检查
    if (s.includes(`"${fullKey}":`)) continue; // 已存在
    // 找到合适的段落插入
    // 简化：直接在根对象闭合前插入
    const lastBrace = s.lastIndexOf('}');
    if (lastBrace < 0) continue;
    // 格式化 key：去掉前缀段名（如 rot_ / menu_ / osac_），保留实际键名
    const actualKey = fullKey.split('_').pop();
    const entry = `  ${fullKey}: ${JSON.stringify(value)},\n`;
    s = s.slice(0, lastBrace) + entry + s.slice(lastBrace);
    added++;
  }
  fs.writeFileSync(filepath, s);
  console.log(fname, '- added', added);
}
