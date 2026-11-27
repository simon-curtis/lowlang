const operators = /[-+*\/%=<>!&|^?:]+/y;
const pairedOperator = /->|=>|::|\.\./y;
const lineTerminator = /[\n\r\u2028\u2029]/;
const spaces = /[\t\n\v\f\r \u00A0\u1680\u2000-\u200A\u2028\u2029\u202F\u205F\u3000\uFEFF]+/y;
const wordRest = /[A-Za-z0-9_]*/y;
const digitsRest = /[A-Za-z0-9_.]*/y;
const wordStart = /[A-Za-z_]/;
const digit = /[0-9]/;
const upper = /[A-Z]/;
const plainStyles = { comment: "c", quote: "s", digits: "n", operator: "o" };

function stickyEnd(pattern, source, from) {
    pattern.lastIndex = from;
    return pattern.test(source) ? pattern.lastIndex : from;
}

function unitWidth(source, at) {
    return source.codePointAt(at) > 0xffff ? 2 : 1;
}

function basicWidth(source, at) {
    return unitWidth(source, at) === 2 ? 0 : 1;
}

const hexDigit = /[0-9A-Fa-f]/;
const escapeDigits = { x: 2, u: 4, U: 8 };

function escapeTail(source, at) {
    const letter = source[at];
    if (letter === "u" && source[at + 1] === "{") {
        let cursor = at + 2;
        while (cursor < source.length && hexDigit.test(source[cursor])) {
            cursor += 1;
        }
        return cursor > at + 2 && source[cursor] === "}" ? cursor + 1 - (at + 1) : 0;
    }
    return escapeDigits[letter] ?? 0;
}

function stringEnd(source, at) {
    let cursor = at + 1;
    while (cursor < source.length) {
        const character = source[cursor];
        if (character === '"') {
            return cursor + 1;
        }
        if (character === "\n") {
            return -1;
        }
        if (character !== "\\") {
            cursor += 1;
            continue;
        }
        const escaped = cursor + 1;
        if (escaped >= source.length || lineTerminator.test(source[escaped])) {
            return -1;
        }
        cursor = escaped + unitWidth(source, escaped);
    }
    return -1;
}

function characterWidth(source, at) {
    if (at >= source.length) {
        return 0;
    }
    const character = source[at];
    if (character === "\\") {
        if (at + 1 >= source.length || lineTerminator.test(source[at + 1])) {
            return 0;
        }
        const escaped = basicWidth(source, at + 1);
        return escaped === 0 ? 0 : 1 + escaped + escapeTail(source, at + 1);
    }
    if (character === "'" || character === "\n") {
        return 0;
    }
    return basicWidth(source, at);
}

function characterEnd(source, at) {
    const width = characterWidth(source, at + 1);
    if (width === 0) {
        return -1;
    }
    const close = at + 1 + width;
    return source[close] === "'" ? close + 1 : -1;
}

function operatorEnd(source, at) {
    const paired = stickyEnd(pairedOperator, source, at);
    return paired > at ? paired : stickyEnd(operators, source, at);
}

function scan(source, at) {
    const character = source[at];
    if (character === "/" && source[at + 1] === "/") {
        const newline = source.indexOf("\n", at);
        return { kind: "comment", end: newline === -1 ? source.length : newline };
    }
    if (character === '"' || character === "'") {
        const end = character === '"' ? stringEnd(source, at) : characterEnd(source, at);
        if (end > at) {
            return { kind: "quote", end };
        }
    }
    if (digit.test(character)) {
        const end = stickyEnd(digitsRest, source, at + 1);
        return { kind: "digits", end };
    }
    if (wordStart.test(character)) {
        const end = stickyEnd(wordRest, source, at + 1);
        return { kind: "word", end };
    }
    const blank = stickyEnd(spaces, source, at);
    if (blank > at) {
        return { kind: "space", end: blank };
    }
    const operator = operatorEnd(source, at);
    if (operator > at) {
        return { kind: "operator", end: operator };
    }
    return { kind: "punctuation", end: at + unitWidth(source, at) };
}

