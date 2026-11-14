const highlighted = ["type", "string", "number", "function", "module", "operator", "comment"];

async function text(path) {
  const response = await fetch(path);
  if (!response.ok) {
    throw new Error(`${path}: ${response.status}`);
  }
  return response.text();
}

export async function loadFonts() {
  const faces = [
    new FontFace("Inter", "url(/site/src/fonts/inter.woff2)", { weight: "100 900" }),
    new FontFace("JetBrains Mono", "url(/site/src/fonts/jetbrains-mono.woff2)", { weight: "100 800" }),
  ];
  for (const face of faces) {
    const loaded = await face.load();
    document.fonts.add(loaded);
  }
}

function cssColour(css, name) {
  const found = css.match(new RegExp(`--${name}:\\s*(#[0-9a-f]{6})`, "i"));
  if (!found) {
    throw new Error(`style.css has no --${name} colour`);
  }
  return found[1];
}

export async function theme() {
  const css = await text("/site/src/style.css");
  const start = css.indexOf("@utility on-ink");
  const block = css.slice(start, css.indexOf("}", start));
  const ink = cssColour(css, "color-ink");
  const paper = cssColour(css, "color-paper");
  const palette = { ink, paper, keyword: paper, name: paper };
  for (const kind of highlighted) {
    palette[kind] = cssColour(block, kind);
  }
  return palette;
}

export async function tagline() {
  const html = await text("/site/content/index.html");
  const page = new DOMParser().parseFromString(html, "text/html");
  const lead = page.querySelector(".lead").textContent;
  return lead.slice(0, lead.indexOf(".") + 1);
}

export async function origin() {
  const config = await text("/site/gen/config.low");
  return config.match(/const origin = "https?:\/\/([^"]+)"/)[1];
}

export async function markPolygons() {
  const svg = await text("/site/src/mark.svg");
  const parsed = new DOMParser().parseFromString(svg, "image/svg+xml");
  const path = parsed.querySelector("path").getAttribute("d");
  const view = parsed.documentElement.getAttribute("viewBox").split(" ").map(Number);
  const polygons = [];
  let current = [];
  let x = 0;
  let y = 0;
  for (const [, command, argument] of path.matchAll(/([MHVLZ])\s*([-\d.\s]*)/g)) {
    const values = argument.trim().split(/\s+/).filter(Boolean).map(Number);
    if (command === "M") {
      current = [];
      [x, y] = values;
    } else if (command === "H") {
      [x] = values;
    } else if (command === "V") {
      [y] = values;
    } else if (command === "L") {
      [x, y] = values;
    } else {
      polygons.push(current);
      continue;
    }
    current.push([x, y]);
  }
  return { polygons, size: view[2] };
}

export async function compilation() {
  const json = await text("/compile.json");
  return JSON.parse(json);
}
