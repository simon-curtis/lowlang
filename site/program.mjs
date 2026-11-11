const declaration = /^(\/\/\/|(pub\s+)?(fn\s+\w|struct\s|enum\s|import\s|extern\s))/;
const continuation = /^(\s|[})\]]|$)/;
const elision = /(^\s*|\{\s*)\.\.\.(?=\s*(\}|$))/gm;

function units(lines, first) {
    const groups = [];
    lines.forEach((text, index) => {
        if (groups.length === 0 || !continuation.test(text)) {
            groups.push({ declares: declaration.test(text), lines: [] });
        }
        groups.at(-1).lines.push({ text, source: index - first });
    });
    return groups;
}

export class Program {
    #rows = new Map();
    #sources = new Map();
    #starts = [];

    constructor(source, preludes = []) {
        const code = preludes.filter((prelude) => prelude.path === "").map((prelude) => prelude.text).join("\n");
        const hidden = code ? code.split("\n") : [];
        const head = hidden.at(-1)?.endsWith("{") ? hidden.pop() : "";
        const elided = source.replace(elision, "$1for {}");
        const groups = units([...hidden, ...elided.split("\n")], hidden.length);
        const declarations = groups.filter((group) => group.declares).flatMap((group) => group.lines);
        const statements = groups.filter((group) => !group.declares).flatMap((group) => group.lines);
        const entry = declarations.some((line) => /^fn main\(/.test(line.text));
        const body = statements.some((line) => line.text.trim() !== "");
        const output = [...declarations];
        if (body) {
            const open = head || (entry ? "fn snippet() {" : "fn main() {");
            output.push({ text: open, source: -1 }, ...statements, { text: "}", source: -1 });
        } else if (!entry) {
            output.push({ text: "fn main() {}", source: -1 });
        }
        output.forEach((line, row) => {
            if (line.source >= 0) {
                this.#rows.set(line.source, row);
                this.#sources.set(row, line.source);
            }
        });
        let start = 0;
        for (const line of source.split("\n")) {
            this.#starts.push(start);
            start += line.length + 1;
        }
        this.source = source;
        this.text = `${output.map((line) => line.text).join("\n")}\n`;
        this.files = preludes.filter((prelude) => prelude.path !== "");
    }

    position(offset) {
        const before = this.source.slice(0, offset);
        const line = before.split("\n").length - 1;
        const character = offset - (before.lastIndexOf("\n") + 1);
        return { line: this.#rows.get(line), character };
    }

    offset(position) {
        const line = this.#sources.get(position.line);
        return line === undefined ? undefined : this.#starts[line] + position.character;
    }
}
