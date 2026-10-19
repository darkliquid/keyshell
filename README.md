# keyshell, Go rewrite (branch `go-rewrite`)

This branch is a clean-room, **no-history** branch of keyshell: it starts from an
empty tree so the Go implementation can be built without dragging the QML
implementation's history along. `main` still holds the working Quickshell/QML
version, and stays the reference for behaviour and for the visual design.

It lives in a separate git worktree (`~/Projects/keyshell-go`) so both
implementations can be built and compared side by side.

## Why Go

From the research in `~/.local/share/crush/research/keyshell-go-reimplementation`:

- `gogpu/gg` already has the affine transform API the design is built on
  (`Shear`, `ShearAbout`, `Rotate`, `Transform`, `TextPath`) with a GPU backend.
- The stack is pure Go and cgo-free: one static binary, no Qt, no GTK.
- The one blocker - layer-shell surfaces - is solved: a ~600-line patch on
  `darkliquid/gogpu@feat/wayland-layer-shell` (PR drafted), carried here as a
  `replace` until it lands upstream.

## Toolchain

Everything goes through [mise](https://mise.jdx.dev):

```bash
mise trust          # once, for this directory
mise install        # Go 1.27.2 + golangci-lint, per mise.toml

mise run build      # go build ./...
mise run test       # go test ./...
mise run lint       # golangci-lint
mise run check      # gofmt check + go vet + tests
```

`CGO_ENABLED=0` is set for every task in this repository.

## Spikes

Throwaway proofs of concept, one question each. They live in `spikes/` and will
be deleted once their findings are folded into the design.

| Task | Question |
| --- | --- |
| `mise run spike:s2` | Does a wlr-layer-shell surface receive keyboard and pointer input, and can on-demand focus be used to implement a launcher overlay? |
| `mise run spike:s3` | Does `gg` render the display typography (all-caps italic Red Hat Display) well enough to replace Qt's text rendering? |
| `mise run spike:s4` | Can a layer surface composite per-pixel alpha, so the overlay can show the desktop through scrims and translucent cards? (`spike:s4:verify`, `spike:s4:verify-straight`, `spike:s4:control` assert it against screenshots) |
| `mise run spike:inject` | Test aid: synthesise real keystrokes and clicks through `/dev/uinput`, so the S2 spike can be driven and asserted without a human at the keyboard. |

## Dependency on a gogpu fork

`go.mod` replaces `github.com/gogpu/gogpu` with the layer-shell branch of the
personal fork:

```
replace github.com/gogpu/gogpu => github.com/darkliquid/gogpu <pseudo-version>
```

When the upstream PR merges, drop the `replace` and bump the version. Until
then, `go get github.com/darkliquid/gogpu@feat/wayland-layer-shell` refreshes
it.

## Layout

```
mise.toml                     toolchain + tasks
spikes/layershell-input       S2: layer surface + input
spikes/text                   S3: display typography rendering
spikes/transparency           S4: per-pixel alpha (spike + screenshot checker)
spikes/tools/inject           test aid: synthetic input via uinput
```

The product layout (`cmd/keyshell`, `internal/...`) is deliberately not created
yet: the spikes decide its shape.