import { folders } from "./config.mjs";
import { low, words } from "./highlight.mjs";
import { escape, isLow } from "./markup.mjs";

const located = /(?<=\/)(std(?:\/\w+)*)\/(\w+\.low)$/;

export function file(path) {
    return `${path.replaceAll("/", "-")}.html`;
}

export function anchor(item) {
    return item.receiver ? `${item.receiver}.${item.name}` : item.name;
}

export class Library {
    #modules;
    #builtins;

    constructor(modules, builtins) {
        this.#modules = new Map(modules.map((module) => [module.path, module]));
        this.#builtins = builtins;
    }

    page({ uri, line }) {
        const [, path, name] = located.exec(uri) ?? [];
        const module = this.#modules.get(path);
        if (!module) {
            return undefined;
        }
        const enclosing = module.items.filter((item) => item.file === name && item.line <= line + 1).reduce((best, item) => (best && best.line >= item.line ? best : item), undefined);
        return enclosing ? `${folders.packages}${file(path)}#${anchor(enclosing)}` : `${folders.packages}${file(path)}`;
    }

    builtin(name) {
        return this.#builtins.get(name);
    }
}

export function codeRenderer(linker, checks) {
    return ({ text, lang }) => {
        const checked = checks.get(text);
        const body = isLow(lang) ? low(text, checked && linker.facts(checked, text)) : escape(text);
        return `<pre><code>${body}</code></pre>\n`;
    };
}

export class Linker {
    #library;
    #root;
    #snippets = 0;

    constructor(library, root) {
        this.#library = library;
        this.#root = root;
    }

    facts(checked, source) {
        this.#snippets += 1;
        const links = new Map();
        const anchors = new Map();
        for (const offset of words(source)) {
            const target = checked.definitions.get(offset);
            const page = target === undefined ? this.#library.builtin(/^\w+/.exec(source.slice(offset))[0]) : target.uri && this.#library.page(target);
            if (page) {
                links.set(offset, `${this.#root}${page}`);
            } else if (target?.offset !== undefined && target.offset !== offset) {
                const id = `snippet${this.#snippets}-${target.offset}`;
                links.set(offset, `#${id}`);
                anchors.set(target.offset, id);
            }
        }
        return { ...checked, links, anchors };
    }
}
