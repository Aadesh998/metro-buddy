#!/usr/bin/env python3
"""Generate the illustrated avatar set with Gemini, once, offline.

The key is read from .local/secrets/gemini.key (git-ignored) or GEMINI_API_KEY and
never ships: only the finished PNGs in apps/mobile/assets/avatars are committed.

    .local/toolchain/py/bin/python scripts/gen-avatars.py [ids...]
"""
import base64, io, json, os, sys, time, urllib.request
from pathlib import Path

from PIL import Image

ROOT = Path(__file__).resolve().parent.parent
OUT = ROOT / "apps/mobile/assets/avatars"
RAW = ROOT / ".local/avatars-raw"
MODEL = os.environ.get("GEMINI_IMAGE_MODEL", "gemini-3.1-flash-image")

STYLE = (
    "Flat vector illustration avatar, head and shoulders, centered, facing the viewer with a warm friendly expression. "
    "Clean modern style with soft shading, bold simple shapes, no outlines, no text, no logos, no watermark. "
    "Solid plain {bg} background filling the whole square. Square 1:1 image. "
    "A young Indian adult who commutes by metro in Delhi, Mumbai or Bengaluru. "
)

# Varied looks; backgrounds come from the app palette so avatars sit well on the dark UI.
PEOPLE = {
    "m01": ("warm yellow", "man with short wavy black hair, light stubble, round glasses, mustard hoodie, over-ear headphones around his neck"),
    "m02": ("mint green", "man with a neat undercut and trimmed beard, denim jacket over a white t-shirt"),
    "m03": ("sky blue", "man wearing a dark blue turban, full neatly-kept beard, olive bomber jacket"),
    "m04": ("coral orange", "man with curly hair, clean shaven, striped polo shirt, small backpack straps visible"),
    "m05": ("lavender purple", "man with long hair tied in a bun, short beard, earthy kurta"),
    "m06": ("pink", "man with short spiky hair, thin moustache, graphic tee and a cap worn backwards"),
    "m07": ("teal", "man with side-parted hair, rectangular glasses, formal light-blue shirt and lanyard, office commuter"),
    "m08": ("light yellow", "man with a buzz cut, dark skin, bright smile, sporty track jacket"),
    "m09": ("mint green", "older college student man with messy hair, wireless earbuds, flannel shirt"),
    "m10": ("sky blue", "man with wavy shoulder-length hair, light beard, round sunglasses pushed up on his head, linen shirt"),
    "f01": ("coral orange", "woman with long straight black hair, small bindi, mustard kurti with a light dupatta"),
    "f02": ("lavender purple", "woman with a short bob haircut, round glasses, oversized denim jacket"),
    "f03": ("mint green", "woman with curly hair in a high puff, gold hoop earrings, bright tee"),
    "f04": ("pink", "woman wearing a soft-coloured hijab, warm smile, cardigan"),
    "f05": ("warm yellow", "woman with a long braid over her shoulder, nose stud, office blazer and lanyard"),
    "f06": ("teal", "woman with wavy shoulder-length hair, headphones on, hoodie"),
    "f07": ("sky blue", "woman with a pixie cut, dark skin, small studs, sporty zip jacket"),
    "f08": ("light yellow", "woman with hair in a messy bun with a pencil, college student, tote bag strap, striped top"),
    "f09": ("coral orange", "woman with long wavy hair, cat-eye glasses, printed kurta"),
    "f10": ("lavender purple", "woman with two braids, freckles, cozy sweater and a scarf. Nothing behind her but the flat solid colour: no scenery, no train, no frame or border"),
}


def key():
    k = os.environ.get("GEMINI_API_KEY") or (ROOT / ".local/secrets/gemini.key").read_text().strip()
    if not k:
        sys.exit("no Gemini key")
    return k


def generate(pid, bg, who, k):
    body = {"contents": [{"parts": [{"text": STYLE.format(bg=bg) + "The person: " + who + "."}]}],
            "generationConfig": {"responseModalities": ["IMAGE"]}}
    req = urllib.request.Request(
        f"https://generativelanguage.googleapis.com/v1beta/models/{MODEL}:generateContent",
        data=json.dumps(body).encode(), headers={"Content-Type": "application/json", "x-goog-api-key": k})
    with urllib.request.urlopen(req, timeout=180) as r:
        data = json.load(r)
    for part in data["candidates"][0]["content"]["parts"]:
        inline = part.get("inlineData") or part.get("inline_data")
        if inline:
            return base64.b64decode(inline["data"])
    raise RuntimeError(f"{pid}: no image in response")


def main():
    OUT.mkdir(parents=True, exist_ok=True)
    RAW.mkdir(parents=True, exist_ok=True)
    k = key()
    ids = sys.argv[1:] or list(PEOPLE)
    for pid in ids:
        bg, who = PEOPLE[pid]
        for attempt in range(3):
            try:
                raw = generate(pid, bg, who, k)
                break
            except Exception as e:  # rate limits and the odd empty response: retry
                print(pid, "retry", attempt + 1, e, flush=True)
                time.sleep(10 * (attempt + 1))
        else:
            print(pid, "FAILED", flush=True)
            continue
        (RAW / f"{pid}.png").write_bytes(raw)
        img = Image.open(io.BytesIO(raw)).convert("RGB")
        side = min(img.size)
        img = img.crop(((img.width - side) // 2, (img.height - side) // 2, (img.width + side) // 2, (img.height + side) // 2))
        img.resize((256, 256), Image.LANCZOS).save(OUT / f"{pid}.png", optimize=True)
        print(pid, "ok", flush=True)


if __name__ == "__main__":
    main()
