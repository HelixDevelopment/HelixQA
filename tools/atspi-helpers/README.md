# atspi-helpers

Three tiny command-line helpers the Linux desktop engine (`pkg/nexus/desktop/linux.go`)
shells out to. They wrap `gdbus call` against the AT-SPI DBus service so
the Nexus engine can find, act on, and type into accessible elements
without pulling in a CGo DBus binding.

- `atspi-find` — print a handle for the first matching element.
- `atspi-action` — invoke an accessible action on a handle.
- `atspi-type` — type text into the focused element of a process.

All three are pure shell scripts using `gdbus`, `grep`, `awk`, and
`xdotool` (X11 only). Wayland users rely on `wtype` for input actions.
