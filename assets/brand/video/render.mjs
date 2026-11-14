import { execFile, spawn } from "node:child_process";
import { createReadStream, mkdtempSync, readFileSync, rmSync, statSync, writeFileSync } from "node:fs";
import { createServer } from "node:http";
import { tmpdir } from "node:os";
import { delimiter, extname, join, normalize, resolve, sep } from "node:path";
import { parseArgs, promisify } from "node:util";
import puppeteer from "puppeteer-core";

const here = import.meta.dirname;
const root = resolve(here, "../../..");
const browsers = ["chromium", "chromium-browser", "google-chrome-stable", "google-chrome"];
const types = {
  ".html": "text/html; charset=utf-8",
  ".js": "text/javascript; charset=utf-8",
  ".css": "text/css; charset=utf-8",
  ".svg": "image/svg+xml",
  ".woff2": "font/woff2",
};

function executable(name) {
  for (const directory of (process.env.PATH ?? "").split(delimiter)) {
    const candidate = join(directory, name);
    try {
      if (statSync(candidate).isFile()) {
        return candidate;
      }
    } catch {
      continue;
    }
  }
  return null;
}

function findBrowser() {
  if (process.env.CHROMIUM) {
    return process.env.CHROMIUM;
  }
  for (const name of browsers) {
    const found = executable(name);
    if (found) {
      return found;
    }
  }
  throw new Error(`no browser found: set CHROMIUM or install one of ${browsers.join(", ")}`);
}

function findLow() {
  const found = process.env.LOW ?? executable("low");
  if (!found) {
    throw new Error("no low compiler found: set LOW or put low on PATH");
  }
  return found;
}

async function lowOutput(low, args) {
  const run = promisify(execFile);
  try {
    const result = await run(low, args);
    return result.stdout;
  } catch (error) {
    const reason = error.stderr?.trim() || error.message;
    throw new Error(`${low} ${args[0]} failed: ${reason}`);
  }
}

async function compile(scratch) {
  const low = findLow();
  const program = join(here, "hello.low");
  const binary = join(scratch, "hello");
  const ir = await lowOutput(low, ["ir", program]);
  const assembly = await lowOutput(low, ["asm", program]);
  await lowOutput(low, ["build", program, "-o", binary]);
  const bytes = readFileSync(binary);
  const header = [...bytes.subarray(0, 48)];
  const source = readFileSync(program, "utf8");
  const compiled = { source, ir, assembly, header, size: bytes.length };
  return JSON.stringify(compiled);
}

function server(compiled) {
  return (request, response) => serve(request, response, compiled);
}

function serve(request, response, compiled) {
  const address = new URL(request.url, "http://local");
  const relative = decodeURIComponent(address.pathname);
  if (request.method === "GET" && relative === "/compile.json") {
    response.writeHead(200, { "Content-Type": "application/json" }).end(compiled);
    return;
  }
  const path = normalize(join(root, relative));
  if (request.method !== "GET" || !path.startsWith(`${root}${sep}`)) {
    response.writeHead(404).end();
    return;
  }
  const stream = createReadStream(path);
  const headers = { "Content-Type": types[extname(path)] ?? "application/octet-stream" };
  stream.on("open", () => {
    response.writeHead(200, headers);
    stream.pipe(response);
  });
  stream.on("error", () => response.writeHead(404).end());
}

function listen(server) {
  return new Promise((done, fail) => {
    server.once("error", fail);
    server.listen(0, "127.0.0.1", () => done(server.address().port));
  });
}

function encoder(output, settings) {
  const filters = [
    `scale=${settings.width}:${settings.height}:flags=lanczos:out_color_matrix=bt709:out_range=tv`,
    "format=yuv420p",
    "setparams=color_primaries=bt709:color_trc=bt709:colorspace=bt709:range=tv",
  ];
  const args = [
    "-hide_banner", "-loglevel", "error", "-y",
    "-f", "image2pipe", "-c:v", "png", "-framerate", String(settings.fps), "-i", "-",
    "-vf", filters.join(","),
    "-c:v", "libx264", "-preset", "veryslow", "-crf", String(settings.crf), "-tune", "animation",
    "-profile:v", "high", "-level", "4.0",
    "-movflags", "+faststart", "-an",
    output,
  ];
  const ffmpeg = spawn("ffmpeg", args, { stdio: ["pipe", "inherit", "inherit"] });
  ffmpeg.stdin.on("error", () => {});
  const finished = new Promise((done, fail) => {
    ffmpeg.once("error", (error) => fail(new Error(`ffmpeg failed to start: ${error.message}`)));
    ffmpeg.once("exit", (code) => code === 0 ? done() : fail(new Error(`ffmpeg exited with ${code}`)));
  });
  finished.catch(() => {});
  return { process: ffmpeg, finished };
}

