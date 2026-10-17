#!/usr/bin/env python3
"""Build every logo and icon from one master mark, so app, splash, web and admin all match.

Source: apps/mobile/assets/brand/source.png (Gemini output from scripts/gen-brand.py, dark mark on light).
Run:    .local/toolchain/py/bin/python scripts/build-brand.py
"""
from pathlib import Path

from PIL import Image, ImageDraw, ImageFilter, ImageOps

ROOT = Path(__file__).resolve().parent.parent
SRC = ROOT / "apps/mobile/assets/brand/source.png"
MOBILE = ROOT / "apps/mobile/assets"
INK, PAPER, PLUM = (27, 26, 25), (250, 247, 243), (94, 43, 99)


def mark_alpha():
    """Alpha mask of the mark: dark pixels opaque, background transparent, tightly cropped and square."""
    g = ImageOps.grayscale(Image.open(SRC))
    # Smooth anti-aliased edges: map luminance 60..200 to alpha 255..0.
    a = g.point(lambda v: 255 if v < 60 else 0 if v > 200 else int((200 - v) * 255 / 140))
    box = a.point(lambda v: 255 if v > 20 else 0).getbbox()
    a = a.crop(box)
    side = max(a.size)
    sq = Image.new("L", (side, side), 0)
    sq.paste(a, ((side - a.width) // 2, (side - a.height) // 2))
    return sq.resize((1024, 1024), Image.LANCZOS)


def colored(alpha, rgb, size, scale=1.0, bg=None, radius=0):
    """The mark in one colour, `scale` of the canvas wide, centred; optional background tile."""
    canvas = Image.new("RGBA", (size, size), (*bg, 255) if bg else (0, 0, 0, 0))
    inner = max(1, int(size * scale))
    m = alpha.resize((inner, inner), Image.LANCZOS)
    layer = Image.new("RGBA", (inner, inner), (*rgb, 255))
    layer.putalpha(m)
    off = (size - inner) // 2
    canvas.alpha_composite(layer, (off, off))
    if radius:
        mask = Image.new("L", (size, size), 0)
        ImageDraw.Draw(mask).rounded_rectangle((0, 0, size - 1, size - 1), radius=radius, fill=255)
        canvas.putalpha(mask)
    return canvas


def save(img, path, **kw):
    path.parent.mkdir(parents=True, exist_ok=True)
    img.save(path, optimize=True, **kw)
    print("wrote", path.relative_to(ROOT))


def svg(alpha, path, fill):
    import vtracer

    tmp = ROOT / ".local/brand/trace.png"
    bw = Image.new("RGB", alpha.size, (255, 255, 255))
    bw.paste((0, 0, 0), mask=alpha.point(lambda v: 255 if v > 127 else 0))
    bw.save(tmp)
    out = ROOT / ".local/brand/trace.svg"
    vtracer.convert_image_to_svg_py(str(tmp), str(out), colormode="binary", mode="spline", filter_speckle=8, path_precision=2)
    body = out.read_text()
    paths = body[body.index("<path"): body.rindex("</svg>")]
    paths = paths.replace('fill="#000000"', f'fill="{fill}"')
    path.write_text(f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1024 1024">{paths}</svg>\n')
    print("wrote", path.relative_to(ROOT))
    return paths


def main():
    a = mark_alpha()
    hexc = lambda c: "#%02X%02X%02X" % c

    # Master marks for in-app use and documents.
    save(colored(a, INK, 1024), MOBILE / "brand/mark-ink.png")
    save(colored(a, PAPER, 1024), MOBILE / "brand/mark-paper.png")

    # App icon (iOS / store): paper mark on an ink tile.
    save(colored(a, PAPER, 1024, 0.58, INK).convert("RGB"), MOBILE / "images/icon.png")
    # Android adaptive icon: the system masks a 108dp canvas to ~66dp, so keep the mark inside the middle 45%.
    save(colored(a, PAPER, 1024, 0.45), MOBILE / "images/android-icon-foreground.png")
    save(Image.new("RGB", (1024, 1024), INK), MOBILE / "images/android-icon-background.png")
    save(colored(a, (255, 255, 255), 1024, 0.45), MOBILE / "images/android-icon-monochrome.png")
    # Splash: ink mark on paper (light), paper mark on ink (dark).
    save(colored(a, INK, 512, 0.9), MOBILE / "images/splash-icon.png")
    save(colored(a, PAPER, 512, 0.9), MOBILE / "images/splash-icon-dark.png")
    # Web favicon (Expo web).
    save(colored(a, PAPER, 196, 0.62, INK, radius=44), MOBILE / "images/favicon.png")

    # Vector mark for the admin console and the public website.
    paths = svg(a, ROOT / ".local/brand/mark.svg", hexc(INK))
    tile = lambda: (f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1024 1024"><rect width="1024" height="1024" rx="230" fill="{hexc(INK)}"/>'
                    f'<g transform="translate(215 215) scale(0.58)">{paths.replace(hexc(INK), hexc(PAPER))}</g></svg>\n')
    for p in (ROOT / "admin/public/favicon.svg", ROOT / "server/internal/web/static/favicon.svg"):
        p.write_text(tile())
        print("wrote", p.relative_to(ROOT))
    for p in (ROOT / "admin/public/logo.svg", ROOT / "server/internal/web/static/logo.svg"):
        p.write_text((ROOT / ".local/brand/mark.svg").read_text())
        print("wrote", p.relative_to(ROOT))

    og(a)


def og(alpha):
    """Link preview card for the website: mark, name and tagline on paper."""
    from PIL import ImageFont

    fonts = ROOT / "apps/mobile/node_modules/@expo-google-fonts"
    img = Image.new("RGBA", (1200, 630), (*PAPER, 255))
    img.alpha_composite(colored(alpha, INK, 220), (110, 205))
    d = ImageDraw.Draw(img)
    d.text((380, 225), "Metro Buddy", font=ImageFont.truetype(str(fonts / "lora/600SemiBold/Lora_600SemiBold.ttf"), 92), fill=INK)
    d.text((384, 350), "Anonymous company for your daily commute.", font=ImageFont.truetype(str(fonts / "poppins/400Regular/Poppins_400Regular.ttf"), 30), fill=(95, 90, 86))
    d.rectangle((0, 610, 1200, 630), fill=PLUM)
    save(img.convert("RGB"), ROOT / "server/internal/web/static/og.png")


if __name__ == "__main__":
    main()
