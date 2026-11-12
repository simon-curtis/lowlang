import { execFile } from "node:child_process";
import { mkdir, rm, writeFile } from "node:fs/promises";
import { promisify } from "node:util";
import { Marked } from "marked";
import { all, categories, orphaned } from "./categories.mjs";
import { compilerSource, folders } from "./config.mjs";
import { codespan, low, words } from "./highlight.mjs";
import { layout, rootOf } from "./layout.mjs";
import { anchor, codeRenderer, file, Linker } from "./links.mjs";
import { escape, isLow, textOf } from "./markup.mjs";
import { Program } from "./program.mjs";

const run = promisify(execFile);
const output = new URL(folders.packages, import.meta.url);
const github = '<svg class="size-5" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true"><path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z"/></svg>';
const markdown = new Marked({ renderer: { codespan } });

const comment = /^( *)\/\/\/ ?(.*)$/;

function blocks(text) {
    const groups = [];
    for (const line of text.split("\n")) {
        const found = comment.exec(line);
        const group = found ? { kind: "note", indent: found[1].length, line: found[2] } : { kind: "code", line };
        const last = groups.at(-1);
        if (last?.kind === group.kind && last.indent === group.indent) {
            last.lines.push(group.line);
        } else {
            groups.push({ kind: group.kind, indent: group.indent, lines: [group.line] });
        }
    }
    return groups;
}

function note(indent, lines) {
    const paragraphs = lines.join("\n").trim().split(/\n{2,}/).map((text) => `<span>${markdown.parseInline(text)}</span>`);
    return `<span class="note pl-[${indent}ch]"><span>${paragraphs.join("")}</span></span>`;
}

function summary(text, limit) {
    return text.length <= limit ? text : `${text.slice(0, text.lastIndexOf(" ", limit))}…`;
}

function members(module) {
    const types = module.items.filter((item) => item.kind !== "fn");
    const functions = module.items.filter((item) => item.kind === "fn" && !item.receiver);
    return { types, functions };
}

function tags(path) {
    return categories(path).map((name) => `<a class="tag" href="index.html?category=${name}">#${name}</a>`).join(" ");
}

function descriptions(design) {
    const about = new Map();
    markdown.walkTokens(design, (token) => {
        for (const [cell, text] of token.type === "table" ? token.rows : []) {
            const path = /^`(std[\w/]*)`$/.exec(cell.text)?.[1];
            if (path) {
                about.set(path, markdown.parseInline(text.text));
            }
        }
    });
    return about;
}

function sidebarList(items) {
    const links = items.map((item) => `<li><a class="hover:underline hover:underline-offset-4" href="#${item.name}">${escape(item.name)}</a></li>`);
    return `<ul class="sidelist">\n${links.join("\n")}\n</ul>`;
}

class Page {
    #module;
    #description;
    #types;
    #functions;
    #typeNames;
    #exports;
    #library;
    #root;
    #markdown;

    constructor(module, description, exports, examples, library) {
        this.#module = module;
        this.#description = description;
        const { types, functions } = members(module);
        this.#types = types;
        this.#functions = functions;
        this.#typeNames = new Set(types.map((item) => item.name));
        this.#exports = exports;
        this.#library = library;
        this.#root = rootOf(this);
        const linker = new Linker(library, this.#root);
        this.#markdown = new Marked({ renderer: { code: codeRenderer(linker, examples), codespan } });
    }

    get path() {
        return `${folders.packages}${file(this.#module.path)}`;
    }

    #external(alias, name, whole) {
        const imported = this.#module.imports.find((entry) => entry.alias === alias);
        if (!imported || !this.#exports.get(imported.path)?.has(name)) {
            return whole;
        }
        return `<a class="m" href="${file(imported.path)}">${alias}</a>.<a class="t" href="${file(imported.path)}#${name}">${name}</a>`;
    }

