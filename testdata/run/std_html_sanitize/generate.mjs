import { writeFileSync } from 'node:fs'
import { createRequire } from 'node:module'

const modules = process.env.NODE_MODULES_DIR
const output = process.argv[2]
if (!modules || !output) {
  console.error('usage: NODE_MODULES_DIR=<directory holding node_modules with parse5> node generate.mjs <cases.json>')
  process.exit(2)
}
const parse5 = createRequire(`${modules}/`)('parse5')
const { NS } = parse5.html
const adapter = parse5.defaultTreeAdapter
const rounds = 5

const defaultElements = [
  'a', 'abbr', 'address', 'article', 'aside', 'b', 'bdi', 'bdo', 'blockquote', 'br', 'caption', 'cite', 'code', 'col', 'colgroup',
  'dd', 'del', 'details', 'dfn', 'div', 'dl', 'dt', 'em', 'figcaption', 'figure', 'footer', 'h1', 'h2', 'h3', 'h4', 'h5', 'h6',
  'header', 'hr', 'i', 'img', 'ins', 'kbd', 'li', 'main', 'mark', 'nav', 'ol', 'p', 'pre', 'q', 'rp', 'rt', 'ruby', 's', 'samp',
  'section', 'small', 'span', 'strong', 'sub', 'summary', 'sup', 'table', 'tbody', 'td', 'tfoot', 'th', 'thead', 'time', 'tr',
  'u', 'ul', 'var', 'wbr'
]
const defaultSpecific = {
  a: ['href', 'hreflang', 'rel'],
  img: ['src', 'srcset', 'alt', 'width', 'height'],
  ol: ['start', 'type', 'reversed'],
  li: ['value'],
  td: ['colspan', 'rowspan', 'headers'],
  th: ['colspan', 'rowspan', 'headers', 'scope', 'abbr'],
  col: ['span'],
  colgroup: ['span'],
  blockquote: ['cite'],
  q: ['cite'],
  del: ['cite', 'datetime'],
  ins: ['cite', 'datetime'],
  time: ['datetime'],
  details: ['open']
}
const defaultDiscard = [
  'script', 'style', 'noscript', 'template', 'iframe', 'object', 'embed', 'applet', 'noembed', 'noframes', 'plaintext', 'xmp',
  'title', 'textarea', 'select', 'head', 'frameset', 'frame', 'audio', 'video', 'canvas', 'button', 'dialog', 'datalist'
]

const policies = {
  default: {
    elements: defaultElements,
    global: ['class', 'dir', 'lang', 'title'],
    specific: defaultSpecific,
    schemes: ['http', 'https', 'mailto'],
    relative: true,
    discard: defaultDiscard,
    svg: [],
    mathml: []
  },
  rich: {
    elements: [...defaultElements, 'form', 'input', 'button', 'video', 'font'],
    global: ['class', 'id', 'dir', 'lang', 'title', 'd', 'fill', 'viewbox', 'href', 'xlink:href', 'xmlns', 'xmlns:xlink', 'width', 'height'],
    specific: {
      ...defaultSpecific,
      form: ['action', 'method'],
      input: ['type', 'name', 'value', 'formaction'],
      button: ['formaction', 'type'],
      video: ['src', 'poster', 'controls'],
      font: ['color']
    },
    schemes: ['http', 'https', 'mailto', 'tel'],
    relative: false,
    discard: defaultDiscard.filter((n) => n !== 'button' && n !== 'video'),
    svg: ['svg', 'g', 'path', 'a', 'text', 'title', 'desc', 'linearGradient', 'circle', 'rect'],
    mathml: ['math', 'mi', 'mo', 'mn', 'mrow', 'mtext', 'mglyph', 'annotation-xml', 'maction']
  }
}

const alwaysRemoved = new Set([
  'script', 'style', 'noscript', 'template', 'iframe', 'object', 'embed', 'applet', 'noembed', 'noframes', 'plaintext', 'xmp',
  'frame', 'frameset', 'base', 'link', 'meta'
])
const svgDenied = new Set([
  'script', 'style', 'foreignObject', 'use', 'animate', 'animateMotion', 'animateTransform', 'animateColor', 'set', 'handler',
  'listener', 'feImage', 'image', 'iframe'
])
const mathDenied = new Set(['annotation-xml', 'maction', 'script', 'style'])
const singleUrl = new Set([
  'href', 'src', 'action', 'formaction', 'poster', 'cite', 'xlink:href', 'background', 'data', 'longdesc', 'manifest', 'usemap',
  'codebase', 'classid', 'profile', 'icon', 'lowsrc', 'dynsrc'
])

