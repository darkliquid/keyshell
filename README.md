# keyshell

A keyboard-driven application launcher for niri, built with Quickshell. One
summoned overlay, no status bar, no daemon.

## Run (development)

Requires Quickshell 0.3.2+ and a running Wayland session (niri).

```bash
qs -p quickshell/keyshell
```

Toggle it from another terminal:

```bash
qs ipc call keyshell toggle
```

If more than one Quickshell instance is running, address this one by pid:
`qs ipc --pid <pid> call keyshell toggle`.

## Install

```bash
mkdir -p ~/.config/quickshell
ln -s "$PWD/quickshell/keyshell" ~/.config/quickshell/keyshell
qs -c keyshell
```

The IPC calls become `qs -c keyshell ipc call keyshell toggle`.

## Keybind (niri)

Add to `~/.config/niri/config.kdl`:

```kdl
binds {
    Mod+Space { spawn "qs" "-c" "keyshell" "ipc" "call" "keyshell" "toggle"; }
}
```

`spawn` passes arguments directly and does not run a shell.

## Keys

| Key | Action |
| --- | --- |
| printable characters | filter the merged result list |
| Backspace | edit the query |
| Left / Right | move the selection |
| Tab / Shift+Tab | move the selection |
| Enter | launch the selected app |
| Escape | dismiss |

## Search & ranking

Typing with no prefix searches everything at once: applications, open windows
and files. Results are ordered by how well they match, and ties are broken by
how often and how recently you picked them, so your regulars rise to the top.

Explicit prefixes still jump straight to a single mode (for example `@` for
windows, `/` for files).

History lives at `${XDG_STATE_HOME:-~/.local/state}/keyshell/frecency.json`.
Delete that file to reset ranking; entries older than `frecencyMaxAgeDays` are
dropped automatically on load.

## Tuning

All tunables are constants in `quickshell/keyshell/Config.qml`: the rotation
range, shear range, band size, minimum visible end height, and result cap.
Colours and typography are in `quickshell/keyshell/Theme.qml`; set
`fontFamily` to a display face you have installed (the default is Red Hat
Display; Teko or Bebas Neue suit the look if present).

Every reveal re-randomises the band tilt and the card lean within those ranges,
while keeping at least `minEndHeightPx` of the band visible at each end.
