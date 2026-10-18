# Clipboard History Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Keep a bounded, searchable history of text you have copied, reachable from the launcher, with explicit controls over capture and retention.

**Architecture:** A helper script captures one entry per clipboard change and owns the store, so all the retention and validation rules live in plain Python that a test can drive directly. A `ClipboardBackend` singleton owns the watcher process, reads the store, and serves items in its own mode. The watcher is the first process in this codebase that must keep running while the launcher is hidden.

**Tech Stack:** Quickshell 0.3.2 (Qt 6 QML), `Quickshell.Io` (`Process`, `FileView`, `Timer`), `wl-clipboard` (`wl-paste --watch`), Python 3 for the helper, `node` for the test harness.

**Spec:** `docs/superpowers/specs/2026-10-18-clipboard-history-design.md`

## Global Constraints

- Quickshell 0.3.2+ floor; target niri, degrade gracefully on other Wayland compositors.
- No hardcoded magic numbers: every cap, duration and path belongs in `Config.qml`.
- No inline hex colours; use `Theme.*`. Outlines 2-3px, shadows solid offset, no rounded pills or blurs.
- Every global manager is a `pragma Singleton`.
- Never use em dashes in source files.
- `wl-copy` parks in the foreground to serve the selection, so always invoke it through `Quickshell.execDetached` and never in a way the shell waits on. Verification commands must wrap clipboard calls in `timeout`.
- The watcher is deliberately **not** stopped by `Launcher.dismiss()` or by `BackendManager.setMode()`; only `P` stops it. Do not add it to the plugin cleanup path.
- The secret policy is explicit controls only. Do not add app-name heuristics, secret detection, or any wording that implies a guarantee.
- Commit steps: create the commit shown **only** when the user has authorised committing. If they have not, stop after the verification step and ask. Never push to a remote.

---

### Task 1: Config tunables

**Files:**
- Modify: `quickshell/keyshell/Config.qml`

**Interfaces:**
- Consumes: nothing.
- Produces: `Config.clipboardTrigger`, `Config.maxClipboardEntries`, `Config.clipboardTtlMinutes`, `Config.maxClipboardEntryBytes`, `Config.clipboardRestartDelayMs`, `Config.clipboardMaxRestarts`.

- [ ] **Step 1: Add the tunables**

In `Config.qml`, insert after the `pluginSearchDirs` property block:

```qml
    readonly property string clipboardTrigger: ",cb"
    readonly property int maxClipboardEntries: 50
    readonly property int clipboardTtlMinutes: 1440
    readonly property int maxClipboardEntryBytes: 4096
    readonly property int clipboardRestartDelayMs: 2000
    readonly property int clipboardMaxRestarts: 3
```

- [ ] **Step 2: Verify the shell still loads**

Run: `timeout 8 qs -p quickshell/keyshell 2>&1 | tail -4`
Expected: `Configuration Loaded`, no QML errors.

- [ ] **Step 3: Commit (only if authorised)**

```bash
git add quickshell/keyshell/Config.qml
git commit -m "Add clipboard history settings"
```

---

### Task 2: The capture helper

**Files:**
- Create: `quickshell/keyshell/clipboard-watch.py`
- Modify: `tests/run.js`

**Interfaces:**
- Consumes: nothing.
- Produces: an executable that reads the selection on stdin and updates the store. Environment contract: `KEYSHELL_CLIPBOARD_STORE` (path), `KEYSHELL_CLIPBOARD_MAX_ENTRIES`, `KEYSHELL_CLIPBOARD_TTL_MINUTES`, `KEYSHELL_CLIPBOARD_MAX_BYTES`. The store is a JSON array of `{"text": string, "copiedAt": epochMillis}`, newest first.

- [ ] **Step 1: Write the failing test**

In `tests/run.js`, add the requires beside the existing ones:

```javascript
const fs = require("fs");
const os = require("os");
```

and append before the final `console.log`:

