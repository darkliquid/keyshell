# Installing keyshell

keyshell is a Quickshell configuration, not a package. Installing it means two
things: putting the configuration where Quickshell can find it **by name**, and
starting it once per session as a **session shell**.

A session shell is a single long-lived process that runs for as long as your
Wayland session does. keyshell draws no bar, dock or widget, so once it is
running it is invisible; the only thing you see is the overlay, summoned by a
keybind over the IPC socket. Running it this way is what makes the summon
instant, and it is what keeps the clipboard watcher alive between summons.

## Requirements

| Component | Needed for | Without it |
| --- | --- | --- |
| Quickshell 0.3.2+ (`qs`) | Everything | Nothing runs |
| A Wayland session (niri, sway, Hyprland, River, labwc) | Layer-shell overlay | Nothing runs |
| `wl-clipboard` (`wl-paste`, `wl-copy`) | Clipboard history, calculator and run-command copy | Clipboard mode stays empty; copying a result does nothing |
| `dsearch` plus its service | File search (`dsearch search --json`) | Files mode returns nothing |
| `python3` | The clipboard capture helper, `clipboard-watch.py` | Clipboard history records nothing |
| `xdg-open` | Opening a file, or a plugin's `open-url` action | Opening a result does nothing |
| A terminal in `Config.terminalCandidates` (`ghostty`, `alacritty`) or `$TERMINAL` | Run command | Enter copies the command instead |
| PipeWire | Audio mode and the volume slider | Audio mode stays empty |
| NetworkManager | Network mode | Network mode stays empty |
| BlueZ | Bluetooth mode | Bluetooth mode stays empty |
| UPower | Battery readout in power mode | Power mode shows no battery |

dsearch is a separate service. If it is not already running:

```bash
systemctl --user enable --now dsearch.service
```

## Install the configuration

Quickshell registers every `<config dir>/quickshell/<name>/shell.qml` as a
configuration called `<name>`. Link this checkout in as `keyshell`:

```bash
mkdir -p "${XDG_CONFIG_HOME:-$HOME/.config}/quickshell"
ln -sfn "$PWD/quickshell/keyshell" "${XDG_CONFIG_HOME:-$HOME/.config}/quickshell/keyshell"
```

The directory name is the configuration name, so it must be `keyshell`: that is
what `qs -c keyshell` and every keybind below refer to. Run this, and the
service install below, from the repository root, since both use relative paths
into the checkout.

> `~/.config/quickshell/shell.qml` is treated as the configuration named
> `default`, and its presence makes Quickshell ignore all named subdirectories.
> If you already have one, keyshell will not be found until you move it.

Running `qs -p quickshell/keyshell` from the repository, as the README's
development section does, still works, but that instance is known only by its
path. `qs -c keyshell ipc call ...` selects instances of the named
configuration, so it will not reach a `-p` instance; use `-p` for development
and install for daily use.

## Start it with the session

### Option A: a systemd user service (recommended)

This is the supervised option: the shell starts with the graphical session and
is restarted if it crashes.

```bash
install -Dm644 contrib/keyshell.service ~/.config/systemd/user/keyshell.service
systemctl --user daemon-reload
systemctl --user enable --now keyshell.service
```

The unit is `WantedBy=graphical-session.target`, which is the target your
compositor pulls in: niri's `niri.service` is `BindsTo=graphical-session.target`,
and other session-managed compositors follow the same convention.

Useful variants, if you want them:

- Add `Environment=QS_NO_RELOAD_POPUP=1` to `[Service]` to silence the reload
  notification Quickshell pops up whenever a file in the configuration changes.
  Editing a checkout you are developing against makes that popup frequent.
- Add `Environment=QS_DISABLE_CRASH_HANDLER=1` to make systemd the only thing
  supervising the process. Quickshell otherwise relaunches itself after a crash,
  which is redundant when a unit already owns the process.

### Option B: your compositor's autostart

Simpler, with no supervision and no restart on crash. For niri, add to
`~/.config/niri/config.kdl`:

