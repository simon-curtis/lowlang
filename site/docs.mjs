import { mkdir, readFile, rm, writeFile } from "node:fs/promises";
import { Marked } from "marked";
import { escape, low, words } from "./highlight.mjs";
import { Program } from "./program.mjs";

const repository = new URL("../", import.meta.url);
const output = new URL("docs/", import.meta.url);
const origin = "https://low.simoncurtis.dev";
const hidden = /^<!-- prelude ?(\S*)\n([\s\S]*?)\n?-->/;

function slug(text) {
    return text.toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, "");
}

function plain(tokens) {
    return tokens.map((token) => token.text ?? "").join(" ").replace(/\s+/g, " ").trim();
}

function sections(tokens, depth) {
    const pages = [];
    for (const token of tokens) {
        if (token.type === "heading" && token.depth === depth) {
            pages.push({ title: token.text, depth, tokens: [] });
        } else if (pages.length > 0) {
            pages.at(-1).tokens.push(token);
        }
    }
    return pages;
}

function preamble(tokens) {
    const end = tokens.findIndex((token) => token.type === "heading" && token.depth === 2);
    return tokens.slice(0, end).filter((token) => token.type !== "heading");
}

function render(page, facts) {
    const marked = new Marked({
        renderer: {
            heading({ tokens, depth, text }) {
                const level = depth - page.depth + 1;
                const id = slug(text);
                return `<h${level} id="${id}"><a href="#${id}">${this.parser.parseInline(tokens)}</a></h${level}>\n`;
            },
            html({ text }) {
                return hidden.test(text) ? "" : text;
            },
            code({ text, lang }) {
                const body = lang === "low" ? low(text, facts.get(text)) : escape(text);
                return `<pre><code>${body}</code></pre>\n`;
            },
            table(token) {
                const head = token.header.map((cell) => `<th scope="col">${this.parser.parseInline(cell.tokens)}</th>`).join("");
                const rows = token.rows.map((row) => `<tr>${row.map((cell) => `<td>${this.parser.parseInline(cell.tokens)}</td>`).join("")}</tr>`).join("\n");
                return `<div class="overflow-x-auto"><table>\n<thead><tr>${head}</tr></thead>\n<tbody>\n${rows}\n</tbody>\n</table></div>\n`;
            },
        },
    });
    const links = Object.assign([...page.tokens], { links: {} });
    return marked.parser(links);
}

function contents(pages) {
    const items = pages.filter((page) => page.file !== "index.html").map((page) => `<li><a href="${page.file}">${escape(page.title)}</a></li>`);
    return `<ul>\n${items.join("\n")}\n</ul>\n`;
}

function document(page, pages) {
    const items = pages.map((entry) => {
        const current = entry === page ? ' class="font-bold underline underline-offset-4" aria-current="page"' : ' class="hover:underline hover:underline-offset-4"';
        return `<li><a${current} href="${entry.file}">${escape(entry.title)}</a></li>`;
    });
    const title = page.file === "index.html" ? "Low documentation" : `${page.title}: Low documentation`;
    const description = escape(page.description);
    return `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>${escape(title)}</title>
<meta name="description" content="${description}">
<meta property="og:type" content="article">
<meta property="og:title" content="${escape(title)}">
<meta property="og:description" content="${description}">
<meta property="og:url" content="${origin}/docs/${page.file}">
<meta property="og:image" content="${origin}/og-image.png">
<meta name="twitter:card" content="summary_large_image">
<link rel="canonical" href="${origin}/docs/${page.file}">
<meta name="theme-color" content="#f5f5f3">
<link rel="icon" href="../favicon.ico" sizes="48x48">
<link rel="icon" href="../icon.svg" type="image/svg+xml">
<link rel="apple-touch-icon" href="../apple-touch-icon.png">
<link rel="manifest" href="../site.webmanifest">
<link rel="preload" href="../fonts/inter.woff2" as="font" type="font/woff2" crossorigin>
<link rel="preload" href="../fonts/jetbrains-mono.woff2" as="font" type="font/woff2" crossorigin>
<link rel="stylesheet" href="../style.css">
</head>
<body>
<a class="skip" href="#main">Skip to content</a>
<header class="absolute inset-x-0 top-0 z-10">
<div class="wrap flex h-16 items-center justify-between">
<a class="flex items-center gap-2.5 text-lg font-bold" href="../index.html"><span class="mark size-5" aria-hidden="true"></span>Low</a>
<nav class="flex gap-3 text-sm font-semibold sm:gap-8">
<a class="hover:underline hover:underline-offset-8" href="../index.html">Overview</a>
<a class="hover:underline hover:underline-offset-8" href="../reference.html">Reference</a>
<a class="underline underline-offset-8" href="index.html">Docs</a>
<a class="hover:underline hover:underline-offset-8" href="https://github.com/simon-curtis/lowlang">GitHub</a>
</nav>
</div>
</header>

<div class="wrap grid grid-cols-[minmax(0,1fr)] gap-x-16 gap-y-10 pt-28 pb-24 lg:grid-cols-[13rem_minmax(0,1fr)]">
<nav class="lg:sticky lg:top-8 lg:self-start" aria-label="Documentation">
<ul class="flex flex-wrap gap-x-5 gap-y-2 lg:block lg:space-y-2">
${items.join("\n")}
</ul>
</nav>
<main class="prose" id="main">
<h1>${escape(page.title)}</h1>
${page.body}</main>
</div>
<script src="../lenis.js" defer></script>
<script src="../scroll.js" defer></script>
<script src="../hover.js" defer></script>
</body>
</html>
`;
}

async function read(path) {
    const text = await readFile(new URL(path, repository), "utf8");
    return new Marked().lexer(text);
}

async function inspect(pages, session) {
    const facts = new Map();
    const unchecked = [];
    for (const page of pages) {
        let preludes = [];
        for (const token of page.tokens) {
            const found = token.type === "html" ? hidden.exec(token.text) : null;
            if (found) {
                preludes.push({ path: found[1], text: found[2] });
            } else if (token.type === "code" && token.lang === "low" && !facts.has(token.text)) {
                const program = new Program(token.text, preludes);
                const checked = await session.inspect(program, words(token.text));
                facts.set(token.text, checked);
                if (checked.errors.length > 0) {
                    unchecked.push(`${page.title}: ${token.text.split("\n")[0]}: ${checked.errors.join("; ")}`);
                }
                preludes = [];
            } else if (token.type !== "space") {
                preludes = [];
            }
        }
    }
    return { facts, unchecked };
}

export async function docs(session) {
    const design = await read("docs/design.md");
    const editors = await read("editors/README.md");
    const chapters = sections(design, 2).map((page) => ({ ...page, file: `${slug(page.title)}.html` }));
    const editor = { title: "Editors", depth: 1, tokens: editors.filter((token) => token.type !== "heading" || token.depth !== 1), file: "editors.html" };
    const home = { title: "Documentation", depth: 1, tokens: preamble(design), file: "index.html" };
    const pages = [home, ...chapters, editor];
    const { facts, unchecked } = await inspect(pages, session);
    for (const page of pages) {
        const lead = page.tokens.find((token) => token.type === "paragraph");
        page.description = lead ? plain(lead.tokens).slice(0, 200) : `${page.title} in the Low documentation.`;
        page.body = render(page, facts);
    }
    home.body = `${home.body}${contents(pages)}`;
    await rm(output, { recursive: true, force: true });
    await mkdir(output);
    for (const page of pages) {
        const html = document(page, pages);
        await writeFile(new URL(page.file, output), html);
    }
    return { pages: pages.length, unchecked };
}
