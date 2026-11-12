export function isLow(lang) {
    return lang === "low";
}

export function unescape(text) {
    return text.replace(/&lt;/g, "<").replace(/&gt;/g, ">").replace(/&quot;/g, '"').replace(/&#39;/g, "'").replace(/&amp;/g, "&");
}
