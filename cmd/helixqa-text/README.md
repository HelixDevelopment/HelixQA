# `helixqa-text`

Python sidecar exposing EAST + MSER + PP-OCR over HTTP. Consumed by `pkg/vision/text` on the Go side.

**Status:** operator-action. The Go client is code-complete + 92% tested (M51); this sidecar is the deliverable operators need to build.

## Wire contract

Request:

```
POST /detect
Content-Type: multipart/form-data
  image = <PNG bytes>
```

Response:

```json
{
  "width": 1920,
  "height": 1080,
  "regions": [
    {"bbox": [x1, y1, x2, y2], "text": "Sign In", "confidence": 0.94}
  ]
}
```

- `text` is OPTIONAL — present only when the sidecar runs OCR on detected regions. A pure-detector sidecar (EAST only, no OCR) emits `text: ""`.
- `bbox` with len() != 4 → Go client returns `ErrInvalidBBox`.

## Build recipe

```dockerfile
FROM docker.io/library/python:3.11-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    libgl1 libglib2.0-0 \
 && rm -rf /var/lib/apt/lists/*

WORKDIR /app
RUN pip install --no-cache-dir \
    opencv-python-headless==4.10.* \
    paddleocr==2.8.* paddlepaddle==2.6.* \
    flask pillow

COPY weights /models
COPY server.py /app/

EXPOSE 7870
CMD ["python", "/app/server.py"]
```

`server.py` outline:

```python
from flask import Flask, request, jsonify
from paddleocr import PaddleOCR
from PIL import Image
import io, numpy as np

app = Flask(__name__)
ocr = PaddleOCR(use_angle_cls=True, lang="en", show_log=False)

@app.route("/detect", methods=["POST"])
def detect():
    img_bytes = request.files["image"].read()
    img = np.array(Image.open(io.BytesIO(img_bytes)))
    h, w = img.shape[:2]
    result = ocr.ocr(img)
    regions = []
    for line in (result[0] or []):
        bbox_pts, (text, conf) = line
        xs = [p[0] for p in bbox_pts]; ys = [p[1] for p in bbox_pts]
        regions.append({
            "bbox": [int(min(xs)), int(min(ys)), int(max(xs)), int(max(ys))],
            "text": text,
            "confidence": float(conf),
        })
    return jsonify({"width": w, "height": h, "regions": regions})

if __name__ == "__main__":
    app.run(host="0.0.0.0", port=7870, threaded=True)
```

## Podman run

```bash
podman run --rm -d \
  --name helixqa-text \
  --network host \
  --cpus=2 --memory=4g \
  localhost/helixqa-text:latest
```

## Acceptance

1. `POST http://$HOST:7870/detect` with a busy 1080p screenshot returns ≥ 1 region.
2. `PH2-TEXT-INT-001` passes.
