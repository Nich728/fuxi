// First-load JavaScript budget: the gzipped size of every script index.html
// loads up front (the entry chunk and its modulepreloads). Lazy chunks don't
// count. Fails the build above BUDGET_KB, so the phone stays fast.
import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { gzipSync } from 'node:zlib'

const BUDGET_KB = 150
const dist = fileURLToPath(new URL('../dist/', import.meta.url))
const html = readFileSync(join(dist, 'index.html'), 'utf8')
const files = [...html.matchAll(/(?:src|href)="\/([^"]+\.js)"/g)].map((m) => m[1])
if (files.length === 0) throw new Error('no scripts found in dist/index.html')

let total = 0
for (const f of new Set(files)) {
  const kb = gzipSync(readFileSync(join(dist, f)), { level: 9 }).length / 1024
  total += kb
  console.log(`${kb.toFixed(1).padStart(7)} KB  ${f}`)
}
console.log(`${total.toFixed(1).padStart(7)} KB  first-load JS (gzipped), budget ${BUDGET_KB} KB`)
if (total > BUDGET_KB) {
  console.error(`Over budget by ${(total - BUDGET_KB).toFixed(1)} KB.`)
  process.exit(1)
}
