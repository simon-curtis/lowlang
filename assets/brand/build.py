import json
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path

TILE = "#3f7d5c"
DOG = "#15181d"
INK = "#f5f1e8"
NAME = "LowLang"
TAGLINE = [
    "A compiled language with Go's speed",
    "and footprint, plus enums, typed errors",
    "and expression-oriented syntax.",
]

SKULL = [(290, 85), (330, 74), (372, 68), (425, 70), (482, 76), (545, 88), (582, 82), (604, 114), (640, 155), (665, 195), (676, 232), (680, 266), (702, 290), (750, 316), (798, 336), (820, 360), (822, 382), (815, 400)]
EAR = [(290, 85), (245, 115), (205, 150), (160, 190), (148, 228), (175, 262), (208, 296), (240, 322), (292, 346), (318, 372), (338, 396), (362, 382), (380, 340), (388, 280), (382, 215), (373, 160), (382, 112), (415, 82)]
JAW = [(420, 403), (458, 432), (488, 470), (505, 508), (525, 535), (553, 544), (582, 532), (604, 524), (640, 534), (672, 530), (712, 513), (742, 506), (778, 497), (794, 470), (800, 432), (815, 400)]
NECK_BACK = [(208, 296), (199, 318), (186, 350), (178, 405), (168, 462), (154, 512), (122, 563), (96, 604), (66, 632), (46, 658), (40, 700), (26, 736), (12, 768), (2, 800), (-30, 1000), (-80, 1300)]
NECK_FRONT = [(598, 538), (591, 592), (583, 650), (579, 712), (602, 728), (620, 750), (624, 800), (632, 1000), (650, 1300)]
THROAT = [(508, 548), (482, 575), (466, 610), (460, 648), (443, 668)]
EYE = [(550, 222), (522, 226), (501, 239), (522, 246), (549, 248)]
NOSE = [(802, 349), (764, 349), (740, 364), (738, 390), (762, 404), (790, 400)]
NOSTRILS = [(776, 390, 9), (816, 393, 10)]
EYE_DOT = (527, 236, 26)
EAR_HEM = EAR[6:11]
SILHOUETTE = SKULL + JAW[-2:6:-1] + NECK_FRONT + NECK_BACK[::-1][:-1] + EAR[6:0:-1]

ICON_VIEW = (-20, -60, 860)
ICON_RADIUS = 190
SMALL_VIEW = (110, 10, 760)
PIXELS_16 = (
    "................",
    "....######......",
    "...#########....",
    "..##########....",
    ".#######.###....",
    ".############...",
    "...###########..",
    "..#.###########.",
    ".###.##########.",
    ".#############..",
    ".############...",
    "##########......",
    "##########......",
    "##########......",
    "##########......",
    "###########.....",
)
MASKABLE_VIEW = (-15, -200, 1000)


