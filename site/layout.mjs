import { escape } from "./highlight.mjs";

export const origin = "https://low.simoncurtis.dev";

const sections = [
    { name: "Overview", path: "index.html" },
    { name: "Reference", path: "reference.html" },
    { name: "Docs", path: "docs/index.html" },
    { name: "Packages", path: "packages/index.html" },
];

function navigation(root, current) {
    const links = sections.map((section) => {
        const marker = section.name === current ? ' aria-current="page"' : "";
        return `<a${marker} href="${root}${section.path}">${section.name}</a>`;
    });
    return links.join("\n");
}

export function layout({ path, heading, title, description, current, sidebar, body, scripts = [] }) {
    const root = "../".repeat(path.split("/").length - 1);
    const scriptTags = ["lenis.js", "scroll.js", "hover.js", ...scripts].map((script) => `<script src="${root}${script}" defer></script>`).join("\n");
    return `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>${escape(title)}</title>
<meta name="description" content="${escape(description)}">
<meta property="og:type" content="article">
<meta property="og:title" content="${escape(title)}">
<meta property="og:description" content="${escape(description)}">
<meta property="og:url" content="${origin}/${path}">
<meta property="og:image" content="${origin}/og-image.png">
<meta name="twitter:card" content="summary_large_image">
<link rel="canonical" href="${origin}/${path}">
<meta name="theme-color" content="#f5f5f3">
<link rel="icon" href="${root}favicon.ico" sizes="48x48">
<link rel="icon" href="${root}icon.svg" type="image/svg+xml">
<link rel="apple-touch-icon" href="${root}apple-touch-icon.png">
<link rel="manifest" href="${root}site.webmanifest">
<link rel="preload" href="${root}fonts/inter.woff2" as="font" type="font/woff2" crossorigin>
<link rel="preload" href="${root}fonts/jetbrains-mono.woff2" as="font" type="font/woff2" crossorigin>
<link rel="stylesheet" href="${root}style.css">
</head>
<body>
<a class="skip" href="#main">Skip to content</a>
<header class="masthead">
<div class="wrap">
<a class="brand" href="${root}index.html"><span class="mark size-5" aria-hidden="true"></span>Low</a>
<nav class="menu">
${navigation(root, current)}
</nav>
<a class="repo" href="https://github.com/simon-curtis/lowlang">GitHub</a>
</div>
</header>

<div class="wrap grid grid-cols-[minmax(0,1fr)] gap-x-16 gap-y-10 pt-36 pb-24 sm:pt-28 lg:grid-cols-[13rem_minmax(0,1fr)]">
<nav class="sidebar" aria-label="${escape(current)}">
${sidebar}
</nav>
<main class="prose" id="main">
<h1>${escape(heading).replaceAll("/", "/<wbr>")}</h1>
${body}</main>
</div>
${scriptTags}
</body>
</html>
`;
}