function urlAllowed(value, policy) {
  const stripped = value.replace(/[\t\n\r]/g, '').replace(/^[\u0000- ]+/, '').replace(/[\u0000- ]+$/, '')
  const stop = stripped.search(/[:/?#]/)
  if (stop < 0 || stripped[stop] !== ':') return policy.relative
  return policy.schemes.includes(stripped.slice(0, stop).toLowerCase())
}

function candidatesAllowed(value, policy) {
  let at = 0
  while (at < value.length) {
    while (at < value.length && (/[\t\n\f\r ]/.test(value[at]) || value[at] === ',')) at++
    const start = at
    while (at < value.length && !/[\t\n\f\r ]/.test(value[at])) at++
    let end = at
    while (end > start && value[end - 1] === ',') end--
    if (end > start && !urlAllowed(value.slice(start, end), policy)) return false
    if (end === at) {
      let depth = 0
      while (at < value.length && !(value[at] === ',' && depth === 0)) {
        if (value[at] === '(') depth++
        if (value[at] === ')' && depth > 0) depth--
        at++
      }
    }
  }
  return true
}

function attributeAllowed(element, attr, policy) {
  const key = (attr.prefix ? `${attr.prefix}:${attr.name}` : attr.name).toLowerCase()
  if (key.startsWith('on') || key === 'srcdoc') return false
  const own = policy.specific[element.tagName.toLowerCase()] ?? []
  if (!policy.global.includes(key) && !own.includes(key)) return false
  if (singleUrl.has(key)) return urlAllowed(attr.value, policy)
  if (key === 'srcset' || key === 'imagesrcset') return candidatesAllowed(attr.value, policy)
  if (key === 'ping') return attr.value.split(/[\t\n\f\r ]+/).filter((p) => p !== '').every((p) => urlAllowed(p, policy))
  return true
}

function verdict(node, policy) {
  if (node.nodeName === '#text') return 'keep'
  if (node.nodeName.startsWith('#')) return 'remove'
  const name = node.tagName
  if (node.namespaceURI === NS.HTML) {
    if (alwaysRemoved.has(name)) return 'remove'
    if (policy.elements.includes(name)) return 'keep'
    return policy.discard.includes(name) ? 'remove' : 'unwrap'
  }
  if (node.namespaceURI === NS.SVG) return policy.svg.includes(name) && !svgDenied.has(name) ? 'keep' : 'remove'
  if (node.namespaceURI === NS.MATHML) return policy.mathml.includes(name) && !mathDenied.has(name) ? 'keep' : 'remove'
  return 'remove'
}

function clean(parent, policy) {
  let changes = 0
  let i = 0
  while (i < parent.childNodes.length) {
    const child = parent.childNodes[i]
    const v = verdict(child, policy)
    if (v === 'remove') {
      parent.childNodes.splice(i, 1)
      changes++
    } else if (v === 'unwrap') {
      for (const grand of child.childNodes) grand.parentNode = parent
      parent.childNodes.splice(i, 1, ...child.childNodes)
      changes++
    } else {
      if (child.attrs) {
        const kept = child.attrs.filter((a) => attributeAllowed(child, a, policy))
        changes += child.attrs.length - kept.length
        child.attrs = kept
      }
      if (child.childNodes) changes += clean(child, policy)
      i++
    }
  }
  return changes
}

function fragment(input) {
  const context = adapter.createElement('body', NS.HTML, [])
  return parse5.parseFragment(context, input, { scriptingEnabled: false })
}

function render(node) {
  const padded = []
  const walk = (parent) => {
    for (const child of parent.childNodes ?? []) {
      if (child.namespaceURI === NS.HTML && ['pre', 'textarea', 'listing'].includes(child.tagName)) {
        const first = child.childNodes[0]
        if (first && first.nodeName === '#text' && first.value.startsWith('\n')) {
          first.value = `\n${first.value}`
          padded.push(first)
        }
      }
      walk(child)
    }
  }
  walk(node)
  const out = parse5.serialize(node, { scriptingEnabled: false })
  for (const text of padded) text.value = text.value.slice(1)
  return out
}

let unsettled = 0

function sanitize(input, policy) {
  let current = input
  for (let round = 0; round < rounds; round++) {
    const first = fragment(current)
    clean(first, policy)
    const out = render(first)
    const again = fragment(out)
    const reserialized = render(again)
    const changes = clean(again, policy)
    if (changes === 0 && reserialized === out) return out
    current = render(again)
  }
  unsettled++
  return ''
}

function violations(output) {
  const found = []
  const walk = (node) => {
    if (node.nodeName === 'script') found.push('script element')
    for (const a of node.attrs ?? []) {
      const name = (a.prefix ? `${a.prefix}:${a.name}` : a.name).toLowerCase()
      if (name.startsWith('on')) found.push(`event handler ${name}`)
      if (name === 'srcdoc') found.push('srcdoc')
      const tokens = ['srcset', 'imagesrcset', 'ping'].includes(name) ? a.value.split(/[\t\n\f\r ,]+/) : [a.value]
      for (const token of tokens) {
        const value = token.replace(/[\t\n\r]/g, '').replace(/^[\u0000- ]+/, '').toLowerCase()
        if (value.startsWith('javascript:') || value.startsWith('vbscript:')) found.push(`script URL in ${name}`)
      }
    }
    for (const c of node.childNodes ?? []) walk(c)
    if (node.content) walk(node.content)
  }
  walk(fragment(output))
  return found
}

const urlPayloads = [
  'javascript:alert(1)', 'JAVASCRIPT:alert(1)', 'JaVaScRiPt:alert(1)', ' javascript:alert(1)', '\tjavascript:alert(1)',
  '&#9;javascript:alert(1)', 'java&Tab;script:alert(1)', 'java&NewLine;script:alert(1)', '&#106;avascript:alert(1)',
  '&#x6A;avascript:alert(1)', 'javascript&colon;alert(1)', '&#0000106avascript:alert(1)', 'java\nscript:alert(1)',
  '\u0001javascript:alert(1)', '\u001fjavascript:alert(1)', 'JAVASCRIPT&#58;alert(1)', 'javascript://%0aalert(1)',
  'java script:alert(1)', 'data:text/html,<script>alert(1)</script>', 'data:text/html;base64,PHNjcmlwdD5hbGVydCgxKTwvc2NyaXB0Pg==',
  'vbscript:msgbox(1)', 'jar:http://x/!/', 'blob:http://x/y', 'file:///etc/passwd', 'ftp://a/b', 'http://a/b', 'HTTPS://a/b',
  ' https://a/b ', 'https://a/b?x=javascript:1', '//a/b', '/a/b', 'a/b', '?q=1', '#frag', 'a:b', 'mailto:a@b.c', 'MAILTO:a@b.c',
  'tel:+1', ':alert(1)', 'javascript', 'x:y/z', '1x:y', 'http&colon;//a', 'http:', ' ', '', '\\\\a\\b', 'h\u0000ttp://a'
]
const urlContexts = [
  (v) => `<a href="${v}">x</a>`,
  (v) => `<img src="${v}">`,
  (v) => `<form action="${v}"><input name=n></form>`,
  (v) => `<button formaction="${v}">b</button>`,
  (v) => `<input type=submit formaction="${v}">`,
  (v) => `<video poster="${v}" src="${v}"></video>`,
  (v) => `<blockquote cite="${v}">q</blockquote>`,
  (v) => `<img srcset="${v} 1x, b.png 2x">`,
  (v) => `<img srcset="b.png 1x,${v}">`,
  (v) => `<svg><a xlink:href="${v}"><text>t</text></a></svg>`,
  (v) => `<svg><a href="${v}"><text>t</text></a></svg>`,
  (v) => `<a href="${v}" ping="${v} https://a/b">p</a>`
]

const handwritten = [
  '<script>alert(1)</script>',
  '<SCRIPT SRC=//x/y.js></SCRIPT>',
  '<p onclick=x>hi <b>there</b></p><script>alert(1)</script>',
  '<img src=x onerror=alert(1)>',
  '<svg onload=alert(1)>',
  '<body onload=alert(1)>',
  '<iframe srcdoc="<script>alert(1)</script>"></iframe>',
  '<a href=javascript:alert(1)>x</a>',
  '<math><mi xlink:href="javascript:alert(1)">x</mi></math>',
  '<svg><use xlink:href="data:image/svg+xml,<svg id=x xmlns=http://www.w3.org/2000/svg><script>alert(1)</script></svg>#x"/></svg>',
  '<svg></p><style><a id="</style><img src=1 onerror=alert(1)>">',
  '<math><mtext><table><mglyph><style><!--</style><img title="--&gt;&lt;img src=1 onerror=alert(1)&gt;">',
  '<noscript><p title="</noscript><img src=x onerror=alert(1)>">',
  '<form><math><mtext></form><form><mglyph><style></math><img src onerror=alert(1)>',
  '<svg><foreignObject><p><style><!--</style><img src onerror=alert(1)>--></p></foreignObject></svg>',
  '<select><template><style><img src=x onerror=alert(1)></style></template></select>',
  '<textarea></textarea><img src=x onerror=alert(1)>',
  '<textarea><img src=x onerror=alert(1)></textarea>',
  '<xmp><img src=x onerror=alert(1)></xmp>',
  '<listing><img src=x onerror=alert(1)></listing>',
  '<table><colgroup><template><img src=x onerror=alert(1)></template></colgroup></table>',
  '<svg><p><img src=x onerror=alert(1)></p></svg>',
  '<math><annotation-xml encoding="text/html"><script>alert(1)</script></annotation-xml></math>',
  '<math><annotation-xml encoding="text/html"><img src=x onerror=alert(1)></annotation-xml></math>',
  '<svg><![CDATA[</desc><script>alert(1)</script>]]></svg>',
  '<svg><title><style><img src onerror=alert(1)></style>',
  '<a href="x" onclick="alert(1)" ONMOUSEOVER=alert(1) data-x=1 style="x">t</a>',
  '<div style="background:url(javascript:alert(1))">d</div>',
  '<!--><script>alert(1)</script>-->',
  '<!-- --!><img src=x onerror=alert(1)>-->',
  '<?xml ?><img src=x onerror=alert(1)>',
  '<![CDATA[x]]><b>y</b>',
  '<plaintext><img src=x onerror=alert(1)>',
  '<noembed><img src=x onerror=alert(1)></noembed>',
  '<iframe><img src=x onerror=alert(1)></iframe>',
  '<style>x{}</style>text',
  '<template><img src=x onerror=alert(1)></template>',
  '<object data=javascript:alert(1)></object>',
  '<embed src=javascript:alert(1)>',
  '<base href=javascript:alert(1)//>',
  '<link rel=import href=data:text/html,x>',
  '<meta http-equiv=refresh content="0;url=javascript:alert(1)">',
  '<button formaction=javascript:alert(1)>x</button>',
  '<input onfocus=alert(1) autofocus>',
  '<details open ontoggle=alert(1)><summary>s</summary>d</details>',
  '<video><source onerror=alert(1)></video>',
  '<b><p>x</b>y</p>',
  '<a href=/x><div><a href=/y>z</a></div></a>',
  '<table><a href=/x>t<tr><td>c</table>',
  '<table><tr><td><svg><desc><td>',
  '<h1><h2>x</h2></h1>',
  '<ul><li>a<li>b</ul>',
  '<select><option>a<option>b</select>',
  '<p>a<p>b',
  '<dl><dt>a<dd>b<dt>c',
  '<ruby>a<rt>b<rp>c</ruby>',
  '<table><td>a<caption>b',
  '<frameset><frame>',
  '<math><mi>x</mi><mo>+</mo></math>',
  'a &amp; b &lt; c &nbsp; d',
  '&notin; &amp &copy &#0; &#x110000; &#128512;',
  '<p title="a&quot;b\'c<>&amp;">t</p>',
  '<img src=&#106;avascript:alert(1)>',
  '<custom-elem><b>x</b></custom-elem>',
  '<font color=red face=x>x</font>',
  '<center>c</center><marquee>m</marquee><blink>b</blink>',
  '<form><input name=x><label>l</label></form>',
  '<html lang=x><head><title>t</title></head><body class=y>b</body></html>',
  '<svg viewBox="0 0 1 1"><path d="M0 0"/><a href=javascript:1><text>t</text></a><script>1</script><foreignObject><p>x</p></foreignObject><use href=#a /><image href=x /><animate onbegin=alert(1) attributeName=x /><set attributeName=onmouseover to=alert(1) /></svg>',
  '<svg><linearGradient id=g><stop offset=0 /></linearGradient><circle r=1 fill=url(#g) /><rect width=1 onclick=alert(1) /></svg>',
  '<math><mi>x</mi><mo>+</mo><annotation-xml encoding=text/html><img src onerror=alert(1)></annotation-xml><maction actiontype=statusline xlink:href=javascript:alert(1)>x</maction></math>',
  '<svg><a xlink:href="javascript:alert(1)"><text>x</text></a></svg>',
  '<a href=/x href=javascript:alert(1)>d</a>',
  '<a HREF=JAVASCRIPT:x>u</a>',
  '<img src="https://a/b.png" alt="a" width=1 height=2 onload=x>',
  '<a title="a&#10;b" href="/x">n</a>',
  '<p class="a b" id=x lang=en dir=rtl title=t hidden data-a=b>p</p>',
  '<ol start=3 reversed type=a><li value=5>x</ol>',
  '<table><tr><td colspan=2 rowspan=2 headers=a>c</td><th scope=col abbr=x>h</table>',
  '<time datetime=2020>t</time><q cite=javascript:1>q</q><del cite=https://x datetime=1>d</del>',
  '<p>héllo 😀     end</p>',
  '',
  ' ',
  'text only',
  '<',
  '<>',
  '</>',
  '<a',
  '</p>',
  '<p',
  '&',
  '<img srcset="a.png, b.png 2x">',
  '<img srcset="a.png 1x,javascript:alert(1)">',
  '<img srcset="data:image/png;base64,AAA 1x">',
  '<img srcset="  https://a/b.png  100w ,  /c.png 200w ">',
  '<img srcset="a.png 1x, b.png(1,2) 2x">',
  '<img srcset=",">',
  '<img srcset="">',
  '<img srcset="a.png,,,b.png">',
  '<img srcset="//x.y/a.png 1x">',
  '<img srcset="a.png 1x, javascript:alert(1) 2x" src=ok.png>',
  '<a ping="https://a javascript:1">p</a>',
  '<pre>\nx</pre>',
  '<pre>\n\nx</pre>',
  `<pre>${'\n'.repeat(12)}x</pre>`,
  '<pre><font>\nx</font></pre>',
  '<pre><b>\n</b>\ny</pre>',
  '<pre>\n<script>1</script>\nz</pre>',
  '<listing>\n\nq</listing>',
  '<textarea>\n\nq</textarea>',
  '<pre>\r\nx</pre>',
  `${'<div>'.repeat(200)}x${'</div>'.repeat(200)}`,
  `${'<font>'.repeat(500)}hi${'</font>'.repeat(500)}`,
  `${'<span>'.repeat(300)}<script>alert(1)</script>${'</span>'.repeat(300)}`,
  `${'<b>'.repeat(50)}<p>x</b>y</p>`,
  `${'<table>'.repeat(20)}<td>x`
]

function seeded(seed) {
  let state = seed
  return () => {
    state = (state * 1103515245 + 12345) % 2147483648
    return state / 2147483648
  }
}

const vocabulary = [
  '<p>', '</p>', '<b>', '</b>', '<i>', '</i>', '<a href=x>', '</a>', '<table>', '</table>', '<tr>', '<td>', '</td>', '<svg>', '</svg>',
  '<math>', '</math>', '<mtext>', '<mglyph>', '<style>', '</style>', '<script>', '</script>', '<noscript>', '</noscript>',
  '<template>', '</template>', '<select>', '<option>', '</select>', '<form>', '</form>', '<img src=x onerror=1>', '<!--', '-->',
  '"', '>', '<title>', '</title>', '<textarea>', '</textarea>', '<foreignObject>', '</foreignObject>', '<desc>', '<annotation-xml>',
  '<div>', '</div>', '<li>', '<ul>', '<h1>', '<br>', '<caption>', '<colgroup>', '<col>', '<frameset>', '<xmp>', '<iframe>', '<plaintext>',
  '<a href=javascript:1>', '<font>', '<center>', '<nobr>', '<button>', '</button>', '<details>', '<summary>', 'x', 'y z', '&amp;'
]
const random = seeded(18)
const generated = []
for (let i = 0; i < 160; i++) {
  const length = 4 + Math.floor(random() * 12)
  let doc = ''
  for (let k = 0; k < length; k++) doc += vocabulary[Math.floor(random() * vocabulary.length)]
  generated.push(doc)
}

const inputs = []
for (const context of urlContexts) for (const url of urlPayloads) inputs.push(context(url))
inputs.push(...handwritten, ...generated)

const cases = []
let problems = 0
for (const name of Object.keys(policies)) {
  for (const input of inputs) {
    const expected = sanitize(input, policies[name])
    const found = violations(expected)
    if (found.length > 0) {
      problems++
      console.error(`unsafe output for ${JSON.stringify(input)}: ${found.join(', ')}`)
    }
    cases.push({ policy: name, input, expected })
  }
}
const emptied = cases.filter((c) => c.expected === '' && c.input !== '').length
writeFileSync(output, JSON.stringify({ cases }))
console.log(`${cases.length} cases, ${problems} unsafe, ${emptied} emptied, ${unsettled} unsettled`)
