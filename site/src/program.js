const draftKey = "low.playground";
const pendingKey = "low.playground.link";
const saveAfter = 400;

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

function currentFragment() {
    return location.hash.slice(1);
}

function unlink() {
    history.replaceState(null, "", `${location.pathname}${location.search}`);
}

export class Program {
    #area;
    #example;
    #timer = 0;
    #linked = false;
    #fragment = "";
    #encoded = { text: null, fragment: "" };

    constructor(area, example) {
        this.#area = area;
        this.#example = example;
    }

    async restore() {
        const fragment = currentFragment();
        this.flush();
        this.#fragment = fragment;
        this.#linked = fragment !== "";
        if (!this.#linked) {
            this.#area.value = this.#readDraft() ?? this.#example;
            return true;
        }
        const pending = this.#readPending(fragment);
        if (pending !== null) {
            this.#area.value = pending;
            this.#relink();
            return true;
        }
        try {
            const text = await decode(fragment);
            if (this.#fragment === fragment) {
                this.#area.value = text;
                this.#encoded = { text, fragment };
            }
            return true;
        } catch {
            if (this.#fragment !== fragment) {
                return true;
            }
            this.#linked = false;
            this.#fragment = "";
            unlink();
            this.#area.value = this.#readDraft() ?? this.#example;
            return false;
        }
    }

    edited() {
        clearTimeout(this.#timer);
        this.#timer = setTimeout(() => this.flush(), saveAfter);
    }

    flush() {
        if (this.#timer === 0) {
            return;
        }
        clearTimeout(this.#timer);
        this.#timer = 0;
        if (this.#linked) {
            this.#savePending();
            this.#relink();
        } else {
            this.#saveDraft();
        }
    }

    async share() {
        const fragment = await this.#encode();
        this.#linked = true;
        this.#fragment = fragment;
        history.replaceState(null, "", `#${fragment}`);
    }

    async #encode() {
        const text = this.#area.value;
        if (this.#encoded.text !== text) {
            const fragment = await encode(text);
            this.#encoded = { text, fragment };
        }
        return this.#encoded.fragment;
    }

    async #relink() {
        const from = this.#fragment;
        try {
            const fragment = await this.#encode();
            if (this.#linked && this.#fragment === from && currentFragment() === from) {
                this.#fragment = fragment;
                history.replaceState(null, "", `#${fragment}`);
                this.#dropPending();
            }
        } catch {
            if (this.#fragment === from) {
                this.#linked = false;
                this.#fragment = "";
                unlink();
            }
        }
    }

    #readDraft() {
        try {
            return localStorage.getItem(draftKey);
        } catch {
            return null;
        }
    }

    #saveDraft() {
        const text = this.#area.value;
        try {
            if (text === this.#example) {
                localStorage.removeItem(draftKey);
            } else {
                localStorage.setItem(draftKey, text);
            }
        } catch {}
    }

    #readPending(fragment) {
        try {
            const saved = JSON.parse(sessionStorage.getItem(pendingKey) ?? "null");
            return saved?.fragment === fragment ? saved.text : null;
        } catch {
            return null;
        }
    }

    #dropPending() {
        try {
            sessionStorage.removeItem(pendingKey);
        } catch {}
    }

    #savePending() {
        const fragment = this.#fragment;
        const text = this.#area.value;
        try {
            sessionStorage.setItem(pendingKey, JSON.stringify({ fragment, text }));
        } catch {}
    }
}
