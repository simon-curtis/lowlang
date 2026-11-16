const unit = "    ";
const closers = { "(": ")", "[": "]", "{": "}", '"': '"' };
const blockOpeners = new Set(["(", "[", "{"]);
const typedOver = new Set([")", "]", "}", '"']);
const pairableBefore = /^[\s)\]},;]?$/;
const wordCharacter = /[\p{L}\p{N}_]/u;
const straightQuotes = { "“": '"', "”": '"', "„": '"', "‘": "'", "’": "'" };

function characterOffset(text, byteColumn) {
    const encoder = new TextEncoder();
    let bytes = 0;
    let offset = 0;
    for (const character of text) {
        if (bytes >= byteColumn - 1) {
            break;
        }
        bytes += encoder.encode(character).length;
        offset += character.length;
    }
    return offset;
}

export class Editor {
    #area;
    #pairs;
    #applying = false;
    #released = false;
    #closers = [];
    #goal = { at: -1, column: 0 };
    #text;

    constructor(area, pairs) {
        this.#area = area;
        this.#pairs = pairs;
        this.#text = area.value;
        area.addEventListener("beforeinput", (event) => this.#beforeInput(event));
        area.addEventListener("input", (event) => this.#afterInput(event));
        area.addEventListener("keydown", (event) => this.#keyDown(event));
        area.addEventListener("focus", () => {
            this.#released = false;
        });
    }

    reset() {
        this.#text = this.#area.value;
        this.#closers = [];
        this.#goal = { at: -1, column: 0 };
    }

    type(text) {
        this.#track();
        if (text.length === 1 && this.#pairing(text)) {
            return;
        }
        const { selectionStart, selectionEnd } = this.#area;
        this.#replace(selectionStart, selectionEnd, text);
    }

    #newline() {
        const { value, selectionStart: start, selectionEnd: end } = this.#area;
        const before = value.slice(this.#lineStart(start), start);
        const { text, caret } = this.#lineBreak(before, value[end]);
        this.#replace(start, end, `\n${text}`, start + 1 + caret);
    }

    indent() {
        const { value, selectionStart: start, selectionEnd: end } = this.#area;
        if (!value.slice(start, end).includes("\n")) {
            this.#replace(start, end, unit);
            return;
        }
        this.#shiftLines((line) => (line === "" ? line : `${unit}${line}`));
    }

    #dedent() {
        this.#shiftLines((line) => line.replace(/^ {1,4}/, ""));
    }

    move(offset) {
        const { value, selectionStart: start, selectionEnd: end } = this.#area;
        const edge = offset < 0 ? start : end;
        const at = start === end ? Math.min(Math.max(edge + offset, 0), value.length) : edge;
        this.#area.setSelectionRange(at, at);
        this.reveal();
    }

    moveLine(offset) {
        const { value, selectionStart } = this.#area;
        const lines = value.split("\n");
        const row = value.slice(0, selectionStart).split("\n").length - 1;
        const column = this.#goal.at === selectionStart ? this.#goal.column : selectionStart - this.#lineStart(selectionStart);
        const target = Math.min(Math.max(row + offset, 0), lines.length - 1);
        const start = lines.slice(0, target).reduce((sum, line) => sum + line.length + 1, 0);
        const at = start + Math.min(column, lines[target].length);
        this.#area.setSelectionRange(at, at);
        this.#goal = { at, column };
        this.reveal();
    }

    undo() {
        document.execCommand("undo");
        this.reveal();
    }

    goTo(line, byteColumn) {
        const area = this.#area;
        const lines = area.value.split("\n");
        const row = Math.min(Math.max(line, 1), lines.length) - 1;
        const start = lines.slice(0, row).reduce((sum, text) => sum + text.length + 1, 0);
        const at = start + characterOffset(lines[row], byteColumn);
        area.focus({ preventScroll: true });
        area.setSelectionRange(at, at);
        this.reveal();
        area.scrollIntoView({ block: "nearest" });
    }

    caretVisible() {
        const { y, height } = this.#caretRow();
        const area = this.#area;
        return y >= area.scrollTop && y + height <= area.scrollTop + area.clientHeight;
    }

    reveal() {
        const area = this.#area;
        const { y, height, padding } = this.#caretRow();
        const bottom = area.scrollTop + area.clientHeight - padding;
        if (y < area.scrollTop + padding) {
            area.scrollTop = Math.max(y - padding, 0);
        } else if (y + height > bottom) {
            area.scrollTop = y + height - area.clientHeight + padding;
        }
    }

    #caretRow() {
        const area = this.#area;
        const style = getComputedStyle(area);
        const height = parseFloat(style.lineHeight);
        const padding = parseFloat(style.paddingTop);
        const row = area.value.slice(0, area.selectionStart).split("\n").length - 1;
        return { y: padding + row * height, height, padding };
    }

    #beforeInput(event) {
        this.#track();
        if (this.#applying || event.isComposing) {
            return;
        }
        const kind = event.inputType;
        const data = event.data ?? "";
        const newline = kind === "insertLineBreak" || kind === "insertParagraph" || (kind === "insertText" && data === "\n");
        if (newline) {
            event.preventDefault();
            this.#newline();
        } else if (kind === "insertText" && this.#pairs() && straightQuotes[data] !== undefined) {
            event.preventDefault();
            this.type(straightQuotes[data]);
        } else if (kind === "insertText" && data.length === 1 && this.#pairing(data)) {
            event.preventDefault();
        } else if (kind === "deleteContentBackward" && this.#backspace()) {
            event.preventDefault();
        }
    }