```javascript
// --- clipboard helper: retention and validation live here ---
const clipboardHelper = path.join(__dirname, "..", "quickshell", "keyshell", "clipboard-watch.py");

function clipboardStorePath(dir) {
    return path.join(dir, "clipboard.json");
}

function capture(dir, payload, limits) {
    const settings = limits || {};
    execFileSync(clipboardHelper, [], {
        input: payload,
        encoding: "utf8",
        timeout: 20000,
        env: Object.assign({}, process.env, {
            KEYSHELL_CLIPBOARD_STORE: clipboardStorePath(dir),
            KEYSHELL_CLIPBOARD_MAX_ENTRIES: String(settings.maxEntries || 50),
            KEYSHELL_CLIPBOARD_TTL_MINUTES: String(settings.ttlMinutes === undefined ? 1440 : settings.ttlMinutes),
            KEYSHELL_CLIPBOARD_MAX_BYTES: String(settings.maxBytes || 4096)
        })
    });
    try {
        return JSON.parse(fs.readFileSync(clipboardStorePath(dir), "utf8"));
    } catch (e) {
        return null;
    }
}

const clipDir = fs.mkdtempSync(path.join(os.tmpdir(), "keyshell-clip-"));

const firstStore = capture(clipDir, "first entry");
assert.strictEqual(firstStore.length, 1, "a first capture is stored");
assert.strictEqual(firstStore[0].text, "first entry", "the text is stored verbatim");
assert.strictEqual(typeof firstStore[0].copiedAt, "number", "a timestamp is stored");

const secondStore = capture(clipDir, "second entry");
assert.strictEqual(secondStore.length, 2, "a second capture is added");
assert.strictEqual(secondStore[0].text, "second entry", "the newest entry is first");

const dedupedStore = capture(clipDir, "second entry");
assert.strictEqual(dedupedStore.length, 2, "identical content is not duplicated");

const promotedStore = capture(clipDir, "first entry");
assert.strictEqual(promotedStore.length, 2, "re-copying an older entry does not duplicate it");
assert.strictEqual(promotedStore[0].text, "first entry", "re-copying an older entry promotes it to the front");

const capDir = fs.mkdtempSync(path.join(os.tmpdir(), "keyshell-clip-"));
capture(capDir, "one", { maxEntries: 2 });
capture(capDir, "two", { maxEntries: 2 });
const cappedStore = capture(capDir, "three", { maxEntries: 2 });
assert.strictEqual(cappedStore.length, 2, "the entry cap is enforced");
assert.strictEqual(cappedStore[0].text, "three", "the newest entry survives the cap");

const emptyStore = capture(capDir, "   ");
assert.strictEqual(emptyStore.length, 2, "a blank payload is ignored");

const oversizeStore = capture(capDir, "x".repeat(200), { maxBytes: 32 });
assert.strictEqual(oversizeStore.length, 2, "an oversize payload is rejected");

const binaryStore = capture(capDir, Buffer.from([0xff, 0xfe, 0x00, 0x01]));
assert.strictEqual(binaryStore.length, 2, "a payload that is not UTF-8 text is rejected");

const ttlDir = fs.mkdtempSync(path.join(os.tmpdir(), "keyshell-clip-"));
capture(ttlDir, "old entry", { ttlMinutes: 1440 });
const ttlStore = capture(ttlDir, "new entry", { ttlMinutes: 0 });
assert.strictEqual(ttlStore.length, 1, "entries older than the TTL are pruned");
assert.strictEqual(ttlStore[0].text, "new entry", "the surviving entry is the fresh one");
```

- [ ] **Step 2: Run it to make sure it fails**

Run: `node tests/run.js`
Expected: FAIL with `spawnSync ... clipboard-watch.py ENOENT`.

- [ ] **Step 3: Write the minimal implementation**

Create `quickshell/keyshell/clipboard-watch.py`:

