const dialog = document.querySelector(".palette");
const input = dialog.querySelector("input");
const status = dialog.querySelector(".palette-status");
const list = dialog.querySelector("#palette-results");
const opener = document.querySelector(".find");
const groupsShown = 12;
const itemsShown = 4;
const reach = 40;
const boundary = /[^a-z0-9]/;

function quality(field, term) {
    let best = 0;
    let count = 0;
    for (let at = field.indexOf(term); at >= 0 && count < 20; at = field.indexOf(term, at + term.length)) {
        count += 1;
        const start = at === 0 || boundary.test(field[at - 1]);
        const end = at + term.length === field.length || boundary.test(field[at + term.length]);
        best = Math.max(best, start && end ? 1 : start ? 0.8 : 0.5);
    }
    return { best, count };
}

function phrase(name, query) {
    const lower = name.toLowerCase();
    const last = lower.split(/[./]/).at(-1);
    if (lower === query || last === query) {
        return 40;
    }
    return lower.startsWith(query) || last.startsWith(query) ? 15 : 0;
}

class Field {
    constructor(text, weight) {
        this.text = text.toLowerCase();
        this.weight = weight;
    }
}

class Searchable {
    #fields;
    #body;

    constructor(fields, body) {
        this.#fields = fields;
        this.#body = body.toLowerCase();
    }

    match(terms) {
        let total = 0;
        for (const term of terms) {
            let best = 0;
            for (const field of this.#fields) {
                best = Math.max(best, field.weight * quality(field.text, term).best);
            }
            const body = quality(this.#body, term);
            const fromBody = body.count === 0 ? 0 : body.best + Math.min(body.count - 1, 4) * 0.25;
            const score = Math.max(best, fromBody);
            if (score === 0) {
                return 0;
            }
            total += score;
        }
        return total;
    }
}

class Page {
    #search;

    constructor([title, area, href, description, text], base) {
        this.title = title;
        this.area = area;
        this.href = new URL(href, base).href;
        this.description = description;
        this.text = text;
        this.#search = new Searchable([new Field(title, 10), new Field(description, 5)], text);
    }

    score(query, terms) {
        const score = this.#search.match(terms);
        return score === 0 ? 0 : score + phrase(this.title, query);
    }
}

class Item {
    #search;

    constructor([page, kind, name, href, signature, text], pages, base) {
        this.page = pages[page];
        this.kind = kind;
        this.name = name;
        this.href = new URL(href, base).href;
        this.signature = signature;
        this.text = text;
        this.#search = new Searchable([new Field(name, kind === "section" ? 12 : 10), new Field(signature, 3), new Field(this.page.title, 2)], text);
    }

    score(query, terms) {
        const score = this.#search.match(terms);
        return score === 0 ? 0 : score + phrase(this.name, query);
    }
}

class Group {
    constructor(page) {
        this.page = page;
        this.pageScore = 0;
        this.items = [];
    }

    get rank() {
        const best = Math.max(this.pageScore, this.items[0]?.score ?? 0);
        return best + Math.min(this.items.length, 4) * 0.25;
    }
}

function excerpt(text, terms) {
    const lower = text.toLowerCase();
    const positions = terms.map((term) => lower.indexOf(term)).filter((position) => position >= 0);
    const first = positions.length === 0 ? 0 : Math.min(...positions);
    const start = first > reach ? text.indexOf(" ", first - reach) + 1 : 0;
    const shown = text.slice(start, start + reach * 6);
    return start > 0 ? `…${shown}` : shown;
}

function highlighted(text, terms, className) {
    const escaped = terms.map((term) => term.replace(/[.*+?^${}()|[\]\\]/g, "\\$&"));
    const pattern = new RegExp(`(${escaped.join("|")})`, "gi");
    const nodes = text.split(pattern).map((part, position) => {
        if (position % 2 === 0) {
            return document.createTextNode(part);
        }
        const mark = document.createElement("mark");
        mark.textContent = part;
        return mark;
    });
    const span = document.createElement("span");
    span.className = className;
    span.append(...nodes);
    return span;
}

function plainSpan(text, className) {
    const span = document.createElement("span");
    span.className = className;
    span.textContent = text;
    return span;
}

function option(href, className, parts) {
    const link = document.createElement("a");
    link.href = href;
    link.tabIndex = -1;
    link.append(...parts);
    const entry = document.createElement("li");
    entry.className = className;
    entry.setAttribute("role", "option");
    entry.setAttribute("aria-selected", "false");
    entry.append(link);
    return entry;
}

function header(group, terms) {
    const { page, items } = group;
    const head = document.createElement("span");
    head.className = "palette-head";
    head.append(highlighted(page.title, terms, "palette-title"));
    if (page.area !== page.title) {
        head.append(plainSpan(page.area, "palette-area"));
    }
    if (items.length > itemsShown) {
        head.append(plainSpan(`${items.length - itemsShown} more`, "palette-more"));
    }
    const parts = [head];
    const summary = page.description || page.text;
    if (summary !== "") {
        const shown = excerpt(summary, terms);
        parts.push(highlighted(shown, terms, "palette-excerpt"));
    }
    return option(page.href, "palette-page", parts);
}

function row(item, terms) {
    const head = document.createElement("span");
    head.className = "palette-head";
    if (item.kind !== "section") {
        head.append(plainSpan(item.kind, "palette-kind"));
    }
    head.append(highlighted(item.name, terms, "palette-name"));
    const parts = [head];
    if (item.signature !== "") {
        parts.push(plainSpan(item.signature, "palette-signature"));
    }
    if (item.text !== "") {
        const shown = excerpt(item.text, terms);
        parts.push(highlighted(shown, terms, "palette-excerpt"));
    }
    const entry = option(item.href, "palette-item", parts);
    entry.dataset.kind = item.kind;
    return entry;
}

class Palette {
    #pages;
    #items;
    #loading;
    #options = [];
    #active = -1;

