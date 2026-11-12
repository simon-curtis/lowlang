const gap = 6;
const margin = 8;
const grace = 300;
const describable = "[data-hover], pre a[href]";
const tip = document.createElement("div");
tip.className = "hover";
tip.setAttribute("role", "tooltip");
tip.hidden = true;
document.body.append(tip);

let pointer = "mouse";
let shown;
let confirmed;
let traced;
let leaving;

function scheme(target) {
    const block = target.closest("pre");
    const background = getComputedStyle(block).getPropertyValue("--bg").trim();
    return background === getComputedStyle(document.documentElement).getPropertyValue("--color-ink").trim() ? "on-paper" : "on-ink";
}

function show(target) {
    clearTimeout(leaving);
    shown = target;
    tip.innerHTML = target.dataset.hover ?? "";
    if (target.matches("a[href]")) {
        const jump = document.createElement("a");
        jump.className = "jump";
        jump.href = target.href;
        jump.textContent = "Go to definition";
        tip.append(jump);
    }
    tip.classList.remove("on-paper", "on-ink");
    tip.classList.add(scheme(target));
    const box = target.getBoundingClientRect();
    const roomAbove = box.top - gap - margin;
    const roomBelow = innerHeight - box.bottom - gap - margin;
    tip.style.maxHeight = `${Math.floor(Math.max(roomAbove, roomBelow))}px`;
    tip.hidden = false;
    tip.classList.toggle("clipped", tip.scrollHeight > tip.clientHeight);
    const top = tip.offsetHeight <= roomAbove ? box.top - gap - tip.offsetHeight : box.bottom + gap;
    const left = Math.max(margin, Math.min(box.left, innerWidth - tip.offsetWidth - margin));
    tip.style.top = `${top}px`;
    tip.style.left = `${left}px`;
}

function hide() {
    clearTimeout(leaving);
    tip.hidden = true;
    shown = undefined;
    confirmed = undefined;
}

function settle(target) {
    if (target) {
        show(target);
    } else {
        hide();
    }
}

function move(target) {
    clearTimeout(leaving);
    if (pointer === "mouse" && !tip.hidden && tip.querySelector(".jump")) {
        leaving = setTimeout(() => settle(target), grace);
    } else {
        settle(target);
    }
}

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

function destination(link) {
    if (link.pathname !== location.pathname || !link.hash) {
        return undefined;
    }
    const node = document.getElementById(decodeURIComponent(link.hash.slice(1)));
    return node?.closest("pre") ? node : undefined;
}

function jump(link, node) {
    hide();
    untrace();
    history.pushState(null, "", link.hash);
    node.scrollIntoView({ block: "nearest", inline: "nearest" });
    node.classList.add("flash");
    node.addEventListener("animationend", () => node.classList.remove("flash"), { once: true });
}

document.addEventListener("pointerover", (event) => {
    if (tip.contains(event.target)) {
        clearTimeout(leaving);
        return;
    }
    pointer = event.pointerType;
    trace(event.target.closest("pre [data-ref]"));
    const target = event.target.closest(describable);
    if (target && target === shown) {
        clearTimeout(leaving);
        confirmed = target;
    } else {
        move(target);
    }
});
document.addEventListener("focusin", (event) => {
    const target = event.target.closest(describable);
    if (target) {
        show(target);
    }
});
document.addEventListener("focusout", (event) => {
    if (!tip.contains(event.relatedTarget)) {
        hide();
    }
});
document.addEventListener(
    "click",
    (event) => {
        const link = event.target.closest("pre a[href], .hover a[href]");
        if (!link) {
            return;
        }
        if (pointer !== "mouse" && link.closest("pre") && confirmed !== link) {
            event.preventDefault();
            event.stopPropagation();
            return;
        }
        const node = destination(link);
        if (node) {
            event.preventDefault();
            event.stopPropagation();
            jump(link, node);
        }
    },
    { capture: true },
);
addEventListener(
    "scroll",
    () => {
        hide();
        untrace();
    },
    { passive: true, capture: true },
);
