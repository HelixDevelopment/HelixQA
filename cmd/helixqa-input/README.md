# cmd/helixqa-input

Native sidecar that speaks the libei wire protocol (flatbuffers over a Unix
socket) so the HelixQA Go host can emit keyboard + pointer + scroll events
on Wayland without linking libei via cgo.

**Language:** Rust (reuses the upstream `libei`/`reis` ecosystem — a
pure-Go EI client would duplicate ~5 kLoC of protocol code).

## Contract

Input channel — stdin JSON commands (one per line):

```json
{"type":"key_down","code":30}
{"type":"key_up","code":30}
{"type":"key_tap","code":28}
{"type":"pointer_rel","dx":10,"dy":0}
{"type":"button","code":272,"press":true}
{"type":"scroll","ticks":3}
{"type":"sync"}
```

Startup argv:

```
helixqa-input --eis-fd <N>     # FD obtained from libei.Service.EISFile()
helixqa-input --health          # prints "ok\n", exits 0
```

The FD is passed via `exec.Cmd.ExtraFiles` — the Go host does the
portal handshake via `pkg/navigator/linux/libei`, hands the resulting
`*os.File` to this sidecar, and speaks JSON over stdin.

## Build

```sh
# One-time Rust install (operator — no runtime sudo):
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh

cargo install --path cmd/helixqa-input
# Or: cargo build --release; install -m 0755 target/release/helixqa-input ~/.local/bin/
```

## Source layout (to be written)

```
cmd/helixqa-input/
├── README.md              (this file)
├── Cargo.toml             reis (rust libei) + serde_json
├── build.sh               installer wrapper
└── src/
    ├── main.rs            argv + FD adoption + JSON command loop
    ├── ei.rs              reis handshake + device creation
    ├── encode.rs          JSON command -> EI protocol messages
    └── tests/
        └── integration.rs integration smoke against reis
```

## Why Rust + reis?

`reis` (<https://github.com/ids1024/reis>) is the canonical pure-Rust
libei client. Reusing it avoids re-implementing the flatbuffers wire
protocol from scratch. Alternative paths considered:

- **cgo libei**: 3–4 MB binary bloat, harder to cross-compile.
- **pure-Go EI client**: ~800–1500 LoC of flatbuffers machinery, not worth
  the re-invention cost for this use case.

## Status

Planned. Release blocker for Wayland input emulation (today the legacy
`pkg/navigator/x11_executor.go` covers X11; Wayland has no input
alternative until this ships). Tracked in
`docs/openclawing/OpenClawing4-Handover.md` §3.1.
