import { words } from "./highlight.mjs";

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
        return enclosing ? `packages/${file(path)}#${anchor(enclosing)}` : `packages/${file(path)}`;
    }

    builtin(name) {
        return this.#builtins.get(name);
    }
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
