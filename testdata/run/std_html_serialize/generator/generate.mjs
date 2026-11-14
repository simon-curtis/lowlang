import { readFileSync, readdirSync, writeFileSync } from 'node:fs'
import { createRequire } from 'node:module'

const require = createRequire(`${process.env.NODE_MODULES_DIR}/`)
const parse5 = require('parse5')

const dir = process.argv[2]
const out = process.argv[3]
const stride = Number(process.argv[4] ?? 1)

function cases(text) {
  const lines = text.split('\n')
  const result = []
  let start = 0
  while (start < lines.length && lines[start] === '') start++
  for (let i = start; i <= lines.length; i++) {
    const boundary = i === lines.length || (lines[i] === '' && (lines[i + 1] === '#data' || lines[i + 1] === '#description'))
    if (!boundary) continue
    let end = i
    while (end > start && lines[end - 1] === '') end--
    if (end > start) result.push(lines.slice(start, end))
    start = i + 1
  }
  return result
}

function read(lines) {
  let section = ''
  const data = []
  const document = []
  let context = null
  let script = false
  for (const l of lines) {
    if (l === '#description') section = 'description'
    else if (l === '#data') section = 'data'
    else if (l === '#errors' || l === '#new-errors') section = 'errors'
    else if (l === '#document-fragment') section = 'fragment'
    else if (l === '#document') section = 'document'
    else if (l === '#script-on') script = true
    else if (l === '#script-off') section = 'off'
    else if (section === 'data') data.push(l)
    else if (section === 'document') document.push(l)
    else if (section === 'fragment') {
      context = l
      section = 'off'
    }
  }
  return { input: data.join('\n'), context, script, document: document.join('\n') }
}

function contextNode(spec) {
  const parts = spec.split(' ')
  const adapter = parse5.defaultTreeAdapter
  if (parts.length === 2) {
    const ns = parts[0] === 'svg' ? parse5.html.NS.SVG : parse5.html.NS.MATHML
    return adapter.createElement(parts[1], ns, [])
  }
  return adapter.createElement(spec, parse5.html.NS.HTML, [])
}

function dump(root) {
  const out = []
  const walk = (node, depth) => {
    for (const c of node.childNodes ?? []) {
      const pad = '  '.repeat(depth)
      if (c.nodeName === '#text') out.push(`| ${pad}"${c.value}"`)
      else if (c.nodeName === '#comment') out.push(`| ${pad}<!-- ${c.data} -->`)
      else if (c.nodeName === '#documentType') {
        const pub = c.publicId ?? ''
        const sys = c.systemId ?? ''
        out.push(pub === '' && sys === '' ? `| ${pad}<!DOCTYPE ${c.name}>` : `| ${pad}<!DOCTYPE ${c.name} "${pub}" "${sys}">`)
      } else {
        const ns = c.namespaceURI
        const label = ns === parse5.html.NS.SVG ? `svg ${c.tagName}` : ns === parse5.html.NS.MATHML ? `math ${c.tagName}` : c.tagName
        out.push(`| ${pad}<${label}>`)
        const attrs = c.attrs.map((a) => [a.prefix ? `${a.prefix} ${a.name}` : a.name, a.value])
        attrs.sort((x, y) => (x[0] < y[0] ? -1 : x[0] > y[0] ? 1 : 0))
        for (const [k, v] of attrs) out.push(`| ${pad}  ${k}="${v}"`)
        if (c.tagName === 'template' && ns === parse5.html.NS.HTML) {
          out.push(`| ${pad}  content`)
          walk(c.content, depth + 2)
        } else walk(c, depth + 1)
      }
    }
  }
  walk(root, 0)
  return out
}

let skipped = 0
const result = []
let n = 0
for (const name of readdirSync(dir).filter((f) => f.endsWith('.dat')).sort()) {
  if (name.startsWith('scripted')) continue
  for (const lines of cases(readFileSync(`${dir}/${name}`, 'utf8'))) {
    const c = read(lines)
    if (c.script) continue
    let tree
    if (c.context === null) {
      tree = parse5.parse(c.input, { scriptingEnabled: false })
    } else {
      tree = parse5.parseFragment(contextNode(c.context), c.input, { scriptingEnabled: false })
    }
    if (dump(tree).join('\n') !== c.document) {
      skipped++
      continue
    }
    n++
    if (n % stride !== 0) continue
    result.push({ context: c.context, input: c.input, output: parse5.serialize(tree) })
  }
}
console.log('skipped where parse5 differs from the expected tree:', skipped)

const extras = [
  '<p title=\'a "q" & <b> &nbsp;x\'>1 &amp; 2 &lt; 3 &gt; 4&nbsp;5 &copy;  </p>',
  '<script>if (a < b && c > "d") { x = "</" + "p>" }</script>',
  '<style>a > b { content: "&amp;" }</style>',
  '<xmp><b>&amp;</b></xmp>',
  '<iframe><p>&amp;</p></iframe>',
  '<noembed><p>&amp;</p></noembed>',
  '<noframes><p>&amp;</p></noframes>',
  '<noscript><p>&amp;</p></noscript>',
  '<textarea>\n<b>&amp;</b></textarea>',
  '<title>a &amp; <b></title>',
  '<pre>\nx</pre><pre>\n\ny</pre><listing>\nz</listing>',
  '<br><hr><img src=x alt=""><input value=y disabled><wbr><area><base><col><embed><link><meta><source><track><param><keygen>',
  '<template><p>x</p><template><i>y</i></template></template>',
  '<svg viewBox="0 0 1 1" xlink:href="#a" xml:lang=en xmlns:xlink="http://www.w3.org/1999/xlink"><foreignObject><p>x</p></foreignObject><a/><script>a<b</script></svg>',
  '<math definitionurl=x><mi>a</mi><annotation-xml encoding="text/html"><p>q</p></annotation-xml></math>',
  '<!-- a -- b --><p>x<!--y-->',
  '<input disabled class="" data-x=\'a"b\'>',
  '<plaintext><b>x</b> &amp;',
  '<a href="?a=1&b=2&amp;c=3">link</a>',
  '<table><tr><td>a<td>b</table>',
  '<p>one<p>two<ul><li>a<li>b</ul>',
  '<div>' .repeat(3000) + 'deep' + '</div>'.repeat(3000),
  '  &nbsp;&nbsp;',
  '<p> \u0001\u007f &#x1F600; 😀</p>'
]
for (const input of extras) {
  const tree = parse5.parseFragment(contextNode('body'), input, { scriptingEnabled: false })
  result.push({ context: 'body', input, output: parse5.serialize(tree) })
  const first = tree.childNodes.find((c) => c.nodeName !== '#text' && c.nodeName !== '#comment')
  if (first) {
    result.push({ context: 'body', input, outer: true, output: parse5.serializeOuter(first) })
  }
}
writeFileSync(out, JSON.stringify(result))
console.log(result.length)
