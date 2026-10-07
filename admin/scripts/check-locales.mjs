// 校验各 locale 文件的 key 集合完全一致。
//
// 漏 key 是 i18n 改造最常见的失败模式,而它在运行时只表现为界面上出现
// 一个原样的 key 字符串(如 "users.table.nickname"),很容易漏过 review。
// 挂在 build 前置,缺 key 即构建失败。
import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const here = dirname(fileURLToPath(import.meta.url))
const LOCALES = ['zh-CN', 'en']
const BASE = LOCALES[0]

function flatten(obj, prefix = '', out = []) {
  for (const [k, v] of Object.entries(obj)) {
    const key = prefix ? `${prefix}.${k}` : k
    if (v && typeof v === 'object' && !Array.isArray(v)) flatten(v, key, out)
    else out.push(key)
  }
  return out
}

const keys = {}
for (const l of LOCALES) {
  const file = resolve(here, '../src/locales', `${l}.json`)
  keys[l] = new Set(flatten(JSON.parse(readFileSync(file, 'utf8'))))
}

let bad = false
for (const l of LOCALES.slice(1)) {
  const missing = [...keys[BASE]].filter((k) => !keys[l].has(k))
  const extra = [...keys[l]].filter((k) => !keys[BASE].has(k))
  if (missing.length) {
    bad = true
    console.error(`\n[${l}] 缺 ${missing.length} 个 key:\n  ${missing.join('\n  ')}`)
  }
  if (extra.length) {
    bad = true
    console.error(`\n[${l}] 多出 ${extra.length} 个 key(${BASE} 里没有):\n  ${extra.join('\n  ')}`)
  }
}

if (bad) {
  console.error('\nlocale key 不对齐,构建中止。')
  process.exit(1)
}
console.log(`locale key 对齐 ✓ (${keys[BASE].size} 个 key × ${LOCALES.length} 种语言)`)
