const compilerUrl = "/playground.wasm";
const outputLimit = 262144;
const stdoutFd = 1;
const stderrFd = 2;
const noteFd = 0;

const classes = {
    [noteFd]: "run-note",
    [stdoutFd]: "run-out",
    [stderrFd]: "run-err",
};

const labels = {
    idle: "",
    loading: "Loading the compiler…",
    compiling: "Compiling…",
    running: "Running…",
};

let compiler;

function loadCompiler() {
    if (compiler === undefined) {
        const response = fetch(compilerUrl);
        compiler = WebAssembly.compileStreaming(response).catch((error) => {
            compiler = undefined;
            throw error;
        });
    }
    return compiler;
}

function describe(error) {
    return error instanceof Error ? error.message : String(error);
}

class Output {
    #element;
    #pending = [];
    #frame = 0;
    #length = 0;
    #last = null;

    constructor(element) {
        this.#element = element;
    }

    get empty() {
        return this.#length === 0 && this.#pending.length === 0;
    }

    clear() {
        cancelAnimationFrame(this.#frame);
        this.#frame = 0;
        this.#pending = [];
        this.#length = 0;
        this.#last = null;
        this.#element.replaceChildren();
    }

    write(fd, text) {
        this.#pending.push({ fd, text });
        if (this.#frame === 0) {
            this.#frame = requestAnimationFrame(() => this.flush());
        }
    }

    flush() {
        cancelAnimationFrame(this.#frame);
        this.#frame = 0;
        const pending = this.#pending;
        this.#pending = [];
        for (const { fd, text } of pending) {
            this.#append(fd, text);
        }
        this.#trim();
        this.#element.scrollTop = this.#element.scrollHeight;
    }

    #append(fd, text) {
        this.#length += text.length;
        if (fd !== noteFd && this.#last?.fd === fd) {
            this.#last.node.appendData(text);
            return;
        }
        const element = document.createElement(fd === noteFd ? "div" : "span");
        element.className = classes[fd];
        const node = document.createTextNode(text);
        element.append(node);
        this.#element.append(element);
        this.#last = fd === noteFd ? null : { fd, node, element };
    }

    #trim() {
        while (this.#length > outputLimit && this.#element.firstChild !== null) {
            const first = this.#element.firstChild;
            const node = first.firstChild;
            const excess = this.#length - outputLimit;
            if (node.length > excess) {
                node.deleteData(0, excess);
                this.#length -= excess;
            } else {
                this.#length -= node.length;
                if (this.#last?.element === first) {
                    this.#last = null;
                }
                first.remove();
            }
        }
    }
}

export class Session {
    #output;
    #onstate;
    #worker = null;
    #generation = 0;
    #busy = false;

    constructor(output, onstate) {
        this.#output = output;
        this.#onstate = onstate;
    }

    get busy() {
        return this.#busy;
    }

    async run(source) {
        if (this.#busy) {
            return;
        }
        this.#busy = true;
        this.#generation += 1;
        const generation = this.#generation;
        this.#output.clear();
        this.#onstate("loading");
        let module;
        try {
            module = await loadCompiler();
        } catch (error) {
            if (generation === this.#generation) {
                const reason = describe(error);
                this.#output.write(stderrFd, `Could not load the compiler: ${reason}\n`);
                this.#end();
            }
            return;
        }
        if (generation !== this.#generation) {
            return;
        }
        try {
            this.#spawn(module, source);
        } catch (error) {
            const reason = describe(error);
            this.#output.write(stderrFd, `Could not start the program: ${reason}\n`);
            this.#end();
        }
    }

    stop() {
        if (!this.#busy) {
            return;
        }
        this.#generation += 1;
        this.#output.write(noteFd, "Stopped.");
        this.#end();
    }

    #spawn(module, source) {
        const worker = new Worker(new URL("./run-worker.js", import.meta.url), { type: "module" });
        this.#worker = worker;
        worker.addEventListener("message", (event) => this.#receive(worker, event.data));
        worker.addEventListener("error", (event) => {
            event.preventDefault();
            this.#fail(worker, event.message || "the worker stopped");
        });
        worker.addEventListener("messageerror", () => this.#fail(worker, "the worker sent an unreadable message"));
        worker.postMessage({ compiler: module, source });
    }

    #receive(worker, message) {
        if (worker !== this.#worker) {
            return;
        }
        if (message.kind === "output") {
            this.#output.write(message.fd, message.text);
        } else if (message.kind === "phase") {
            this.#onstate(message.phase === "compile" ? "compiling" : "running");
        } else if (message.kind === "done") {
            this.#finish(message);
        } else if (message.kind === "fault") {
            this.#fail(worker, message.message);
        }
    }

    #finish({ phase, status }) {
        if (status !== 0) {
            const note = phase === "compile" ? `Compilation failed with exit status ${status}.` : `Exit status ${status}.`;
            this.#output.write(noteFd, note);
        } else if (this.#output.empty) {
            this.#output.write(noteFd, "Finished with no output.");
        }
        this.#end();
    }

    #fail(worker, message) {
        if (worker !== this.#worker) {
            return;
        }
        this.#output.write(stderrFd, `panic: ${message}\n`);
        this.#end();
    }

    #end() {
        this.#worker?.terminate();
        this.#worker = null;
        this.#busy = false;
        this.#output.flush();
        this.#onstate("idle");
    }
}

export function bind({ run, stop, status, output, source }) {
    const printer = new Output(output);
    const session = new Session(printer, (state) => {
        const idle = state === "idle";
        run.disabled = !idle;
        stop.hidden = idle;
        status.textContent = labels[state];
    });
    run.addEventListener("click", () => {
        output.hidden = false;
        const program = source();
        session.run(program);
    });
    stop.addEventListener("click", () => session.stop());
    return session;
}

function button(label) {
    const control = document.createElement("button");
    control.type = "button";
    control.className = "run-button";
    control.textContent = label;
    return control;
}

function attach(block) {
    const bar = document.createElement("div");
    bar.className = "run-bar";
    const run = button("Run");
    const stop = button("Stop");
    stop.hidden = true;
    const status = document.createElement("span");
    status.className = "run-status";
    status.setAttribute("role", "status");
    bar.append(run, stop, status);
    const output = document.createElement("div");
    output.className = "run-output";
    output.setAttribute("role", "log");
    output.tabIndex = 0;
    output.hidden = true;
    block.append(bar, output);
    bind({ run, stop, status, output, source: () => block.dataset.program });
}

for (const block of document.querySelectorAll("[data-program]")) {
    attach(block);
}