```python
#!/usr/bin/env python3
"""Appends one clipboard entry to the keyshell clipboard history store.

Run by `wl-paste --watch`, once per selection change: the selection arrives on
stdin and this process exits as soon as it has recorded it. All of the
retention and validation rules live here rather than in QML, so a test can drive
them directly.

Environment: KEYSHELL_CLIPBOARD_STORE, KEYSHELL_CLIPBOARD_MAX_ENTRIES,
KEYSHELL_CLIPBOARD_TTL_MINUTES, KEYSHELL_CLIPBOARD_MAX_BYTES.
"""

import json
import os
import sys
import tempfile
import time

DEFAULT_MAX_ENTRIES = 50
DEFAULT_TTL_MINUTES = 1440
DEFAULT_MAX_BYTES = 4096


def env_int(name, default):
    raw = os.environ.get(name, "")
    try:
        return int(raw)
    except (TypeError, ValueError):
        return default


def store_path():
    configured = os.environ.get("KEYSHELL_CLIPBOARD_STORE", "")
    if configured:
        return configured
    state = os.environ.get("XDG_STATE_HOME") or os.path.join(
        os.environ.get("HOME", ""), ".local", "state"
    )
    return os.path.join(state, "keyshell", "clipboard.json")


def load(path):
    try:
        with open(path, "r", encoding="utf-8") as handle:
            data = json.load(handle)
    except (OSError, ValueError):
        return []
    if not isinstance(data, list):
        return []

    entries = []
    for item in data:
        if not isinstance(item, dict):
            continue
        text = item.get("text")
        copied_at = item.get("copiedAt")
        if isinstance(text, str) and isinstance(copied_at, (int, float)):
            entries.append({"text": text, "copiedAt": copied_at})
    return entries


def prune(entries, now, ttl_minutes, max_entries):
    cutoff = now - ttl_minutes * 60000
    fresh = [entry for entry in entries if entry["copiedAt"] >= cutoff]
    fresh.sort(key=lambda entry: entry["copiedAt"], reverse=True)
    return fresh[:max_entries]


def save(path, entries):
    directory = os.path.dirname(path)
    if directory:
        os.makedirs(directory, exist_ok=True)
    handle, temporary = tempfile.mkstemp(dir=directory, prefix=".clipboard-", suffix=".tmp")
    try:
        with os.fdopen(handle, "w", encoding="utf-8") as stream:
            json.dump(entries, stream, ensure_ascii=False)
        os.replace(temporary, path)
    except BaseException:
        try:
            os.unlink(temporary)
        except OSError:
            pass
        raise


def main():
    max_bytes = env_int("KEYSHELL_CLIPBOARD_MAX_BYTES", DEFAULT_MAX_BYTES)
    raw = sys.stdin.buffer.read(max_bytes + 1)
    if len(raw) > max_bytes:
        return

    try:
        text = raw.decode("utf-8")
    except UnicodeDecodeError:
        return

    if text.strip() == "":
        return

    now = int(time.time() * 1000)
    path = store_path()

    entries = [
        entry for entry in load(path) if entry["text"] != text
    ]
    entries.insert(0, {"text": text, "copiedAt": now})

    max_entries = env_int("KEYSHELL_CLIPBOARD_MAX_ENTRIES", DEFAULT_MAX_ENTRIES)
    ttl_minutes = env_int("KEYSHELL_CLIPBOARD_TTL_MINUTES", DEFAULT_TTL_MINUTES)
    save(path, prune(entries, now, ttl_minutes, max_entries))


if __name__ == "__main__":
    main()
```

Then: `chmod +x quickshell/keyshell/clipboard-watch.py`

- [ ] **Step 4: Run the tests and make sure they pass**

Run: `node tests/run.js`
Expected: PASS, prints `all assertions passed`.

- [ ] **Step 5: Commit (only if authorised)**

```bash
git add tests/run.js quickshell/keyshell/clipboard-watch.py
git commit -m "Capture clipboard entries with tested retention rules"
```

---

### Task 3: The clipboard card

**Files:**
- Create: `quickshell/keyshell/components/ClipboardCardDelegate.qml`

**Interfaces:**
- Consumes: the item fields `title`, `subtitle`, `glyph`.
- Produces: a delegate for clipboard items.

- [ ] **Step 1: Create the delegate**

Create `quickshell/keyshell/components/ClipboardCardDelegate.qml`, following the house idiom (`PluginCardDelegate.qml` is the closest sibling):

