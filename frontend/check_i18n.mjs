import { readFileSync, readdirSync, statSync } from 'fs';
import { join } from 'path';

const srcDir = 'src';
const files = [];
(function walk(d) {
  for (const f of readdirSync(d)) {
    const p = join(d, f);
    if (statSync(p).isDirectory()) walk(p);
    else if (/\.(vue|js)$/.test(f) && !p.includes('i18n/')) files.push(p);
  }
})(srcDir);

const used = new Map();
for (const f of files) {
  const src = readFileSync(f, 'utf-8');
  const re = /[\$.]t\(\s*['"]([^'"]+)['"]/g;
  let m;
  while ((m = re.exec(src))) {
    if (!used.has(m[1])) used.set(m[1], []);
    used.get(m[1]).push(f);
  }
}
const { default: zh } = await import('./src/i18n/zh-CN.js');
const { default: en } = await import('./src/i18n/en-US.js');
const get = (obj, path) => path.split('.').reduce((o, k) => (o == null ? undefined : o[k]), obj);
let missing = 0;
for (const [key, fs] of [...used.entries()].sort()) {
  const inZh = get(zh, key) !== undefined;
  const inEn = get(en, key) !== undefined;
  if (!inZh || !inEn) {
    missing++;
    console.log(`MISSING ${key}  zh:${inZh} en:${inEn}  used in: ${[...new Set(fs)].join(', ')}`);
  }
}
console.log(`total ${used.size} keys, missing ${missing}`);