function lex(source) {
    const tokens = [];
    let at = 0;
    while (at < source.length) {
        const { kind, end } = scan(source, at);
        tokens.push({ kind, text: source.slice(at, end) });
        at = end;
    }
    return tokens;
}

function isInlineSpace(token) {
    return token.kind === "space" && !token.text.includes("\n");
}

function isValue(token) {
    return token.kind === "word" || token.kind === "digits" || token.kind === "quote";
}

function wordSet(list) {
    return new Set((list ?? "").split(" ").filter(Boolean));
}

function appendRun(runs, style, text) {
    const last = runs.at(-1);
    if (style === "" && last?.style === "") {
        last.text = `${last.text}${text}`;
        return;
    }
    runs.push({ style, text });
}

function runNode({ style, text }) {
    if (style === "") {
        return text;
    }
    const span = document.createElement("span");
    span.className = style;
    span.textContent = text;
    return span;
}

function appendLineRun(lines, run) {
    const { style, text } = run;
    if (!text.includes("\n")) {
        lines.at(-1).push(run);
        return;
    }
    const parts = text.split("\n");
    parts.forEach((part, index) => {
        const closing = index < parts.length - 1;
        const piece = closing ? `${part}\n` : part;
        if (piece !== "") {
            lines.at(-1).push({ style, text: piece });
        }
        if (closing) {
            lines.push([]);
        }
    });
}

function splitLines(runs) {
    const lines = [[]];
    for (const run of runs) {
        appendLineRun(lines, run);
    }
    return lines;
}

function sameLine(old, runs) {
    return old.runs.length === runs.length && runs.every(({ style, text }, index) => old.runs[index].style === style && old.runs[index].text === text);
}

function lineNode(runs) {
    const line = document.createElement("span");
    for (const run of runs) {
        const node = runNode(run);
        line.append(node);
    }
    return line;
}

class Tokens {
    #list;
    #closers = new Map();

    constructor(source) {
        this.#list = lex(source);
        this.#matchBrackets();
    }

    get length() {
        return this.#list.length;
    }

    at(index) {
        return this.#list[index];
    }

    text(index) {
        return this.#list[index]?.text ?? "";
    }

    after(index) {
        let at = index + 1;
        while (at < this.#list.length && isInlineSpace(this.#list[at])) {
            at += 1;
        }
        return at < this.#list.length ? at : -1;
    }

    before(index) {
        let at = index - 1;
        while (at >= 0 && this.#list[at].kind === "space") {
            at -= 1;
        }
        return at;
    }

    sameLineBefore(index) {
        let at = index - 1;
        while (at >= 0 && isInlineSpace(this.#list[at])) {
            at -= 1;
        }
        return at >= 0 && this.#list[at].kind === "space" ? -1 : at;
    }

    startsLine(index) {
        const previous = this.#list[index - 1];
        return previous === undefined || (previous.kind === "space" && previous.text.includes("\n"));
    }

    startsValue(index) {
        const current = this.#list[index];
        return current !== undefined && (isValue(current) || "([!-*~".includes(current.text));
    }

    closes(index) {
        const close = this.#closers.get(index);
        return close !== undefined && this.text(this.after(close)) === "(";
    }

    imported() {
        const names = new Set();
        this.#list.forEach((token, index) => {
            if (token.text === "import") {
                this.#addImportLine(names, index + 1);
            }
        });
        return names;
    }

    #addImportLine(names, from) {
        for (let at = from; at < this.#list.length && !this.#list[at].text.includes("\n"); at += 1) {
            if (this.#list[at].kind === "word") {
                names.add(this.#list[at].text);
            }
        }
    }

    #matchBrackets() {
        const open = [];
        this.#list.forEach((token, index) => {
            if (token.text === "[") {
                open.push(index);
            } else if (token.text === "]" && open.length > 0) {
                this.#closers.set(open.pop(), index);
            }
        });
    }
}

export class Styler {
    #keywords;
    #contextual;
    #types;
    #constants;

    constructor({ keywords, contextual, types, constants }) {
        this.#keywords = wordSet(keywords);
        this.#contextual = wordSet(contextual);
        this.#types = wordSet(types);
        this.#constants = wordSet(constants);
    }

