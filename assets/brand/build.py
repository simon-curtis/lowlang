import json
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path

INK = "#111111"
PAPER = "#f5f5f3"
TYPEFACE = "Noto Sans"
NAME = "Low"
TAGLINE = [
    "A compiled, garbage-collected language",
    "with green threads, enums and typed errors.",
    "Small static binaries for Linux and Windows.",
]

VIEW = 512
STROKE_RATIO = 0.3
GAP_RATIO = 0.0625


def mark(left, top, size, stroke, gap):
    right = left + size
    bottom = top + size
    upright = f"M{left} {top}H{left + stroke}V{bottom - stroke - gap}L{left} {bottom - gap}Z"
    foot = f"M{left + gap} {bottom}L{left + stroke + gap} {bottom - stroke}H{right}V{bottom}Z"
    return f"{upright}{foot}"


def proportioned(left, top, size):
    return mark(left, top, size, round(size * STROKE_RATIO), round(size * GAP_RATIO))


def bare_mark():
    return f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {VIEW} {VIEW}"><path d="{proportioned(0, 0, VIEW)}"/></svg>\n'


def tile(radius, inset, stroke, gap):
    return (
        f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {VIEW} {VIEW}">'
        f'<rect width="{VIEW}" height="{VIEW}" rx="{radius}" fill="{INK}"/>'
        f'<path d="{mark(inset, inset, VIEW - 2 * inset, stroke, gap)}" fill="{PAPER}"/></svg>\n'
    )


def standard_tile(radius, inset):
    size = VIEW - 2 * inset
    return tile(radius, inset, round(size * STROKE_RATIO), round(size * GAP_RATIO))


def banner(width, height):
    unit = width / 1200
    left = round(96 * unit)
    lines = "".join(
        f'<text x="{left}" y="{(430 + index * 50) * unit:.2f}">{line}</text>'
        for index, line in enumerate(TAGLINE)
    )
    return (
        f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {width} {height}">'
        f'<rect width="{width}" height="{height}" fill="{PAPER}"/>'
        f'<path d="{proportioned(left, round(96 * unit), round(128 * unit))}" fill="{INK}"/>'
        f'<text x="{left - 6 * unit:.2f}" y="{350 * unit:.2f}" fill="{INK}" font-family="{TYPEFACE}" font-weight="700" font-size="{104 * unit:.2f}">{NAME}</text>'
        f'<g fill="{INK}" font-family="{TYPEFACE}" font-weight="400" font-size="{34 * unit:.2f}">{lines}</g></svg>\n'
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
    assets = brand.parent
    site = assets.parent / "site"
    rounded = standard_tile(112, 144)
    (assets / "icon.svg").write_text(rounded)
    (site / "icon.svg").write_text(rounded)
    (site / "mark.svg").write_text(bare_mark())
    favicons = {
        16: tile(64, 96, 104, 40),
        32: tile(96, 104, 96, 28),
        48: tile(104, 120, 84, 20),
    }
    with tempfile.TemporaryDirectory() as scratch:
        frames = []
        for size, source in favicons.items():
            frame = Path(scratch) / f"favicon-{size}.png"
            rasterise(source, frame, size, size)
            frames.append(frame)
        pack_ico(frames, site / "favicon.ico")
    rasterise(standard_tile(0, 144), site / "apple-touch-icon.png", 180, 180)
    rasterise(rounded, site / "icon-192.png", 192, 192)
    rasterise(rounded, site / "icon-512.png", 512, 512)
    rasterise(standard_tile(0, 168), site / "icon-maskable-512.png", 512, 512)
    rasterise(banner(1200, 630), site / "og-image.png", 1200, 630)
    rasterise(banner(1280, 640), brand / "github-social-preview.png", 1280, 640)
    (site / "site.webmanifest").write_text(manifest())


def main():
    missing = [tool for tool in ("rsvg-convert", "magick", "fc-match") if shutil.which(tool) is None]
    if missing:
        sys.exit(f"missing tools: {', '.join(missing)}")
    try:
        build(Path(__file__).resolve().parent)
    except (OSError, RuntimeError) as error:
        sys.exit(str(error))


main()
