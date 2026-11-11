const gap = 6;
const margin = 8;
const tip = document.createElement("div");
tip.className = "hover";
tip.setAttribute("role", "tooltip");
tip.hidden = true;
document.body.append(tip);

function scheme(target) {
    const block = target.closest("pre");
    const background = getComputedStyle(block).getPropertyValue("--bg").trim();
    return background === getComputedStyle(document.documentElement).getPropertyValue("--color-ink").trim() ? "on-paper" : "on-ink";
}

function show(target) {
    tip.innerHTML = target.dataset.hover;
    tip.classList.remove("on-paper", "on-ink");
    tip.classList.add(scheme(target));
    tip.hidden = false;
    const box = target.getBoundingClientRect();
    const above = box.top - tip.offsetHeight - gap;
    const top = above < margin ? box.bottom + gap : above;
    const left = Math.max(margin, Math.min(box.left, innerWidth - tip.offsetWidth - margin));
    tip.style.top = `${top}px`;
    tip.style.left = `${left}px`;
}

let traced;

function untrace() {
    traced?.classList.remove("tracing");
    for (const node of traced?.querySelectorAll(".traced") ?? []) {
        node.classList.remove("traced");
    }
    traced = undefined;
}

function trace(target) {
    untrace();
    if (target) {
        traced = target.closest("pre");
        traced.classList.add("tracing");
        for (const node of traced.querySelectorAll(`[data-ref="${target.dataset.ref}"]`)) {
            node.classList.add("traced");
        }
    }
}

function hide() {
    tip.hidden = true;
}

document.addEventListener("pointerover", (event) => {
    trace(event.target.closest("pre [data-ref]"));
    const target = event.target.closest("[data-hover]");
    if (target) {
        show(target);
    } else {
        hide();
    }
});
addEventListener("scroll", hide, { passive: true });
