import { writeFile } from "node:fs/promises";
import { textOf } from "./markup.mjs";

const output = new URL("search.json", import.meta.url);
const heading = /<h([2-4])\b([^>]*)>([\s\S]*?)<\/h\1>/g;
const identifier = /\bid="([^"]*)"/;
const block = /<\/?(?:p|li|ul|ol|tr|td|th|table|pre|div|section|h[1-6]|br)\b[^>]*>/g;

export function plain(html) {
    return textOf(html.replace(block, " ")).replace(/\s+/g, " ").trim();
}

export class SearchIndex {
    #pages = [];
    #sections = [];

    page({ title, area, href, description, text }) {
        this.#pages.push([title, area, href, description, text]);
        return this.#pages.length - 1;
    }

    section(page, { kind, name, href, signature = "", text }) {
        this.#sections.push([page, kind, name, href, signature, text]);
    }

    document({ title, area, path, description, html }) {
        const found = [...html.matchAll(heading)];
        const lead = html.slice(0, found[0]?.index ?? html.length);
        const page = this.page({ title, area, href: path, description, text: plain(lead) });
        found.forEach((match, position) => {
            const end = found[position + 1]?.index ?? html.length;
            const anchor = identifier.exec(match[2])?.[1];
            const text = plain(html.slice(match.index + match[0].length, end));
            this.section(page, { kind: "section", name: plain(match[3]), href: anchor ? `${path}#${anchor}` : path, text });
        });
    }

    async write() {
        const index = JSON.stringify({ pages: this.#pages, sections: this.#sections });
        await writeFile(output, index);
        return `${this.#sections.length} sections from ${this.#pages.length} pages`;
    }
}
