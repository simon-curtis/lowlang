import { mkdir, rm, writeFile } from "node:fs/promises";
import { Marked } from "marked";
import { codespan, words } from "./highlight.mjs";
import { folders } from "./config.mjs";
import { Program } from "./program.mjs";
import { layout, rootOf } from "./layout.mjs";
import { codeRenderer, Linker } from "./links.mjs";
import { escape, isLow } from "./markup.mjs";
import { lexed } from "./source.mjs";

const output = new URL(folders.docs, import.meta.url);
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

function render(page, facts, library) {
    const linker = new Linker(library, rootOf({ path: `${folders.docs}${page.file}` }));
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
            code: codeRenderer(linker, facts),
            codespan,
            table(token) {
                const head = token.header.map((cell) => `<th scope="col">${this.parser.parseInline(cell.tokens)}</th>`).join("");
                const labels = token.header.map((cell) => (token.header.length > 2 ? ` data-label="${escape(cell.text)}"` : ""));
                const rows = token.rows.map((row) => `<tr>${row.map((cell, column) => `<td${labels[column]}>${this.parser.parseInline(cell.tokens)}</td>`).join("")}</tr>`).join("\n");
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
    const sidebar = `<ul class="sidelist">\n${items.join("\n")}\n</ul>`;
    const title = page.file === "index.html" ? "Low documentation" : `${page.title}: Low documentation`;
    return layout({ path: `${folders.docs}${page.file}`, heading: page.title, title, description: page.description, current: "Docs", sidebar, body: page.body });
}

async function inspect(pages, session) {
    const facts = new Map();
    for (const page of pages) {
        let preludes = [];
        for (const token of page.tokens) {
            const found = token.type === "html" ? hidden.exec(token.text) : null;
            if (found) {
                preludes.push({ path: found[1], text: found[2] });
            } else if (token.type === "code" && isLow(token.lang) && !facts.has(token.text)) {
                const program = new Program(token.text, preludes);
                const label = `${page.title}: ${token.text.split("\n")[0]}`;
                const checked = await session.inspect(program, words(token.text), label);
                facts.set(token.text, checked);
                preludes = [];
            } else if (token.type !== "space") {
                preludes = [];
            }
        }
    }
    return facts;
}

function named(text) {
    return /^`(\w+)`$/.exec(text)?.[1];
}

export function builtins(design) {
    const chapter = sections(design, 2).find((page) => page.title === "Builtins");
    if (!chapter) {
        throw new Error("docs/design.md has no Builtins chapter");
    }
    const page = `${folders.docs}${slug(chapter.title)}.html`;
    const entries = new Map();
    let section = "";
    for (const token of chapter.tokens) {
        if (token.type === "heading") {
            section = slug(token.text);
            entries.set(named(token.text), `${page}#${section}`);
        } else if (token.type === "table") {
            for (const row of token.rows) {
                entries.set(named(row[0].text), `${page}#${section}`);
            }
        }
    }
    entries.delete(undefined);
    return entries;
}

export async function docs(design, session, library) {
    const editors = await lexed("editors/README.md");
    const chapters = sections(design, 2).map((page) => ({ ...page, file: `${slug(page.title)}.html` }));
    const editor = { title: "Editors", depth: 1, tokens: editors.filter((token) => token.type !== "heading" || token.depth !== 1), file: "editors.html" };
    const home = { title: "Documentation", depth: 1, tokens: preamble(design), file: "index.html" };
    const pages = [home, ...chapters, editor];
    const facts = await inspect(pages, session);
    for (const page of pages) {
        const lead = page.tokens.find((token) => token.type === "paragraph");
        page.description = lead ? plain(lead.tokens).slice(0, 200) : `${page.title} in the Low documentation.`;
        page.body = render(page, facts, library);
    }
    home.body = `${home.body}${contents(pages)}`;
    await rm(output, { recursive: true, force: true });
    await mkdir(output);
    for (const page of pages) {
        const html = document(page, pages);
        await writeFile(new URL(page.file, output), html);
    }
    return pages.length;
}
