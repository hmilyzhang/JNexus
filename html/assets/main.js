// JNexus 运维平台 — By JJ Zhang, Version 1.0
// 宣传站脚本：中英双语（跟随浏览器语言自动切换）+ 终端动画 + 入场动效

/* ===== 双语词典 ===== */
const I18N = {
  zh: {
    'nav.features': '特性', 'nav.architecture': '架构', 'nav.quickstart': '快速开始', 'nav.security': '安全',
    'hero.badge': '🔥 开源 · 自托管 · 永久免费',
    'hero.title1': '轻量级自托管',
    'hero.title2': '运维平台',
    'hero.subtitle': 'Web 终端、浏览器内 RDP、批量执行、监控告警与 Kubernetes 管理——打包成一个二进制文件，五分钟接入你的机房。',
    'hero.cta.github': 'GitHub 仓库', 'hero.cta.quickstart': '快速开始 →',
    'hero.stats.binary': '单二进制部署', 'hero.stats.platforms': 'Linux / Windows 双平台',
    'hero.stats.modules': '功能模块', 'hero.stats.opensource': '开源开放',
    'features.title': '一个平台，接管日常运维',
    'features.subtitle': '从一台主机到多个 Kubernetes 集群，从手工敲命令到流程化作业。',
    'f1.t': 'Web 终端', 'f1.d': '基于 xterm.js 的多标签终端：主题切换、全屏模式、会话保持。Linux 走 SSH，Windows 走 WinRM，同一个入口。',
    'f2.t': '浏览器内 RDP', 'f2.d': '集成 Guacamole 网关，Windows 远程桌面直接在浏览器打开。零客户端安装，一次性令牌连接，更安全。',
    'f3.t': '批量执行与脚本库', 'f3.d': '跨百台主机并发执行命令，结果按主机聚合比对；常用操作沉淀为脚本库，版本化、可复用、可审计。',
    'f4.t': '监控与容量规划', 'f4.d': 'CPU / 内存 / 磁盘指标采集，阈值告警实时推送；线性回归预测到达水位的天数，扩容不再靠拍脑袋。',
    'f5.t': 'Kubernetes 管理', 'f5.d': '多集群接入：工作负载伸缩、YAML 在线编辑、Helm 发布、Pod Shell 与实时日志、集群容量趋势。',
    'f6.t': '应用与发布', 'f6.d': '应用目录与发布流程标准化：上传制品、目标主机、一键发布与回滚，发布记录全程可追溯。',
    'f7.t': '计划任务', 'f7.d': '类 Cron 表达式调度，主机脚本与容器任务统一编排，执行历史与失败告警一目了然。',
    'f8.t': '月度运营报告', 'f8.d': '自动汇总月度可用率、告警时间线、P1/P2 事件与容量红绿灯，运维价值一页呈现。',
    'f9.t': '权限与安全', 'f9.d': '能力位权限模型 + 自定义角色，主机凭据 AES 加密存储，MFA 两步验证，操作全量审计。',
    'arch.title': '为「简单部署」而生',
    'arch.subtitle': '一个后端二进制 + 一份 Docker Compose，没有 K8s 也能运维 K8s。',
    'arch.be.chip': '后端', 'arch.be.d': '单二进制内建调度器、WebSocket 网关与 SSH/WinRM 客户端，交叉编译覆盖 Linux / Windows / ARM。',
    'arch.fe.chip': '前端', 'arch.fe.d': '构建产物为纯静态文件，由后端直接托管；中英双语开箱即用。',
    'arch.data.chip': '数据与交付', 'arch.data.d': '结构自动迁移，密钥首次启动自动生成并持久化；Compose 一键拉起服务、数据库与 RDP 网关（guacd）。',
    'flow.browser': '浏览器', 'flow.hosts': '主机 SSH / WinRM', 'flow.k8s': 'Kubernetes API', 'flow.db': 'PostgreSQL',
    'qs.title': '五分钟，跑起来',
    'qs.subtitle': '只需要一台装了 Docker 的机器。首次启动会自动生成并持久化加密密钥，后续升级不丢失数据。',
    'qs.s1': '克隆仓库并启动 Compose',
    'qs.s2': '浏览器打开控制台，使用 README 中的默认账号登录',
    'qs.s3': '添加主机 / 导入 kubeconfig，开始接管你的机房',
    'qs.terminal': '终端', 'qs.copy': '复制',
    'sec.title': '企业级安全，开箱即用',
    'sec.subtitle': '权限、加密、认证、审计——四道防线默认开启。',
    'sec1.t': '凭据加密', 'sec1.d': '主机账号密码与 kubeconfig 以 AES 加密落库；RDP 连接令牌一次性取用，用后即焚。',
    'sec2.t': 'MFA 两步验证', 'sec2.d': 'TOTP 动态口令，用户自助开关，防止弱口令带来的横向风险。',
    'sec3.t': 'LDAP 集成', 'sec3.d': '对接企业域目录，账号免维护，显示名自动同步。',
    'sec4.t': '全量审计', 'sec4.d': '登录、执行、发布、删除全部留痕，谁在什么时候动了哪台机器，一目了然。',
    'cta.title': '准备好接管你的机房了吗？',
    'cta.subtitle': 'Star & Fork，和社区一起把它变得更好。',
    'cta.btn': '前往 GitHub',
    'footer.tagline': '开源的轻量级运维平台', 'footer.license': '许可证',
  },
  en: {
    'nav.features': 'Features', 'nav.architecture': 'Architecture', 'nav.quickstart': 'Quick Start', 'nav.security': 'Security',
    'hero.badge': '🔥 Open Source · Self-hosted · Free Forever',
    'hero.title1': 'The Lightweight',
    'hero.title2': 'Self-hosted Ops Platform',
    'hero.subtitle': 'Web terminal, in-browser RDP, batch execution, monitoring & Kubernetes management — packed into a single binary, ready in five minutes.',
    'hero.cta.github': 'GitHub Repo', 'hero.cta.quickstart': 'Quick Start →',
    'hero.stats.binary': 'Single binary', 'hero.stats.platforms': 'Linux & Windows',
    'hero.stats.modules': 'Modules', 'hero.stats.opensource': 'Open source',
    'features.title': 'One platform for everyday ops',
    'features.subtitle': 'From a single host to multiple Kubernetes clusters — from hand-typed commands to standardized workflows.',
    'f1.t': 'Web Terminal', 'f1.d': 'Multi-tab xterm.js terminal with themes, fullscreen and persistent sessions. SSH for Linux, WinRM for Windows — one entry point.',
    'f2.t': 'In-browser RDP', 'f2.d': 'Built-in Guacamole gateway: open Windows remote desktops right in the browser. No client install, one-time tokens for safer access.',
    'f3.t': 'Batch Execution & Scripts', 'f3.d': 'Run commands across hundreds of hosts concurrently with per-host result aggregation; turn repeated work into a versioned script library.',
    'f4.t': 'Monitoring & Capacity', 'f4.d': 'CPU / memory / disk sampling with threshold alerting; linear-regression forecasts tell you how many days until you run out of headroom.',
    'f5.t': 'Kubernetes Management', 'f5.d': 'Multi-cluster: scale workloads, edit YAML online, deploy Helm charts, hop into pod shells and stream logs, track cluster capacity trends.',
    'f6.t': 'Apps & Releases', 'f6.d': 'Standardized release pipeline: upload artifacts, pick target hosts, one-click deploy and rollback — fully traceable.',
    'f7.t': 'Scheduled Jobs', 'f7.d': 'Cron-style scheduling for host scripts and container jobs, with execution history and failure alerts.',
    'f8.t': 'Monthly Ops Report', 'f8.d': 'Auto-compiled monthly report: availability, alert timeline, P1/P2 incidents and capacity traffic lights.',
    'f9.t': 'RBAC & Security', 'f9.d': 'Capability-based permissions with custom roles, AES-encrypted credentials, MFA and full audit trail.',
    'arch.title': 'Designed for simple deployment',
    'arch.subtitle': 'One backend binary plus one Docker Compose file — manage Kubernetes even without a K8s cluster of your own.',
    'arch.be.chip': 'Backend', 'arch.be.d': 'A single binary with built-in scheduler, WebSocket gateway and SSH/WinRM clients. Cross-compiled for Linux, Windows and ARM.',
    'arch.fe.chip': 'Frontend', 'arch.fe.d': 'The build output is pure static files served by the backend; Chinese and English out of the box.',
    'arch.data.chip': 'Data & Delivery', 'arch.data.d': 'Auto schema migration; encryption keys are generated and persisted on first boot; Compose brings up app, database and the guacd RDP gateway.',
    'flow.browser': 'Browser', 'flow.hosts': 'Hosts via SSH / WinRM', 'flow.k8s': 'Kubernetes API', 'flow.db': 'PostgreSQL',
    'qs.title': 'Up and running in 5 minutes',
    'qs.subtitle': 'All you need is a machine with Docker. Encryption keys are auto-generated and persisted on first boot — upgrades keep your data.',
    'qs.s1': 'Clone the repo and start Compose',
    'qs.s2': 'Open the console in your browser and sign in with the default account from the README',
    'qs.s3': 'Add hosts or import a kubeconfig — start taking over your fleet',
    'qs.terminal': 'Terminal', 'qs.copy': 'Copy',
    'sec.title': 'Enterprise-grade security, out of the box',
    'sec.subtitle': 'Permissions, encryption, authentication, auditing — four lines of defense enabled by default.',
    'sec1.t': 'Encrypted Credentials', 'sec1.d': 'Host passwords and kubeconfigs are AES-encrypted at rest; RDP tokens are one-time and burned after use.',
    'sec2.t': 'MFA', 'sec2.d': 'TOTP one-time codes, self-service enablement, protecting against weak-password lateral movement.',
    'sec3.t': 'LDAP Integration', 'sec3.d': 'Plug into your corporate directory — zero-maintenance accounts with automatic display-name sync.',
    'sec4.t': 'Full Audit Trail', 'sec4.d': 'Logins, executions, releases and deletions are all recorded: who touched which machine, and when.',
    'cta.title': 'Ready to take over your fleet?',
    'cta.subtitle': 'Star & Fork — help make it even better.',
    'cta.btn': 'Open GitHub',
    'footer.tagline': 'The open-source lightweight ops platform', 'footer.license': 'License',
  },
};

