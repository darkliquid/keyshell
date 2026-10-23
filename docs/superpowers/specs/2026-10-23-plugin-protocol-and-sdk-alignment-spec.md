# Specification: Plugin Protocol & SDK Alignment (Option B)

**Status:** Draft / Specification  
**Date:** 2026-10-23  
**Authors:** Google DeepMind pair programming assistant & Darkliquid  

---

## 1. Problem Statement & Motivation

Keyshell supports two types of backends:
1. **In-process backends**: `apps`, `files`, `windows`, `calc`, `run` implementing [`backend.Backend`](file:///home/darkliquid/Projects/keyshell/internal/backend/backend.go).
2. **Subprocess plugins**: `audio`, `media`, `network`, `power`, `clipboard`, `tray` implementing [`sdk.Backend`](file:///home/darkliquid/Projects/keyshell/internal/plugin/sdk/sdk.go) over stdin/stdout JSON-RPC.

While both models share almost identical data structures, there are two protocol omissions that prevent internal backends from cleanly converting to external plugins:
- In-process backends declare `Searchable: false` (e.g. `calc`, `run`), preventing them from polluting general search results. The external protocol [`proto.Hello`](file:///home/darkliquid/Projects/keyshell/internal/plugin/proto/proto.go) lacks a `searchable` field, causing the host to treat all configured external plugins as searchable.
- The host sends an `Init` message containing `Instance` and `Config json.RawMessage` (from `[[instances.config]]` in `config.toml`), but [`sdk.Serve`](file:///home/darkliquid/Projects/keyshell/internal/plugin/sdk/sdk.go) discards this payload, preventing external plugins from receiving dynamic instance-specific configuration.

---

## 2. Design Goals

1. **Protocol Parity**: Add `searchable` (boolean) to `proto.Hello` and `sdk.Info` so plugins can declare trigger-only participation without relying on host overrides.
2. **Dynamic Configuration Delivery**: Pass the `Init.Config` payload from the host through to the `sdk.Backend` implementation via a clean interface or callback.
3. **External Conversion Feasibility**: Prove that any built-in backend (such as `calc` or `run`) can be built as a standalone external binary with zero code changes to host ranking or launcher query fan-out.
4. **Auto-Discovery**: Support automatic plugin discovery in `$XDG_CONFIG_HOME/keyshell/plugins/` (and system directories) so users can drop binary or script plugins into a folder without manually adding `[[instances]]` entries in `config.toml`.

---

## 3. Detailed Specification

### 3.1. `Searchable` Protocol Extension

In [`internal/plugin/proto/proto.go`](file:///home/darkliquid/Projects/keyshell/internal/plugin/proto/proto.go):
```go
type Hello struct {
    Protocol   int      `json:"protocol"`
    ID         string   `json:"id"`
    Name       string   `json:"name"`
    Badge      string   `json:"badge"`
    Trigger    string   `json:"trigger"`
    Controls   []string `json:"controls"`
    Hot        bool     `json:"hot"`
    StableIDs  bool     `json:"stable_ids"`
    Searchable *bool    `json:"searchable,omitempty"` // nil means default (true)
}
```

In [`internal/plugin/sdk/sdk.go`](file:///home/darkliquid/Projects/keyshell/internal/plugin/sdk/sdk.go):
```go
type InfoShape struct {
    ID         string
    Name       string
    Badge      string
    Trigger    string
    Controls   []string
    Hot        bool
    StableIDs  bool
    Searchable bool // default true when not set or explicitly true
}
```

In [`internal/plugin/host.go`](file:///home/darkliquid/Projects/keyshell/internal/plugin/host.go):
When unpacking `Hello`, the host sets:
```go
searchable := true
if h.hello.Searchable != nil {
    searchable = *h.hello.Searchable
}
bInfo.Searchable = searchable
```

### 3.2. Instance Configuration Delivery

In [`internal/plugin/sdk/sdk.go`](file:///home/darkliquid/Projects/keyshell/internal/plugin/sdk/sdk.go):
Introduce an optional interface for plugins that accept instance configuration:
```go
type Configurable interface {
    Configure(instance string, config []byte) error
}
```
During the handshake in `sdk.Serve`:
```go
if conf, ok := backend.(Configurable); ok && len(msg.Init.Config) > 0 {
    if err := conf.Configure(msg.Init.Instance, msg.Init.Config); err != nil {
        return fmt.Errorf("sdk: configure: %w", err)
    }
}
```

### 3.3. Plugin Directory Auto-Discovery

In [`internal/cli/host.go`](file:///home/darkliquid/Projects/keyshell/internal/cli/host.go):
Add plugin scanning:
- Directories:
  - Linux: `$XDG_CONFIG_HOME/keyshell/plugins/` and `/usr/lib/keyshell/plugins/`
  - macOS: `~/Library/Application Support/keyshell/plugins/`
  - Windows: `%APPDATA%\keyshell\plugins\`
- For any executable found with a matching manifest or executing `plugin --info`:
  - Register as an available instance automatically.

---

## 4. Verification & Testing

- Unit tests for proto serialization/deserialization of `searchable`.
- Tests for `sdk.Serve` verifying `Configure` is called with the exact bytes from `Init.Config`.
- End-to-end launcher test verifying that an external plugin with `searchable: false` is not queried in merged search, but is queried when its trigger prefix is typed.
