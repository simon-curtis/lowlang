export class Keys {
    #bar;
    #area;
    #editing;
    #run;
    #coarse;
    #view;
    #frame = 0;
    #refit = false;
    #repeat = { timer: 0, fired: false };
    #repeated = false;

    constructor(bar, area, editing, { run, coarse }) {
        this.#bar = bar;
        this.#area = area;
        this.#editing = editing;
        this.#run = run;
        this.#coarse = coarse;
        this.#view = window.visualViewport ?? null;
        bar.addEventListener("pointerdown", (event) => this.#down(event));
        bar.addEventListener("mousedown", (event) => event.preventDefault());
        bar.addEventListener("click", (event) => this.#press(event));
        bar.addEventListener("focusout", (event) => this.#leave(event));
        for (const type of ["pointerup", "pointercancel"]) {
            bar.addEventListener(type, () => this.#release());
        }
        bar.querySelector(".key-strip").addEventListener("scroll", () => this.#release(), { passive: true });
        area.addEventListener("focus", () => this.#show());
        area.addEventListener("blur", (event) => this.#leave(event));
        this.#view?.addEventListener("resize", () => this.#schedule(true));
        this.#view?.addEventListener("scroll", () => this.#schedule(false));
        addEventListener("scroll", () => this.#schedule(false), { passive: true });
        const runKey = bar.querySelector('[data-action="run"]');
        const mirror = new MutationObserver(() => {
            runKey.disabled = run.disabled;
        });
        mirror.observe(run, { attributes: true, attributeFilter: ["disabled"] });
    }

    #down(event) {
        event.preventDefault();
        this.#release();
        this.#repeated = false;
        const key = event.target.closest("[data-repeat]");
        if (key === null) {
            return;
        }
        const { insert, action } = key.dataset;
        const step = () => {
            this.#repeat.fired = true;
            this.#apply(insert, action);
        };
        this.#repeat.timer = setTimeout(() => {
            this.#repeat.timer = setInterval(step, 60);
            step();
        }, 400);
    }

    #release() {
        clearTimeout(this.#repeat.timer);
        clearInterval(this.#repeat.timer);
        this.#repeated = this.#repeat.fired;
        this.#repeat = { timer: 0, fired: false };
    }

    #press(event) {
        const key = event.target.closest("button");
        if (key === null) {
            return;
        }
        if (this.#repeated) {
            this.#repeated = false;
            return;
        }
        const { insert, action } = key.dataset;
        if (action === "run") {
            this.#run.click();
            return;
        }
        if (action === "done") {
            this.#area.blur();
            this.#hide();
            return;
        }
        this.#area.focus({ preventScroll: true });
        this.#apply(insert, action);
    }

    #apply(insert, action) {
        const editing = this.#editing;
        if (insert !== undefined) {
            editing.type(insert);
        } else if (action === "indent") {
            editing.indent();
        } else if (action === "left") {
            editing.move(-1);
        } else if (action === "right") {
            editing.move(1);
        } else if (action === "up") {
            editing.moveLine(-1);
        } else if (action === "down") {
            editing.moveLine(1);
        } else if (action === "undo") {
            editing.undo();
        }
    }

    #leave(event) {
        const to = event.relatedTarget;
        if (to === this.#area || this.#bar.contains(to)) {
            return;
        }
        this.#hide();
    }

    #show() {
        if (!this.#coarse.matches || this.#view === null) {
            return;
        }
        const showing = this.#bar.hidden;
        this.#bar.hidden = false;
        this.#fit(showing);
    }

    #hide() {
        this.#release();
        cancelAnimationFrame(this.#frame);
        this.#frame = 0;
        this.#refit = false;
        this.#bar.hidden = true;
        this.#area.style.removeProperty("max-height");
        this.#area.style.removeProperty("min-height");
        document.body.style.removeProperty("padding-bottom");
    }

    #schedule(refit) {
        if (this.#bar.hidden) {
            return;
        }
        this.#refit ||= refit;
        if (this.#frame !== 0) {
            return;
        }
        this.#frame = requestAnimationFrame(() => {
            this.#frame = 0;
            const fit = this.#refit;
            this.#refit = false;
            if (fit) {
                this.#fit(false);
            } else {
                this.#place();
            }
        });
    }

    #barTop() {
        const view = this.#view;
        return view.offsetTop + view.height - this.#bar.offsetHeight / view.scale;
    }

    #place() {
        const view = this.#view;
        const transform = `translate(${view.offsetLeft}px, ${this.#barTop()}px) scale(${1 / view.scale})`;
        if (this.#bar.style.transform !== transform) {
            this.#bar.style.transform = transform;
        }
    }

    #fit(showing) {
        const view = this.#view;
        if (view.scale !== 1) {
            this.#place();
            return;
        }
        const follow = showing || this.#editing.caretVisible();
        const area = this.#area;
        area.style.minHeight = "0";
        area.style.maxHeight = `${view.height - this.#bar.offsetHeight}px`;
        const covered = Math.max(innerHeight - view.offsetTop - view.height, 0);
        document.body.style.paddingBottom = `${covered + this.#bar.offsetHeight}px`;
        const top = this.#barTop();
        const box = area.getBoundingClientRect();
        if (box.bottom > top) {
            scrollBy(0, box.bottom - top);
        } else if (box.top < view.offsetTop) {
            scrollBy(0, box.top - view.offsetTop);
        }
        const placed = area.getBoundingClientRect();
        const bottom = this.#barTop();
        if (placed.bottom > bottom) {
            area.style.maxHeight = `${Math.max(bottom - placed.top, 0)}px`;
        }
        this.#place();
        if (follow) {
            this.#editing.reveal();
        }
    }
}
