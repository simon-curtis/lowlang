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
  build: {
    outDir: "dist",
    emptyOutDir: true,
    rollupOptions: {
      input: [".", "docs", "packages"].flatMap(pagesIn),
    },
  },
  run: {
    tasks: {
      generate: {
        command: "low run site/gen",
        cwd: "..",
        cache: false,
      },
      site: {
        command: "vp build",
        dependsOn: ["generate"],
        cache: false,
      },
    },
  },
});
