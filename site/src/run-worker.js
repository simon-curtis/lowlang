import { run } from "../../compiler/wasm/host.mjs";

const encoder = new TextEncoder();
const stdout = 1;
const stderr = 2;
const nanosecondsPerMillisecond = 1000000n;

function post(message) {
    self.postMessage(message);
}

function streamTo(fd) {
    const decoder = new TextDecoder();
    return {
        write(bytes) {
            const text = decoder.decode(bytes, { stream: true });
            if (text !== "") {
                post({ kind: "output", fd, text });
            }
        },
        finish() {
            const text = decoder.decode();
            if (text !== "") {
                post({ kind: "output", fd, text });
            }
        },
    };
}

function platform(read, write) {
    return {
        read,
        write,
        monotonic: () => {
            const nanoseconds = Math.round(performance.now() * 1e6);
            return BigInt(nanoseconds);
        },
        wall: () => {
            const milliseconds = BigInt(Date.now());
            return milliseconds * nanosecondsPerMillisecond;
        },
        entropy: (bytes) => crypto.getRandomValues(bytes),
    };
}

function concatenate(chunks) {
    const length = chunks.reduce((total, chunk) => total + chunk.length, 0);
    const joined = new Uint8Array(length);
    let at = 0;
    for (const chunk of chunks) {
        joined.set(chunk, at);
        at += chunk.length;
    }
    return joined;
}

async function compile(compiler, source) {
    const input = encoder.encode(source);
    const image = [];
    const errors = streamTo(stderr);
    let consumed = 0;
    const read = (fd, bytes) => {
        const count = Math.min(bytes.length, input.length - consumed);
        bytes.set(input.subarray(consumed, consumed + count));
        consumed += count;
        return count;
    };
    const write = (fd, bytes) => {
        if (fd === stdout) {
            image.push(bytes.slice());
        } else {
            errors.write(bytes);
        }
    };
    const environment = platform(read, write);
    const status = await run(compiler, environment);
    errors.finish();
    return { status, image: concatenate(image) };
}

async function execute(image) {
    const outputs = streamTo(stdout);
    const errors = streamTo(stderr);
    const write = (fd, bytes) => (fd === stdout ? outputs.write(bytes) : errors.write(bytes));
    const environment = platform(() => 0, write);
    const status = await run(image, environment);
    outputs.finish();
    errors.finish();
    return status;
}

async function start({ compiler, source }) {
    post({ kind: "phase", phase: "compile" });
    const compiled = await compile(compiler, source);
    if (compiled.status !== 0) {
        post({ kind: "done", phase: "compile", status: compiled.status });
        return;
    }
    post({ kind: "phase", phase: "run" });
    const status = await execute(compiled.image);
    post({ kind: "done", phase: "run", status });
}

self.addEventListener(
    "message",
    async (event) => {
        try {
            await start(event.data);
        } catch (error) {
            const message = error instanceof Error ? error.message : String(error);
            post({ kind: "fault", message });
        }
    },
    { once: true },
);