class Canvas:
    def __init__(self, x, y, width, height, crisp=False):
        self.x = x
        self.y = y
        self.width = width
        self.height = height
        self.crisp = crisp
        self.defs = []
        self.body = []

    def render(self):
        rendering = ' shape-rendering="crispEdges"' if self.crisp else ""
        lines = [f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="{self.x} {self.y} {self.width} {self.height}"{rendering}>']
        if self.defs:
            lines.append("  <defs>")
            lines.extend(f"    {d}" for d in self.defs)
            lines.append("  </defs>")
        lines.extend(f"  {b}" for b in self.body)
        lines.append("</svg>")
        return "\n".join(lines) + "\n"

    def frame(self, radius=0):
        self.defs.append(f'<clipPath id="tile"><rect x="{self.x}" y="{self.y}" width="{self.width}" height="{self.height}" rx="{radius}"/></clipPath>')

    def background(self, radius=0):
        self.frame(radius)
        self.body.append(f'<rect x="{self.x}" y="{self.y}" width="{self.width}" height="{self.height}" rx="{radius}" fill="{TILE}"/>')

    def dog(self, transform="", detail=True, colour=DOG):
        cut = 'fill="none" stroke="#000" stroke-linecap="round" stroke-linejoin="round"'
        mask = [f'<mask id="lines" maskUnits="userSpaceOnUse" x="-200" y="-200" width="1400" height="1600">', '  <rect x="-200" y="-200" width="1400" height="1600" fill="#fff"/>']
        if detail:
            mask.extend(f'  <path d="{smooth(points)}" {cut} stroke-width="20"/>' for points in (EAR, JAW, THROAT))
            mask.append(f'  <path d="{smooth(NOSE)}" {cut} stroke-width="14"/>')
            mask.extend(f'  <circle cx="{cx}" cy="{cy}" r="{r}" fill="#000"/>' for cx, cy, r in NOSTRILS)
            mask.append(f'  <path d="{smooth(EYE)}" {cut} stroke-width="16"/>')
        else:
            cx, cy, r = EYE_DOT
            mask.append(f'  <path d="{smooth(EAR_HEM)}" {cut} stroke-width="44"/>')
            mask.append(f'  <circle cx="{cx}" cy="{cy}" r="{r}" fill="#000"/>')
        mask.append("</mask>")
        self.defs.extend(mask)
        group = f' transform="{transform}"' if transform else ""
        self.body.append(f'<g clip-path="url(#tile)"><g{group}><path d="{smooth(SILHOUETTE, closed=True)}" fill="{colour}" mask="url(#lines)"/></g></g>')

    def pixels(self, rows, colour):
        cells = "".join(f'<rect x="{x}" y="{y}" width="1" height="1"/>' for y, row in enumerate(rows) for x, cell in enumerate(row) if cell == "#")
        self.body.append(f'<g fill="{colour}">{cells}</g>')

    def text(self, x, y, content, size, weight, opacity=1):
        escaped = content.replace("&", "&amp;").replace("'", "&#39;")
        self.body.append(f'<text x="{x}" y="{y}" font-family="Open Sans" font-size="{size}" font-weight="{weight}" fill="{INK}" fill-opacity="{opacity}">{escaped}</text>')


def smooth(points, closed=False):
    count = len(points)

    def at(k):
        if closed:
            return points[k % count]
        return points[max(0, min(count - 1, k))]

    segments = count if closed else count - 1
    path = [f"M{points[0][0]} {points[0][1]}"]
    for i in range(segments):
        p0, p1, p2, p3 = at(i - 1), at(i), at(i + 1), at(i + 2)
        c1 = (p1[0] + (p2[0] - p0[0]) / 6, p1[1] + (p2[1] - p0[1]) / 6)
        c2 = (p2[0] - (p3[0] - p1[0]) / 6, p2[1] - (p3[1] - p1[1]) / 6)
        path.append(f"C{c1[0]:.1f} {c1[1]:.1f} {c2[0]:.1f} {c2[1]:.1f} {p2[0]} {p2[1]}")
    if closed:
        path.append("Z")
    return "".join(path)


def icon(rounded):
    x, y, size = ICON_VIEW
    canvas = Canvas(x, y, size, size)
    canvas.background(ICON_RADIUS if rounded else 0)
    canvas.dog()
    return canvas.render()


def glyph(view, detail, crisp=False):
    x, y, size = view
    canvas = Canvas(x, y, size, size, crisp)
    canvas.frame()
    canvas.dog(detail=detail, colour=TILE)
    return canvas.render()


def pixel_glyph(rows):
    canvas = Canvas(0, 0, len(rows[0]), len(rows), crisp=True)
    canvas.pixels(rows, TILE)
    return canvas.render()


def maskable():
    x, y, size = MASKABLE_VIEW
    canvas = Canvas(x, y, size, size)
    canvas.background()
    canvas.dog()
    return canvas.render()


def banner(width, height):
    canvas = Canvas(0, 0, width, height)
    canvas.background()
    scale = height * 0.82 / 860
    margin = height * 0.07
    canvas.dog(f"translate({margin:.1f} {height * 0.12:.1f}) scale({scale:.4f})")
    left = round(margin + 840 * scale + width * 0.05)
    canvas.text(left, round(height * 0.45), NAME, round(height * 0.15), 800)
    for index, line in enumerate(TAGLINE):
        canvas.text(left, round(height * (0.58 + index * 0.068)), line, round(height * 0.04), 400, 0.85)
    return canvas.render()


def rasterise(svg, output, width, height):
    result = subprocess.run(["rsvg-convert", "-w", str(width), "-h", str(height), "-o", str(output)], input=svg.encode(), capture_output=True)
    if result.returncode != 0:
        raise RuntimeError(f"rsvg-convert failed for {output.name}: {result.stderr.decode().strip()}")


def pack_ico(sources, output):
    result = subprocess.run(["magick", *map(str, sources), str(output)], capture_output=True)
    if result.returncode != 0:
        raise RuntimeError(f"magick failed for {output.name}: {result.stderr.decode().strip()}")


def manifest():
    return json.dumps({
        "name": NAME,
        "short_name": NAME,
        "icons": [
            {"src": "icon-192.png", "sizes": "192x192", "type": "image/png"},
            {"src": "icon-512.png", "sizes": "512x512", "type": "image/png"},
            {"src": "icon-maskable-512.png", "sizes": "512x512", "type": "image/png", "purpose": "maskable"},
        ],
        "theme_color": TILE,
        "background_color": TILE,
        "display": "standalone",
    }, indent=2) + "\n"


def build(brand):
    rounded = icon(rounded=True)
    square = icon(rounded=False)
    (brand.parent / "icon.svg").write_text(rounded)
    favicons = {
        16: pixel_glyph(PIXELS_16),
        32: glyph(SMALL_VIEW, detail=False, crisp=True),
        48: glyph(ICON_VIEW, detail=True),
    }
    with tempfile.TemporaryDirectory() as scratch:
        frames = []
        for size, source in favicons.items():
            frame = Path(scratch) / f"favicon-{size}.png"
            rasterise(source, frame, size, size)
            frames.append(frame)
        pack_ico(frames, brand / "favicon.ico")
    rasterise(square, brand / "apple-touch-icon.png", 180, 180)
    rasterise(rounded, brand / "icon-192.png", 192, 192)
    rasterise(rounded, brand / "icon-512.png", 512, 512)
    rasterise(maskable(), brand / "icon-maskable-512.png", 512, 512)
    rasterise(banner(1200, 630), brand / "og-image.png", 1200, 630)
    rasterise(banner(1280, 640), brand / "github-social-preview.png", 1280, 640)
    (brand / "site.webmanifest").write_text(manifest())


def main():
    missing = [tool for tool in ("rsvg-convert", "magick") if shutil.which(tool) is None]
    if missing:
        sys.exit(f"missing tools: {', '.join(missing)}")
    try:
        build(Path(__file__).resolve().parent)
    except (OSError, RuntimeError) as error:
        sys.exit(str(error))


main()
