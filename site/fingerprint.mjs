import { createHash } from "node:crypto";
import { readdir, readFile, writeFile } from "node:fs/promises";
import { outputs } from "./config.mjs";

const site = new URL("./", import.meta.url);
const reference = /\b(href|src)="(\/|(?:\.\.\/)*)([\w.-]+\.(?:css|js))(?:\?v=\w+)?"/g;

class Fingerprints {
    #hashes = new Map();

    async of(name) {
        if (!this.#hashes.has(name)) {
            const content = await readFile(new URL(name, site));
            this.#hashes.set(name, createHash("sha256").update(content).digest("hex").slice(0, 10));
        }
        return this.#hashes.get(name);
    }

    async stamp(path) {
        const html = await readFile(path, "utf8");
        const names = [...new Set([...html.matchAll(reference)].map((match) => match[3]))];
        const hashes = new Map();
        for (const name of names) {
            const hash = await this.of(name);
            hashes.set(name, hash);
        }
        const stamped = html.replace(reference, (whole, attribute, prefix, name) => `${attribute}="${prefix}${name}?v=${hashes.get(name)}"`);
        if (stamped !== html) {
            await writeFile(path, stamped);
        }
    }
}

const fingerprints = new Fingerprints();
let pages = 0;
for (const folder of outputs) {
    const directory = new URL(folder, site);
    const files = await readdir(directory);
    for (const file of files.filter((name) => name.endsWith(".html"))) {
        await fingerprints.stamp(new URL(file, directory));
        pages += 1;
    }
}
console.log(`fingerprint: ${pages} pages`);
