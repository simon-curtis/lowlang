import { execFile } from "node:child_process";
import { mkdir, rm, writeFile } from "node:fs/promises";
import { promisify } from "node:util";
import { Marked } from "marked";
import { all, categories, orphaned } from "./categories.mjs";
import { compilerSource, folders } from "./config.mjs";
import { codespan, isTypeName, linked, low, words } from "./highlight.mjs";
import { layout, rootOf } from "./layout.mjs";
import { anchor, codeRenderer, file, Linker } from "./links.mjs";
import { escape, isLow, textOf } from "./markup.mjs";
import { Program } from "./program.mjs";
import { plain } from "./search.mjs";

const run = promisify(execFile);
const output = new URL(folders.packages, import.meta.url);
const github = '<svg class="size-5" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true"><path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z"/></svg>';
const markdown = new Marked({ renderer: { codespan } });

const comment = /^( *)\/\/\/ ?(.*)$/;
const current = "Packages";

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
    const types = module.items.filter((item) => item.kind === "struct" || item.kind === "enum");
    const constants = module.items.filter((item) => item.kind === "const");
    const functions = module.items.filter((item) => item.kind === "fn" && !item.receiver);
    return { types, constants, functions };
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

function exampleAnchor(example) {
    return `example-${example.name}`;
}

function exampleSource(example) {
    return example.source.trimEnd();
}

function sidebarList(items) {
    const links = items.map((item) => `<li><a class="hover:underline hover:underline-offset-4" href="#${item.id ?? item.name}">${escape(item.name)}</a></li>`);
    return `<ul class="sidelist">\n${links.join("\n")}\n</ul>`;
}

class Page {
    #module;
    #description;
    #types;
    #constants;
    #functions;
    #typeNames;
    #exports;
    #library;
    #root;
    #markdown;
    #linker;
    #snippets;

    constructor(module, description, exports, snippets, library) {
        this.#module = module;
        this.#description = description;
        const { types, constants, functions } = members(module);
        this.#types = types;
        this.#constants = constants;
        this.#functions = functions;
        this.#typeNames = new Set(types.map((item) => item.name));
        this.#exports = exports;
        this.#library = library;
        this.#root = rootOf(this);
        this.#linker = new Linker(library, this.#root);
        this.#snippets = snippets;
        this.#markdown = new Marked({ renderer: { code: codeRenderer(this.#linker, snippets), codespan } });
    }

    get path() {
        return `${folders.packages}${file(this.#module.path)}`;
    }

