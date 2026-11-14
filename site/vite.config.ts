import { readdirSync } from "node:fs";
import { resolve } from "node:path";
import tailwindcss from "@tailwindcss/vite";
import { defineConfig } from "vite-plus";

const root = import.meta.dirname;

function pagesIn(folder: string): string[] {
  const directory = resolve(root, folder);
  return readdirSync(directory)
    .filter((name) => name.endsWith(".html"))
    .map((name) => resolve(directory, name));
}

export default defineConfig({
  root,
  publicDir: "public",
  plugins: [tailwindcss()],
  server: {
    fs: {
      allow: [resolve(root, "..")],
    },
  },
  worker: {
    format: "es",
    rollupOptions: {
      external: [/^node:/],
    },
  },
  build: {
    outDir: "dist",
    emptyOutDir: true,
    rollupOptions: {
      input: [".", "docs", "packages", "playground"].flatMap(pagesIn),
    },
  },
  run: {
    tasks: {
      generate: {
        command: "low run site/gen",
        cwd: "..",
        cache: false,
      },
      playground: {
        command: "low build compiler/playground -target browser/wasm32 -o site/public/playground.wasm",
        cwd: "..",
        cache: false,
      },
      site: {
        command: "vp build",
        dependsOn: ["generate", "playground"],
        cache: false,
      },
    },
  },
});
