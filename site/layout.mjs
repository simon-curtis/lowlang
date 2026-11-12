import { folders, origin, repository } from "./config.mjs";
import { escape } from "./highlight.mjs";

const sections = [
    { name: "Overview", path: "index.html" },
    { name: "Docs", path: `${folders.docs}index.html` },
    { name: "Packages", path: `${folders.packages}index.html` },
];
const grid = "wrap grid grid-cols-[minmax(0,1fr)] gap-x-16 gap-y-10 pt-36 pb-24 sm:pt-28 lg:grid-cols-[13rem_minmax(0,1fr)]";
const substitutions = { origin, repository };

function navigation(root, current) {
    const links = sections.map((section) => {
        const marker = section.name === current ? ' aria-current="page"' : "";
        return `<a${marker} href="${root}${section.path}">${section.name}</a>`;
    });
    return links.join("\n");
}

function address(path) {
    return path === "index.html" ? `${origin}/` : `${origin}/${path}`;
}

function head({ path, title, description, type = "article", error }, root) {
    const fonts = error ? ["inter"] : ["inter", "jetbrains-mono"];
    const discovery = error
        ? ['<meta name="robots" content="noindex">']
        : [
              `<meta name="description" content="${escape(description)}">`,
              `<meta property="og:type" content="${type}">`,
              `<meta property="og:title" content="${escape(title)}">`,
              `<meta property="og:description" content="${escape(description)}">`,
              `<meta property="og:url" content="${address(path)}">`,
              `<meta property="og:image" content="${origin}/og-image.png">`,
              '<meta name="twitter:card" content="summary_large_image">',
              `<link rel="canonical" href="${address(path)}">`,
          ];
    const preloads = fonts.map((font) => `<link rel="preload" href="${root}fonts/${font}.woff2" as="font" type="font/woff2" crossorigin>`);
    const lines = [
        '<meta charset="utf-8">',
        '<meta name="viewport" content="width=device-width, initial-scale=1">',
        `<title>${escape(title)}</title>`,
        ...discovery,
        '<meta name="theme-color" content="#f5f5f3">',
        `<link rel="icon" href="${root}favicon.ico" sizes="48x48">`,
        `<link rel="icon" href="${root}icon.svg" type="image/svg+xml">`,
        `<link rel="apple-touch-icon" href="${root}apple-touch-icon.png">`,
        `<link rel="manifest" href="${root}site.webmanifest">`,
        ...preloads,
        `<link rel="stylesheet" href="${root}style.css">`,
    ];
    return lines.join("\n");
}

function masthead(root, current) {
    return `<header class="masthead">
<div class="wrap">
<a class="brand" href="${root}index.html"><span class="mark size-5" aria-hidden="true"></span>Low</a>
<nav class="menu">
${navigation(root, current)}
</nav>
<a class="repo" href="${repository}">GitHub</a>
</div>
</header>`;
}

function scriptTags({ scripts = [] }, root) {
    return scripts.map((script) => `<script src="${root}${script}" defer></script>\n`).join("");
}

function shell(page, content) {
    const root = page.error ? "/" : "../".repeat(page.path.split("/").length - 1);
    return `<!doctype html>
<html lang="en">
<head>
${head(page, root)}
</head>
<body>
<a class="skip" href="#main">Skip to content</a>
${masthead(root, page.current)}

${content}
${scriptTags(page, root)}</body>
</html>
`;
}

function expand(body) {
    return body.replace(/\{\{(\w+)\}\}/g, (whole, name) => {
        if (!(name in substitutions)) {
            throw new Error(`unknown placeholder ${whole}`);
        }
        return substitutions[name];
    });
}

export function layout(page) {
    const { heading, current, sidebar, body } = page;
    const content = `<div class="${grid}">
<nav class="sidebar" aria-label="${escape(current)}">
${sidebar}
</nav>
<main class="prose" id="main">
<h1>${escape(heading).replaceAll("/", "/<wbr>")}</h1>
${body}</main>
</div>`;
    return shell({ ...page, scripts: ["hover.js", ...(page.scripts ?? [])] }, content);
}

export function panels(page) {
    return shell(page, `<main id="main">\n${expand(page.body)}</main>`);
}