```kdl
spawn-at-startup "qs" "-c" "keyshell" "--no-duplicate"
```

`spawn-at-startup` passes its arguments directly; use `spawn-sh-at-startup` if
you would rather write one shell string.

### Pick one, and do not mix them

`--no-duplicate` makes a second instance of the same configuration exit
immediately with `An instance of this configuration is already running.` That
protects you from two overlays answering the same keybind, but it also means the
two methods above compete for the first start:

- Under systemd alone, this never happens.
- If your compositor autostarts keyshell *and* the unit is enabled, whichever
  starts first wins. If the unit loses, it exits cleanly, so
  `Restart=on-failure` leaves it inactive while the compositor's instance keeps
  running. That is harmless, but `systemctl --user status keyshell` will then
  report `inactive (dead)` while a keyshell is plainly running, which is
  confusing. Choose one method.

## Keybinds

Keybinds only send an IPC call to the running shell. For niri:

```kdl
binds {
    Mod+Space { spawn "qs" "-c" "keyshell" "ipc" "call" "keyshell" "toggle"; }
    Mod+W     { spawn "qs" "-c" "keyshell" "ipc" "call" "keyshell" "toggleMode" "windows"; }
    Mod+F     { spawn "qs" "-c" "keyshell" "ipc" "call" "keyshell" "toggleMode" "files"; }
    Mod+A     { spawn "qs" "-c" "keyshell" "ipc" "call" "keyshell" "openVolume"; }
    Mod+M     { spawn "qs" "-c" "keyshell" "ipc" "call" "keyshell" "openMedia"; }
    Mod+T     { spawn "qs" "-c" "keyshell" "ipc" "call" "keyshell" "openTray"; }
    Mod+B     { spawn "qs" "-c" "keyshell" "ipc" "call" "keyshell" "openBluetooth"; }
    Mod+P     { spawn "qs" "-c" "keyshell" "ipc" "call" "keyshell" "openPower"; }
    Mod+N     { spawn "qs" "-c" "keyshell" "ipc" "call" "keyshell" "openNetwork"; }
}
```

