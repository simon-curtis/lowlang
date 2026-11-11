import json
import math
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path

PAPER = "#f2ead9"
GRAIN = "#e2d6bd"
SPOT = "#ff5a1f"
INK = "#16130f"
TYPEFACE = "Noto Sans"
NAME = "LowLang"
TAGLINE = [
    "A compiled language with Go's speed",
    "and footprint, plus enums, typed errors",
    "and expression-oriented syntax.",
]

VIEW = 512
LETTER_LEFT = 136
LETTER_TOP = 96
LETTER_RIGHT = 376
LETTER_BOTTOM = 416
STROKE = 96


def letter_path():
    inner_x = LETTER_LEFT + STROKE
    inner_y = LETTER_BOTTOM - STROKE
    return (
        f"M{LETTER_LEFT} {LETTER_TOP}H{inner_x}V{inner_y}H{LETTER_RIGHT}V{LETTER_BOTTOM}H{LETTER_LEFT}Z"
    )


def letter_tone(x, y):
    along = (x - LETTER_LEFT) / (LETTER_RIGHT - LETTER_LEFT)
    up = (LETTER_BOTTOM - y) / (LETTER_BOTTOM - LETTER_TOP)
    return 1.1 - 1.05 * max(along, up)


def screen(left, top, right, bottom, step, tone):
    circles = []
    rows = int((bottom - top) / step) + 1
    columns = int((right - left) / (2 * step)) + 1
    for row in range(rows):
        y = top + row * step
        for column in range(columns):
            x = left + (2 * column + row % 2) * step
            level = min(tone(x, y), 1.1)
            if level > 0:
                circles.append(f'<circle cx="{x:.2f}" cy="{y:.2f}" r="{step * math.sqrt(level):.2f}"/>')
    return "".join(circles)


def tile(step, radius, scale):
    overprint = screen(LETTER_LEFT, LETTER_TOP, LETTER_RIGHT, LETTER_BOTTOM, step, letter_tone)
    shift = VIEW / 2 * (1 - scale)
    return (
        f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {VIEW} {VIEW}">'
        f'<defs><clipPath id="letter"><path d="{letter_path()}"/></clipPath></defs>'
        f'<rect width="{VIEW}" height="{VIEW}" rx="{radius}" fill="{PAPER}"/>'
        f'<g transform="translate({shift} {shift}) scale({scale})">'
        f'<path d="{letter_path()}" fill="{SPOT}"/>'
        f'<g clip-path="url(#letter)" fill="{INK}">{overprint}</g></g></svg>\n'
    )


def pixel_tile():
    return (
        '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" shape-rendering="crispEdges">'
        f'<rect width="16" height="16" fill="{PAPER}"/>'
        f'<path d="M4 2H8V10H13V14H4Z" fill="{SPOT}"/>'
        f'<path d="M4 8H8V10H10V14H4Z" fill="{INK}"/></svg>\n'
    )


def banner(width, height):
    unit = width / 1200
    left = 72 * unit
    size = 212 * unit
    baseline = height / 2 - 30 * unit
    word_width = 840 * unit
    step = 7 * unit

    def word_tone(x, y):
        return 1.12 - 1.1 * max((x - left) / word_width, 0) ** 1.6

    def grain_tone(x, y):
        return 0.55 * (x / width) ** 2 * (y / height) ** 2

    word = f'<text x="{left:.2f}" y="{baseline:.2f}" font-family="{TYPEFACE}" font-weight="900" font-size="{size:.2f}">{NAME}</text>'
    overprint = screen(left, baseline - size, left + word_width + size, baseline + size / 3, step, word_tone)
    grain = screen(0, 0, width, height, 10 * unit, grain_tone)
    lines = "".join(
        f'<text x="{left + 6 * unit:.2f}" y="{baseline + (96 + index * 50) * unit:.2f}">{line}</text>'
        for index, line in enumerate(TAGLINE)
    )
    return (
        f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {width} {height}">'
        f'<defs><clipPath id="word">{word}</clipPath></defs>'
        f'<rect width="{width}" height="{height}" fill="{PAPER}"/>'
        f'<g fill="{GRAIN}">{grain}</g>'
        f'<g fill="{SPOT}">{word}</g>'
        f'<g clip-path="url(#word)" fill="{INK}">{overprint}</g>'
        f'<g fill="{INK}" font-family="{TYPEFACE}" font-weight="500" font-size="{36 * unit:.2f}">{lines}</g></svg>\n'
    )


def rasterise(svg, output, width, height):
    result = subprocess.run(["rsvg-convert", "-w", str(width), "-h", str(height), "-o", str(output)], input=svg.encode(), capture_output=True)
    if result.returncode != 0:
        raise RuntimeError(f"rsvg-convert failed for {output.name}: {result.stderr.decode().strip()}")


def pack_ico(sources, output):
    result = subprocess.run(["magick", *map(str, sources), str(output)], capture_output=True)
    if result.returncode != 0:
        raise RuntimeError(f"magick failed for {output.name}: {result.stderr.decode().strip()}")


def require_typeface():
    result = subprocess.run(["fc-match", "-f", "%{family}", TYPEFACE], capture_output=True)
    families = result.stdout.decode().split(",")
    if result.returncode != 0 or TYPEFACE not in families:
        raise RuntimeError(f"typeface {TYPEFACE} is not installed")


def manifest():
    return json.dumps({
        "name": NAME,
        "short_name": NAME,
        "icons": [
            {"src": "icon-192.png", "sizes": "192x192", "type": "image/png"},
            {"src": "icon-512.png", "sizes": "512x512", "type": "image/png"},
            {"src": "icon-maskable-512.png", "sizes": "512x512", "type": "image/png", "purpose": "maskable"},
        ],
        "theme_color": PAPER,
        "background_color": PAPER,
        "display": "standalone",
    }, indent=2) + "\n"


def build(brand):
    require_typeface()
    rounded = tile(16, 112, 1)
    (brand.parent / "icon.svg").write_text(rounded)
    favicons = {
        16: pixel_tile(),
        32: tile(40, 80, 1.15),
        48: tile(32, 96, 1.1),
    }
    with tempfile.TemporaryDirectory() as scratch:
        frames = []
        for size, source in favicons.items():
            frame = Path(scratch) / f"favicon-{size}.png"
            rasterise(source, frame, size, size)
            frames.append(frame)
        pack_ico(frames, brand / "favicon.ico")
    rasterise(tile(16, 0, 1), brand / "apple-touch-icon.png", 180, 180)
    rasterise(rounded, brand / "icon-192.png", 192, 192)
    rasterise(rounded, brand / "icon-512.png", 512, 512)
    rasterise(tile(16, 0, 0.78), brand / "icon-maskable-512.png", 512, 512)
    rasterise(banner(1200, 630), brand / "og-image.png", 1200, 630)
    rasterise(banner(1280, 640), brand / "github-social-preview.png", 1280, 640)
    (brand / "site.webmanifest").write_text(manifest())


def main():
    missing = [tool for tool in ("rsvg-convert", "magick", "fc-match") if shutil.which(tool) is None]
    if missing:
        sys.exit(f"missing tools: {', '.join(missing)}")
    try:
        build(Path(__file__).resolve().parent)
    except (OSError, RuntimeError) as error:
        sys.exit(str(error))


main()
