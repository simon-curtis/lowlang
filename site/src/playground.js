import { Editor } from "./editor.js";
import { Highlighter } from "./highlight.js";
import { Keys } from "./keys.js";
import { Program } from "./program.js";
import { bind } from "./run.js";

const copiedFor = 2500;
const sourceLocation = /main\.low:(\d+):(\d+)/g;

const editor = document.getElementById("source");
const run = document.querySelector("[data-run]");
const stop = document.querySelector("[data-stop]");
const share = document.querySelector("[data-share]");
const status = document.querySelector(".run-status");
const notice = document.querySelector(".share-status");
const output = document.querySelector(".run-output");
const keyBar = document.querySelector(".key-bar");
const coarse = matchMedia("(pointer: coarse)");
const editing = new Editor(editor, () => coarse.matches);
const program = new Program(editor, editor.value);
const highlighter = new Highlighter(editor, document.querySelector(".playground-paint"));
let noticeTimer;

function announce(message) {
    clearTimeout(noticeTimer);
    notice.textContent = message;
    noticeTimer = setTimeout(() => {
        notice.textContent = "";
    }, copiedFor);
}

async function restore() {
    const restored = await program.restore();
    editing.reset();
    highlighter.paint();
    if (!restored) {
        announce("The link does not hold a program.");
    }
}

async function publish() {
    try {
        await program.share();
    } catch {
        announce("Sharing needs a newer browser.");
        return;
    }
    const url = location.href;
    if (coarse.matches && typeof navigator.share === "function") {
        try {
            await navigator.share({ url });
            return;
        } catch (error) {
            if (error.name === "AbortError") {
                return;
            }
        }
    }
    try {
        await navigator.clipboard.writeText(url);
        announce("Link copied.");
    } catch {
        announce("The link is in the address bar.");
    }
}

function locationButton([text, line, column]) {
    const button = document.createElement("button");
    button.type = "button";
    button.className = "run-location";
    button.dataset.line = line;
    button.dataset.column = column;
    button.ariaLabel = `Go to line ${line}, column ${column}`;
    button.textContent = text;
    return button;
}

function showOutput() {
    if (!output.hidden) {
        output.scrollIntoView({ block: "nearest" });
    }
}

function finished(printer) {
    printer.link(sourceLocation, locationButton);
    showOutput();
}

bind({ run, stop, status, output, source: () => editor.value, onidle: finished });
new Keys(keyBar, editor, editing, { run, coarse });

editor.addEventListener("keydown", (event) => {
    if (event.key === "Enter" && (event.ctrlKey || event.metaKey)) {
        event.preventDefault();
        run.click();
    }
});
editor.addEventListener("input", () => program.edited());
run.addEventListener("click", () => {
    if (coarse.matches) {
        editor.blur();
    }
    requestAnimationFrame(showOutput);
});
output.addEventListener("click", (event) => {
    const target = event.target.closest(".run-location");
    if (target !== null) {
        editing.goTo(Number(target.dataset.line), Number(target.dataset.column));
    }
});
share.addEventListener("click", publish);
addEventListener("hashchange", restore);
addEventListener("pagehide", () => program.flush());
document.addEventListener("visibilitychange", () => {
    if (document.visibilityState === "hidden") {
        program.flush();
    }
});
await restore();