/* ===== 语言初始化：手动选择 > 浏览器语言自动判断 ===== */
let lang = localStorage.getItem('jx-lang')
  || ((navigator.language || navigator.userLanguage || 'en').toLowerCase().startsWith('zh') ? 'zh' : 'en');

function applyLang() {
  const dict = I18N[lang];
  document.querySelectorAll('[data-i18n]').forEach(el => {
    const key = el.getAttribute('data-i18n');
    if (dict[key] != null) el.textContent = dict[key];
  });
  document.documentElement.lang = lang === 'zh' ? 'zh-CN' : 'en';
  document.title = lang === 'zh'
    ? 'JNexus — 开源轻量级运维平台'
    : 'JNexus — Open-source Ops Platform';
  document.getElementById('langBtn').textContent = lang === 'zh' ? 'EN' : '中文';
  localStorage.setItem('jx-lang', lang);
}

document.getElementById('langBtn').addEventListener('click', () => {
  lang = lang === 'zh' ? 'en' : 'zh';
  applyLang();
});

applyLang();
document.getElementById('year').textContent = new Date().getFullYear();

/* ===== Hero 终端打字动画 ===== */
const termLines = [
  { type: 'cmd', text: 'ssh web-01' },
  { type: 'out', text: 'root@web-01:~#' },
  { type: 'cmd', text: 'kubectl get pods -n prod' },
  { type: 'out', text: 'NAME                  READY  STATUS    RESTARTS' },
  { type: 'out', text: 'api-7d9f4b5c6-xk2vp   1/1    Running   0' },
  { type: 'out', text: 'web-5c8d7f6b4-m9tqz   1/1    Running   0' },
  { type: 'ok',  text: '✔ 2 clusters · 128 hosts · all green' },
];

