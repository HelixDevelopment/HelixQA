# `helixqa-frida-bridge`

Python sidecar bridging `frida-python` to HTTP. Consumed by `pkg/observe/frida` on the Go side.

**Status:** operator-action. Go client code-complete + 97.4% tested (M55).

## Wire contract

```
POST   /sessions                            → {session_id}
DELETE /sessions/{id}                       → 204 (or 404 = already gone)
POST   /sessions/{id}/scripts               → {script_id}
POST   /sessions/{id}/scripts/{sid}/call    → {result}
```

All bodies JSON. The Go client documents the exact shapes in
`pkg/observe/frida/frida.go`.

## Build recipe

```dockerfile
FROM docker.io/library/python:3.11-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    usbutils \
 && rm -rf /var/lib/apt/lists/*

WORKDIR /app
RUN pip install --no-cache-dir frida==16.* flask

COPY server.py /app/

EXPOSE 17430
CMD ["python", "/app/server.py"]
```

`server.py` outline:

```python
from flask import Flask, request, jsonify, abort
import frida, uuid

app = Flask(__name__)
sessions = {}     # id → frida.Session
scripts  = {}     # id → frida.Script

@app.route("/sessions", methods=["POST"])
def attach():
    body = request.get_json()
    target, kind = body["target"], body.get("kind", "package")
    device = frida.get_usb_device(timeout=5)  # or get_local_device
    if kind == "pid":
        sess = device.attach(int(target))
    else:
        sess = device.attach(target)
    sid = uuid.uuid4().hex
    sessions[sid] = sess
    return jsonify({"session_id": sid})

@app.route("/sessions/<sid>", methods=["DELETE"])
def detach(sid):
    sess = sessions.pop(sid, None)
    if sess is None: abort(404)
    sess.detach()
    return ("", 204)

@app.route("/sessions/<sid>/scripts", methods=["POST"])
def load(sid):
    body = request.get_json()
    sess = sessions.get(sid) or abort(404)
    script = sess.create_script(body["script"])
    script.load()
    sid2 = uuid.uuid4().hex
    scripts[sid2] = script
    return jsonify({"script_id": sid2})

@app.route("/sessions/<sid>/scripts/<scriptid>/call", methods=["POST"])
def call(sid, scriptid):
    body = request.get_json()
    script = scripts.get(scriptid) or abort(404)
    result = script.exports_sync.__getattr__(body["method"])(*body.get("args", []))
    return jsonify({"result": result})

if __name__ == "__main__":
    app.run(host="0.0.0.0", port=17430, threaded=True)
```

## Deployment

- **Android**: USB-connected device with `frida-server` pushed + running as root (`adb push frida-server-android-arm64 /data/local/tmp/` + `su -c /data/local/tmp/frida-server &`).
- **iOS**: jailbroken device with Cydia Frida repo.
- **Desktop**: `frida-server` running on the target host.

## Acceptance

1. `POST /sessions` with a real running target returns a valid session id.
2. A minimal `rpc.exports = { ping: () => "pong" }` script loads + `Call(..., "ping", nil, &result)` returns `"pong"`.
