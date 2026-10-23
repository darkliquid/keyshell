# keyshell

A Wayland launcher: a layer-shell overlay with a search field, ranked
results from in-process backends and subprocess plugins, cards, and a KSL
theme language for the visual scene. Pure Go, single static binary, no Qt,
no GTK.

## Why Go

- `gogpu/gg` already has the affine transform API the design is built on
  (`Shear`, `ShearAbout`, `Rotate`, `Transform`, `TextPath`) with a GPU backend.
- The stack is pure Go and cgo-free: one static binary, no Qt, no GTK.

## Architecture

```
cmd/keyshell          entry point; dispatches into internal/cli
internal/cli          command surface: run, headless, tui, query, verbs, plugin mode
internal/launcher    overlay state machine: mode, query, selection, ranking
internal/backend     the seam every result source crosses (apps, files, windows, calc, run)
internal/backend/…   in-process backends
internal/plugin      subprocess plugin host (handshake, ping, restart) + SDK and proto
internal/ipc         line-delimited verb socket (toggle, query, reload, ...)
internal/capability  broker for host-executed actions (copy, exec, open-url, ...)
internal/ksl         the KSL theme language: lexer, parser, validator
internal/ksl/eval    scene resolve: bindings, cascade, layout
internal/render      overlay drawing from the resolved scene; live scene reload
internal/input       keyboard/pointer to launcher calls
internal/rank        fuzzy ranking and frecency
internal/telemetry   OpenTelemetry traces and metrics
internal/logging     slog setup; internal/debugsrv: pprof/expvar endpoint
```

## Running

```bash
keyshell                  # graphical host (starts hidden when not on a TTY)
keyshell toggle           # summon/dismiss from a compositor keybind
keyshell openMode audio   # open directly in a mode
```

Configuration is TOML at `$XDG_CONFIG_HOME/keyshell/config.toml`
(overridable with `KEYSHELL_CONFIG` or `--config`); a missing file is fine,
defaults apply. Plugin instances are declared per instance with `id`,
`command`, and `mode` (`auto`, `hot`, `lazy`). See [Configuration Reference](docs/configuration.md)
for all options, defaults, and examples.

Theming: point `[ui] scene` at a `.ksl` file, or pass `--scene`. The scene
resolves against live launcher state (query, results, selection, reveal
geometry) and supports transitions and keyframes. See [KSL Reference](docs/ksl.md)
for language syntax, primitives, flex layout, and case studies (including the
default slanted band and dropdown scenes).

## Toolchain

Everything goes through [mise](https://mise.jdx.dev):

```bash
mise trust          # once, for this directory
mise install        # Go 1.27.3 + golangci-lint, per mise.toml

mise run build      # go build ./...
mise run test       # go test ./...
mise run lint       # golangci-lint
mise run check      # gofmt check + go vet + tests
mise run race       # tests under the pure-Go race detector (no cgo)
```

`mise run race` needs `go install github.com/kolkov/racedetector/cmd/racedetector@latest`
once. It is a pure-Go FastTrack detector, so it works with `CGO_ENABLED=0`,
where `go test -race` cannot.

`CGO_ENABLED=0` is set for every task in this repository.

## CLI

`keyshell help` prints the full reference. Commands:

| Command | Effect |
| --- | --- |
| `keyshell [run] [flags]` | run the graphical host |
| `keyshell headless [flags]` | run the host without the overlay: launcher, plugins, IPC |
| `keyshell query [flags] TEXT` | query a running host over IPC, or a one-shot local query |
| `keyshell tui [flags]` | drive an in-process launcher from the terminal |
| `keyshell VERB [args]` | send one IPC verb to a running host |
| `keyshell plugin NAME` | internal: run a plugin subprocess (host-spawned) |
| `keyshell plugin-clip-capture` | internal: capture clipboard selections |
| `keyshell version`, `keyshell help` | build info, full usage |

Verbs: `toggle`, `open`, `close`, `openMode MODE`, `volumeUp`, `volumeDown`,
`toggleMute`, `playPause`, `nextTrack`, `previousTrack`, `reload`. Clients
accept `--socket` to target a non-default host.

## Development and debugging

The whole launcher works without a Wayland session:

```bash
keyshell headless --log-level debug --debug-addr 127.0.0.1:6060
    # full host (launcher, plugins, IPC); pprof and expvar on the debug addr
keyshell query "ghost" --mode apps --json
    # against the running headless host, or one-shot if none is up
keyshell tui
    # interactive: type to search, arrows to select, Tab to cycle modes,
    # Enter to run the selected action, Ctrl-C to quit
```

While developing the visual scene, run the graphical host against your theme
with live reload:

```bash
keyshell run --scene themes/my.ksl --watch
    # save the file and the overlay repaints; broken edits keep the old scene
keyshell reload
    # force a re-parse from a script or editor hook
```

Host flags on `run`/`headless`/`tui`:

| Flag | Effect |
| --- | --- |
| `--config path` | config file override |
| `--plugins a,b` / `--plugins none` | override the plugin instance list |
| `--scene path` | KSL scene override (beats `ui.scene`) |
| `--watch` / `--watch=false` | live-reload the scene and its includes on change |
| `--log-level`, `--log-format` | slog level and text/json output |
| `--debug-addr` | pprof and expvar HTTP endpoint |
| `--otel URL` / `--otel-stdout` | OTLP/HTTP export or stdout JSON export |

## Telemetry

Every query, backend and plugin round trip, IPC verb and rendered frame is
instrumented with OpenTelemetry.

Tracing follows one request end to end: `keyshell.input.key` ->
`keyshell.query` -> `keyshell.backend.query` / `keyshell.plugin.query` ->
`keyshell.render.frame`, joined by span links per job generation, so a
keystroke, the plugin that answered it and the paint that showed the results
form one trace.

Metrics include `keyshell.query.duration`, `keyshell.query.results`,
`keyshell.backend.query.duration`, `keyshell.plugin.load.duration`,
`keyshell.plugin.starts`, `keyshell.plugin.restarts`,
`keyshell.plugin.active`, `keyshell.ipc.verbs`,
`keyshell.render.frame.duration`, `keyshell.ksl.resolve.duration` and
`keyshell.ksl.scene.reloads`.

Without flags everything is a no-op. Use `--otel-stdout` for zero-setup
JSON output, or point `--otel` at an OTLP/HTTP collector
(e.g. `http://localhost:4318`) to feed Grafana Tempo/Jaeger or similar.

## Dependency on a gogpu fork

`go.mod` replaces `github.com/gogpu/gogpu` with the layer-shell branch of the
personal fork:

```
replace github.com/gogpu/gogpu => github.com/darkliquid/gogpu <pseudo-version>
```

When the upstream PR merges, drop the `replace` and bump the version. Until
then, `go get github.com/darkliquid/gogpu@feat/wayland-layer-shell` refreshes
it.
