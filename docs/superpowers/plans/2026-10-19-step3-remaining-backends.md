# Step 3 Implementation Plan: remaining backends as plugins (media, network, power)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Port the media, network and power backends as first-party plugins, each owning its domain through a CLI tool and declaring controls — proving the audio plugin's pattern generalizes to different control kinds (choice, toggle) and richer item models.

**Architecture:** Identical shape to the audio plugin (`internal/backend/audioplugin`): an `sdk.Backend` that shells out to a domain tool, declares `Info`, and exposes controls. Each is enabled by default as a config instance; the host's existing hybrid provider runs them as subprocesses. Nothing in the host changes except `config` defaults and `cmd/keyshell`'s `runPlugin` dispatch.

**Tech Stack:** Go 1.27 (mise), stdlib only. Domain tools: `playerctl` (media), `nmcli` (network), `upower` + `powerprofilesctl` (power). All injectable for tests.

**Spec:** `docs/superpowers/specs/2026-10-19-keyshell-go-architecture-design.md` (Result/control schema; plugin protocol; lifecycle).

## Global Constraints

- `CGO_ENABLED=0`; everything under `mise exec -- …` / `mise run …`.
- Each backend is a separate package implementing `sdk.Backend`; the tool binary is injectable (a constructor param), so tests stub it.
- Controls are semantic kinds only: `range`, `toggle`, `choice`, `text`. No stylistic info.
- `stable_ids` is true only when item ids survive restarts (media player ids are stable; network ssid/device names are stable; power profiles are stable).
- Only `mise run check`-green, committed tasks count.

---

### Task 1: Media plugin (`playerctl`)

**Files:**
- Create: `internal/backend/mediaplugin/media.go` + `media_test.go`.
- Modify: `cmd/keyshell/main.go` — dispatch `plugin media`.
- Modify: `internal/config/config.go` — default instance `{id:media, mode:hot}`.

**Interfaces:**
- Produces: `sdk.Backend` whose `Info` = `{id:media, badge:MEDIA, trigger:>, controls:[range,choice], hot:true, stable_ids:true}`.
- `Query`: `playerctl -f "{{playerName}}\t{{status}}\t{{title}}\t{{artist}}" list` → one item per player; controls: `playback` (choice: play/pause/next/prev) and `position` (range, 0..duration) where available.
- `Control`: map `playback` choice → `playerctl play|pause|play-pause|next|previous --player <id>`; `position` → `playerctl position <sec> --player <id>`.
- Test: stub `playerctl` emitting canned list; assert items and control mapping.

### Task 2: Network plugin (`nmcli`)

**Files:**
- Create: `internal/backend/netplugin/net.go` + `net_test.go`.
- Modify: `cmd/keyshell/main.go` — dispatch `plugin network`.
- Modify: `internal/config/config.go` — default instance `{id:network}`.

**Interfaces:**
- Produces: `Info` = `{id:network, badge:NET, trigger:&, controls:[toggle,choice]}`.
- `Query`: `nmcli -t -f NAME,TYPE,DEVICE,STATE,ACTIVE connection show` → one item per connection; `connect` toggle reflects active state; a `wifi` choice lists available access points (`nmcli -t -f SSID,SIGNAL device wifi list`).
- `Control`: `connect` toggle → `nmcli connection up|down <name>`; `wifi` choice connects to the chosen SSID.
- Test: stub `nmcli`; assert item shape and control mapping. (Secret-agent/PSK entry is out of scope and deferred to step 4.)

### Task 3: Power plugin (`upower` + `powerprofilesctl`)

**Files:**
- Create: `internal/backend/powerplugin/power.go` + `power_test.go`.
- Modify: `cmd/keyshell/main.go` — dispatch `plugin power`.
- Modify: `internal/config/config.go` — default instance `{id:power}`.

**Interfaces:**
- Produces: `Info` = `{id:power, badge:POWER, trigger:%, controls:[choice]}`.
- `Query`: `upower -d` and `powerprofilesctl get` → one item with battery percentage (subtitle) and a `profile` choice (power-saver / balanced / performance) reflecting the active profile.
- `Control`: `profile` choice → `powerprofilesctl set <profile>`.
- Test: stub `upower`/`powerprofilesctl`; assert the item reads the battery and the choice maps to the set command.

### Task 4: Wire and verify

**Files:**
- Modify: `mise.toml` — extend `plugin:demo` to demo all three new plugins.
- Run the full suite and a live handshake for each new plugin via its stub JSON.

**Interfaces:**
- Verifies: `keyshell plugin media|network|power` complete hello→ready→query→done under a scripted `init`.

---

## Self-Review

**Spec coverage:** media/network/power map to the Result/control schema (`range`/`choice`/`toggle`) and the plugin transport; each is one package + one dispatch + one default instance. Deferred: network PSK agent, tray (SNI), clipboard — recorded in the ledger, addressed in step 4.
**Placeholders:** none; every type, command, and test file is named.
**Type consistency:** each backend returns `sdk.Item`/`sdk.Control` and consumes `sdk.Change`, matching the audio plugin and the SDK.
