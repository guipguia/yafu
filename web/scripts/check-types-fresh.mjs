#!/usr/bin/env node
// CI guard that fails when src/lib/api-types.ts is stale relative
// to api/openapi.yaml. Regenerates the types into a temp file and
// byte-compares to the checked-in version. If a maintainer
// updated the spec without running `npm run gen:types`, this fires
// and the diff is printed for review.

import { execSync } from 'node:child_process'
import { readFileSync, mkdtempSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import process from 'node:process'

const tmp = mkdtempSync(join(tmpdir(), 'yafu-types-'))
const fresh = join(tmp, 'api-types.ts')
const checkedIn = 'src/lib/api-types.ts'

try {
  execSync(`npx openapi-typescript ../api/openapi.yaml -o ${fresh}`, {
    stdio: 'inherit',
  })
  const a = readFileSync(checkedIn, 'utf8')
  const b = readFileSync(fresh, 'utf8')
  if (a === b) {
    console.log('api-types.ts is fresh relative to api/openapi.yaml')
    process.exit(0)
  }
  console.error('---')
  console.error('api-types.ts is stale. Run `npm run gen:types` and commit the result.')
  console.error('---')
  // Show a small diff hint — first divergence line.
  const aLines = a.split('\n')
  const bLines = b.split('\n')
  for (let i = 0; i < Math.max(aLines.length, bLines.length); i++) {
    if (aLines[i] !== bLines[i]) {
      console.error(`first divergence at line ${i + 1}:`)
      console.error(`  checked-in: ${aLines[i] ?? '(eof)'}`)
      console.error(`  fresh     : ${bLines[i] ?? '(eof)'}`)
      break
    }
  }
  process.exit(1)
} finally {
  rmSync(tmp, { recursive: true, force: true })
}