The command is the same everywhere else, wrapped in that compositor's own
syntax, for example sway's `bindsym $mod+space exec qs -c keyshell ipc call
keyshell toggle`, or Hyprland's `bind = SUPER, SPACE, exec, qs -c keyshell ipc
call keyshell toggle`.

Hardware volume keys are worth wiring to the shell's own volume actions, which
also raise the volume hud:

```kdl
XF86AudioRaiseVolume { spawn "qs" "-c" "keyshell" "ipc" "call" "keyshell" "volumeUp" "0.05"; }
XF86AudioLowerVolume { spawn "qs" "-c" "keyshell" "ipc" "call" "keyshell" "volumeDown" "0.05"; }
XF86AudioMute        { spawn "qs" "-c" "keyshell" "ipc" "call" "keyshell" "toggleMute"; }
XF86AudioPlay        { spawn "qs" "-c" "keyshell" "ipc" "call" "keyshell" "playPause"; }
XF86AudioNext        { spawn "qs" "-c" "keyshell" "ipc" "call" "keyshell" "nextTrack"; }
XF86AudioPrev        { spawn "qs" "-c" "keyshell" "ipc" "call" "keyshell" "previousTrack"; }
```

### Mode names for `toggleMode` and `openMode`

| Mode | Id | Reached by |
| --- | --- | --- |
| Merged search | `all` | Typing with no prefix |
| Applications | `apps` | No prefix, or `Tab` |
| Windows | `windows` | `@` |
| Files | `files` | `/` |
| Audio | `audio` | `!` |
| Media | `media` | `>` |
| Tray | `tray` | `^` |
| Bluetooth | `bluetooth` | `*` |
| Power | `power` | `%` |
| Network | `network` | `&` |
| Calculator | `calc` | `=` |
| Run command | `run` | `:` |
| Clipboard | `clipboard` | `,cb` |

`toggleMode` and `openMode` take the id: `toggleMode` flips the overlay (so a
second press dismisses it), `openMode` only ever opens it. `open` and `close`
do the same for whatever mode you are already in.

Other actions worth binding, all under the same `keyshell` target: `lockSession`
and `suspendSession`; `toggleWifi`, `scanWifi` and `disconnectNetwork`;
`toggleBluetoothPower` and `toggleBluetoothScan`; `setPowerProfile "balanced"`
(or `power-saver`, or `performance` where the machine offers it); `openVolume`,
`openMedia`, `openTray`,
`openBluetooth`, `openPower`, `openNetwork`, `openPowerConsole` and
`openNetworkConsole` to jump straight to a console view; and `toggleKeepAwake`.

## Verify it is running

```bash
qs -c keyshell list        # the running instance, its pid and config path
qs -c keyshell log -t 50   # recent logs for this instance
qs -c keyshell ipc call keyshell isOpen
```

`isOpen` returns `false` when the overlay is closed and the shell is alive. That
one call proves the session shell is up and reachable over IPC, which is exactly
what a keybind needs. Pressing a keybind should then draw the overlay.

To stop or restart it by hand:

```bash
systemctl --user restart keyshell.service   # Option A
qs -c keyshell kill                         # either option
```

## Troubleshooting

| Symptom | Cause and fix |
| --- | --- |
| `Could not find "keyshell" config directory in any valid config path.` | The symlink from [Install the configuration](#install-the-configuration) is missing, or `~/.config/quickshell/shell.qml` exists and is shadowing named configurations |
| Keybind does nothing, and prints `No running instances for ...` | The session shell is not running, so the IPC call has nothing to talk to (it exits 255). Start it, and check `qs -c keyshell log` |
| Two overlays appear, or the overlay toggles twice | Two instances of the configuration are running. Start it from one place only, and keep `--no-duplicate` |
| `systemctl --user status keyshell` says `inactive (dead)` yet keyshell works | The compositor autostarted its own instance first, so the unit exited cleanly. Pick one method |
| Clipboard mode is empty | `wl-clipboard` is missing, or `python3` cannot run `clipboard-watch.py`; check `qs -c keyshell log` |
| Clipboard mode is empty on a fresh install | Expected until the first copy. A warning that `clipboard.json` does not exist is first-run noise, not an error |
| Files mode returns nothing | The `dsearch` service is not running (`systemctl --user enable --now dsearch.service`) |
| Windows mode lists windows but cannot focus them | Your compositor's IPC is not visible to the shell. keyshell prefers `$NIRI_SOCKET`, `$SWAYSOCK`, then Hyprland's socket, and falls back to the generic toplevel protocol, which can list windows but is not always able to raise them |
| Audio, network, bluetooth or battery modes are empty | The matching service (PipeWire, NetworkManager, BlueZ, UPower) is not reachable, often because the session shell did not inherit the session environment |
| The overlay never appears | The shell is running but the compositor did not grant layer-shell, or another application holds the keyboard. Check `qs -c keyshell log` |

## Update

Because the configuration is a symlink into this checkout, updating is a pull (or
a rebase) and a restart:

```bash
git pull
systemctl --user restart keyshell.service
```

A plugin manifest is read at startup, so a new plugin needs that restart too.
The bundled examples install with `plugins/processes/install.sh` and
`plugins/emoji/install.sh`, both of which rewrite the manifest to point at your
checkout.

Edits to a file in the checkout are picked up while the shell is running, because
Quickshell watches its configuration and reloads it. That reload is also what
raises the notification popup a `QS_NO_RELOAD_POPUP=1` in the unit suppresses.

## Remove

```bash
systemctl --user disable --now keyshell.service
rm -f ~/.config/systemd/user/keyshell.service
rm -f "${XDG_CONFIG_HOME:-$HOME/.config}/quickshell/keyshell"
```

That leaves your history behind, if you want to clear it too:

```bash
rm -rf "${XDG_STATE_HOME:-$HOME/.local/state}/keyshell"
```

That directory holds `frecency.json` (the ranking history) and `clipboard.json`
(clipboard history). Deleting it resets ranking and forgets every clipboard
entry.