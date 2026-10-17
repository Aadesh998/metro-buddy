"""Image safety scorer for Metro Buddy. POST raw image bytes, get {"nsfw": 0..1}.

Uses the open-source Yahoo Open NSFW model via opennsfw2 (MIT). Images are scored in memory
and never written to disk.
"""
import io
import os

import numpy as np
import opennsfw2 as n2
from flask import Flask, abort, jsonify, request
from PIL import Image

app = Flask(__name__)
model = n2.make_open_nsfw_model()
MAX_BYTES = 6 * 1024 * 1024
TOKEN = os.environ.get("NSFW_TOKEN", "")


@app.get("/healthz")
def health():
    return {"ok": True}


@app.post("/score")
def score():
    if TOKEN and request.headers.get("Authorization") != f"Bearer {TOKEN}":
        abort(401)
    data = request.get_data(cache=False)
    if not data or len(data) > MAX_BYTES:
        abort(413)
    try:
        img = Image.open(io.BytesIO(data)).convert("RGB")
    except Exception:
        abort(415)
    x = n2.preprocess_image(img, n2.Preprocessing.YAHOO)
    prob = float(model.predict(np.expand_dims(x, 0), verbose=0)[0][1])
    return jsonify({"nsfw": prob})
