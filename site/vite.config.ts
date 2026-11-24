import { readdirSync } from "node:fs";
import { resolve } from "node:path";
import tailwindcss from "@tailwindcss/vite";
import { defineConfig } from "vite-plus";

const root = import.meta.dirname;

function pagesIn(folder: string, deep = false): string[] {
  const directory = resolve(root, folder);
  return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    if (entry.isDirectory()) {
      return deep ? pagesIn(`${folder}/${entry.name}`, true) : [];
    }
    return entry.name.endsWith(".html") ? [resolve(directory, entry.name)] : [];
  });
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
      input: [...pagesIn("."), ...pagesIn("docs"), ...pagesIn("packages", true), ...pagesIn("playground")],
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
        command: "low build compiler/playground --target browser/wasm32 -o site/public/playground.wasm",
        cwd: "..",
        cache: false,
      },
      highlight: {
        command: "node check/highlight.mjs",
        dependsOn: ["generate"],
        cache: false,
      },
      site: {
        command: "vp build",
        dependsOn: ["generate", "highlight", "playground"],
        cache: false,
      },
    },
  },
});
