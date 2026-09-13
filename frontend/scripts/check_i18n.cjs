// Audit: find i18n keys used in src but missing from locale files
const fs = require('fs')
const path = require('path')

const SRC = path.join(__dirname, '..')
const LOCALE_DIR = path.join(SRC, 'src', 'i18n')

function flatten(obj, prefix = '') {
  const out = {}
  for (const [k, v] of Object.entries(obj)) {
    const key = prefix ? `${prefix}.${k}` : k
    if (v && typeof v === 'object' && !Array.isArray(v)) Object.assign(out, flatten(v, key))
    else out[key] = v
  }
  return out
}

function loadLocale(file) {
  let src = fs.readFileSync(path.join(LOCALE_DIR, file), 'utf8')
  src = src.replace(/^\s*\/\/.*$/gm, '')
  const module = { exports: {} }
  new Function('module', 'exports', src.replace(/export\s+default/, 'module.exports ='))(module, module.exports)
  return flatten(module.exports.default || module.exports)
}

function walk(dir, exts, files = []) {
  for (const f of fs.readdirSync(dir, { withFileTypes: true })) {
    const p = path.join(dir, f.name)
    if (f.isDirectory()) {
      if (f.name !== 'node_modules' && f.name !== 'dist') walk(p, exts, files)
    } else if (exts.some(e => f.name.endsWith(e))) files.push(p)
  }
  return files
}

const zh = loadLocale('zh-CN.js')
const en = loadLocale('en-US.js')

const used = new Set()
const files = walk(path.join(SRC, 'src'), ['.vue', '.js']).filter(f => !f.includes('i18n' + path.sep))
const re = /[\$\s\(\{]t\(\s*'([a-zA-Z0-9_.]+)'\s*[,\)]/g
for (const f of files) {
  const text = fs.readFileSync(f, 'utf8')
  let m
  while ((m = re.exec(text))) used.add(m[1])
}

const missingZh = [...used].filter(k => !(k in zh))
const missingEn = [...used].filter(k => !(k in en))
const localeFiles = Object.keys(zh).length + ' zh keys, ' + Object.keys(en).length + ' en keys, ' + used.size + ' used'
console.log('Locale stats:', localeFiles)
console.log('Missing in zh-CN:', missingZh.length ? missingZh : 'none')
console.log('Missing in en-US:', missingEn.length ? missingEn : 'none')
// keys defined in one locale but not the other
const zhOnly = Object.keys(zh).filter(k => !(k in en))
const enOnly = Object.keys(en).filter(k => !(k in zh))
console.log('zh-only keys:', zhOnly.length ? zhOnly : 'none')
console.log('en-only keys:', enOnly.length ? enOnly : 'none')