```qml
import QtQuick
import Quickshell
import qs

Item {
    id: delegate

    readonly property var card: parent ? parent.card : null
    readonly property var itemData: card ? card.entry : null
    readonly property bool active: card ? card.active : false

    anchors.fill: parent
    anchors.margins: 10

    Item {
        id: middleRow
        anchors.top: parent.top
        anchors.bottom: footerText.top
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.bottomMargin: 6

        Text {
            id: glyphText
            anchors.left: parent.left
            anchors.top: parent.top
            anchors.topMargin: 2
            width: 32
            horizontalAlignment: Text.AlignHCenter
            text: delegate.itemData && delegate.itemData.glyph ? delegate.itemData.glyph : "📋"
            font: Theme.heavy(20)
            color: delegate.active ? Theme.slab : Theme.ink
            renderType: Text.QtRendering
        }

        Text {
            id: entryText
            anchors.left: glyphText.right
            anchors.right: parent.right
            anchors.top: parent.top
            anchors.bottom: ageText.top
            anchors.leftMargin: 8
            anchors.bottomMargin: 4
            wrapMode: Text.WrapAtWordBoundaryOrAnywhere
            maximumLineCount: 3
            elide: Text.ElideRight
            verticalAlignment: Text.AlignTop
            text: delegate.itemData ? delegate.itemData.title : ""
            font: Theme.heavy(11)
            color: delegate.active ? Theme.slab : Theme.ink
            renderType: Text.QtRendering
        }

        Text {
            id: ageText
            anchors.left: glyphText.right
            anchors.right: parent.right
            anchors.bottom: parent.bottom
            anchors.leftMargin: 8
            maximumLineCount: 1
            elide: Text.ElideRight
            text: delegate.itemData ? delegate.itemData.subtitle : ""
            font: Theme.heavy(9)
            color: delegate.active ? Theme.slab : Theme.muted
            renderType: Text.QtRendering
        }
    }

    Text {
        id: footerText
        anchors.bottom: parent.bottom
        anchors.left: parent.left
        anchors.right: parent.right
        text: "[ENTER] COPY  //  [X] DELETE"
        font: Theme.heavy(9)
        color: delegate.active ? Theme.slab : Theme.muted
        horizontalAlignment: Text.AlignHCenter
        elide: Text.ElideRight
        renderType: Text.QtRendering
    }
}
```

- [ ] **Step 2: Verify it parses**

Run: `qmllint quickshell/keyshell/components/ClipboardCardDelegate.qml; echo "exit=$?"`
Expected: `exit=0`.

- [ ] **Step 3: Commit (only if authorised)**

```bash
git add quickshell/keyshell/components/ClipboardCardDelegate.qml
git commit -m "Add a card for clipboard entries"
```

---

### Task 4: The clipboard backend

**Files:**
- Create: `quickshell/keyshell/ClipboardBackend.qml`

**Interfaces:**
- Consumes: `Config.clipboard*`, the helper from Task 2, the delegate from Task 3.
- Produces: the backend contract — `id`, `name`, `badge`, `prefix`, `searchable`, `trackable`, `cardDelegate`, `search(query, limit)`, `refresh()`, `signal resultsReady(string query, var items)` — plus `paused`, `unavailable`, `togglePause()`, `deleteEntry(itemId)`, `clearHistory()`.

- [ ] **Step 1: Create the singleton**

Create `quickshell/keyshell/ClipboardBackend.qml`:

