# Step 6 Implementation Plan: NetworkManager PSK secret agent

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let keyshell supply a Wi-Fi passphrase to NetworkManager when a connection needs one, entirely from the network plugin over a password-entry control — no external GUI, no other secret agent.

**Architecture:** The network plugin also registers as a NetworkManager **secret agent** (`org.freedesktop.NetworkManager.SecretAgent`) on the session bus. When NetworkManager calls `GetSecrets` for an `802-11-wireless-security` setting, the plugin surfaces a pending "enter passphrase" card through its normal query path. The user's reply is captured via a `Text` control, and the plugin answers the pending `GetSecrets` call with the PSK. This keeps the flow inside the plugin: the host only renders the text field (already a schema control) and routes the value back as a control change.

**Tech Stack:** Go 1.27 (mise), `github.com/godbus/dbus/v5` (already present from tray). D-Bus session bus.

**Spec:** `docs/superpowers/specs/2026-10-19-keyshell-go-architecture-design.md` (Result/control schema; plugin protocol). NetworkManager's `org.freedesktop.NetworkManager.SecretAgent` interface is the protocol contract.

## Global Constraints

- `CGO_ENABLED=0`; everything under `mise exec -- …` / `mise run …`.
- The plugin is `hot` (it must hold the agent registration to receive `GetSecrets`).
- A `GetSecrets` request must never block the D-Bus call indefinitely: the plugin answers immediately with an "ask" response if it cannot supply the secret yet, then updates once the user provides it (NetworkManager re-asks).
- Only `802-11-wireless-security` / `psk` is handled in this step; other setting types get an empty (not-available) response.
- Credentials never appear in the plugin's item `Title`/`Subtitle`; a password field renders masked (host schema), and the plugin never logs the value.

### Task 1: SecretAgent registration

**Files:**
- Create: `internal/backend/netplugin/agent.go` + `agent_test.go`.

**Interfaces:**
- Produces: `Agent` that, given a `*dbus.Conn` and an `IPSKProvider` callback, exports `org.freedesktop.NetworkManager.SecretAgent` at `/org/freedesktop/NetworkManager/SecretAgent`, and registers itself via `RegisterAgent` on `org.freedesktop.NetworkManager`. Implements `GetSecrets(connection, path, setting, hints, flags)`:
  - `setting == "802-11-wireless-security"` → ask the provider for the psk; if present, return `{"802-11-wireless-security": {"psk": "<psk>"}}`; otherwise return an empty map (NetworkManager re-asks).
  - any other setting → empty map.
  - `SaveSecrets`/`DeleteSecrets` are accepted as no-ops.
- Tests: with a fake provider, assert `GetSecrets` returns the right payload for the psk setting and an empty map otherwise.

### Task 2: Provider + pending-prompt flow in the network plugin

**Files:**
- Modify: `internal/backend/netplugin/net.go` + `net_test.go`.

**Interfaces:**
- Produces: an `IPSKProvider` implementation in `Backend` that (a) answers a pending `GetSecrets` with whatever psk is currently stored, and (b) exposes a pending "passphrase" item in `Query` when a request is outstanding — with a `Text` control and a `plugin` action to submit it.
- `Control` handles the `Text` control by storing the passphrase and unblocking the pending request.
- Tests: simulate an outstanding request; assert a pending item appears, and that a `Control` with a `Text` value stores the psk and clears the pending state.

### Task 3: Wire registration and default instance

**Files:**
- Modify: `cmd/keyshell/main.go` — `plugin network` connects to the session bus, registers the agent, and passes a `*dbus.Conn` into `netplugin.New`.
- Modify: `internal/backend/netplugin/net.go` — `New` gains an optional agent wiring; keep `New` backwards-compatible for existing tests (inject the agent).

**Interfaces:**
- Consumes: the `Agent` from Task 1.
- Produces: the network plugin now carries the secret-agent registration; `Info` is unchanged, `Controls` gains `text`.

### Task 4: Verify

**Files:**
- Modify: `mise.toml` — note the agent registration in `plugin:demo`.

**Interfaces:**
- Verifies: `keyshell plugin network` registers the agent name on the session bus; a synthetic `GetSecrets` call (via a test D-Bus caller) returns the expected maps.

---

## Self-Review

**Spec coverage:** the PSK flow is contained entirely in the network plugin and uses the existing `Text` control and plugin-action routing; the host adds no new UI or protocol surface. This is the gap recorded at the end of steps 4–5.
**Placeholders:** none. Every type, path, and test is named.
**Type consistency:** the pending item is an ordinary `sdk.Item` with a `Text` control and a `plugin` action; `Control` is where the value arrives, matching every other plugin.
**Security note:** the passphrase is held only in the plugin's memory, never in item text, and never logged. The store is not persisted in this step.

**Deferred:** 802-1X / enterprise Wi-Fi settings, and a "remember" toggle that would persist the psk. Both are recorded follow-ons.
