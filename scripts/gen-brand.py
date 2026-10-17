#!/usr/bin/env python3
"""Generate brand artwork with Gemini, offline. Key from .local/secrets/gemini.key; only
finished images are committed.

    .local/toolchain/py/bin/python scripts/gen-brand.py candidates   # logo ideas -> .local/brand/
"""
import base64, json, os, sys, urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
RAW = ROOT / ".local/brand"
MODEL = os.environ.get("GEMINI_IMAGE_MODEL", "gemini-3.1-flash-image")

BASE = (
    "App icon logo mark for 'Metro Buddy', an anonymous companion app for daily metro commuters in India. "
    "Flat vector, bold and simple enough to read at 48px, at most two colours: near-black #1B1A19 and warm off-white #FAF7F3, "
    "with an optional small accent in deep plum #5E2B63. No text, no letters, no gradients, no shadows, no mockup, no border. "
    "Centered on a plain #FAF7F3 square background with generous empty margin around the mark. "
)

CONCEPTS = {
    "a": "Concept: two rounded speech bubbles that overlap, their outline forming a metro line with two station dots.",
    "b": "Concept: the front of a friendly rounded metro train whose two headlights read as eyes and whose windscreen is a speech bubble.",
    "c": "Concept: a single continuous metro route line that bends into a heart-shaped loop with three station dots.",
    "d": "Concept: a round metro station sign (circle with a horizontal bar) where the bar is a speech bubble tail, very minimal.",
}


def key():
    return os.environ.get("GEMINI_API_KEY") or (ROOT / ".local/secrets/gemini.key").read_text().strip()


def generate(prompt):
    body = {"contents": [{"parts": [{"text": prompt}]}], "generationConfig": {"responseModalities": ["IMAGE"]}}
    req = urllib.request.Request(
        f"https://generativelanguage.googleapis.com/v1beta/models/{MODEL}:generateContent",
        data=json.dumps(body).encode(), headers={"Content-Type": "application/json", "x-goog-api-key": key()})
    with urllib.request.urlopen(req, timeout=180) as r:
        data = json.load(r)
    for part in data["candidates"][0]["content"]["parts"]:
        inline = part.get("inlineData") or part.get("inline_data")
        if inline:
            return base64.b64decode(inline["data"])
    raise RuntimeError("no image in response")


if __name__ == "__main__":
    RAW.mkdir(parents=True, exist_ok=True)
    if sys.argv[1:2] == ["candidates"]:
        for k, c in CONCEPTS.items():
            for n in (1, 2):
                (RAW / f"logo-{k}{n}.png").write_bytes(generate(BASE + c))
                print(k, n, "ok", flush=True)
