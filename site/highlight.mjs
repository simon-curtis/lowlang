import { escape } from "./markup.mjs";

const keywords = new Set([
    "fn", "let", "var", "if", "else", "for", "break", "continue", "return", "enum", "match", "fail", "catch", "or",
    "struct", "const", "mut", "in", "import", "pub", "extern", "defer", "spawn", "select", "using", "drop",
]);
const contextual = new Set(["after", "when"]);
const types = new Set([
    "i64", "i32", "u64", "u32", "u16", "u8", "f64", "f32", "bool", "str", "ptr", "chan",
    "u8x16", "u8x32", "u16x8", "u16x16", "i32x4", "i32x8", "u32x4", "u32x8", "i64x2", "i64x4", "u64x2", "u64x4", "f32x4", "f32x8", "f64x2", "f64x4",
]);
const constants = new Set(["true", "false", "none", "null"]);
const pattern = /([\u0001\u0002])|(\/\/[^\n]*)|("(?:[^"\\\n]|\\.)*"|'(?:[^'\\\n]|\\.)')|(\d[\w.]*)|([A-Za-z_]\w*)|(\s+)|(->|=>|::|\.\.|[-+*\/%=<>!&|^?:]+)|([\s\S])/g;

function unbroken(part) {
    return part.length <= 28 && /[^\p{L}\p{N}_\s]/u.test(part);
}

export function codespan({ text }) {
    const parts = text.split(/(\s+)/).map((part) => (unbroken(part) ? `<span class="whitespace-nowrap">${escape(part)}</span>` : escape(part)));
    return `<code>${parts.join("")}</code>`;
}

function lex(source) {
    const tokens = [];
    let offset = 0;
    for (const match of source.matchAll(pattern)) {
        const [text, marker, comment, string, number, word, space, operator] = match;
        const kind = marker ? "marker" : comment ? "comment" : string ? "string" : number ? "number" : word ? "word" : space ? "space" : operator ? "operator" : "punctuation";
        tokens.push({ kind, text, offset });
        if (!marker) {
            offset += text.length;
        }
    }
    return tokens;
}

function after(tokens, index) {
    let at = index + 1;
    while (tokens[at]?.kind === "marker" || (tokens[at]?.kind === "space" && !tokens[at].text.includes("\n"))) {
        at += 1;
    }
    return tokens[at];
}

function before(tokens, index) {
    let at = index - 1;
    while (tokens[at]?.kind === "space" || tokens[at]?.kind === "marker") {
        at -= 1;
    }
    return tokens[at];
}

function sameLineBefore(tokens, index) {
    let at = index - 1;
    while (tokens[at]?.kind === "marker" || (tokens[at]?.kind === "space" && !tokens[at].text.includes("\n"))) {
        at -= 1;
    }
    return tokens[at]?.kind === "space" ? undefined : tokens[at];
}

function startsLine(tokens, index) {
    let at = index - 1;
    while (tokens[at]?.kind === "marker") {
        at -= 1;
    }
    return tokens[at] === undefined || (tokens[at].kind === "space" && tokens[at].text.includes("\n"));
}

function closes(tokens, index) {
    let depth = 0;
    for (let at = index; at < tokens.length; at += 1) {
        const text = tokens[at].text;
        if (text === "[") {
            depth += 1;
        } else if (text === "]") {
            depth -= 1;
            if (depth === 0) {
                return after(tokens, at)?.text === "(";
            }
        }
    }
    return false;
}

function modules(tokens) {
    const names = new Set();
    tokens.forEach((token, index) => {
        if (token.text === "import") {
            let at = index + 1;
            while (tokens[at] && !tokens[at].text.includes("\n")) {
                if (tokens[at].kind === "word") {
                    names.add(tokens[at].text);
                }
                at += 1;
            }
        }
    });
    return names;
}

function endsValue(token) {
    if (token === undefined || keywords.has(token.text)) {
        return false;
    }
    return token.kind === "word" || token.kind === "number" || token.kind === "string" || ")]}?".includes(token.text);
}

function startsValue(token) {
    if (token === undefined) {
        return false;
    }
    return token.kind === "word" || token.kind === "number" || token.kind === "string" || "([!-*~".includes(token.text);
}

function contextualKeyword(tokens, index) {
    const next = after(tokens, index);
    if (tokens[index].text === "after") {
        return startsLine(tokens, index) && next?.text === "(";
    }
    return endsValue(sameLineBefore(tokens, index)) && startsValue(next);
}

function classify(tokens, index, imported) {
    const previous = before(tokens, index);
    const next = after(tokens, index);
    const text = tokens[index].text;
    if (keywords.has(text) || (contextual.has(text) && contextualKeyword(tokens, index))) {
        return "k";
    }
    if (constants.has(text)) {
        return "n";
    }
    if (previous?.text === "import" || (previous?.text === "/" && imported.has(text))) {
        return "m";
    }
    if (imported.has(text) && next?.text === ".") {
        return "m";
    }
    if (/^[A-Z]/.test(text)) {
        return "t";
    }
    if (previous?.text === "fn" || next?.text === "(" || (next?.text === "[" && !types.has(text) && closes(tokens, tokens.indexOf(next)))) {
        return "f";
    }
    if (types.has(text)) {
        return "t";
    }
    return "";
}

function wrap(text, style, { hover, group, link, id } = {}) {
    const name = style ? ` class="${style}"` : "";
    const target = id ? ` id="${id}"` : "";
    const tip = hover ? `${low(hover.code)}${hover.doc ? `<span class="doc">${escape(hover.doc)}</span>` : ""}` : "";
    const detail = tip ? ` data-hover="${escape(tip)}"` : "";
    const reference = group === undefined ? "" : ` data-ref="${group}"`;
    if (link) {
        return `<a href="${link}"${target}${name}${detail}${reference}>${text}</a>`;
    }
    return name || target || detail || reference ? `<span${target}${name}${detail}${reference}>${text}</span>` : text;
}

function paint(token, index, tokens, imported, facts) {
    const text = escape(token.text);
    switch (token.kind) {
        case "marker":
            return token.text;
        case "comment":
            return wrap(text, "c");
        case "string":
            return wrap(text, "s");
        case "number":
            return wrap(text, "n");
        case "operator":
            return wrap(text, "o");
        case "word":
            break;
        default:
            return text;
    }
    const style = classify(tokens, index, imported);
    if (style === "k") {
        return wrap(text, "k");
    }
    const hover = facts.hovers.get(token.offset);
    const useful = hover && hover.code !== token.text ? hover : undefined;
    const offset = token.offset;
    return wrap(text, style, { hover: useful, group: facts.references.get(offset), link: facts.links.get(offset), id: facts.anchors.get(offset) });
}

export function words(source) {
    return lex(source).filter((token) => token.kind === "word" && !keywords.has(token.text)).map((token) => token.offset);
}

export function isTypeName(name) {
    return /^[A-Z]/.test(name) || types.has(name);
}

export function linked(links, modules) {
    return { hovers: new Map(), references: new Map(), links, anchors: new Map(), modules };
}

const unchecked = linked(new Map(), new Set());

export function low(source, facts = unchecked) {
    const tokens = lex(source);
    const imported = new Set([...modules(tokens), ...(facts.modules ?? [])]);
    return tokens.map((token, index) => paint(token, index, tokens, imported, facts)).join("");
}
