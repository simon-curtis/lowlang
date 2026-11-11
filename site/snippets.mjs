import { readFile, writeFile } from "node:fs/promises";
import { low, words } from "./highlight.mjs";
import { Program } from "./program.mjs";

const pages = ["index.html"];
const block = /(<pre class="code"[^>]*data-lang="low"[^>]*><code>)([\s\S]*?)(<\/code><\/pre>)/g;
const open = "\u0001";
const close = "\u0002";

function unescape(text) {
    return text.replace(/&lt;/g, "<").replace(/&gt;/g, ">").replace(/&quot;/g, '"').replace(/&amp;/g, "&");
}

async function repaint(body, session) {
    const marked = unescape(body.replace(/<mark>/g, open).replace(/<\/mark>/g, close).replace(/<[^>]+>/g, ""));
    const source = marked.replaceAll(open, "").replaceAll(close, "");
    const program = new Program(source);
    const facts = await session.inspect(program, words(source));
    return low(marked, facts).replaceAll(open, "<mark>").replaceAll(close, "</mark>");
}

async function highlight(page, session) {
    const path = new URL(page, import.meta.url);
    const html = await readFile(path, "utf8");
    const blocks = [...html.matchAll(block)];
    let painted = html;
    for (const [whole, start, body, end] of blocks) {
        const code = await repaint(body, session);
        painted = painted.replace(whole, () => `${start}${code}${end}`);
    }
    await writeFile(path, painted);
    return blocks.length;
}

export async function snippets(session) {
    const counts = [];
    for (const page of pages) {
        const count = await highlight(page, session);
        counts.push(`${page} ${count}`);
    }
    return counts.join(", ");
}
