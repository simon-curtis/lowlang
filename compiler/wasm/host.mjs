class Exit {
  constructor(status) {
    this.status = status;
  }
}

const panicStatus = 2;
const entropyChunk = 65536;
const stressWords = 8192;
const pageBits = 16;
const addressablePages = 65536;
const monotonicClock = 1;
const noInput = -1;
const lostStatus = "the program stopped without finishing";
const noPromiseIntegration = "green threads need WebAssembly JavaScript Promise Integration";

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
  let pageLimit = addressablePages;
  const view = (ptr, len) => (len === 0 ? new Uint8Array(0) : new Uint8Array(memory.buffer, ptr >>> 0, len >>> 0));
  const entropy = (data) => {
    for (let at = 0; at < data.length; at += entropyChunk) {
      host.entropy(data.subarray(at, at + entropyChunk));
    }
  };
  const exit = (status) => {
    throw new Exit(status);
  };
  const imports = {
    low: {
      read: (fd, ptr, len) => (host.read ? host.read(fd, view(ptr, len)) : noInput),
      write: (fd, ptr, len) => host.write(fd, view(ptr, len)),
      exit,
      halt: exit,
      clock: (id) => (id === monotonicClock ? host.monotonic() : host.wall()),
      entropy: (ptr, len) => entropy(view(ptr, len)),
      gc_stress: () => (host.stress ? stressWords : 0),
      page_limit: () => pageLimit,
    },
    m: math,
  };
  const module = await WebAssembly.compile(bytes);
  const threaded = WebAssembly.Module.imports(module).some((entry) => entry.module === "low" && entry.name === "switch");
  const report = (error) => {
    if (error instanceof Exit) {
      return error.status;
    }
    host.write(2, new TextEncoder().encode(`panic: ${trapMessage(error)}\n`));
    return panicStatus;
  };
  if (threaded && typeof WebAssembly.Suspending === "undefined") {
    return report(new Error(noPromiseIntegration));
  }
  const scheduler = threaded ? new Scheduler() : null;
  if (threaded) {
    imports.low.switch = new WebAssembly.Suspending((from, to) => scheduler.switch(from >>> 0, to >>> 0));
    imports.low.wait = new WebAssembly.Suspending((ms) => scheduler.wait(ms >>> 0));
  }
  const instance = await WebAssembly.instantiate(module, imports);
  memory = instance.exports.memory;
  if (host.maxPages !== undefined) {
    pageLimit = Math.min(addressablePages, (memory.buffer.byteLength >>> pageBits) + host.maxPages);
  }
  if (!threaded) {
    try {
      instance.exports._start();
      return 0;
    } catch (error) {
      return report(error);
    }
  }
  const start = WebAssembly.promising(instance.exports._start);
  const runThread = WebAssembly.promising(instance.exports.run_thread);
  return scheduler.run(start, runThread, report);
}

class Scheduler {
  constructor() {
    this.resumers = new Map();
    this.timeouts = new Set();
    this.settled = false;
    this.settle = null;
    this.runThread = null;
  }

  switch(from, to) {
    return new Promise((resolve) => {
      this.resumers.set(from, resolve);
      queueMicrotask(() => this.go(to));
    });
  }

  wait(ms) {
    return new Promise((resolve) => {
      const id = setTimeout(() => {
        this.timeouts.delete(id);
        resolve();
      }, ms);
      this.timeouts.add(id);
    });
  }

  go(thread) {
    if (this.settled) {
      return;
    }
    const resume = this.resumers.get(thread);
    if (resume !== undefined) {
      this.resumers.delete(thread);
      resume();
      return;
    }
    let running;
    try {
      running = this.runThread(BigInt(thread));
    } catch (error) {
      this.finish(error);
      return;
    }
    running.then(
      (next) => this.go(Number(BigInt.asUintN(32, next))),
      (error) => this.finish(error),
    );
  }

  finish(error) {
    if (this.settled) {
      return;
    }
    this.settled = true;
    for (const id of this.timeouts) {
      clearTimeout(id);
    }
    this.timeouts.clear();
    this.settle(error);
  }

  run(start, runThread, report) {
    this.runThread = runThread;
    return new Promise((resolve) => {
      this.settle = (error) => resolve(error === null ? 0 : report(error));
      start().then(
        () => this.finish(null),
        (error) => this.finish(error),
      );
    });
  }
}

const workerStackMb = 256;

async function runFile(path) {
  const fs = await import("node:fs");
  const crypto = await import("node:crypto");
  const bytes = fs.readFileSync(path);
  const pause = new Int32Array(new SharedArrayBuffer(4));
  const limit = process.env.LOW_WASM_MAX_PAGES;
  return run(bytes, {
    stress: process.env.LOW_GC === "stress",
    maxPages: limit === undefined ? undefined : Number(limit),
    read: (fd, data) => {
      for (;;) {
        try {
          return fs.readSync(fd, data, 0, data.length, null);
        } catch (error) {
          if (error.code === "EOF") {
            return 0;
          }
          if (error.code !== "EAGAIN") {
            return noInput;
          }
          Atomics.wait(pause, 0, 0, 1);
        }
      }
    },
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
  if (typeof WebAssembly.Suspending === "undefined") {
    (await import("node:v8")).setFlagsFromString("--experimental-wasm-jspi");
  }
  const worker = new Worker(new URL(import.meta.url), {
    workerData: process.argv[2],
    resourceLimits: { stackSizeMb: workerStackMb },
  });
  let reported = false;
  worker.on("message", (status) => {
    reported = true;
    process.exitCode = status;
  });
  worker.on("error", (error) => {
    reported = true;
    process.stderr.write(`panic: ${trapMessage(error)}\n`);
    process.exitCode = panicStatus;
  });
  worker.on("exit", () => {
    if (!reported) {
      process.stderr.write(`panic: ${lostStatus}\n`);
      process.exitCode = panicStatus;
    }
  });
}

if (typeof process !== "undefined" && process.argv[1] && import.meta.url.endsWith(process.argv[1].split("/").pop())) {
  await main();
}
