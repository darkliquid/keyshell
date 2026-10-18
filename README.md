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

## Calculator and run command

`=` opens the calculator: type `=` then an expression such as `8/2`, and Enter
copies the answer to the clipboard. It understands `+ - * / % ^`, parentheses,
unary minus, `pi`/`e`, and `sqrt abs round floor ceil pow min max`. Anything it
cannot parse shows no result rather than an error, so divide-by-zero and typos
are simply inert.

`:` opens run command: type `:` then a command, and Enter runs it in a terminal
so you can see the output. The terminal is `$TERMINAL` if set, else `ghostty`,
else `alacritty`; if none is found, Enter copies the command instead. The
candidate list is `Config.terminalCandidates`.

Neither mode joins the merged default query, so `=` and `:` are the only ways
in unless you cycle to them with `Tab`.

## Plugins

A plugin is any executable plus a manifest, so you can write one in any
language without touching QML. Put it at
`${XDG_CONFIG_HOME:-~/.config}/keyshell/plugins/<name>/plugin.json`:

```json
{
  "name": "Docker",
  "badge": "DOCKER",
  "trigger": ";docker",
  "command": ["/usr/bin/docker-plugin"],
  "glyph": "🐳",
  "searchable": false,
  "trackable": false
}
```

The trigger must be at least two characters, so it can never collide with a
single-character mode such as `=` or `@`. Type the trigger to open the plugin's
mode, or reach it with `Tab`. A manifest is read at startup, so a newly added
plugin appears after you restart the shell.

The plugin is started when its mode opens and stopped when you leave it or
dismiss the launcher. Your plugin reads one JSON request per line on stdin:

```json
{"query": "ps", "limit": 60, "merged": false}
```

and writes one JSON object per line on stdout, ending the response with
`{"done": true}`:

```json
{"id": "web-1", "title": "web-1", "subtitle": "running", "glyph": "🐳", "badge": "UP",
 "actions": [{"type": "exec", "argv": ["docker", "logs", "web-1"]}]}
{"done": true}
```

Actions are declarative and only these five run: `copy` (`text`), `open-url`
(`url`), `exec` (`argv`), `exec-terminal` (`argv`) and `set-query` (`query`).
Anything malformed is ignored, so a broken plugin shows no results rather than
breaking the launcher.

Add `"destructive": true` to an item whose action is dangerous. The shell then
refuses to run it on a plain Enter and requires the same confirmation the power
actions use, a hold of Enter for `Config.powerHoldDurationMs` or a double-tap,
with a meter drawn on the card while you hold.

Set `"searchable": true` to let a plugin answer the merged default query, and
`"trackable": true` to let its activations feed the ranking history.

`merged` says where the request came from: `true` for the merged default query,
`false` for the plugin's own mode. A searchable plugin is asked both, and the
same call serves each, so the flag is the only way to tell them apart. The usual
shape is to return nothing for an empty merged query, so a plain launcher open
is never flooded, while listing your busiest entries for an empty query in your
own mode.

### Bundled example: processes

`plugins/processes` is a complete working plugin, written in Python, that lists
running processes and terminates the one you pick:

```bash
plugins/processes/install.sh
```

Then restart keyshell and type `,ps`. Results are sorted by CPU, and you reach a
specific process either by typing its name prefix (`gho` for `ghostty`) or its
pid exactly (`2256`). The query is deliberately strict, and an empty query
returns nothing at all, so there is no way to reach the launcher with an
arbitrary process selected. Both of those rules keep process names out of your
default results and make accidental termination impossible.

Terminating a process is marked destructive, so a plain Enter will not do it.
The card draws a hold meter and you must hold Enter for `Config.powerHoldDurationMs`
(a second, or a double-tap) before the kill runs. It sends `SIGTERM`, never
`SIGKILL`, so a process is never force-killed from the launcher.

Run `node tests/run.js` to check the plugin still speaks the protocol.

## Tuning

All tunables are constants in `quickshell/keyshell/Config.qml`: the rotation
range, shear range, band size, minimum visible end height, and result cap.
Colours and typography are in `quickshell/keyshell/Theme.qml`; set
`fontFamily` to a display face you have installed (the default is Red Hat
Display; Teko or Bebas Neue suit the look if present).

Every reveal re-randomises the band tilt and the card lean within those ranges,
while keeping at least `minEndHeightPx` of the band visible at each end.