    #localTarget(name) {
        if (this.#typeNames.has(name)) {
            return `#${name}`;
        }
        const builtin = this.#library.builtin(name);
        return builtin ? `${this.#root}${builtin}` : undefined;
    }

    #importedTarget(alias, name) {
        const imported = this.#module.imports.find((entry) => entry.alias === alias);
        return imported && this.#exports.get(imported.path)?.has(name) ? file(imported.path) : undefined;
    }

    #signature(source) {
        const links = new Map();
        const modules = new Set();
        for (const offset of words(source)) {
            const name = /^\w+/.exec(source.slice(offset))[0];
            if (!isTypeName(name)) {
                continue;
            }
            const alias = /(\w+)\.$/.exec(source.slice(0, offset))?.[1];
            if (alias === undefined) {
                const target = this.#localTarget(name);
                if (target) {
                    links.set(offset, target);
                }
                continue;
            }
            const page = this.#importedTarget(alias, name);
            if (page) {
                links.set(offset - alias.length - 1, page);
                links.set(offset, `${page}#${name}`);
                modules.add(alias);
            }
        }
        return linked(links, modules);
    }

    #code(text) {
        const groups = blocks(text).map((group) => {
            if (group.kind === "note") {
                return note(group.indent, group.lines);
            }
            const source = group.lines.join("\n");
            return low(source, this.#signature(source));
        });
        return `<pre class="signature"><code>${groups.join("")}</code></pre>\n`;
    }

    #heading(id, name, depth, target, where) {
        const link = `${compilerSource}/${this.#module.path}/${target}`;
        const view = `<a class="-m-3 shrink-0 p-3 no-underline opacity-60 hover:opacity-100" href="${link}" title="${escape(where)}" aria-label="${escape(where)} on GitHub">${github}</a>`;
        return `<h${depth} id="${id}" class="flex items-center justify-between gap-4"><a href="#${id}">${escape(name)}</a>${view}</h${depth}>\n`;
    }

    #item(item, depth) {
        const heading = this.#heading(anchor(item), item.name, depth, `${item.file}#L${item.line}`, `${item.file}:${item.line}`);
        const doc = item.doc ? this.#markdown.parse(item.doc) : "";
        return `<section class="item">\n${heading}${this.#code(item.text)}${doc}</section>\n`;
    }

    #example(example) {
        const where = `examples/${example.name}.low`;
        const heading = this.#heading(exampleAnchor(example), example.name, 3, where, where);
        const source = exampleSource(example);
        const facts = this.#linker.facts(this.#snippets.get(source), source);
        const code = `<pre><code>${low(source, facts)}</code></pre>\n`;
        const output = example.output ? `<p class="text-sm font-semibold uppercase tracking-wide">Output</p>\n<pre><code>${escape(example.output.trimEnd())}</code></pre>\n` : "";
        return `<section class="item">\n${heading}${code}${output}</section>\n`;
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
        if (this.#module.examples.length > 0) {
            parts.push('<h2 id="examples"><a href="#examples">Examples</a></h2>\n');
            parts.push(...this.#module.examples.map((example) => this.#example(example)));
        }
        if (this.#types.length > 0) {
            parts.push('<h2 id="types"><a href="#types">Types</a></h2>\n');
            parts.push(...this.#types.map((type) => `${this.#item(type, 3)}${this.#methods(type)}`));
        }
        if (this.#constants.length > 0) {
            parts.push('<h2 id="constants"><a href="#constants">Constants</a></h2>\n');
            parts.push(...this.#constants.map((item) => this.#item(item, 3)));
        }
        if (this.#functions.length > 0) {
            parts.push('<h2 id="functions"><a href="#functions">Functions</a></h2>\n');
            parts.push(...this.#functions.map((item) => this.#item(item, 3)));
        }
        return parts.join("");
    }

    sidebar() {
        const groups = [];
        if (this.#module.examples.length > 0) {
            const links = this.#module.examples.map((example) => ({ name: example.name, id: exampleAnchor(example) }));
            groups.push(`<p class="mb-2 text-sm font-semibold uppercase tracking-wide">Examples</p>\n${sidebarList(links)}`);
        }
        if (this.#types.length > 0) {
            groups.push(`<p class="mb-2 text-sm font-semibold uppercase tracking-wide">Types</p>\n${sidebarList(this.#types)}`);
        }
        if (this.#constants.length > 0) {
            groups.push(`<p class="mb-2 text-sm font-semibold uppercase tracking-wide">Constants</p>\n${sidebarList(this.#constants)}`);
        }
        if (this.#functions.length > 0) {
            groups.push(`<p class="mb-2 text-sm font-semibold uppercase tracking-wide">Functions</p>\n${sidebarList(this.#functions)}`);
        }
        const back = '<p class="font-bold"><a class="hover:underline hover:underline-offset-4" href="index.html">← std</a></p>';
        return `<div class="space-y-8">\n${back}\n${groups.join("\n")}\n</div>`;
    }

    #entry(item) {
        const lines = item.text.split("\n");
        const code = lines.filter((line) => !comment.test(line)).join(" ");
        const notes = lines.map((line) => comment.exec(line)?.[2]).filter((line) => line !== undefined);
        const doc = markdown.parse(item.doc ?? "");
        const fields = markdown.parseInline(notes.join(" "));
        const alias = this.#module.path.split("/").at(-1);
        return {
            kind: item.receiver ? "method" : item.kind,
            name: item.receiver ? anchor(item) : `${alias}.${item.name}`,
            href: `${this.path}#${anchor(item)}`,
            signature: code.replace(/\s+/g, " ").trim().replace(/^pub /, ""),
            text: plain(`${doc} ${fields}`),
        };
    }

    register(search) {
        const description = this.#description ? plain(this.#description) : "";
        const page = search.page({ title: this.#module.path, area: current, href: this.path, description, text: "" });
        for (const example of this.#module.examples) {
            const text = exampleSource(example).replace(/\s+/g, " ");
            search.section(page, { kind: "example", name: example.name, href: `${this.path}#${exampleAnchor(example)}`, text });
        }
        const methods = this.#types.flatMap((type) => this.#module.items.filter((item) => item.receiver === type.name));
        for (const item of [...this.#types, ...methods, ...this.#constants, ...this.#functions]) {
            const entry = this.#entry(item);
            search.section(page, entry);
        }
    }

    html() {
        const path = this.#module.path;
        const text = this.#description ? textOf(this.#description) : `The ${path} module of the Low standard library.`;
        return layout({ path: this.path, heading: path, title: `${path}: Low packages`, description: summary(text, 200), current, sidebar: this.sidebar(), body: this.body() });
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
    const body = `<p>The packages that ship with Low, extracted from their source by <code>low doc std</code>. Each page lists a module's public types, the functions that take them as their first argument, its public constants and its other public functions.</p>
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
    return layout({ path: `${folders.packages}index.html`, heading: "std", title: "Low packages", description: `The Low standard library: ${modules.length} modules.`, current, sidebar, body, scripts: ["filter.js"] });
}

export async function standard(binary) {
    const { stdout } = await run(binary, ["doc", "std", "-target", "linux/amd64"], { maxBuffer: 64 * 1024 * 1024 });
    return JSON.parse(stdout);
}

function imports(text, path) {
    return text.split("\n").some((line) => new RegExp(`^import (\\w+ = )?${path}$`).test(line));
}

async function inspect(module, snippets, session) {
    for (const item of module.items) {
        const blocks = new Marked().lexer(item.doc ?? "").filter((token) => token.type === "code" && isLow(token.lang) && !snippets.has(token.text));
        for (const block of blocks) {
            const preludes = imports(block.text, module.path) ? [] : [{ path: "", text: `import ${module.path}` }];
            const program = new Program(block.text, preludes);
            const checked = await session.inspect(program, words(block.text), `${module.path} ${anchor(item)}`);
            snippets.set(block.text, checked);
        }
    }
    for (const example of module.examples) {
        const source = exampleSource(example);
        const checked = await session.inspect(new Program(source), words(source), `${module.path} examples/${example.name}.low`);
        snippets.set(source, checked);
    }
}

export async function packages(modules, design, session, library, search) {
    const uncategorised = modules.filter((module) => categories(module.path).length === 0).map((module) => module.path);
    if (uncategorised.length > 0) {
        throw new Error(`packages without a category: ${uncategorised.join(", ")}`);
    }
    const orphans = orphaned(modules.map((module) => module.path));
    if (orphans.length > 0) {
        throw new Error(`categories for packages that do not exist: ${orphans.join(", ")}`);
    }
    const snippets = new Map();
    for (const module of modules) {
        await inspect(module, snippets, session);
    }
    const about = descriptions(design);
    const exports = new Map(modules.map((module) => [module.path, new Set(members(module).types.map((item) => item.name))]));
    await rm(output, { recursive: true, force: true });
    await mkdir(output);
    await writeFile(new URL("index.html", output), index(modules, about));
    for (const module of modules) {
        const page = new Page(module, about.get(module.path), exports, snippets, library);
        const html = page.html();
        page.register(search);
        await writeFile(new URL(file(module.path), output), html);
    }
    return modules.length;
}
