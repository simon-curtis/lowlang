const keywords = new Set([
    "fn", "let", "var", "if", "else", "for", "break", "continue", "return", "enum", "match", "fail", "catch",
    "struct", "mut", "in", "import", "pub", "extern", "defer", "spawn", "select", "using", "drop",
]);
const types = new Set(["void", "never", "i8", "i16", "i32", "i64", "u8", "u16", "u32", "u64", "f32", "f64", "bool", "str", "ptr", "cstr", "chan"]);
const constants = new Set(["true", "false", "none", "null"]);
const pattern = /([\u0001\u0002])|(\/\/[^\n]*)|("(?:[^"\\\n]|\\.)*"|'(?:[^'\\\n]|\\.)')|(\d[\w.]*)|([A-Za-z_]\w*)|(\s+)|(->|=>|::|\.\.|[-+*\/%=<>!&|^?:]+)|([\s\S])/g;

export function escape(text) {
    return text.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");
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

function classify(tokens, index, imported) {
    const previous = before(tokens, index);
    const next = after(tokens, index);
    const text = tokens[index].text;
    if (keywords.has(text)) {
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

function wrap(text, style, hover, group) {
    const name = style ? ` class="${style}"` : "";
    const tip = hover ? `${low(hover.code)}${hover.doc ? `<span class="doc">${escape(hover.doc)}</span>` : ""}` : "";
    const detail = tip ? ` data-hover="${escape(tip)}"` : "";
    const reference = group === undefined ? "" : ` data-ref="${group}"`;
    return name || detail || reference ? `<span${name}${detail}${reference}>${text}</span>` : text;
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
    return wrap(text, style, useful, facts.references.get(token.offset));
}

export function words(source) {
    return lex(source).filter((token) => token.kind === "word" && !keywords.has(token.text)).map((token) => token.offset);
}

export function low(source, facts = { hovers: new Map(), references: new Map() }) {
    const tokens = lex(source);
    const imported = modules(tokens);
    return tokens.map((token, index) => paint(token, index, tokens, imported, facts)).join("");
}