    #afterInput(event) {
        this.#track();
        const kind = event.inputType;
        if (this.#applying || (kind !== "insertLineBreak" && kind !== "insertParagraph")) {
            return;
        }
        const at = this.#area.selectionStart;
        if (event.isComposing) {
            this.#area.addEventListener("compositionend", () => this.#indentBreak(at), { once: true });
        } else {
            this.#indentBreak(at);
        }
    }

    #indentBreak(start) {
        const { value, selectionStart, selectionEnd } = this.#area;
        if (selectionStart !== start || selectionEnd !== start || this.#lineStart(start) !== start) {
            return;
        }
        const previous = value.slice(this.#lineStart(start - 1), start - 1);
        const { text, caret } = this.#lineBreak(previous, value[start]);
        this.#replace(start, start, text, start + caret);
    }

    #track() {
        const before = this.#text;
        const after = this.#area.value;
        if (before === after) {
            return;
        }
        this.#text = after;
        const shortest = Math.min(before.length, after.length);
        let prefix = 0;
        while (prefix < shortest && before[prefix] === after[prefix]) {
            prefix += 1;
        }
        let suffix = 0;
        while (suffix < shortest - prefix && before[before.length - 1 - suffix] === after[after.length - 1 - suffix]) {
            suffix += 1;
        }
        const end = before.length - suffix;
        const delta = after.length - before.length;
        this.#closers = this.#closers
            .filter((offset) => offset < prefix || offset >= end)
            .map((offset) => (offset >= end ? offset + delta : offset));
    }

    #lineBreak(before, next) {
        const indent = /^ */.exec(before)[0];
        const opener = before.trimEnd().at(-1);
        if (!blockOpeners.has(opener)) {
            return { text: indent, caret: indent.length };
        }
        const inner = `${indent}${unit}`;
        if (next === closers[opener]) {
            return { text: `${inner}\n${indent}`, caret: inner.length };
        }
        return { text: inner, caret: inner.length };
    }

    #keyDown(event) {
        if (event.key === "Escape") {
            this.#released = true;
            return;
        }
        const plain = !event.ctrlKey && !event.metaKey && !event.altKey;
        if (event.key === "Tab" && plain && !this.#released) {
            event.preventDefault();
            if (event.shiftKey) {
                this.#dedent();
            } else {
                this.indent();
            }
        } else if (event.key !== "Shift" && event.key !== "Tab") {
            this.#released = false;
        }
    }

    #pairing(character) {
        if (!this.#pairs()) {
            return false;
        }
        const { value, selectionStart: start, selectionEnd: end } = this.#area;
        const ownCloser = this.#closers.indexOf(end);
        if (start === end && typedOver.has(character) && value[end] === character && ownCloser !== -1) {
            this.#closers.splice(ownCloser, 1);
            this.#area.setSelectionRange(end + 1, end + 1);
            return true;
        }
        const closer = closers[character];
        if (closer === undefined) {
            return false;
        }
        if (start !== end) {
            const selected = value.slice(start, end);
            this.#replace(start, end, `${character}${selected}${closer}`);
            this.#area.setSelectionRange(start + 1, end + 1);
            return true;
        }
        const quoteAfterWord = character === '"' && wordCharacter.test(value[start - 1] ?? "");
        if (quoteAfterWord || !pairableBefore.test(value[end] ?? "")) {
            return false;
        }
        this.#replace(start, end, `${character}${closer}`, start + 1);
        this.#closers.push(start + 1);
        return true;
    }

    #backspace() {
        const { value, selectionStart: start, selectionEnd: end } = this.#area;
        if (start !== end || start === 0) {
            return false;
        }
        const previous = value[start - 1];
        const paired = closers[previous] !== undefined && closers[previous] === value[start] && this.#closers.includes(start);
        if (this.#pairs() && paired) {
            this.#replace(start - 1, start + 1, "");
            return true;
        }
        const before = value.slice(this.#lineStart(start), start);
        if (!/^ +$/.test(before)) {
            return false;
        }
        const width = before.length % unit.length || unit.length;
        this.#replace(start - width, start, "");
        return true;
    }

    #shiftLines(shift) {
        const { value, selectionStart: start, selectionEnd: end } = this.#area;
        const from = this.#lineStart(start);
        const last = end > start && value[end - 1] === "\n" ? end - 1 : end;
        const lineEnd = value.indexOf("\n", last);
        const to = lineEnd === -1 ? value.length : lineEnd;
        const original = value.slice(from, to);
        const shifted = original.split("\n").map(shift).join("\n");
        if (shifted === original) {
            return;
        }
        if (start === end) {
            const caret = Math.max(from, start + shifted.length - original.length);
            this.#replace(from, to, shifted, caret);
        } else {
            this.#replace(from, to, shifted);
            this.#area.setSelectionRange(from, from + shifted.length);
        }
    }

    #lineStart(at) {
        return this.#area.value.lastIndexOf("\n", at - 1) + 1;
    }

    #replace(from, to, text, caret) {
        const area = this.#area;
        area.setSelectionRange(from, to);
        const command = text === "" ? "delete" : "insertText";
        this.#applying = true;
        let applied = false;
        try {
            applied = document.execCommand(command, false, text);
        } finally {
            this.#applying = false;
        }
        if (!applied) {
            area.setRangeText(text, from, to, "end");
            area.dispatchEvent(new Event("input", { bubbles: true }));
        }
        this.#track();
        const at = caret ?? from + text.length;
        area.setSelectionRange(at, at);
    }
}