```qml
pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Io
import qs.components

Singleton {
    id: clipboardBackend

    readonly property string id: "clipboard"
    readonly property string name: "Clipboard"
    readonly property string badge: "CLIP"
    readonly property string prefix: Config.clipboardTrigger
    readonly property bool searchable: false
    readonly property bool trackable: false
    readonly property Component cardDelegate: ClipboardCardDelegate {}

    readonly property string storeDir: (Quickshell.env("XDG_STATE_HOME") || (Quickshell.env("HOME") + "/.local/state")) + "/keyshell"
    readonly property string storePath: storeDir + "/clipboard.json"
    readonly property string helperPath: Quickshell.shellDir + "/clipboard-watch.py"

    property bool paused: false
    property bool unavailable: false
    property var entries: []
    property int restartCount: 0
    property real watchStartedAt: 0

    signal resultsReady(string query, var items)

    Process {
        id: watcher
        command: ["wl-paste", "--type", "text", "--watch", clipboardBackend.helperPath]
        running: false

        environment: ({
            "KEYSHELL_CLIPBOARD_STORE": clipboardBackend.storePath,
            "KEYSHELL_CLIPBOARD_MAX_ENTRIES": String(Config.maxClipboardEntries),
            "KEYSHELL_CLIPBOARD_TTL_MINUTES": String(Config.clipboardTtlMinutes),
            "KEYSHELL_CLIPBOARD_MAX_BYTES": String(Config.maxClipboardEntryBytes)
        })

        onStarted: clipboardBackend.watchStartedAt = Date.now()

        onExited: (exitCode, exitStatus) => {
            if (clipboardBackend.paused)
                return;
            if (Date.now() - clipboardBackend.watchStartedAt < 1000)
                clipboardBackend.restartCount++;
            else
                clipboardBackend.restartCount = 0;
            if (clipboardBackend.restartCount > Config.clipboardMaxRestarts) {
                clipboardBackend.unavailable = true;
                return;
            }
            restartTimer.restart();
        }
    }

    Timer {
        id: restartTimer
        interval: Config.clipboardRestartDelayMs
        repeat: false
        onTriggered: {
            if (!clipboardBackend.paused && !clipboardBackend.unavailable)
                watcher.running = true;
        }
    }

    FileView {
        id: storeFile
        path: clipboardBackend.storePath
        blockLoading: true
        onLoaded: clipboardBackend.adopt(storeFile.text())
        onLoadFailed: clipboardBackend.entries = []
    }

    Component.onCompleted: {
        clipboardBackend.startWatch();
        clipboardBackend.reload();
    }

    function startWatch() {
        if (!watcher.running && !unavailable)
            watcher.running = true;
    }

    function reload() {
        storeFile.reload();
    }

    function adopt(raw) {
        let parsed = [];
        try {
            const data = JSON.parse(raw);
            if (Array.isArray(data))
                parsed = data;
        } catch (e) {
            parsed = [];
        }

        const now = Date.now();
        const cutoff = now - Config.clipboardTtlMinutes * 60000;
        const kept = [];
        for (let i = 0; i < parsed.length; i++) {
            const entry = parsed[i];
            if (!entry || typeof entry.text !== "string" || typeof entry.copiedAt !== "number")
                continue;
            if (entry.copiedAt < cutoff)
                continue;
            kept.push(entry);
        }
        kept.sort(function (a, b) { return b.copiedAt - a.copiedAt; });
        const trimmed = kept.slice(0, Config.maxClipboardEntries);

        entries = trimmed;
        if (trimmed.length !== parsed.length)
            storeFile.setText(JSON.stringify(trimmed));

        clipboardBackend.emit();
    }

    function emit() {
        resultsReady(Launcher.query, clipboardBackend.search(Launcher.query, Config.maxResults));
    }

    function relativeAge(copiedAt) {
        const seconds = Math.max(0, Math.round((Date.now() - copiedAt) / 1000));
        if (seconds < 60)
            return "just now";
        const minutes = Math.round(seconds / 60);
        if (minutes < 60)
            return minutes === 1 ? "1 minute ago" : (minutes + " minutes ago");
        const hours = Math.round(minutes / 60);
        if (hours < 24)
            return hours === 1 ? "1 hour ago" : (hours + " hours ago");
        const days = Math.round(hours / 24);
        return days === 1 ? "yesterday" : (days + " days ago");
    }

    function toItem(entry) {
        const oneLine = entry.text.replace(/\s+/g, " ").trim();
        return {
            "id": "clip:" + entry.copiedAt,
            "title": oneLine,
            "subtitle": clipboardBackend.relativeAge(entry.copiedAt),
            "glyph": "📋",
            "hint": "[ENTER] COPY  //  [X] DELETE",
            "icon": "",
            "previewSource": null,
            "badge": "CLIP",
            "delegate": clipboardBackend.cardDelegate,
            "actions": [
                {
                    "label": "Copy",
                    "execute": function () {
                        Quickshell.execDetached(["wl-copy", entry.text]);
                    }
                }
            ]
        };
    }

    function search(q, limit) {
        const query = (q || "").toLowerCase();
        const items = [];
        for (let i = 0; i < entries.length && items.length < limit; i++) {
            const entry = entries[i];
            if (query.length > 0 && entry.text.toLowerCase().indexOf(query) === -1)
                continue;
            items.push(toItem(entry));
        }
        return items;
    }

    function refresh() {
        reload();
    }

    function writeEntries(list) {
        entries = list;
        storeFile.setText(JSON.stringify(list));
        clipboardBackend.emit();
    }

    function togglePause() {
        paused = !paused;
        if (paused) {
            if (watcher.running)
                watcher.running = false;
            return;
        }
        unavailable = false;
        restartCount = 0;
        clipboardBackend.startWatch();
    }

    function deleteEntry(itemId) {
        const kept = [];
        for (let i = 0; i < entries.length; i++) {
            if (("clip:" + entries[i].copiedAt) !== itemId)
                kept.push(entries[i]);
        }
        clipboardBackend.writeEntries(kept);
    }

    function clearHistory() {
        clipboardBackend.writeEntries([]);
    }
}
```

