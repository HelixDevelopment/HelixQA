# HelixQA LD_PRELOAD Observer — Operator Guide

## Overview

The `observe/ld_preload` backend launches a target process with a custom
shared library (`shim.so`) preloaded via `LD_PRELOAD`. The shim overrides
libc functions to record calls as newline-delimited JSON into a named pipe
(FIFO). HelixQA reads the FIFO and emits `contracts.Event` values with
`Kind == "hook"` into the observer channel.

No root is required — the shim is placed in a user-writable directory and
the FIFO is created in `/tmp`.

---

## Step 1 — Compile the shim

```bash
gcc -shared -fPIC -o /tmp/shim.so docs/hooks/ld-preload-shim.c -ldl
```

Add additional function overrides to the C file before compiling. The
template includes `open(2)` as a starting example. Common additions:

```c
ssize_t read(int fd, void *buf, size_t count);
ssize_t write(int fd, const void *buf, size_t count);
int connect(int sockfd, const struct sockaddr *addr, socklen_t addrlen);
int socket(int domain, int type, int protocol);
```

Each override calls `shim_emit("function_name", first_string_arg)` before
forwarding to the real implementation via `dlsym(RTLD_NEXT, ...)`.

---

## Step 2 — Create the FIFO

```bash
mkfifo /tmp/helix-shim.fifo
```

HelixQA creates the FIFO automatically when `productionProducer.Produce`
runs, so this step is only needed for manual inspection.

---

## Step 3 — Wire HelixQA

Choose one of two ways to supply the shim path:

### Option A — bank YAML label (recommended)

```yaml
- name: Observe open() calls in my-app
  target:
    process_name: my-app
    labels:
      exec_path:  /usr/bin/my-app
      shim_path:  /tmp/shim.so
```

### Option B — environment variable

```bash
export HELIXQA_LD_SHIM=/tmp/shim.so
```

The `exec_path` label (or `target.ProcessName`) identifies the binary to
launch. HelixQA sets `LD_PRELOAD` and `HELIXQA_LD_SHIM_FIFO` automatically
in the child process environment.

---

## Step 4 — Read events

Each JSON line written by the shim becomes a `contracts.Event`:

| Field | Value |
|---|---|
| `Kind` | `"hook"` |
| `Timestamp` | Nanosecond-precision wall clock from `clock_gettime` |
| `Payload["fn"]` | Overridden function name (e.g. `"open"`) |
| `Payload["arg"]` | First string argument (e.g. `"/etc/passwd"`) |

---

## Kill-switch

Set `HELIXQA_OBSERVE_LDPRELOAD_STUB=1` to force `ErrNotWired` without
touching the filesystem or launching any process. Useful in CI environments
where the shim is unavailable.

---

## JSON record format

```json
{"ts_ns":1714000000000000000,"fn":"open","arg":"/etc/ld.so.cache"}
```

- `ts_ns` — nanoseconds since the Unix epoch (`CLOCK_REALTIME`)
- `fn` — overridden libc function name
- `arg` — first argument cast to string; empty string when not applicable

---

## Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| `ErrNotWired` at `Start` | Shim file not found | Verify path in `shim_path` label or `HELIXQA_LD_SHIM` |
| No events emitted | `HELIXQA_LD_SHIM_FIFO` not set in child env | Should be automatic; check observer logs |
| FIFO open blocks forever | Target exited before opening FIFO | Ensure the target binary actually calls the overridden function |
| Partial JSON line | Target crashed mid-write | Shim flushes after every record; crash between `fprintf` and `fflush` can produce a partial line — the observer skips malformed lines |
