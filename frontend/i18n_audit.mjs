
import zh from './src/i18n/zh-CN.js'
import en from './src/i18n/en-US.js'

const flat = (obj, prefix, out) => {
  for (const [k, v] of Object.entries(obj)) {
    const key = prefix ? prefix + '.' + k : k
    if (v && typeof v === 'object') flat(v, key, out)
    else out[key] = String(v)
  }
  return out
}
const zhF = flat(zh, '', {})
const enF = flat(en, '', {})
const hasCJK = /[一-鿿]/
const issues = []
// 1) 英文值疑似漏翻（含 CJK 且与中文完全一致）
for (const k of Object.keys(enF)) {
  if (enF[k] === zhF[k] && hasCJK.test(enF[k]) && enF[k].length >= 4) {
    issues.push(['same-cjk', k, enF[k].slice(0, 40)])
  }
}
// 2) 空值
for (const k of Object.keys(enF)) if (!enF[k].trim()) issues.push(['empty-en', k, ''])
for (const k of Object.keys(zhF)) if (!zhF[k].trim()) issues.push(['empty-zh', k, ''])
// 3) 英文含 CJK（非漏翻但也值得看）
for (const k of Object.keys(enF)) {
  if (hasCJK.test(enF[k]) && enF[k] !== zhF[k]) issues.push(['cjk-in-en', k, enF[k].slice(0, 40)])
}
console.log('zh keys:', Object.keys(zhF).length, 'en keys:', Object.keys(enF).length)
console.log('issues:', issues.length)
for (const i of issues.slice(0, 40)) console.log(i.join(' | '))
