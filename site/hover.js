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

function hide() {
    tip.hidden = true;
}

document.addEventListener("pointerover", (event) => {
    const target = event.target.closest("[data-hover]");
    if (target) {
        show(target);
    } else {
        hide();
    }
});
addEventListener("scroll", hide, { passive: true });
