class Exit {
  constructor(status) {
    this.status = status;
  }
}

const panicStatus = 2;
const entropyChunk = 65536;
const monotonicClock = 1;

function round(x) {
  return x < 0 ? -Math.round(-x) : Math.round(x);
}

const math = {
  sin: Math.sin,
  cos: Math.cos,
  tan: Math.tan,
  asin: Math.asin,
  acos: Math.acos,
  atan: Math.atan,
  atan2: Math.atan2,
  sinh: Math.sinh,
  cosh: Math.cosh,
  tanh: Math.tanh,
  pow: Math.pow,
  exp: Math.exp,
  log: Math.log,
  log2: Math.log2,
  log10: Math.log10,
  cbrt: Math.cbrt,
  hypot: Math.hypot,
  ceil: Math.ceil,
  round,
  trunc: Math.trunc,
  fmod: (x, y) => x % y,
};

function trapMessage(error) {
  if (error instanceof RangeError && /call stack/.test(error.message)) {
    return "stack overflow";
  }
  if (error instanceof WebAssembly.RuntimeError && /out of bounds/.test(error.message)) {
    return "invalid memory access";
  }
  return error.message;
}

export async function run(bytes, host) {
  let memory = null;
  const view = (ptr, len) => (len === 0 ? new Uint8Array(0) : new Uint8Array(memory.buffer, ptr >>> 0, len >>> 0));
  const entropy = (data) => {
    for (let at = 0; at < data.length; at += entropyChunk) {
      host.entropy(data.subarray(at, at + entropyChunk));
    }
  };
  const imports = {
    low: {
      write: (fd, ptr, len) => host.write(fd, view(ptr, len)),
      exit: (status) => {
        throw new Exit(status);
      },
      clock: (id) => (id === monotonicClock ? host.monotonic() : host.wall()),
      entropy: (ptr, len) => entropy(view(ptr, len)),
    },
    m: math,
  };
  const { instance } = await WebAssembly.instantiate(bytes, imports);
  memory = instance.exports.memory;
  try {
    instance.exports._start();
    return 0;
  } catch (error) {
    if (error instanceof Exit) {
      return error.status;
    }
    host.write(2, new TextEncoder().encode(`panic: ${trapMessage(error)}\n`));
    return panicStatus;
  }
}

const workerStackMb = 256;

async function runFile(path) {
  const fs = await import("node:fs");
  const crypto = await import("node:crypto");
  const bytes = fs.readFileSync(path);
  const pause = new Int32Array(new SharedArrayBuffer(4));
  return run(bytes, {
    write: (fd, data) => {
      let at = 0;
      while (at < data.length) {
        try {
          at += fs.writeSync(fd, data, at, data.length - at);
        } catch (error) {
          if (error.code !== "EAGAIN") {
            return;
          }
          Atomics.wait(pause, 0, 0, 1);
        }
      }
    },
    monotonic: () => process.hrtime.bigint(),
    wall: () => BigInt(Math.round((performance.timeOrigin + performance.now()) * 1000)) * 1000n,
    entropy: (data) => crypto.getRandomValues(data),
  });
}

async function main() {
  const { Worker, isMainThread, parentPort, workerData } = await import("node:worker_threads");
  if (!isMainThread) {
    parentPort.postMessage(await runFile(workerData));
    return;
  }
  const worker = new Worker(new URL(import.meta.url), {
    workerData: process.argv[2],
    resourceLimits: { stackSizeMb: workerStackMb },
  });
  worker.on("message", (status) => {
    process.exitCode = status;
  });
  worker.on("error", (error) => {
    process.stderr.write(`panic: ${trapMessage(error)}\n`);
    process.exitCode = panicStatus;
  });
}

if (typeof process !== "undefined" && process.argv[1] && import.meta.url.endsWith(process.argv[1].split("/").pop())) {
  await main();
}
