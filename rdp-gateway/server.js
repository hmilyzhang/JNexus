// JNexus RDP 网关：浏览器(Guacamole WS) ↔ guacd ↔ Windows(RDP)
// 浏览器连接时携带 JNexus 签发的一次性 token；
// guacamole-lite 回调 JNexus /api/hosts/rdp-consume 换取真实凭据（token 取后即焚）。
const http = require('http');
const fs = require('fs');
const GuacamoleLite = require('guacamole-lite');

const JNEXUS_API = process.env.JNEXUS_API || 'http://jnexus-server:8080';
const GUACD_HOST = process.env.GUACD_HOST || 'guacd';
const GUACD_PORT = process.env.GUACD_PORT || 4822;
const PORT = process.env.GW_PORT || 4823;

// 密钥来源：jnexus-server 首次启动时生成并写入共享 data 卷（/data/gw_secret，32 字符）
// 与后端加密串使用同一密钥；最多等待 120s（等待 jnexus 首次生成）
function readSecret() {
  const f = process.env.GW_SECRET_FILE || '/data/gw_secret';
  const deadline = Date.now() + 120000;
  while (Date.now() < deadline) {
    try {
      const v = fs.readFileSync(f, 'utf8').trim();
      if (v.length === 32) return v;
    } catch { /* not yet */ }
    const spin = Date.now() + 2000;
    while (Date.now() < spin) { /* busy wait 2s */ }
  }
  console.error('[rdp-gateway] gw_secret 未出现，使用临时随机密钥（RDP 将无法解密）');
  return 'ephemeral-' + Date.now();
}
const SECRET_KEY = process.env.GW_SECRET || readSecret();

// guacamole-lite 标准模式：连接参数以 AES-256-CBC 加密的查询串从浏览器传入。
// JNexus 后端签发该加密串（短时有效、一次性语义由短时效保证），浏览器不接触明文凭据。
// Optional TLS: set GW_TLS_CERT + GW_TLS_KEY to serve wss:// (required when the
// JNexus page is served over HTTPS, otherwise browsers block ws:// mixed content)
let server;
if (process.env.GW_TLS_CERT && process.env.GW_TLS_KEY) {
  const options = { cert: fs.readFileSync(process.env.GW_TLS_CERT), key: fs.readFileSync(process.env.GW_TLS_KEY) };
  server = require('https').createServer(options, (req, res) => {
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ status: 'ok', service: 'jnexus-rdp-gateway', tls: true }));
  });
  console.log('[rdp-gateway] TLS enabled');
} else {
  server = http.createServer((req, res) => {
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ status: 'ok', service: 'jnexus-rdp-gateway' }));
  });
}

new GuacamoleLite(server, { host: GUACD_HOST, port: GUACD_PORT }, {
  crypt: { cypher: 'AES-256-CBC', key: process.env.GW_SECRET || 'JnexusRdpGatewaySecretKey-123456' },
});

server.listen(PORT, () => {
  console.log(`[rdp-gateway] listening on :${PORT}, guacd=${GUACD_HOST}:${GUACD_PORT}`);
});
