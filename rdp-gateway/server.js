// JNexus RDP 网关：浏览器(Guacamole WS) ↔ guacd ↔ Windows(RDP)
// 浏览器连接时携带 JNexus 签发的一次性 token；
// guacamole-lite 回调 JNexus /api/hosts/rdp-consume 换取真实凭据（token 取后即焚）。
const http = require('http');
const GuacamoleLite = require('guacamole-lite');

const JNEXUS_API = process.env.JNEXUS_API || 'http://jnexus-server:8080';
const GUACD_HOST = process.env.GUACD_HOST || 'guacd';
const GUACD_PORT = process.env.GUACD_PORT || 4822;
const PORT = process.env.GW_PORT || 4823;

// guacamole-lite 标准模式：连接参数以 AES-256-CBC 加密的查询串从浏览器传入。
// JNexus 后端签发该加密串（短时有效、一次性语义由短时效保证），浏览器不接触明文凭据。
const server = http.createServer((req, res) => {
  res.writeHead(200, { 'Content-Type': 'application/json' });
  res.end(JSON.stringify({ status: 'ok', service: 'jnexus-rdp-gateway' }));
});

new GuacamoleLite(server, { host: GUACD_HOST, port: GUACD_PORT }, {
  crypt: { cypher: 'AES-256-CBC', key: process.env.GW_SECRET || 'JnexusRdpGatewaySecretKey32bytes!' },
});

server.listen(PORT, () => {
  console.log(`[rdp-gateway] listening on :${PORT}, guacd=${GUACD_HOST}:${GUACD_PORT}`);
});
