import { readFile } from "node:fs/promises";
import { Marked } from "marked";

const repository = new URL("../", import.meta.url);

export async function lexed(path) {
    const text = await readFile(new URL(path, repository), "utf8");
    return new Marked().lexer(text);
}
