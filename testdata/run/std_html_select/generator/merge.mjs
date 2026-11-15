import { readFileSync, writeFileSync } from 'node:fs'

const corpus = JSON.parse(readFileSync('corpus.json', 'utf8'))
const jsdom = JSON.parse(readFileSync('jsdom.json', 'utf8'))
const soup = JSON.parse(readFileSync('soup.json', 'utf8'))
const templateIndex = { 5: 18 }

const jsdomAuthority = new Set([
  ':empty', 'div:empty', 'footer *:empty', 'footer > :empty', 'div:not(:empty)', '*:empty',
  'foreignobject', 'FOREIGNOBJECT', 'lineargradient', '[viewbox]', 'svg[viewbox]',
  'svg|a', 'ns|a', 'a|b', 'a[ns|b]', 'a[b!=c]', 'div:has(:not(:has(p)))', ':has(:not(:has(p)))', ':has(:is(:has(p)))',
  'div:is(p, :unknown)', 'div:is(!)', 'div:where(!)', 'div:is(p, !)', 'div:is(!, p)', 'div:is(p !, span)', 'div:is(p, ::before)', 'div:where(p, !, span)',
  'a /* unterminated'
])
const unsupported = /:(hover|focus|visited|active|target|lang|dir|scope|required|link|any-link|optional|read-only)\b|^&$|^\||[a-z*]\||\[\*?\|/
const agreed = []
const disputed = []
let unsupportedCount = 0
for (let i = 0; i < jsdom.length; i++) {
  const j = jsdom[i]
  const s = soup[i]
  if (unsupported.test(j.selector) && j.docs !== null) {
    unsupportedCount++
    continue
  }
  const sdocs = s.docs?.map((found, d) => found.filter((x) => templateIndex[d] !== x))
  const strictHas = s.error?.includes('complex selectors') && /:has\(/.test(j.selector) && j.docs !== null
  if (jsdomAuthority.has(j.selector) || strictHas) {
    if (j.docs === null) agreed.push({ selector: j.selector, error: true })
    else agreed.push({ selector: j.selector, docs: j.docs, first: j.first, closest: j.closest })
    continue
  }
  if (j.docs === null && s.docs === null) {
    agreed.push({ selector: j.selector, error: true })
  } else if (j.docs === null || s.docs === null) {
    disputed.push({ selector: j.selector, jsdom: j.docs === null ? `error ${j.error}` : 'ok', soupsieve: s.docs === null ? `error ${s.error}` : 'ok' })
  } else if (JSON.stringify(j.docs) === JSON.stringify(sdocs)) {
    agreed.push({ selector: j.selector, docs: j.docs, first: j.first, closest: j.closest })
  } else {
    const where = []
    for (let d = 0; d < j.docs.length; d++) {
      if (JSON.stringify(j.docs[d]) !== JSON.stringify(sdocs[d])) where.push({ doc: corpus.docs[d].name, jsdom: j.docs[d], soupsieve: sdocs[d] })
    }
    disputed.push({ selector: j.selector, where })
  }
}
let valid = 0
const cases = agreed.map((a) => {
  if (a.error) return { selector: a.selector, error: true }
  valid++
  const c = { selector: a.selector, docs: a.docs, first: a.first }
  if (valid % 8 === 0) c.closest = a.closest
  return c
})
writeFileSync('cases.json', JSON.stringify({ docs: corpus.docs, cases }))
writeFileSync('agreed.json', JSON.stringify(agreed))
writeFileSync('disputed.json', JSON.stringify(disputed, null, 1))
console.log('agreed', agreed.length, 'errors', agreed.filter((a) => a.error).length, 'disputed', disputed.length)
for (const d of disputed) console.log(JSON.stringify(d))