- [ ] **Step 2: Verify it parses and `Quickshell.shellDir` resolves**

Run: `qmllint quickshell/keyshell/ClipboardBackend.qml; echo "exit=$?"`
Expected: `exit=0`.

Then confirm the helper path is right, because everything depends on it:

```bash
printf '%s\n' "$(cd "$(dirname "$(find ~/.local/share/quickshell ~/.config/quickshell -name shell.qml 2>/dev/null | head -1)")" 2>/dev/null && pwd)" 2>/dev/null; ls quickshell/keyshell/clipboard-watch.py
```

Expected: the helper exists beside `shell.qml`. If `Quickshell.shellDir` turns out not to point at the config directory, stop and report it rather than guessing a replacement.

- [ ] **Step 3: Commit (only if authorised)**

```bash
git add quickshell/keyshell/ClipboardBackend.qml
git commit -m "Keep a clipboard watcher alive across the shell's lifetime"
```

---

### Task 5: Register the backend

**Files:**
- Modify: `quickshell/keyshell/BackendManager.qml`

**Interfaces:**
- Consumes: `ClipboardBackend` (Task 4).
- Produces: the clipboard mode reachable by trigger and `Tab`.

- [ ] **Step 1: Add it to the built-ins**

In the `builtinBackends` array, append `ClipboardBackend`:

```qml
    readonly property var builtinBackends: [AppBackend, WindowBackend, FileBackend, AudioBackend, MediaBackend, TrayBackend, BluetoothBackend, PowerBackend, NetworkBackend, CalculatorBackend, RunBackend, ClipboardBackend]
```

- [ ] **Step 2: Verify the mode is reachable and the watcher started**

Run: `timeout 12 qs -p quickshell/keyshell` in the background, then:

```bash
qs -p quickshell/keyshell ipc call keyshell isOpen
qs -p quickshell/keyshell ipc call keyshell openMode "clipboard"
sleep 2
qs -p quickshell/keyshell ipc call keyshell isOpen
pgrep -af "clipboard-watch.py" || echo "watcher not running"
pgrep -af "wl-paste --watch" || echo "watch process not running"
```

Expected: `false`, then `true`, and both greps find a process. If the watch process is absent, check the log for a `Process` error before proceeding.

- [ ] **Step 3: Commit (only if authorised)**

```bash
git add quickshell/keyshell/BackendManager.qml
git commit -m "Register the clipboard mode"
```

---

### Task 6: Mode keys

**Files:**
- Modify: `quickshell/keyshell/components/LauncherWindow.qml` (insert after the network mode block, immediately before `const blocked = Qt.ControlModifier | ...`)

**Interfaces:**
- Consumes: `ClipboardBackend.togglePause()`, `deleteEntry(itemId)`, `clearHistory()` (Task 4).
- Produces: `P`, `C` and `X` in the clipboard mode.

- [ ] **Step 1: Add the clipboard key block**

Insert immediately after the closing brace of the network mode block and before `const blocked = Qt.ControlModifier | Qt.AltModifier | Qt.MetaModifier;`:

