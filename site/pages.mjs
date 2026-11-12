import { readFile, writeFile } from "node:fs/promises";
import { low, words } from "./highlight.mjs";
import { panels, rootOf } from "./layout.mjs";
import { Linker } from "./links.mjs";
import { textOf } from "./markup.mjs";
import { Program } from "./program.mjs";

const content = new URL("content/", import.meta.url);
const site = new URL("./", import.meta.url);
const block = /(<pre class="code"[^>]*data-lang="low"[^>]*><code>)([\s\S]*?)(<\/code><\/pre>)/g;
const expectation = / data-error="([^"]*)"/;
const open = "\u0001";
const close = "\u0002";

const sources = [
    {
        path: "index.html",
        title: "Low: a compiled language with enums and typed errors",
        description: "A compiled, garbage-collected language with green threads, enums and typed errors. It builds small static binaries for Linux and Windows.",
        current: "Overview",
        type: "website",
        scripts: ["copy.js", "hover.js"],
    },
    { path: "404.html", title: "Page not found: Low", error: true },
];

async function repaint(body, session, linker, label, expected) {
    const marked = textOf(body.replace(/<mark>/g, open).replace(/<\/mark>/g, close));
    const source = marked.replaceAll(open, "").replaceAll(close, "");
    const program = new Program(source);
    const checked = await session.inspect(program, words(source), label, expected);
    return low(marked, linker.facts(checked, source)).replaceAll(open, "<mark>").replaceAll(close, "</mark>");
}

async function render(page, session, library) {
    const linker = new Linker(library, rootOf(page));
    const html = await readFile(new URL(page.path, content), "utf8");
    const blocks = [...html.matchAll(block)];
    let painted = html;
    for (const [whole, start, body, end] of blocks) {
        const first = textOf(body).trim().split("\n")[0];
        const label = `${page.path}: ${first}`;
        const expected = expectation.exec(start)?.[1];
        const code = await repaint(body, session, linker, label, expected);
        const opening = start.replace(expectation, "");
        painted = painted.replace(whole, () => `${opening}${code}${end}`);
    }
    const document = panels({ ...page, body: painted });
    await writeFile(new URL(page.path, site), document);
    return blocks.length;
}

export async function pages(session, library) {
    const counts = [];
    for (const page of sources) {
        const count = await render(page, session, library);
        counts.push(`${page.path} ${count}`);
    }
    return counts.join(", ");
}
