const fs = require('fs');
const f = 'D:/AutoOps/frontend/src/views/MainLayout.vue';
let s = fs.readFileSync(f, 'utf8');

// ---- 1) AI 脚本函数（插在 loadRotAccounts 之前） ----
const anchor = '\t// ---- 密码轮换：适用账号 + 立即全部轮换（复用异步批次） ----';
if (!s.includes(anchor)) { console.log('ANCHOR NOT FOUND'); process.exit(1); }
const aiCode = [
  '\t// ---- 全站悬浮 AI 对话框 ----',
  '\tconst aiOpen = ref(false)',
  '\tconst aiBusy = ref(false)',
  '\tconst aiInput = ref(\'\')',
  '\tconst aiMessages = ref([])',
  '\tconst aiMsgBox = ref(null)',
  '',
  '\tconst sendToAI = async () => {',
  '\t\tconst text = aiInput.value.trim()',
  '\t\tif (!text || aiBusy.value) return',
  '\t\taiMessages.value.push({ role: \'user\', text })',
  '\t\taiBusy.value = true',
  '\t\ttry {',
  '\t\t\tconst r = await api.post(\'/ai/chat\', { prompt: text })',
  '\t\t\taiMessages.value.push({ role: \'bot\', text: r.reply || \'…\' })',
  '\t\t} catch {',
  '\t\t\taiMessages.value.push({ role: \'bot\', text: t(\'ai.error\') })',
  '\t\t} finally {',
  '\t\t\taiInput.value = \'\'',
  '\t\t\taiBusy.value = false',
  '\t\t\tsetTimeout(() => { const b = aiMsgBox.value; if (b) b.scrollTop = b.scrollHeight }, 50)',
  '\t\t}',
  '\t}',
  '',
].join('\n');
s = s.replace(anchor, aiCode + '\n' + anchor);

// ---- 2) AI 面板样式（追加到文件尾 </style> 之前） ----
const styles = [
  '',
  '<style scoped>',
  '.ai-panel {',
  '  position: fixed; bottom: 84px; right: 22px; z-index: 2001;',
  '  width: 360px; height: 480px; display: flex; flex-direction: column;',
  '  background: var(--el-bg-color); border: 1px solid var(--el-border-color-lighter);',
  '  border-radius: 12px; box-shadow: 0 12px 40px rgba(0,0,0,.18);',
  '  overflow: hidden;',
  '}',
  '.ai-head {',
  '  display: flex; align-items: center; gap: 8px; padding: 12px 14px 8px;',
  '  font-size: 14px; font-weight: 700; color: var(--el-text-color-primary);',
  '}',
  '.ai-messages { flex: 1; overflow-y: auto; padding: 10px 14px; }',
  '.ai-msg { margin-bottom: 10px; }',
  '.ai-msg.user { text-align: right; }',
  '.ai-bubble {',
  '  display: inline-block; max-width: 85%; padding: 8px 12px; border-radius: 10px;',
  '  font-size: 13.5px; line-height: 1.55; word-break: break-word;',
  '  background: var(--el-fill-color); color: var(--el-text-color-primary);',
  '}',
  '.ai-msg.user .ai-bubble { background: var(--el-color-primary-light-9); color: var(--el-color-primary); }',
  '.ai-input-row { display: flex; gap: 8px; padding: 8px 14px 12px; }',
  '.ai-input-row el-input { flex: 1; }',
  '.ai-fab {',
  '  position: fixed; bottom: 22px; right: 22px; z-index: 2000;',
  '  width: 48px; height: 48px; border-radius: 50%;',
  '  background: var(--el-color-primary); color: #fff;',
  '  display: flex; align-items: center; justify-content: center;',
  '  cursor: pointer; box-shadow: 0 4px 16px rgba(64,158,255,.4);',
  '  transition: transform .2s, box-shadow .2s; border: none; font-size: 22px;',
  '}',
  '.ai-fab:hover { transform: scale(1.1); box-shadow: 0 6px 20px rgba(64,158,255,.5); }',
  '.ai-fade-enter-active, .ai-fade-leave-active { transition: opacity .25s, transform .25s; }',
  '.ai-fade-enter-from, .ai-fade-leave-to { opacity: 0; transform: translateY(10px); }',
  '</' + 'style>',
].join('\n');

// 追加到最后一个 </style> 之后
const lastStyle = s.lastIndexOf('</style>');
if (lastStyle > 0) {
  s = s.slice(0, lastStyle) + styles + '\n' + s.slice(s.indexOf('</style>', lastStyle));
} else {
  s += '\n<style scoped>\n' + styles + '\n</' + 'style>\n';
}

fs.writeFileSync(f, s);
console.log('MainLayout AI code + styles added');
