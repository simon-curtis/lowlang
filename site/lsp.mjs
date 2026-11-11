import { spawn, execFile } from "node:child_process";
import { mkdir, mkdtemp, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import { promisify } from "node:util";

const run = promisify(execFile);
const repository = fileURLToPath(new URL("../", import.meta.url));
const settle = 10000;

function frame(message) {
    const body = JSON.stringify(message);
    return `Content-Length: ${Buffer.byteLength(body)}\r\n\r\n${body}`;
}

function unfence(value) {
    return value.replace(/^```\w*\n/, "").replace(/\n```$/, "");
}

export class Session {
    #directory;
    #process;
    #buffer = Buffer.alloc(0);
    #pending = new Map();
    #published = new Map();
    #diagnostics = new Map();
    #next = 1;
    #files = 0;

    async start() {
        this.#directory = await mkdtemp(join(tmpdir(), "low-site-"));
        const binary = join(this.#directory, "low");
        await run("go", ["build", "-o", binary, "./cmd/low"], { cwd: repository });
        this.#process = spawn(binary, ["lsp"], { cwd: this.#directory, stdio: ["pipe", "pipe", "ignore"] });
        this.#process.stdout.on("data", (chunk) => this.#receive(chunk));
        await this.#request("initialize", { processId: process.pid, rootUri: pathToFileURL(this.#directory).href, capabilities: {} });
        this.#notify("initialized", {});
    }

    async stop() {
        if (this.#process) {
            await this.#request("shutdown", null);
            this.#notify("exit", null);
            this.#process.kill();
        }
        if (this.#directory) {
            await rm(this.#directory, { recursive: true, force: true });
        }
    }

    async inspect(program, offsets) {
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
        for (const offset of offsets) {
            const position = program.position(offset);
            const hover = await this.#request("textDocument/hover", { textDocument: { uri }, position });
            if (hover?.contents?.value) {
                hovers.set(offset, unfence(hover.contents.value));
            }
            const locations = await this.#request("textDocument/references", { textDocument: { uri }, position, context: { includeDeclaration: true } });
            const group = (locations ?? []).filter((location) => location.uri === uri).map((location) => program.offset(location.range.start)).filter((found) => found !== undefined);
            if (group.includes(offset)) {
                references.set(offset, Math.min(...group));
            }
        }
        this.#notify("textDocument/didClose", { textDocument: { uri } });
        const errors = (this.#diagnostics.get(uri) ?? []).filter((item) => item.severity === 1).map((item) => item.message);
        return { hovers, references, errors };
    }

    #await(uri) {
        return new Promise((resolve) => {
            const timer = setTimeout(resolve, settle);
            this.#published.set(uri, () => {
                clearTimeout(timer);
                resolve();
            });
        });
    }

    #notify(method, params) {
        this.#process.stdin.write(frame({ jsonrpc: "2.0", method, params }));
    }

    #request(method, params) {
        const id = this.#next;
        this.#next += 1;
        return new Promise((resolve, reject) => {
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
            this.#published.get(message.params.uri)?.();
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