```qml
            if (BackendManager.activeMode === "clipboard") {
                if (event.key === Qt.Key_P) {
                    ClipboardBackend.togglePause();
                    event.accepted = true;
                    return;
                }
                if (event.key === Qt.Key_C) {
                    ClipboardBackend.clearHistory();
                    event.accepted = true;
                    return;
                }
                if (event.key === Qt.Key_X) {
                    const curClip = (Launcher.selectedIndex >= 0 && Launcher.selectedIndex < Launcher.results.length) ? Launcher.results[Launcher.selectedIndex] : null;
                    if (curClip)
                        ClipboardBackend.deleteEntry(curClip.id);
                    event.accepted = true;
                    return;
                }
            }
```

The block must sit with the other mode blocks, before the printable-character path, so these keys act instead of being typed into the query.

- [ ] **Step 2: Verify pausing actually stops capture**

Run: `timeout 90 qs -p quickshell/keyshell` in the background, then:

```bash
C="qs -p quickshell/keyshell ipc call keyshell"
$C openMode "clipboard" >/dev/null
sleep 3
wdotool key p >/dev/null 2>&1
sleep 2
pgrep -af "wl-paste --watch" || echo "capture paused: watch process gone"
wdotool key p >/dev/null 2>&1
sleep 2
pgrep -af "wl-paste --watch" || echo "still paused"
```

Expected: the first grep finds nothing after `P`, and the second finds a process after `P` again. The launcher must be focused for `wdotool` to reach it; click on it first if the keys do nothing.

- [ ] **Step 3: Commit (only if authorised)**

```bash
git add quickshell/keyshell/components/LauncherWindow.qml
git commit -m "Add clear, delete and pause keys for the clipboard mode"
```

---

### Task 7: Status hint

**Files:**
- Modify: `quickshell/keyshell/components/SearchField.qml` (the `matchCountText` binding)

**Interfaces:**
- Consumes: `ClipboardBackend.paused`, `ClipboardBackend.entries`.
- Produces: a status line that says what is happening rather than nothing.

- [ ] **Step 1: Add a clipboard branch**

Inside the chain of `else if (mode === ...)` branches that builds `label`, add:

```qml
            } else if (mode === "clipboard") {
                if (ClipboardBackend.paused)
                    label = "CAPTURE PAUSED  //  [P] RESUME";
                else if (ClipboardBackend.entries.length === 0)
                    label = "NO CLIPBOARD ENTRIES";
                else if (count === 0)
                    label = "NO MATCHES";
            } else {
```

That is, insert the branch so the existing final `else` still handles the rest.

- [ ] **Step 2: Verify it parses and loads**

Run: `qmllint quickshell/keyshell/components/SearchField.qml; echo "exit=$?"` then `timeout 8 qs -p quickshell/keyshell 2>&1 | tail -4`
Expected: `exit=0`, then `Configuration Loaded`.

- [ ] **Step 3: Commit (only if authorised)**

```bash
git add quickshell/keyshell/components/SearchField.qml
git commit -m "Say when clipboard capture is paused or empty"
```

---

### Task 8: Documentation

**Files:**
- Modify: `AGENTS.md`
- Modify: `README.md`

**Interfaces:**
- Consumes: everything above.
- Produces: no code interface.

- [ ] **Step 1: Document the policy in `AGENTS.md`**

Add `ClipboardBackend.qml` and `clipboard-watch.py` to the architecture tree and `ClipboardCardDelegate.qml` to the component list. Add a subsection covering: the watcher is the one process that outlives the launcher and must not be added to the dismiss path; the store and its retention; and the policy in plain words, being explicit that nothing detects secrets, that no app-name heuristics exist, and that the controls are bounded retention plus an explicit pause and delete.

- [ ] **Step 2: Document usage in `README.md`**

Add a clipboard section: the `,cb` trigger, Enter to copy an entry back, `X` to delete one, `C` to clear, `P` to pause, the store path, and the honest caveat that copied passwords can be kept and that the pause and clear keys are the protection.

- [ ] **Step 3: Run the harness one final time**

