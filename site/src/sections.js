const links = new Map();
for (const link of document.querySelectorAll(".sublist a, .toc a")) {
    const id = decodeURIComponent(link.hash.slice(1));
    links.set(id, [...(links.get(id) ?? []), link]);
}
const headings = [...document.querySelectorAll(".prose h2[id]")].filter((heading) => links.has(heading.id));

function mark() {
    const line = innerHeight / 4;
    const passed = headings.filter((heading) => heading.getBoundingClientRect().top <= line);
    const current = passed.at(-1) ?? headings[0];
    for (const [id, group] of links) {
        for (const link of group) {
            link.toggleAttribute("aria-current", id === current?.id);
        }
    }
    links.get(current?.id)?.[0].scrollIntoView({ block: "nearest" });
}

let queued = false;
addEventListener(
    "scroll",
    () => {
        if (queued) {
            return;
        }
        queued = true;
        requestAnimationFrame(() => {
            queued = false;
            mark();
        });
    },
    { passive: true },
);
mark();
