import { readFileSync, readdirSync } from "node:fs";
import { join, relative, resolve } from "node:path";
import { Styler } from "../src/highlight.js";

const site = resolve(import.meta.dirname, "..");
const blockPattern = /<pre\b(?![^>]*\bclass="signature")[^>]*><code>([\s\S]*?)<\/code><\/pre>/g;
const styledPattern = /<(span|a)\b[^>]*?\bclass="([a-z])"[^>]*>([^<]*)<\/\1>/g;
const wordsPattern = /data-(keywords|contextual|types|constants)="([^"]*)"/g;
const entities = { "&amp;": "&", "&lt;": "<", "&gt;": ">", "&quot;": '"' };

function unescape(html) {
    return html.replace(/&(amp|lt|gt|quot);/g, (entity) => entities[entity]);
}

function read(path) {
    try {
        return readFileSync(path, "utf8");
    } catch (error) {
        console.error(`highlight check: cannot read ${relative(site, path)}: ${error.message}`);
        process.exit(1);
    }
}

function pages(folder) {
    const directory = join(site, folder);
    try {
        return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
            if (entry.isDirectory()) {
                return pages(join(folder, entry.name));
            }
            return entry.name.endsWith(".html") ? [join(directory, entry.name)] : [];
        });
    } catch (error) {
        console.error(`highlight check: cannot list ${folder}: ${error.message}`);
        process.exit(1);
    }
}

function wordLists() {
    const playground = read(join(site, "playground", "index.html"));
    const words = Object.fromEntries([...playground.matchAll(wordsPattern)].map(([, name, list]) => [name, list]));
    const missing = ["keywords", "contextual", "types", "constants"].filter((name) => words[name] === undefined);
    if (missing.length > 0) {
        console.error(`highlight check: playground/index.html lacks the word lists ${missing.join(", ")}`);
        process.exit(1);
    }
    return words;
}

function generated(html) {
    return [...html.matchAll(styledPattern)].map(([, , style, text]) => [style, unescape(text)]).filter(([style]) => style !== "m");
}

function difference(want, have) {
    const at = want.findIndex(([style, text], index) => have[index]?.[0] !== style || have[index]?.[1] !== text);
    if (at === -1 && want.length === have.length) {
        return null;
    }
    const index = at === -1 ? Math.min(want.length, have.length) : at;
    return { want: want[index], have: have[index] };
}

const styler = new Styler(wordLists());
const files = [join(site, "index.html"), ...pages("docs"), ...pages("packages")];
const mismatches = [];
let checked = 0;
for (const file of files) {
    for (const [, body] of read(file).matchAll(blockPattern)) {
        const want = generated(body);
        if (want.length === 0) {
            continue;
        }
        checked += 1;
        const source = unescape(body.replace(/<[^>]*>/g, ""));
        const runs = styler.runs(source);
        const have = runs.filter(({ style }) => style !== "" && style !== "m").map(({ style, text }) => [style, text]);
        const found = difference(want, have);
        if (found !== null) {
            mismatches.push({ file: relative(site, file), line: source.split("\n")[0], ...found });
        }
    }
}
if (mismatches.length > 0) {
    for (const { file, line, want, have } of mismatches.slice(0, 10)) {
        console.error(`${file}: ${line}\n  generator ${JSON.stringify(want)}, site/src/highlight.js ${JSON.stringify(have)}`);
    }
    console.error(`highlight check: ${mismatches.length} of ${checked} snippets colour differently in the playground`);
    process.exit(1);
}
console.log(`highlight check: ${checked} snippets colour the same in the playground`);