Run: `node tests/run.js`
Expected: PASS, prints `all assertions passed`.

- [ ] **Step 4: Commit (only if authorised)**

```bash
git add AGENTS.md README.md
git commit -m "Explain clipboard history and what it cannot protect you from"
```

---

### Task 9: End-to-end verification

**Files:**
- No source changes unless a defect is found.

**Interfaces:**
- Consumes: everything above.
- Produces: evidence, not code.

- [ ] **Step 1: Start from a clean store**

```bash
STORE="${XDG_STATE_HOME:-$HOME/.local/state}/keyshell/clipboard.json"
rm -f "$STORE"
```

- [ ] **Step 2: Copy something and find it in the mode**

Start `timeout 180 qs -p quickshell/keyshell` in the background, then:

```bash
C="qs -p quickshell/keyshell ipc call keyshell"
timeout 5 sh -c "printf 'planck-constant-1234' | wl-copy"
sleep 3
$C openMode "clipboard" >/dev/null; sleep 3
echo "isOpen: $($C isOpen)"
timeout 4 wl-paste
```

Expected: `isOpen: true` and `planck-constant-1234`. Then confirm Entry copied it back: seed the clipboard with something else, press Enter in the mode, and confirm the entry's text is on the clipboard again.

- [ ] **Step 3: Confirm pause actually prevents capture**

```bash
wdotool key p >/dev/null 2>&1; sleep 2
timeout 5 sh -c "printf 'while-paused-5678' | wl-copy"
sleep 3
grep -c "while-paused-5678" "${XDG_STATE_HOME:-$HOME/.local/state}/keyshell/clipboard.json" || echo "not stored, as intended"
wdotool key p >/dev/null 2>&1; sleep 2
timeout 5 sh -c "printf 'after-resume-9012' | wl-copy"
sleep 3
grep -c "after-resume-9012" "${XDG_STATE_HOME:-$HOME/.local/state}/keyshell/clipboard.json" || echo "not stored"
```

Expected: `while-paused-5678` is absent (the `grep -c` prints 0), and `after-resume-9012` is present (prints 1).

- [ ] **Step 4: Confirm clear and per-entry delete**

Press `C`, then confirm the store is `[]`. Copy two different values, press `X` on the first, and confirm only the other remains.

- [ ] **Step 5: Check the log for errors**

```bash
LOG=$(ls -t /run/user/1000/quickshell/by-id/*/log.qslog | head -1)
grep -aoE "(TypeError|ReferenceError|SyntaxError|is not a function|Cannot assign)[^\"]{0,90}" "$LOG" | sort -u
```

Expected: no output.

- [ ] **Step 6: Restore the clipboard and stop the shell**

Stop the shell instance. Leave the store in place unless the user asked for it to be removed, since it is the feature's data.

---

## Self-Review

**Spec coverage**

| Spec section | Task |
| --- | --- |
| 3 Components and boundaries | Tasks 2, 3, 4, 5 |
| 4 Capture pipeline | Tasks 2, 4 |
| 5 Policy | Tasks 4, 6, 8 (controls, and the wording) |
| 6 Behaviour | Tasks 3, 4, 6, 7 |
| 7 Errors and degradation | Task 4 (restart guard, unreadable store) |
| 8 Config additions | Task 1 |
| 9 Testing | Tasks 2, 5, 6, 9 |
| 10 Out of scope | No task adds images, pinning, source metadata, secret detection or an editor |

**Placeholder scan** — every step carries real code or a concrete command with an expected result. Task 4's Step 2 deliberately asks for the `Quickshell.shellDir` value to be confirmed rather than assumed, and to stop if it is wrong, because the helper path depends on it.

**Type consistency** — the helper's environment contract in Task 2 matches the `environment` block the watcher sets in Task 4. The item fields built in Task 4 (`title`, `subtitle`, `glyph`, `hint`, `id`, `actions`) match what the delegate in Task 3 and the generic shell plumbing read. `deleteEntry` keys off the `clip:<copiedAt>` id that `toItem` builds and that Task 6 reads back from `Launcher.results`. `paused` and `entries` are read by the SearchField branch in Task 7 exactly as declared in Task 4.