function write(stream, chunk) {
  return new Promise((done, fail) => {
    stream.write(chunk, (error) => error ? fail(error) : done());
  });
}

async function capture(page, seconds) {
  const url = await page.evaluate((time) => window.scene.frame(time), seconds);
  const encoded = url.slice(url.indexOf(",") + 1);
  return Buffer.from(encoded, "base64");
}

async function renderVideo(page, settings, output, cleanup) {
  const video = encoder(output, settings);
  cleanup.push(() => video.process.kill("SIGKILL"));
  const total = Math.round(settings.duration * settings.fps);
  for (let index = 0; index < total; index++) {
    const frame = await capture(page, index / settings.fps);
    try {
      await write(video.process.stdin, frame);
    } catch {
      await video.finished;
      throw new Error("ffmpeg closed its input");
    }
    process.stdout.write(`\rframe ${index + 1}/${total}`);
  }
  process.stdout.write("\n");
  video.process.stdin.end();
  await video.finished;
}

async function renderStills(page, times, output) {
  const stem = output.replace(/\.png$/, "");
  for (const seconds of times) {
    const frame = await capture(page, seconds);
    const path = times.length === 1 ? output : `${stem}-${seconds.toFixed(2)}.png`;
    writeFileSync(path, frame);
  }
}

async function main() {
  const { values, positionals } = parseArgs({
    allowPositionals: true,
    options: {
      draft: { type: "boolean", default: false },
      still: { type: "string" },
      crf: { type: "string", default: "26" },
    },
  });
  if (values.still && !positionals[0]) {
    throw new Error("--still needs an output path");
  }
  const target = positionals[0] ?? join(root, "site/public/og-video.mp4");
  const output = resolve(target);
  if (!values.still && !executable("ffmpeg")) {
    throw new Error("ffmpeg is not on PATH");
  }
  const cleanup = [];
  const stop = () => {
    for (const step of cleanup.reverse()) {
      step();
    }
  };
  process.once("SIGINT", () => {
    stop();
    process.exit(130);
  });
  try {
    const prefix = join(tmpdir(), "low-video-");
    const scratch = mkdtempSync(prefix);
    cleanup.push(() => rmSync(scratch, { recursive: true, force: true }));
    const compiled = await compile(scratch);
    const handler = server(compiled);
    const files = createServer(handler);
    const port = await listen(files);
    cleanup.push(() => {
      files.closeAllConnections();
      files.close();
    });
    const options = {
      executablePath: findBrowser(),
      headless: true,
      userDataDir: join(scratch, "profile"),
      env: { ...process.env, TMPDIR: scratch },
      args: ["--use-angle=swiftshader", "--enable-unsafe-swiftshader", "--force-color-profile=srgb", "--hide-scrollbars"],
    };
    const browser = await puppeteer.launch(options);
    cleanup.push(() => browser.process()?.kill("SIGKILL"));
    const page = await browser.newPage();
    page.on("pageerror", (error) => console.error(`page: ${error.message}`));
    page.on("console", (message) => console.error(`page: ${message.text()}`));
    const scale = values.draft ? 1 : 2;
    const address = `http://127.0.0.1:${port}/assets/brand/video/scene.html?scale=${scale}`;
    await page.goto(address);
    const settings = await page.evaluate(async () => {
      await window.scene.ready;
      return { width: window.scene.width, height: window.scene.height, fps: window.scene.fps, duration: window.scene.duration };
    });
    settings.fps = values.draft ? 15 : settings.fps;
    settings.crf = Number(values.crf);
    if (values.still) {
      const times = values.still.split(",").map(Number);
      await renderStills(page, times, output);
    } else {
      await renderVideo(page, settings, output, cleanup);
    }
    await browser.close();
    stop();
  } catch (error) {
    stop();
    throw error;
  }
}

main().catch((error) => {
  console.error(error.message);
  process.exit(1);
});
