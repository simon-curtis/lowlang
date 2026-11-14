import { createRequire } from 'node:module'

const modules = process.env.NODE_MODULES_DIR
if (!modules) {
  console.error('usage: NODE_MODULES_DIR=<directory holding node_modules with jsdom> node dom.mjs')
  process.exit(2)
}
const { JSDOM } = createRequire(`${modules}/`)('jsdom')
const { document } = new JSDOM('').window

function element(name, attributes = {}) {
  const e = document.createElement(name)
  for (const [k, v] of Object.entries(attributes)) e.setAttribute(k, v)
  return e
}

function attempt(label, action) {
  try {
    action()
    console.log(`${label}: ok`)
  } catch (error) {
    console.log(`${label}: ${error.name === 'HierarchyRequestError' ? 'HierarchyRequest' : error.name === 'NotFoundError' ? 'NotFound' : error.name}`)
  }
}

const root = element('div', { id: 'r' })
const a = element('p', { id: 'a' })
const b = element('p', { id: 'b' })
const s = element('span', { id: 's' })
s.append('text')
a.append(s)
root.append(a, b)
console.log(`built: ${root.outerHTML}`)

attempt('append self', () => a.appendChild(a))
attempt('append ancestor into descendant', () => s.appendChild(root))
attempt('append parent into child', () => s.appendChild(a))
attempt('reference not a child', () => root.insertBefore(element('i'), s))
attempt('append into text', () => s.firstChild.appendChild(element('i')))
const other = document.implementation.createHTMLDocument('')
attempt('append document', () => root.appendChild(other))
console.log(`unchanged: ${root.outerHTML}`)

b.appendChild(s)
console.log(`moved: ${root.outerHTML}`)
const em = element('em')
root.insertBefore(em, a)
console.log(`inserted before: ${root.outerHTML}`)
root.insertBefore(element('u'), null)
console.log(`inserted last: ${root.outerHTML}`)
root.insertBefore(s, em)
console.log(`moved before: ${root.outerHTML}`)
b.remove()
console.log(`removed: ${root.outerHTML} parent ${b.parentNode === null ? 'none' : 'set'}`)

const shallow = a.cloneNode(false)
const deep = root.cloneNode(true)
console.log(`shallow: ${shallow.outerHTML} children ${shallow.childNodes.length}`)
console.log(`deep: ${deep.outerHTML} parent ${deep.parentNode === null ? 'none' : 'set'}`)
deep.firstChild.setAttribute('id', 'copy')
console.log(`copy independent: ${root.outerHTML.includes('copy')}`)

const item = element('li', { id: 'i1' })
item.setAttribute('class', 'x')
item.setAttribute('title', 'a "b" & c')
item.setAttribute('id', 'i2')
console.log(`attributes: ${item.outerHTML}`)
item.removeAttribute('class')
item.removeAttribute('missing')
console.log(`removed attribute: ${item.outerHTML} has id ${item.hasAttribute('id')} has class ${item.hasAttribute('class')}`)
console.log(`attribute value: ${item.getAttribute('title')} ${item.getAttribute('class')}`)

console.log(`contains: ${root.contains(s)} ${s.contains(root)} ${s.contains(s)} ${a.contains(s)} ${root.contains(b)}`)
console.log(`text: ${JSON.stringify(root.textContent)} ${JSON.stringify(s.textContent)} ${JSON.stringify(document.createTextNode('t').textContent)}`)
console.log(`navigation: ${root.firstChild.localName} ${root.lastChild.localName} ${s.nextSibling.localName} ${s.previousSibling === null ? 'none' : s.previousSibling.localName} ${s.parentNode.localName} ${root.lastChild.previousSibling.localName}`)
