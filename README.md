# keyshell

## Why Go

- `gogpu/gg` already has the affine transform API the design is built on
  (`Shear`, `ShearAbout`, `Rotate`, `Transform`, `TextPath`) with a GPU backend.
- The stack is pure Go and cgo-free: one static binary, no Qt, no GTK.

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

## Dependency on a gogpu fork

`go.mod` replaces `github.com/gogpu/gogpu` with the layer-shell branch of the
personal fork:

```
replace github.com/gogpu/gogpu => github.com/darkliquid/gogpu <pseudo-version>
```

When the upstream PR merges, drop the `replace` and bump the version. Until
then, `go get github.com/darkliquid/gogpu@feat/wayland-layer-shell` refreshes
it.