    #type(name, whole) {
        if (this.#typeNames.has(name)) {
            return `<a class="t" href="#${name}">${name}</a>`;
        }
        const builtin = this.#library.builtin(name);
        return builtin ? `<a class="t" href="${this.#root}${builtin}">${name}</a>` : whole;
    }

    #code(text) {
        const groups = blocks(text).map((group) => (group.kind === "note" ? note(group.indent, group.lines) : low(group.lines.join("\n"))));
        const painted = groups
            .join("")
            .replace(/\b(\w+)\.<span class="t">(\w+)<\/span>/g, (whole, alias, name) => this.#external(alias, name, whole))
            .replace(/(?<!\.)<span class="t">(\w+)<\/span>/g, (whole, name) => this.#type(name, whole));
        return `<pre class="signature"><code>${painted}</code></pre>\n`;
    }

    #item(item, depth) {
        const id = anchor(item);
        const link = `${compilerSource}/${this.#module.path}/${item.file}#L${item.line}`;
        const where = `${item.file}:${item.line}`;
        const view = `<a class="-m-3 shrink-0 p-3 no-underline opacity-60 hover:opacity-100" href="${link}" title="${escape(where)}" aria-label="${escape(where)} on GitHub">${github}</a>`;
        const heading = `<h${depth} id="${id}" class="flex items-center justify-between gap-4"><a href="#${id}">${escape(item.name)}</a>${view}</h${depth}>\n`;
        const doc = item.doc ? this.#markdown.parse(item.doc) : "";
        return `<section class="item">\n${heading}${this.#code(item.text)}${doc}</section>\n`;
    }

    #methods(type) {
        return this.#module.items.filter((item) => item.receiver === type.name).map((item) => this.#item(item, 4)).join("");
    }

    body() {
        const parts = [`<p>${tags(this.#module.path)}</p>\n`, `<pre><code>${low(`import ${this.#module.path}`)}</code></pre>\n`];
        if (this.#description) {
            parts.push(`<p>${this.#description}</p>\n`);
        }
        const paths = [...new Set(this.#module.imports.map((entry) => entry.path))];
        if (paths.length > 0) {
            const links = paths.map((path) => `<a href="${file(path)}"><code>${path}</code></a>`);
            parts.push(`<p>Imports ${links.join(", ")}.</p>\n`);
        }
        if (this.#types.length > 0) {
            parts.push('<h2 id="types"><a href="#types">Types</a></h2>\n');
            parts.push(...this.#types.map((type) => `${this.#item(type, 3)}${this.#methods(type)}`));
        }
        if (this.#functions.length > 0) {
            parts.push('<h2 id="functions"><a href="#functions">Functions</a></h2>\n');
            parts.push(...this.#functions.map((item) => this.#item(item, 3)));
        }
        return parts.join("");
    }

    sidebar() {
        const groups = [];
        if (this.#types.length > 0) {
            groups.push(`<p class="mb-2 text-sm font-semibold uppercase tracking-wide">Types</p>\n${sidebarList(this.#types)}`);
        }
        if (this.#functions.length > 0) {
            groups.push(`<p class="mb-2 text-sm font-semibold uppercase tracking-wide">Functions</p>\n${sidebarList(this.#functions)}`);
        }
        const back = '<p class="font-bold"><a class="hover:underline hover:underline-offset-4" href="index.html">← std</a></p>';
        return `<div class="space-y-8">\n${back}\n${groups.join("\n")}\n</div>`;
    }

    html() {
        const path = this.#module.path;
        const text = this.#description ? textOf(this.#description) : `The ${path} module of the Low standard library.`;
        return layout({ path: this.path, heading: path, title: `${path}: Low packages`, description: summary(text, 200), current: "Packages", sidebar: this.sidebar(), body: this.body() });
    }
}

function index(modules, about) {
    const names = all(modules.map((module) => module.path));
    const bar = names.map((name) => `<a class="tag" href="?category=${name}" data-category="${name}">#${name}</a>`);
    const rows = modules.map((module) => {
        const text = about.has(module.path) ? summary(textOf(about.get(module.path)), 160) : "";
        const kinds = categories(module.path).join(" ");
        return `<li class="module" data-categories="${kinds}" data-search="${escape(`${module.path} ${text}`.toLowerCase())}"><a class="hit" href="${file(module.path)}"><code>${module.path}</code></a> <span class="text-sm">${module.items.length} items</span>${text ? `<br>${escape(text)}` : ""}<span class="mt-4 flex flex-wrap gap-x-2 gap-y-5">${tags(module.path)}</span></li>`;
    });
    const body = `<p>The packages that ship with Low, extracted from their source by <code>low doc std</code>. Each page lists a module's public types, the functions that take them as their first argument, and its other public functions.</p>
<label class="mt-6 block"><span class="sr-only">Filter modules</span><input id="filter" type="search" placeholder="Filter ${modules.length} modules" class="w-full border px-3 py-2"></label>
<nav class="tags" aria-label="Categories">
<a class="tag" href="index.html" data-category="">all</a>
${bar.join("\n")}
</nav>
<ul id="modules" class="mt-10 list-none space-y-6 pl-0">
${rows.join("\n")}
</ul>
`;
    const sidebar = '<ul class="sidelist">\n<li><a class="font-bold underline underline-offset-4" aria-current="page" href="index.html">std</a></li>\n</ul>';
    return layout({ path: `${folders.packages}index.html`, heading: "std", title: "Low packages", description: `The Low standard library: ${modules.length} modules.`, current: "Packages", sidebar, body, scripts: ["filter.js"] });
}

export async function standard(binary) {
    const { stdout } = await run(binary, ["doc", "std", "-target", "linux/amd64"], { maxBuffer: 64 * 1024 * 1024 });
    return JSON.parse(stdout);
}

function imports(text, path) {
    return text.split("\n").some((line) => new RegExp(`^import (\\w+ = )?${path}$`).test(line));
}

async function inspect(module, examples, session) {
    for (const item of module.items) {
        const blocks = new Marked().lexer(item.doc ?? "").filter((token) => token.type === "code" && isLow(token.lang) && !examples.has(token.text));
        for (const block of blocks) {
            const preludes = imports(block.text, module.path) ? [] : [{ path: "", text: `import ${module.path}` }];
            const program = new Program(block.text, preludes);
            const checked = await session.inspect(program, words(block.text), `${module.path} ${anchor(item)}`);
            examples.set(block.text, checked);
        }
    }
}

export async function packages(modules, design, session, library) {
    const uncategorised = modules.filter((module) => categories(module.path).length === 0).map((module) => module.path);
    if (uncategorised.length > 0) {
        throw new Error(`packages without a category: ${uncategorised.join(", ")}`);
    }
    const orphans = orphaned(modules.map((module) => module.path));
    if (orphans.length > 0) {
        throw new Error(`categories for packages that do not exist: ${orphans.join(", ")}`);
    }
    const examples = new Map();
    for (const module of modules) {
        await inspect(module, examples, session);
    }
    const about = descriptions(design);
    const exports = new Map(modules.map((module) => [module.path, new Set(members(module).types.map((item) => item.name))]));
    await rm(output, { recursive: true, force: true });
    await mkdir(output);
    await writeFile(new URL("index.html", output), index(modules, about));
    for (const module of modules) {
        const page = new Page(module, about.get(module.path), exports, examples, library);
        const html = page.html();
        await writeFile(new URL(file(module.path), output), html);
    }
    return modules.length;
}
