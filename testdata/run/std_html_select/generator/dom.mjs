import { readFileSync, writeFileSync } from 'node:fs'
import { createRequire } from 'node:module'

const require = createRequire(`${process.env.NODE_MODULES_DIR}/`)
const { JSDOM } = require('jsdom')

const corpus = JSON.parse(readFileSync(process.argv[2], 'utf8'))
const out = []
const doms = corpus.docs.map((d) => new JSDOM(d.html))
const elements = doms.map((dom) => [...dom.window.document.querySelectorAll('*')])

for (const selector of corpus.selectors) {
  const entry = { selector, docs: [], closest: [] }
  for (let d = 0; d < doms.length; d++) {
    const doc = doms[d].window.document
    const all = elements[d]
    try {
      const found = [...doc.querySelectorAll(selector)].map((e) => all.indexOf(e))
      entry.docs.push(found)
      const first = doc.querySelector(selector)
      entry.first ??= []
      entry.first.push(first ? all.indexOf(first) : -1)
      entry.closest.push(all.map((e) => {
        const c = e.closest(selector)
        return c ? all.indexOf(c) : -1
      }))
    } catch (e) {
      entry.error = String(e.message ?? e)
      entry.docs = null
      break
    }
  }
  out.push(entry)
}
writeFileSync(process.argv[3], JSON.stringify(out))
console.log(out.length, elements.map((e) => e.length))