    open() {
        if (!dialog.open) {
            dialog.showModal();
        }
        input.select();
        this.prefetch();
    }

    prefetch() {
        this.#loading ??= this.#load();
    }

    toggle() {
        if (dialog.open) {
            dialog.close();
        } else {
            this.open();
        }
    }

    async #load() {
        try {
            const response = await fetch(dialog.dataset.index);
            if (!response.ok) {
                throw new Error(`${response.status}`);
            }
            const { pages, sections } = await response.json();
            this.#pages = pages.map((page) => new Page(page, response.url));
            this.#items = sections.map((section) => new Item(section, this.#pages, response.url));
            this.render();
        } catch {
            status.textContent = "The search index failed to load.";
            this.#loading = undefined;
        }
    }

    #groups(query, terms) {
        const groups = new Map();
        const groupOf = (page) => {
            if (!groups.has(page)) {
                groups.set(page, new Group(page));
            }
            return groups.get(page);
        };
        for (const page of this.#pages) {
            const score = page.score(query, terms);
            if (score > 0) {
                groupOf(page).pageScore = score;
            }
        }
        for (const item of this.#items) {
            const score = item.score(query, terms);
            if (score > 0) {
                groupOf(item.page).items.push({ item, score });
            }
        }
        const found = [...groups.values()];
        for (const group of found) {
            group.items.sort((a, b) => b.score - a.score || a.item.name.length - b.item.name.length);
        }
        return found.sort((a, b) => b.rank - a.rank || a.page.title.length - b.page.title.length);
    }

    #summarise(groups) {
        const pages = `${groups.length} ${groups.length === 1 ? "page" : "pages"}`;
        return groups.length > groupsShown ? `${pages}, showing the first ${groupsShown}` : pages;
    }

    render() {
        if (!this.#pages) {
            return;
        }
        const query = input.value.trim().toLowerCase().replace(/\s+/g, " ");
        const terms = [...new Set(query.split(" ").filter(Boolean))];
        const groups = terms.length === 0 ? [] : this.#groups(query, terms);
        this.#options = [];
        for (const group of groups.slice(0, groupsShown)) {
            this.#options.push(header(group, terms));
            const rows = group.items.slice(0, itemsShown).map(({ item }) => row(item, terms));
            this.#options.push(...rows);
        }
        this.#options.forEach((entry, position) => (entry.id = `palette-option-${position}`));
        list.replaceChildren(...this.#options);
        list.scrollTop = 0;
        if (terms.length === 0) {
            status.textContent = "";
        } else if (groups.length === 0) {
            status.textContent = `No results for “${input.value.trim()}”`;
        } else {
            status.textContent = this.#summarise(groups);
        }
        this.select(this.#options.length > 0 ? 0 : -1);
    }

    select(position) {
        this.#options[this.#active]?.setAttribute("aria-selected", "false");
        this.#active = position;
        const chosen = this.#options[position];
        if (chosen) {
            chosen.setAttribute("aria-selected", "true");
            chosen.scrollIntoView({ block: "nearest" });
            input.setAttribute("aria-activedescendant", chosen.id);
        } else {
            input.removeAttribute("aria-activedescendant");
        }
    }

    move(step) {
        if (this.#options.length > 0) {
            const next = (this.#active + step + this.#options.length) % this.#options.length;
            this.select(next);
        }
    }

    choose() {
        const link = this.#options[this.#active]?.querySelector("a");
        if (link) {
            dialog.close();
            location.assign(link.href);
        }
    }

    pointAt(target) {
        const entry = target.closest("[role=option]");
        const position = this.#options.indexOf(entry);
        if (position >= 0 && position !== this.#active) {
            this.select(position);
        }
    }
}

const palette = new Palette();
const keys = { ArrowDown: () => palette.move(1), ArrowUp: () => palette.move(-1), Enter: () => palette.choose() };

if (/Mac|iPhone|iPad/.test(navigator.platform)) {
    opener.querySelector("kbd").textContent = "⌘ K";
}

opener.addEventListener("click", () => palette.open());
opener.addEventListener("pointerenter", () => palette.prefetch());
opener.addEventListener("focus", () => palette.prefetch());
input.addEventListener("input", () => palette.render());
input.addEventListener("keydown", (event) => {
    const action = keys[event.key];
    if (action && !event.isComposing) {
        event.preventDefault();
        action();
    }
});
list.addEventListener("pointermove", (event) => palette.pointAt(event.target));
list.addEventListener("click", (event) => {
    if (event.target.closest("a")) {
        dialog.close();
    }
});
dialog.addEventListener("click", (event) => {
    if (event.target === dialog) {
        dialog.close();
    }
});
document.addEventListener("keydown", (event) => {
    if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === "k") {
        event.preventDefault();
        palette.toggle();
    } else if (event.key === "/" && !dialog.open && !event.target.closest("input, textarea, select, [contenteditable]")) {
        event.preventDefault();
        palette.open();
    }
});
