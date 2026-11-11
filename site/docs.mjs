import { mkdir, readFile, rm, writeFile } from "node:fs/promises";
import { Marked } from "marked";
import { escape, low, words } from "./highlight.mjs";
import { Program } from "./program.mjs";
import { layout } from "./layout.mjs";

const repository = new URL("../", import.meta.url);
const output = new URL("docs/", import.meta.url);
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
    const sidebar = `<ul class="flex flex-wrap gap-x-5 gap-y-2 lg:block lg:space-y-2">\n${items.join("\n")}\n</ul>`;
    const title = page.file === "index.html" ? "Low documentation" : `${page.title}: Low documentation`;
    return layout({ path: `docs/${page.file}`, heading: page.title, title, description: page.description, current: "Docs", sidebar, body: page.body });
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
