import { spawn, execFile } from "node:child_process";
import { mkdir, mkdtemp, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import { promisify } from "node:util";

const run = promisify(execFile);
const repository = fileURLToPath(new URL("../", import.meta.url));

function frame(message) {
    const body = JSON.stringify(message);
    return `Content-Length: ${Buffer.byteLength(body)}\r\n\r\n${body}`;
}

function split(value) {
    const [, code = value, doc = ""] = /^```\w*\n([\s\S]*?)\n```(?:\n\n([\s\S]*))?$/.exec(value) ?? [];
    return { code, doc };
}

export class Session {
    #directory;
    #process;
    #buffer = Buffer.alloc(0);
    #pending = new Map();
    #published = new Map();
    #diagnostics = new Map();
    #failures = [];
    #lost;
    #next = 1;
    #files = 0;

    get binary() {
        return join(this.#directory, process.platform === "win32" ? "low.exe" : "low");
    }

    async start() {
        this.#directory = await mkdtemp(join(tmpdir(), "low-site-"));
        await run("go", ["build", "-o", this.binary, "./cmd/low"], { cwd: repository });
        this.#process = spawn(this.binary, ["lsp"], { cwd: this.#directory, stdio: ["pipe", "pipe", "inherit"] });
        this.#process.stdout.on("data", (chunk) => this.#receive(chunk));
        this.#process.stdin.on("error", (error) => this.#abandon(error));
        this.#process.on("error", (error) => this.#abandon(error));
        this.#process.on("exit", (code, signal) => this.#abandon(new Error(`low lsp exited with ${code ?? signal}`)));
        await this.#request("initialize", { processId: process.pid, rootUri: pathToFileURL(this.#directory).href, capabilities: {} });
        this.#notify("initialized", {});
    }

    verify() {
        if (this.#failures.length > 0) {
            throw new Error(`${this.#failures.length} snippets fail to check:\n${this.#failures.join("\n")}`);
        }
    }

    async stop() {
        if (this.#process && !this.#lost) {
            await this.#request("shutdown", null);
            const exited = new Promise((resolve) => this.#process.once("exit", resolve));
            this.#notify("exit", null);
            this.#process.kill();
            await exited;
        }
        if (this.#directory) {
            await rm(this.#directory, { recursive: true, force: true, maxRetries: 5 });
        }
    }

    async inspect(program, offsets, label, expected) {
        let checked;
        try {
            checked = await this.#query(program, offsets);
        } catch (error) {
            throw new Error(`${label}: ${error.message}`, { cause: error });
        }
        const failure = expected === undefined ? checked.errors.join("; ") : this.#mismatch(checked.errors, expected);
        if (failure) {
            this.#failures.push(`${label}: ${failure}`);
        }
        return checked;
    }

    #mismatch(errors, expected) {
        const found = errors.some((message) => message.includes(expected));
        return found ? "" : `expected an error containing "${expected}", got ${errors.length === 0 ? "none" : errors.join("; ")}`;
    }

    async #query(program, offsets) {
        this.#files += 1;
        const folder = join(this.#directory, `snippet${this.#files}`);
        for (const file of [{ path: "main.low", text: program.text }, ...program.files]) {
            const target = join(folder, file.path);
            await mkdir(dirname(target), { recursive: true });
            await writeFile(target, file.text);
        }
        const path = join(folder, "main.low");
        const uri = pathToFileURL(path).href;
        const published = this.#await(uri);
        this.#notify("textDocument/didOpen", { textDocument: { uri, languageId: "low", version: 1, text: program.text } });
        await published;
        const hovers = new Map();
        const references = new Map();
        const definitions = new Map();
        for (const offset of offsets) {
            const position = program.position(offset);
            const hover = await this.#request("textDocument/hover", { textDocument: { uri }, position });
            if (hover?.contents?.value) {
                hovers.set(offset, split(hover.contents.value));
            }
            const locations = await this.#request("textDocument/references", { textDocument: { uri }, position, context: { includeDeclaration: true } });
            const group = (locations ?? []).filter((location) => location.uri === uri).map((location) => program.offset(location.range.start)).filter((found) => found !== undefined);
            if (group.includes(offset)) {
                references.set(offset, Math.min(...group));
            }
            const definition = await this.#request("textDocument/definition", { textDocument: { uri }, position });
            if (definition?.uri === uri) {
                definitions.set(offset, { offset: program.offset(definition.range.start) });
            } else if (definition) {
                definitions.set(offset, { uri: definition.uri, line: definition.range.start.line });
            }
        }
        this.#notify("textDocument/didClose", { textDocument: { uri } });
        const errors = (this.#diagnostics.get(uri) ?? []).filter((item) => item.severity === 1).map((item) => item.message);
        return { hovers, references, definitions, errors };
    }

    #await(uri) {
        return new Promise((resolve, reject) => {
            if (this.#lost) {
                reject(this.#lost);
                return;
            }
            this.#published.set(uri, { resolve, reject });
        });
    }

    #notify(method, params) {
        if (!this.#lost) {
            this.#process.stdin.write(frame({ jsonrpc: "2.0", method, params }));
        }
    }

    #abandon(error) {
        if (this.#lost) {
            return;
        }
        this.#lost = error;
        for (const { reject } of [...this.#pending.values(), ...this.#published.values()]) {
            reject(error);
        }
        this.#pending.clear();
        this.#published.clear();
    }

    #request(method, params) {
        const id = this.#next;
        this.#next += 1;
        return new Promise((resolve, reject) => {
            if (this.#lost) {
                reject(this.#lost);
                return;
            }
            this.#pending.set(id, { resolve, reject });
            this.#process.stdin.write(frame({ jsonrpc: "2.0", id, method, params }));
        });
    }

    #receive(chunk) {
        this.#buffer = Buffer.concat([this.#buffer, chunk]);
        for (;;) {
            const header = this.#buffer.indexOf("\r\n\r\n");
            if (header < 0) {
                return;
            }
            const length = Number(/Content-Length: (\d+)/i.exec(this.#buffer.subarray(0, header).toString())[1]);
            const start = header + 4;
            if (this.#buffer.length < start + length) {
                return;
            }
            const message = JSON.parse(this.#buffer.subarray(start, start + length).toString());
            this.#buffer = this.#buffer.subarray(start + length);
            this.#dispatch(message);
        }
    }

    #dispatch(message) {
        if (message.method === "textDocument/publishDiagnostics") {
            this.#diagnostics.set(message.params.uri, message.params.diagnostics);
            this.#published.get(message.params.uri)?.resolve();
            this.#published.delete(message.params.uri);
        } else if (message.id !== undefined && this.#pending.has(message.id)) {
            const { resolve, reject } = this.#pending.get(message.id);
            this.#pending.delete(message.id);
            if (message.error) {
                reject(new Error(message.error.message));
            } else {
                resolve(message.result);
            }
        }
    }
}
