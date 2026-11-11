if (!matchMedia("(prefers-reduced-motion: reduce)").matches) {
    new Lenis({ autoRaf: true, anchors: true });
}