const termBody = document.getElementById('termBody');
let lineIdx = 0, charIdx = 0, htmlBuf = '';

function renderCaret(buf) {
  return buf + '<span class="caret"></span>';
}

function tick() {
  if (lineIdx >= termLines.length) {
    // 一轮结束：停顿后清屏重来
    setTimeout(() => { htmlBuf = ''; lineIdx = 0; charIdx = 0; termBody.innerHTML = ''; tick(); }, 4200);
    return;
  }
  const line = termLines[lineIdx];
  if (line.type === 'cmd') {
    // 命令逐字敲
    charIdx++;
    const cls = line.type;
    const partial = line.text.slice(0, charIdx);
    termBody.innerHTML = htmlBuf + `<span class="${cls}"><span class="p">$</span> ${partial}</span>\n` + renderCaret('');
    if (charIdx >= line.text.length) {
      htmlBuf += `<span class="${cls}"><span class="p">$</span> ${line.text}</span>\n`;
      lineIdx++; charIdx = 0;
      setTimeout(tick, 500);
    } else {
      setTimeout(tick, 42 + Math.random() * 60);
    }
  } else {
    // 输出整行出现
    htmlBuf += `<span class="${line.type}">${line.text}</span>\n`;
    termBody.innerHTML = renderCaret(htmlBuf);
    lineIdx++;
    setTimeout(tick, line.type === 'ok' ? 400 : 260);
  }
}
tick();

/* ===== 滚动入场动效 ===== */
const io = new IntersectionObserver(entries => {
  entries.forEach(e => {
    if (e.isIntersecting) {
      e.target.classList.add('visible');
      io.unobserve(e.target);
    }
  });
}, { threshold: 0.12 });
document.querySelectorAll('.reveal').forEach(el => io.observe(el));

/* ===== 复制按钮 ===== */
document.querySelectorAll('.btn-copy').forEach(btn => {
  btn.addEventListener('click', () => {
    const code = btn.closest('.code-card').querySelector('code').innerText;
    navigator.clipboard.writeText(code).then(() => {
      const old = btn.textContent;
      btn.textContent = lang === 'zh' ? '已复制 ✓' : 'Copied ✓';
      btn.classList.add('done');
      setTimeout(() => { btn.textContent = old; btn.classList.remove('done'); }, 1600);
    });
  });
});
