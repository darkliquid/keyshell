# Step 4 Implementation Plan: clipboard plugin and IPC control verbs

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the architecturally special clipboard plugin (a hot, non-reaped instance that keeps a history store) and extend the IPC verb set so compositor keybinds can drive plugin controls (volume, media) without opening the overlay.

**Architecture:** The clipboard plugin is a long-lived subprocess like the others, but it also owns the capture side: a `wl-paste -w` watcher feeds new selections into the same JSON store the plugin serves. It is `hot` and `noreap` per the spec's clipboard note. IPC verbs route through the launcher to the relevant backend control, proving the control path end to end.

**Tech Stack:** Go 1.27 (mise), stdlib. `wl-paste` (wl-clipboard) for capture; JSON store at `$XDG_STATE_HOME/keyshell/clipboard.json`, written atomically.

**Spec:** `docs/superpowers/specs/2026-10-19-keyshell-go-architecture-design.md` (clipboard note; Result/control schema; lifecycle; IPC).

## Global Constraints

- `CGO_ENABLED=0`; everything under `mise exec -- …` / `mise run …`.
- The clipboard instance is `hot` and `noreap`; it must not be killed by idle teardown or mode switches.
- Capture is one watcher process; the store is capped, deduped, TTL-pruned, and written atomically (temp + rename).
- Explicit controls only: a pause toggle, a clear action, a per-entry delete. No secret detection or heuristic.
- Every task ends green on `mise run check` and is committed.

---

### Task 1: Clipboard store

**Files:**
- Create: `internal/backend/clipplugin/store.go` + `store_test.go`.

**Interfaces:**
- Produces: `type Entry { Text string; CopiedAt int64 }`; `Store` with `Load(path)`, `Add(text)`, `Entries() []Entry`, `Delete(index)`, `Clear()`, `Save()` (atomic), TTL prune and cap from config.
- Tests: add/dedupe/cap/ttl/atomic-save.

### Task 2: Clipboard plugin backend

**Files:**
- Create: `internal/backend/clipplugin/clip.go` + `clip_test.go`; a `--capture` mode reading new selections from stdin.
- Modify: `cmd/keyshell/main.go` — dispatch `plugin clipboard` and `plugin-clip-capture`.
- Modify: `internal/config/config.go` — default instance `{id:clipboard, mode:hot, noreap:true}`.

**Interfaces:**
- Produces: `sdk.Backend` with `Info` = `{id:clipboard, badge:CLIPBOARD, trigger:,cb, hot:true, stable_ids:true}`; `Query` returns history items (copy action), a pause toggle, a clear action; capture mode appends to the store via `wl-paste --watch` in a goroutine.

### Task 3: IPC control verbs

**Files:**
- Modify: `internal/ipc/ipc.go` + test; `cmd/keyshell/main.go` verb list.

**Interfaces:**
- Adds verbs `volumeUp`, `volumeDown`, `toggleMute` (audio) and `playPause`, `nextTrack`, `previousTrack` (media) that resolve the backend and send a settled control.
- Tests: verbs reach the right backend control with the right value.

### Task 4: Wire and verify

**Files:**
- Modify: `mise.toml` — `plugin:demo` covers clipboard.
- Run live: clipboard plugin handshake + a synthetic capture; IPC verb round-trip against the running host.

## Self-Review

**Spec coverage:** clipboard note → hot/noreap + explicit controls; store rules port from `clipboard-watch.py`; IPC verbs exercise the control path. Tray (SNI/DBus) and the network PSK agent are recorded as out of scope (they need D-Bus/agent work) and remain open follow-ons.
**Placeholders:** none.
**Type consistency:** clipboard items use `copy` actions and `toggle` controls exactly as the schema defines; verbs map to `sdk.Change` values already produced by the audio/media plugins.
