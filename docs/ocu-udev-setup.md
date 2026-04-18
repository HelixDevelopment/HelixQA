# OCU Linux Input Setup

## xdotool / ydotool (current P3.5 path)

The P3.5 Linux interact backend uses `xdotool` (X11) or `ydotool` (Wayland)
to inject mouse and keyboard events. Neither binary requires elevated
privileges — install them with your package manager and they work as a normal
user:

```bash
# Debian / Ubuntu
sudo apt install xdotool          # X11
sudo apt install ydotool          # Wayland

# Fedora / RHEL
sudo dnf install xdotool ydotool

# Arch
sudo pacman -S xdotool ydotool
```

HelixQA auto-detects which tool is available (xdotool preferred; ydotool
fallback). Set `HELIXQA_INTERACT_LINUX_STUB=1` to disable the backend in CI
environments that have no display server.

## /dev/uinput (future raw-uinput path)

A future phase will write raw `uinput` events directly to `/dev/uinput` for
sub-millisecond latency and Wayland-native support without ydotool. That path
requires the running user to be in the `input` group — **no sudo in our code**,
just a one-time operator action:

```bash
sudo usermod -aG input $USER
newgrp input          # activate without logout, or log out and back in
ls -la /dev/uinput    # should show crw-rw---- ... input uinput
```

Verify membership:

```bash
groups | grep input
```

Once the user is in the `input` group, the raw-uinput backend will open
`/dev/uinput` without further privilege escalation. If access is still denied
(`EACCES`), check that the udev rule grants the `input` group write permission:

```
# /etc/udev/rules.d/99-uinput.rules
KERNEL=="uinput", GROUP="input", MODE="0660"
```

Reload rules with `sudo udevadm control --reload && sudo udevadm trigger`.