    runs(source) {
        const tokens = new Tokens(source);
        const imported = tokens.imported();
        const runs = [];
        for (let index = 0; index < tokens.length; index += 1) {
            const style = this.#style(tokens, index, imported);
            appendRun(runs, style, tokens.text(index));
        }
        return runs;
    }

    #style(tokens, index, imported) {
        const { kind } = tokens.at(index);
        if (kind === "word") {
            return this.#classify(tokens, index, imported);
        }
        return plainStyles[kind] ?? "";
    }

    #classify(tokens, index, imported) {
        const previous = tokens.text(tokens.before(index));
        const next = tokens.after(index);
        const following = tokens.text(next);
        const word = tokens.text(index);
        const module = imported.has(word);
        if (this.#keywords.has(word) || (this.#contextual.has(word) && this.#isContextualKeyword(tokens, index))) {
            return "k";
        }
        if (this.#constants.has(word)) {
            return "n";
        }
        if (previous === "import" || (previous === "/" && module) || (module && following === ".")) {
            return "m";
        }
        if (upper.test(word[0])) {
            return "t";
        }
        if (previous === "fn" || following === "(" || (following === "[" && !this.#types.has(word) && tokens.closes(next))) {
            return "f";
        }
        return this.#types.has(word) ? "t" : "";
    }

    #isContextualKeyword(tokens, index) {
        const next = tokens.after(index);
        const word = tokens.text(index);
        if (word === "after") {
            return tokens.startsLine(index) && tokens.text(next) === "(";
        }
        if (word === "fork") {
            return tokens.text(next) === "{" && tokens.text(tokens.before(index)) === "=";
        }
        if (word === "region") {
            const previous = tokens.text(tokens.before(index));
            return tokens.text(next) === "{" && previous !== "if" && previous !== "match" && previous !== "in";
        }
        const previous = tokens.sameLineBefore(index);
        return this.#endsValue(tokens.at(previous)) && tokens.startsValue(next);
    }

    #endsValue(token) {
        if (token === undefined || this.#keywords.has(token.text)) {
            return false;
        }
        return isValue(token) || ")]}?".includes(token.text);
    }
}

export class Highlighter {
    #area;
    #code;
    #editor;
    #styler;
    #end = document.createTextNode(" ");
    #lines = [];

    constructor(area, layer) {
        this.#area = area;
        this.#code = layer.querySelector("code");
        this.#editor = layer.parentElement;
        this.#styler = new Styler(layer.dataset);
        this.#code.replaceChildren(this.#end);
        area.addEventListener("input", () => this.paint());
        area.addEventListener("scroll", () => this.#follow());
    }

    paint() {
        const runs = this.#styler.runs(this.#area.value);
        const lines = splitLines(runs);
        this.#patch(lines);
        this.#editor.classList.add("painted");
        requestAnimationFrame(() => this.#follow());
    }

    #patch(lines) {
        const old = this.#lines;
        const shortest = Math.min(old.length, lines.length);
        let prefix = 0;
        while (prefix < shortest && sameLine(old[prefix], lines[prefix])) {
            prefix += 1;
        }
        let suffix = 0;
        while (suffix < shortest - prefix && sameLine(old[old.length - 1 - suffix], lines[lines.length - 1 - suffix])) {
            suffix += 1;
        }
        const kept = old.length - suffix;
        const anchor = old[kept]?.node ?? this.#end;
        for (const { node } of old.slice(prefix, kept)) {
            node.remove();
        }
        const added = lines.slice(prefix, lines.length - suffix).map((runs) => ({ runs, node: lineNode(runs) }));
        const fresh = document.createDocumentFragment();
        for (const { node } of added) {
            fresh.append(node);
        }
        this.#code.insertBefore(fresh, anchor);
        this.#lines = old.slice(0, prefix).concat(added, old.slice(kept));
    }

    #follow() {
        const { scrollLeft, scrollTop } = this.#area;
        this.#code.style.transform = `translate(${-scrollLeft}px, ${-scrollTop}px)`;
    }
}
