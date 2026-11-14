import { bind } from "./run.js";

const indent = "    ";
const copiedFor = 2500;

const editor = document.getElementById("source");
const run = document.querySelector("[data-run]");
const stop = document.querySelector("[data-stop]");
const share = document.querySelector("[data-share]");
const status = document.querySelector(".run-status");
const notice = document.querySelector(".share-status");
const output = document.querySelector(".run-output");
const example = editor.value;
let noticeTimer;

function toBase64Url(bytes) {
    let binary = "";
    for (let at = 0; at < bytes.length; at += 8192) {
        const slice = bytes.subarray(at, at + 8192);
        binary += String.fromCharCode(...slice);
    }
    const encoded = btoa(binary);
    return encoded.replaceAll("+", "-").replaceAll("/", "_").replace(/=+$/, "");
}

function fromBase64Url(text) {
    const standard = text.replaceAll("-", "+").replaceAll("_", "/");
    const padded = standard.padEnd(Math.ceil(standard.length / 4) * 4, "=");
    const binary = atob(padded);
    return Uint8Array.from(binary, (character) => character.charCodeAt(0));
}

async function transform(bytes, stream) {
    const source = new Blob([bytes]).stream();
    const transformed = source.pipeThrough(stream);
    const buffer = await new Response(transformed).arrayBuffer();
    return new Uint8Array(buffer);
}

async function encode(text) {
    const encoder = new TextEncoder();
    const bytes = encoder.encode(text);
    const compression = new CompressionStream("deflate-raw");
    const compressed = await transform(bytes, compression);
    return toBase64Url(compressed);
}

async function decode(fragment) {
    const compressed = fromBase64Url(fragment);
    const decompression = new DecompressionStream("deflate-raw");
    const bytes = await transform(compressed, decompression);
    const decoder = new TextDecoder();
    return decoder.decode(bytes);
}

function announce(message) {
    clearTimeout(noticeTimer);
    notice.textContent = message;
    noticeTimer = setTimeout(() => {
        notice.textContent = "";
    }, copiedFor);
}

async function restore() {
    const fragment = location.hash.slice(1);
    if (fragment === "") {
        editor.value = example;
        return;
    }
    try {
        editor.value = await decode(fragment);
    } catch {
        editor.value = example;
        announce("The link does not hold a program.");
    }
}

async function publish() {
    const fragment = await encode(editor.value);
    history.replaceState(null, "", `#${fragment}`);
    try {
        await navigator.clipboard.writeText(location.href);
        announce("Link copied.");
    } catch {
        announce("The link is in the address bar.");
    }
}

function insertIndent() {
    if (!document.execCommand("insertText", false, indent)) {
        editor.setRangeText(indent, editor.selectionStart, editor.selectionEnd, "end");
    }
}

bind({ run, stop, status, output, source: () => editor.value });

editor.addEventListener("keydown", (event) => {
    const plain = !event.ctrlKey && !event.metaKey && !event.altKey && !event.shiftKey;
    if (event.key === "Tab" && plain) {
        event.preventDefault();
        insertIndent();
    } else if (event.key === "Enter" && (event.ctrlKey || event.metaKey)) {
        event.preventDefault();
        run.click();
    }
});
share.addEventListener("click", publish);
addEventListener("hashchange", restore);
await restore();
