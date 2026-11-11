import { execFile } from "node:child_process";
import { mkdir, readFile, rm, writeFile } from "node:fs/promises";
import { promisify } from "node:util";
import { Marked } from "marked";
import { all, categories } from "./categories.mjs";
import { escape, low } from "./highlight.mjs";
import { layout } from "./layout.mjs";

const run = promisify(execFile);
const repository = new URL("../", import.meta.url);
const output = new URL("packages/", import.meta.url);
const source = "https://github.com/simon-curtis/lowlang/blob/main/internal/compiler";
const row = /^\| `(std[\w/]*)` \| (.*) \|$/gm;
const github = '<svg class="size-5" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true"><path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z"/></svg>';
const markdown = new Marked({
    renderer: {
        code({ text, lang }) {
            const body = lang === undefined || lang === "" || lang === "low" ? low(text) : escape(text);
            return `<pre><code>${body}</code></pre>\n`;
        },
    },
});

function file(path) {
    return `${path.replaceAll("/", "-")}.html`;
}

function plain(html) {
    return html.replace(/<[^>]+>/g, "").replace(/&amp;/g, "&").replace(/&lt;/g, "<").replace(/&gt;/g, ">").replace(/&quot;/g, '"').replace(/&#39;/g, "'");
}

function summary(text, limit) {
    return text.length <= limit ? text : `${text.slice(0, text.lastIndexOf(" ", limit))}…`;
}

function tags(path) {
    return categories(path).map((name) => `<a class="tag" href="index.html?category=${name}">#${name}</a>`).join(" ");
}

async function descriptions() {
    const design = await readFile(new URL("docs/design.md", repository), "utf8");
    return new Map([...design.matchAll(row)].map(([, path, text]) => [path, markdown.parseInline(text)]));
}

function sidebarList(items) {
    const links = items.map((item) => `<li><a class="hover:underline hover:underline-offset-4" href="#${item.name}">${escape(item.name)}</a></li>`);
    return `<ul class="flex flex-wrap gap-x-5 gap-y-2 lg:block lg:space-y-2">\n${links.join("\n")}\n</ul>`;
}

class Page {
    #module;
    #description;
    #types;
    #exports;

    constructor(module, description, exports) {
        this.#module = module;
        this.#description = description;
        this.#types = new Set(module.items.filter((item) => item.kind !== "fn").map((item) => item.name));
        this.#exports = exports;
    }

    get path() {
        return `packages/${file(this.#module.path)}`;
    }

    #external(alias, name, whole) {
        const imported = this.#module.imports.find((entry) => entry.alias === alias);
        if (!imported || !this.#exports.get(imported.path)?.has(name)) {
            return whole;
        }
        return `<a class="m" href="${file(imported.path)}">${alias}</a>.<a class="t" href="${file(imported.path)}#${name}">${name}</a>`;
    }

    #code(text) {
        const painted = low(text)
            .replace(/\b(\w+)\.<span class="t">(\w+)<\/span>/g, (whole, alias, name) => this.#external(alias, name, whole))
            .replace(/(?<!\.)<span class="t">(\w+)<\/span>/g, (whole, name) => (this.#types.has(name) ? `<a class="t" href="#${name}">${name}</a>` : whole));
        return `<pre><code>${painted}</code></pre>\n`;
    }

    #item(item, depth) {
        const id = item.receiver ? `${item.receiver}.${item.name}` : item.name;
        const link = `${source}/${this.#module.path}/${item.file}#L${item.line}`;
        const where = `${item.file}:${item.line}`;
        const view = `<a class="shrink-0 no-underline opacity-60 hover:opacity-100" href="${link}" title="${escape(where)}" aria-label="${escape(where)} on GitHub">${github}</a>`;
        const heading = `<h${depth} id="${id}" class="flex items-center justify-between gap-4"><a href="#${id}">${escape(item.name)}</a>${view}</h${depth}>\n`;
        const doc = item.doc ? markdown.parse(item.doc) : "";
        return `<section class="item">\n${heading}${this.#code(item.text)}${doc}</section>\n`;
    }

    #methods(type) {
        return this.#module.items.filter((item) => item.receiver === type.name).map((item) => this.#item(item, 4)).join("");
    }

    body() {
        const types = this.#module.items.filter((item) => item.kind !== "fn");
        const functions = this.#module.items.filter((item) => item.kind === "fn" && !item.receiver);
        const parts = [`<p>${tags(this.#module.path)}</p>\n`, `<pre><code>${low(`import ${this.#module.path}`)}</code></pre>\n`];
        if (this.#description) {
            parts.push(`<p>${this.#description}</p>\n`);
        }
        const paths = [...new Set(this.#module.imports.map((entry) => entry.path))];
        if (paths.length > 0) {
            const links = paths.map((path) => `<a href="${file(path)}"><code>${path}</code></a>`);
            parts.push(`<p>Imports ${links.join(", ")}.</p>\n`);
        }
        if (types.length > 0) {
            parts.push('<h2 id="types"><a href="#types">Types</a></h2>\n');
            parts.push(...types.map((type) => `${this.#item(type, 3)}${this.#methods(type)}`));
        }
        if (functions.length > 0) {
            parts.push('<h2 id="functions"><a href="#functions">Functions</a></h2>\n');
            parts.push(...functions.map((item) => this.#item(item, 3)));
        }
        return parts.join("");
    }

    sidebar() {
        const types = this.#module.items.filter((item) => item.kind !== "fn");
        const functions = this.#module.items.filter((item) => item.kind === "fn" && !item.receiver);
        const groups = [];
        if (types.length > 0) {
            groups.push(`<p class="mb-2 text-sm font-semibold uppercase tracking-wide">Types</p>\n${sidebarList(types)}`);
        }
        if (functions.length > 0) {
            groups.push(`<p class="mb-2 text-sm font-semibold uppercase tracking-wide">Functions</p>\n${sidebarList(functions)}`);
        }
        const back = '<p class="font-bold"><a class="hover:underline hover:underline-offset-4" href="index.html">← std</a></p>';
        return `<div class="space-y-8">\n${back}\n${groups.join("\n")}\n</div>`;
    }

    html() {
        const path = this.#module.path;
        const text = this.#description ? plain(this.#description) : `The ${path} module of the Low standard library.`;
        return layout({ path: this.path, heading: path, title: `${path}: Low packages`, description: summary(text, 200), current: "Packages", sidebar: this.sidebar(), body: this.body() });
    }
}

function index(modules, about) {
    const names = all(modules.map((module) => module.path));
    const bar = names.map((name) => `<a class="tag" href="?category=${name}" data-category="${name}">#${name}</a>`);
    const rows = modules.map((module) => {
        const text = about.has(module.path) ? summary(plain(about.get(module.path)), 160) : "";
        const kinds = categories(module.path).join(" ");
        return `<li class="module" data-categories="${kinds}" data-search="${escape(`${module.path} ${text}`.toLowerCase())}"><a href="${file(module.path)}"><code>${module.path}</code></a> <span class="text-sm">${module.items.length} items</span> ${tags(module.path)}<br>${escape(text)}</li>`;
    });
    const body = `<p>The packages that ship with Low, extracted from their source by <code>low doc std</code>. Each page lists a module's public types, the functions that take them as their first argument, and its other public functions.</p>
<nav class="tags" aria-label="Categories">
<a class="tag" href="index.html" data-category="">all</a>
${bar.join("\n")}
</nav>
<label class="mt-6 block"><span class="sr-only">Filter modules</span><input id="filter" type="search" placeholder="Filter ${modules.length} modules" class="w-full border px-3 py-2"></label>
<ul id="modules" class="mt-10 list-none space-y-6 pl-0">
${rows.join("\n")}
</ul>
`;
    const sidebar = '<ul class="flex flex-wrap gap-x-5 gap-y-2 lg:block lg:space-y-2">\n<li><a class="font-bold underline underline-offset-4" aria-current="page" href="index.html">std</a></li>\n</ul>';
    return layout({ path: "packages/index.html", heading: "std", title: "Low packages", description: `The Low standard library: ${modules.length} modules.`, current: "Packages", sidebar, body, scripts: ["filter.js"] });
}

export async function packages(binary) {
    const { stdout } = await run(binary, ["doc", "std"], { maxBuffer: 64 * 1024 * 1024 });
    const modules = JSON.parse(stdout);
    const uncategorised = modules.filter((module) => categories(module.path).length === 0).map((module) => module.path);
    if (uncategorised.length > 0) {
        throw new Error(`packages without a category: ${uncategorised.join(", ")}`);
    }
    const about = await descriptions();
    const exports = new Map(modules.map((module) => [module.path, new Set(module.items.filter((item) => item.kind !== "fn").map((item) => item.name))]));
    await rm(output, { recursive: true, force: true });
    await mkdir(output);
    await writeFile(new URL("index.html", output), index(modules, about));
    for (const module of modules) {
        const page = new Page(module, about.get(module.path), exports);
        const html = page.html();
        await writeFile(new URL(file(module.path), output), html);
    }
    return modules.length;
}
