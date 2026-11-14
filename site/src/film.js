const still = matchMedia("(prefers-reduced-motion: reduce)");

class Film {
    #video;
    #visible = false;

    constructor(video) {
        this.#video = video;
        const watcher = new IntersectionObserver((entries) => this.#seen(entries), { threshold: 0.4 });
        watcher.observe(video);
        still.addEventListener("change", () => this.#update());
        this.#update();
    }

    #seen(entries) {
        this.#visible = entries.at(-1).isIntersecting;
        this.#update();
    }

    #update() {
        this.#video.controls = still.matches;
        if (still.matches || !this.#visible) {
            this.#video.pause();
            return;
        }
        this.#video.play().catch(() => {
            this.#video.controls = true;
        });
    }
}

for (const video of document.querySelectorAll("video.film")) {
    new Film(video);
}
