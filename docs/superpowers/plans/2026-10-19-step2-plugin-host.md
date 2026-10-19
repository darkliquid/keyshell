# Step 2 Implementation Plan: plugin host, protocol, SDK, and the audio plugin

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A backend can run as a separate process speaking a line-delimited JSON protocol, supervised by the host, and the audio control becomes the first such plugin — while every in-process backend from step 1 keeps working.

**Architecture:** The host already programs against `backend.Backend`. This step adds a second adapter at that seam: `proc`, which owns a child process, performs the handshake, and translates `backend.Query`/`backend.Control` into protocol messages over stdio. The SDK is what plugin authors import. The audio plugin is the first-party proof: the same binary in plugin mode (`keyshell plugin audio`) uses `wpctl` to enumerate sinks and set volume/mute through range/toggle controls.

**Tech Stack:** Go 1.27 (mise), stdlib only for transport (`encoding/json`, `bufio`, `os/exec`). No new external deps.

**Spec:** `docs/superpowers/specs/2026-10-19-keyshell-go-architecture-design.md` (Plugin protocol + Lifecycle + Result/control schema sections are the binding authority).

## Global Constraints

- `CGO_ENABLED=0`; everything under `mise exec -- …` / `mise run …`.
- One JSON object per line, UTF-8, 1 MiB line cap; malformed/oversized lines are dropped, never fatal (protocol v1).
- Handshake: host sets `KEYSHELL_PLUGIN=1`, `KEYSHELL_PROTOCOL=1`, `KEYSHELL_INSTANCE=<id>`; plugin sends `hello`; host sends `init`; plugin sends `ready`.
- One query in flight per instance, newest wins; `cancel` sent when superseded; stale ids ignored.
- `ping`/`pong` every `pluginPingIntervalMs` (10s); three unanswered pings restart. Restart backoff 1s/2s/4s/8s, at most 5 attempts in 60s, then mode reports unavailable.
- Host exit: `SIGTERM`, then `SIGKILL` after `pluginExitGraceMs` (2s).
- `protocol` mismatch refuses load and surfaces in the mode's status line.
- Controls are settled by the host before `control` is sent; the plugin replies with `state` for reconciliation (last-write-wins).
- Every task ends with a green `mise run check` and a commit.

---

### Task 1: Protocol messages and framing

**Files:**
- Create: `internal/plugin/proto/proto.go` — the message types (`Hello`, `Init`, `Ready`, `Query`, `Item`, `Done`, `Error`, `Cancel`, `Control`, `State`, `Invoke`, `Ping`, `Pong`) plus a `Decode([]byte) (Message, error)` that routes on the single top-level key.
- Create: `internal/plugin/proto/proto_test.go` — round-trip encode/decode, and unknown/malformed lines dropped.

**Interfaces:**
- Produces: `proto.Protocol = 1`; `proto.Message` covering every wire message; `proto.Decode`.
- Consumes: nothing (pure).

### Task 2: The SDK

**Files:**
- Create: `internal/plugin/sdk/sdk.go` — `Serve(info Info, h Handler)`, where `Handler` exposes `Query(ctx, Query) (Stream, error)` and `Control(ctx, Change) error`; the SDK owns the handshake, the read loop, and reply encoding. Also `NewStream(items ...Item) Stream` and helpers.
- Create: `internal/plugin/sdk/sdk_test.go`.

**Interfaces:**
- Produces: `sdk.Info`, `sdk.Handler`, `sdk.Serve`, `sdk.Stream`, `sdk.Item` (alias/mirror of `backend.Item`, `backend.Control`, `backend.Query`, `backend.Change`). Plugin authors import only this package (and `backend` types are re-exported for convenience).
- Consumes: `internal/plugin/proto`.

### Task 3: Host process wrapper (the `proc` adapter)

**Files:**
- Create: `internal/plugin/host.go` — `Host` implements `backend.Backend` over a child process: spawn, handshake, query streaming, control, ping supervision, restart backoff, teardown.
- Create: `internal/plugin/host_test.go` — against a fake plugin binary (a shell script or a small Go helper in `testdata/`): happy path, garbage lines, protocol mismatch, slow init, crash/restart, ping timeout.

**Interfaces:**
- Produces: `Host` (implements `backend.Backend`), and a `Manager` that tracks instances (hot/lazy/auto), applies idle teardown and restart backoff.
- Consumes: `backend`, `config`, `proto`.

### Task 4: Provider integration

**Files:**
- Modify: `cmd/keyshell/main.go` — build a provider that serves configured plugin instances (starting with `audio`) and falls back to in-process backends for the rest.
- Modify: `internal/config` — add `[[instance]]` declarations (`id`, `command`, `args`, `mode`, `noreap`).
- Create: `internal/config/config_instance_test.go`.

**Interfaces:**
- Consumes: `Host`/`Manager`, `backend.NewInProc`.
- Produces: a single `backend.BackendProvider` the launcher queries as before.

### Task 5: The audio plugin

**Files:**
- Create: `cmd/keyshell/plugin_audio.go` (or `internal/audioplugin`), run via `keyshell plugin audio` when `KEYSHELL_PLUGIN=1`.
- Enumerate sinks with `wpctl status`; emit one item per sink with a `range` control (volume, 0..1 step 0.01) and a `toggle` (mute); handle `Control` by `wpctl set-volume` / `wpctl set-mute`.
- Create: the plugin's own unit tests with a stub `wpctl` script.

**Interfaces:**
- Consumes: `sdk`.
- Produces: `hello` with `controls:["range","toggle"]`, `stable_ids:true` (sink ids are stable), `hot` hint.

### Task 6: End-to-end verification

**Files:**
- Modify: `mise.toml` — `run` / `test` still cover everything; add a `plugin:demo` task that runs the audio plugin standalone for a manual check.

**Interfaces:**
- Verifies: host spawns `keyshell plugin audio`, handshake completes, a query returns sink items, a control round-trip changes volume (verified against `wpctl get-volume`), and teardown is clean.

---

## Self-Review

**Spec coverage:**
- Protocol messages + framing ✓ Task 1; SDK ✓ Task 2; host/manager supervision ✓ Task 3; provider/config wiring ✓ Task 4; audio plugin ✓ Task 5; e2e ✓ Task 6.
- Lifecycle knobs (ping interval, backoff, grace, idle) are config values in `config` and honored by `Manager`.
- The destructive gate stays host-side; `invoke`/domain actions are parsed and surfaced but not executed in step 2 (audio uses controls, not domain actions), matching "controls first".
- **Placeholders:** none. Every type, constant, and test file is named.
- **Type consistency:** SDK mirrors `backend`, types so plugins and host share shapes; `proto` is the only wire format.
