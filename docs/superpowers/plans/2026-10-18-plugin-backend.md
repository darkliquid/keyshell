# Plugin / Scriptable Backend Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let a backend be authored outside this repository, in any language, by discovering a manifest on disk and speaking a line-delimited JSON protocol over a child process.

**Architecture:** A `PluginRegistry` singleton discovers `plugin.json` manifests and creates one `PluginBackend` component instance per plugin, each owning its own `Process`. `BackendManager.backends` becomes built-ins plus the registry's live list, so a plugin satisfies the same interface as a built-in backend and needs no special casing anywhere else. Plugins run as external executables; the shell only ever executes a small, declared set of actions.

**Tech Stack:** Quickshell 0.3.2 (Qt 6 QML), `Quickshell.Io` (`Process`, `SplitParser`, `StdioCollector`), plain JavaScript for the protocol helpers, `node` for the unit harness.

**Spec:** `docs/superpowers/specs/2026-10-18-plugin-backend-design.md`

## Global Constraints

- Quickshell 0.3.2+ floor; target niri, degrade gracefully on other Wayland compositors.
- No hardcoded magic numbers: every duration, cap and path belongs in `Config.qml`.
- No inline hex colours; use `Theme.*`. Outlines 2-3px, shadows solid offset, no rounded pills or blurs.
- Every global manager is a `pragma Singleton`. `PluginBackend.qml` is deliberately **not** a singleton, since one instance exists per plugin.
- Never use em dashes in source files.
- Stop plugin processes on mode exit and on dismiss; discard stale async results with a generation counter or an equivalent guard.
- A property named `id` cannot be assigned in a QML object declaration, because `id:` is the object-id attribute. `PluginBackend` therefore exposes `readonly property string id` as an alias of a writable `pluginId`, and instances are configured through `Component.createObject`.
- `QtObject` has no default property, so child objects cannot be declared inside it — doing so fails with `Cannot assign to non-existent default property`. `PluginBackend` assigns its `Process` and `Timer` to named properties instead: `property Process pluginProcess: Process { ... }`.
- Commit steps: create the commit shown **only** when the user has authorised committing. If they have not, stop after the verification step and ask. Never push to a remote.

---

### Task 1: Config tunables

**Files:**
- Modify: `quickshell/keyshell/Config.qml`

**Interfaces:**
- Consumes: nothing.
- Produces: `Config.pluginSearchDirs`, `Config.pluginTimeoutMs`, `Config.pluginQueryDebounceMs`, `Config.maxPluginResults`, `Config.minTriggerLength`.

- [ ] **Step 1: Add the tunables**

In `Config.qml`, insert after the `terminalCandidates` property:

```qml
    readonly property int pluginTimeoutMs: 2000
    readonly property int pluginQueryDebounceMs: 120
    readonly property int maxPluginResults: 40
    readonly property int minTriggerLength: 2
    readonly property var pluginSearchDirs: {
        const home = Quickshell.env("HOME") || "";
        const configHome = Quickshell.env("XDG_CONFIG_HOME") || (home + "/.config");
        const dataHome = Quickshell.env("XDG_DATA_HOME") || (home + "/.local/share");
        const dataDirs = (Quickshell.env("XDG_DATA_DIRS") || "/usr/local/share:/usr/share").split(":");
        const dirs = [configHome + "/keyshell/plugins", dataHome + "/keyshell/plugins"];
        for (let i = 0; i < dataDirs.length; i++) {
            if (dataDirs[i].length > 0)
                dirs.push(dataDirs[i] + "/keyshell/plugins");
        }
        return dirs;
    }
```

- [ ] **Step 2: Verify the shell still loads**

Run: `timeout 8 qs -p quickshell/keyshell 2>&1 | head -20`
Expected: `Configuration Loaded` with no QML errors.

- [ ] **Step 3: Commit (only if authorised)**

```bash
git add quickshell/keyshell/Config.qml
git commit -m "Add plugin discovery and timeout settings"
```

---

### Task 2: Trigger matching

**Files:**
- Create: `quickshell/keyshell/plugin-proto.js`
- Modify: `tests/run.js`

**Interfaces:**
- Consumes: nothing.
- Produces: `matchTrigger(query, triggers) -> {trigger, remainder} | null`, and the constant `MANIFEST_SEPARATOR`.

- [ ] **Step 1: Write the failing test**

In `tests/run.js`, add the require beside the existing ones:

```javascript
const PluginProto = require("../quickshell/keyshell/plugin-proto.js");
```

and append before the final `console.log`:

```javascript
// --- plugin-proto: matchTrigger ---
assert.deepStrictEqual(PluginProto.matchTrigger(";docker ps", [";docker"]), { trigger: ";docker", remainder: "ps" }, "trigger matches and remainder extracted");
assert.strictEqual(PluginProto.matchTrigger(";docker   ps", [";docker"]).remainder, "ps", "leading space trimmed from remainder");
assert.strictEqual(PluginProto.matchTrigger(";docker", [";docker"]).remainder, "", "bare trigger yields empty remainder");
assert.strictEqual(PluginProto.matchTrigger(";docker ps", [";d", ";docker"]).trigger, ";docker", "longest match wins");
assert.strictEqual(PluginProto.matchTrigger("x;docker", [";docker"]), null, "trigger must anchor at the start");
assert.strictEqual(PluginProto.matchTrigger("=8/2", [";docker"]), null, "a built-in sigil is not a plugin trigger");
assert.strictEqual(PluginProto.matchTrigger("", [";docker"]), null, "empty query matches nothing");
assert.strictEqual(PluginProto.matchTrigger(";docker ps", []), null, "no triggers matches nothing");
```

- [ ] **Step 2: Run it to make sure it fails**

Run: `node tests/run.js`
Expected: FAIL with `Cannot find module '../quickshell/keyshell/plugin-proto.js'`.

- [ ] **Step 3: Write the minimal implementation**

Create `quickshell/keyshell/plugin-proto.js`:

```javascript
// Protocol helpers for external plugin backends. Pure JS: imported by QML via
// `import "plugin-proto.js" as PluginProto` and by tests via `require`.

const MANIFEST_SEPARATOR = "---KEYSHELL-MANIFEST---";

function matchTrigger(query, triggers) {
    const text = query || "";
    let best = null;
    for (let i = 0; i < triggers.length; i++) {
        const trigger = triggers[i];
        if (typeof trigger !== "string" || trigger.length === 0)
            continue;
        if (text.indexOf(trigger) !== 0)
            continue;
        if (best === null || trigger.length > best.length)
            best = trigger;
    }
    if (best === null)
        return null;
    return {
        "trigger": best,
        "remainder": text.slice(best.length).replace(/^\s+/, "")
    };
}

if (typeof module !== "undefined" && module.exports)
    module.exports = { MANIFEST_SEPARATOR: MANIFEST_SEPARATOR, matchTrigger: matchTrigger };
```

- [ ] **Step 4: Run the tests and make sure they pass**

Run: `node tests/run.js`
Expected: PASS, prints `all assertions passed`.

- [ ] **Step 5: Commit (only if authorised)**

```bash
git add tests/run.js quickshell/keyshell/plugin-proto.js
git commit -m "Match plugin triggers against the whole query"
```

---

### Task 3: Line and manifest parsing

**Files:**
- Modify: `quickshell/keyshell/plugin-proto.js`
- Modify: `tests/run.js`

**Interfaces:**
- Consumes: `MANIFEST_SEPARATOR` (Task 2).
- Produces: `parseLine(line) -> {done, item} | null`, `normalizeItem(raw) -> item | null`, `parseManifests(text, minTriggerLength) -> [manifest]`.

- [ ] **Step 1: Write the failing tests**

Append to `tests/run.js` before the final `console.log`:

```javascript
// --- plugin-proto: items ---
const goodItem = { id: "a", title: "A", subtitle: "s", glyph: "x", badge: "UP", actions: [{ type: "exec", argv: ["echo", "hi"] }] };
const parsedItem = PluginProto.parseLine(JSON.stringify(goodItem));
assert.strictEqual(parsedItem.done, false, "an item line is not the terminator");
assert.strictEqual(parsedItem.item.title, "A", "title survives normalisation");
assert.strictEqual(parsedItem.item.subtitle, "s", "subtitle survives normalisation");
assert.strictEqual(PluginProto.parseLine(JSON.stringify({ done: true })).done, true, "done line detected");
assert.strictEqual(PluginProto.parseLine("not json"), null, "malformed line ignored");
assert.strictEqual(PluginProto.parseLine(""), null, "blank line ignored");
assert.strictEqual(PluginProto.parseLine(JSON.stringify({ id: "b" })), null, "item without a title is dropped");
assert.strictEqual(PluginProto.parseLine(JSON.stringify({ title: "B" })), null, "item without an id is dropped");
assert.strictEqual(PluginProto.parseLine(JSON.stringify({ id: "c", title: "C", actions: [{ type: "nope" }] })), null, "unknown action type rejected");
assert.strictEqual(PluginProto.parseLine(JSON.stringify({ id: "d", title: "D", actions: [{ type: "exec", argv: [] }] })), null, "empty argv rejected");
assert.strictEqual(PluginProto.parseLine(JSON.stringify({ id: "e", title: "E", actions: [{ type: "copy" }] })), null, "copy without text rejected");

const sibling = PluginProto.parseLine(JSON.stringify({
    id: "f", title: "F",
    actions: [{ type: "copy", text: "hi" }, { type: "nope" }]
}));
assert.strictEqual(sibling.item.actions.length, 1, "a valid action survives a rejected sibling");

const defaults = PluginProto.parseLine(JSON.stringify({ id: "g", title: "G", actions: [{ type: "set-query", query: "x" }] })).item;
assert.strictEqual(defaults.subtitle, "", "subtitle defaults to empty");
assert.strictEqual(defaults.glyph, "", "glyph defaults to empty");
assert.strictEqual(defaults.badge, "", "badge defaults to empty");

// --- plugin-proto: manifests ---
const manifestText = PluginProto.MANIFEST_SEPARATOR + "\n" + JSON.stringify({
    name: "Docker", badge: "DOCKER", trigger: ";docker", command: ["/bin/true"], glyph: "🐳", trackable: true
}));
const manifests = PluginProto.parseManifests(manifestText, 2);
assert.strictEqual(manifests.length, 1, "one manifest parsed");
assert.strictEqual(manifests[0].badge, "DOCKER", "badge read");
assert.strictEqual(manifests[0].trigger, ";docker", "trigger read");
assert.strictEqual(manifests[0].searchable, false, "searchable defaults to false");
assert.strictEqual(manifests[0].trackable, true, "trackable read");
assert.strictEqual(manifests[0].id, "plugin-docker", "id derived from the name");

assert.strictEqual(PluginProto.parseManifests(PluginProto.MANIFEST_SEPARATOR + "\n" + JSON.stringify({ name: "X", trigger: ";", command: ["/bin/true"] }), 2).length, 0, "trigger shorter than the minimum is skipped");
assert.strictEqual(PluginProto.parseManifests(PluginProto.MANIFEST_SEPARATOR + "\n" + JSON.stringify({ trigger: ";x", command: ["/bin/true"] }), 2).length, 0, "manifest without a name is skipped");
assert.strictEqual(PluginProto.parseManifests(PluginProto.MANIFEST_SEPARATOR + "\n" + JSON.stringify({ name: "X", trigger: ";xx" }), 2).length, 0, "manifest without a command is skipped");
assert.strictEqual(PluginProto.parseManifests(PluginProto.MANIFEST_SEPARATOR + "\nnot json", 2).length, 0, "malformed manifest skipped");
assert.strictEqual(PluginProto.parseManifests("", 2).length, 0, "no manifests yields nothing");
```

- [ ] **Step 2: Run it to make sure it fails**

Run: `node tests/run.js`
Expected: FAIL with `PluginProto.parseLine is not a function`.

- [ ] **Step 3: Write the minimal implementation**

In `plugin-proto.js`, add before the `module.exports` block:

```javascript
const ACTION_TYPES = ["copy", "open-url", "exec", "exec-terminal", "set-query"];

function requiredField(actionType) {
    if (actionType === "copy")
        return "text";
    if (actionType === "open-url")
        return "url";
    if (actionType === "set-query")
        return "query";
    return "argv";
}

function isValidAction(action) {
    if (!action || typeof action !== "object")
        return false;
    if (ACTION_TYPES.indexOf(action.type) === -1)
        return false;

    const field = requiredField(action.type);
    if (field === "argv") {
        if (!Array.isArray(action.argv) || action.argv.length === 0)
            return false;
        for (let i = 0; i < action.argv.length; i++) {
            if (typeof action.argv[i] !== "string")
                return false;
        }
        return true;
    }
    return typeof action[field] === "string" && action[field].length > 0;
}

function normalizeItem(raw) {
    if (!raw || typeof raw !== "object")
        return null;

    const id = (typeof raw.id === "string" && raw.id.length > 0) ? raw.id : null;
    const title = (typeof raw.title === "string" && raw.title.length > 0) ? raw.title : null;
    if (id === null || title === null)
        return null;

    const actions = [];
    const rawActions = Array.isArray(raw.actions) ? raw.actions : [];
    for (let i = 0; i < rawActions.length; i++) {
        if (isValidAction(rawActions[i]))
            actions.push(rawActions[i]);
    }
    if (actions.length === 0)
        return null;

    return {
        "id": id,
        "title": title,
        "subtitle": typeof raw.subtitle === "string" ? raw.subtitle : "",
        "glyph": typeof raw.glyph === "string" ? raw.glyph : "",
        "badge": typeof raw.badge === "string" ? raw.badge : "",
        "actions": actions
    };
}

function parseLine(line) {
    const text = (line || "").trim();
    if (text.length === 0)
        return null;

    let parsed;
    try {
        parsed = JSON.parse(text);
    } catch (e) {
        return null;
    }

    if (parsed && typeof parsed === "object" && parsed.done === true)
        return { "done": true, "item": null };

    const item = normalizeItem(parsed);
    if (item === null)
        return null;
    return { "done": false, "item": item };
}

function normalizeManifest(raw, minTriggerLength) {
    if (!raw || typeof raw !== "object")
        return null;

    const minLength = (typeof minTriggerLength === "number") ? minTriggerLength : 2;
    const name = (typeof raw.name === "string" && raw.name.length > 0) ? raw.name : null;
    const trigger = (typeof raw.trigger === "string" && raw.trigger.length >= minLength) ? raw.trigger : null;
    const command = Array.isArray(raw.command)
        ? raw.command.filter(function (part) { return typeof part === "string" && part.length > 0; })
        : [];

    if (name === null || trigger === null || command.length === 0)
        return null;

    const slug = name.toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-+|-+$/g, "");
    return {
        "id": "plugin-" + (slug.length > 0 ? slug : "unnamed"),
        "name": name,
        "badge": (typeof raw.badge === "string" && raw.badge.length > 0) ? raw.badge : name.toUpperCase().slice(0, 12),
        "trigger": trigger,
        "command": command,
        "glyph": typeof raw.glyph === "string" ? raw.glyph : "",
        "searchable": raw.searchable === true,
        "trackable": raw.trackable === true,
        "description": typeof raw.description === "string" ? raw.description : ""
    };
}

function parseManifests(text, minTriggerLength) {
    const manifests = [];
    const blocks = (text || "").split(MANIFEST_SEPARATOR);
    for (let i = 0; i < blocks.length; i++) {
        const block = blocks[i].trim();
        if (block.length === 0)
            continue;

        let parsed;
        try {
            parsed = JSON.parse(block);
        } catch (e) {
            continue;
        }

        const manifest = normalizeManifest(parsed, minTriggerLength);
        if (manifest !== null)
            manifests.push(manifest);
    }
    return manifests;
}
```

And replace the export line with:

```javascript
if (typeof module !== "undefined" && module.exports)
    module.exports = {
        MANIFEST_SEPARATOR: MANIFEST_SEPARATOR,
        matchTrigger: matchTrigger,
        parseLine: parseLine,
        normalizeItem: normalizeItem,
        parseManifests: parseManifests
    };
```

- [ ] **Step 4: Run the tests and make sure they pass**

Run: `node tests/run.js`
Expected: PASS, prints `all assertions passed`.

- [ ] **Step 5: Commit (only if authorised)**

```bash
git add tests/run.js quickshell/keyshell/plugin-proto.js
git commit -m "Parse plugin output and manifests defensively"
```

---

### Task 4: Shared terminal resolution

The run-command backend already resolves a terminal; plugins need the same for `exec-terminal`, so it moves into one singleton.

**Files:**
- Create: `quickshell/keyshell/Terminal.qml`
- Modify: `quickshell/keyshell/RunBackend.qml`

**Interfaces:**
- Consumes: `Config.terminalCandidates`.
- Produces: `Terminal.path` (string), `Terminal.resolved()` signal, `Terminal.run(argv: array) -> bool`.

- [ ] **Step 1: Create the singleton**

Create `quickshell/keyshell/Terminal.qml`:

```qml
pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Io

Singleton {
    id: terminal

    property string path: ""

    signal resolved()

    function probeScript() {
        const candidates = [];
        const fromEnv = Quickshell.env("TERMINAL");
        if (fromEnv && fromEnv.length > 0)
            candidates.push(fromEnv);
        for (let i = 0; i < Config.terminalCandidates.length; i++)
            candidates.push(Config.terminalCandidates[i]);

        let script = "";
        for (let i = 0; i < candidates.length; i++) {
            const name = String(candidates[i]).replace(/'/g, "");
            if (name.length === 0)
                continue;
            script += "command -v '" + name + "' >/dev/null 2>&1 && { command -v '" + name + "'; exit 0; }; ";
        }
        return script + "exit 1";
    }

    Process {
        id: probe
        command: ["sh", "-c", terminal.probeScript()]
        running: true

        stdout: StdioCollector {
            id: probeOutput
        }

        onExited: (exitCode, exitStatus) => {
            terminal.path = (exitCode === 0) ? probeOutput.text.trim() : "";
            terminal.resolved();
        }
    }

    function run(argv) {
        if (terminal.path.length === 0 || !Array.isArray(argv) || argv.length === 0)
            return false;
        Quickshell.execDetached([terminal.path, "-e"].concat(argv));
        return true;
    }
}
```

- [ ] **Step 2: Verify the probe script logic**

Run the same script shape directly, substituting the configured candidates:

```bash
sh -c "command -v 'ghostty' >/dev/null 2>&1 && { command -v 'ghostty'; exit 0; }; command -v 'alacritty' >/dev/null 2>&1 && { command -v 'alacritty'; exit 0; }; exit 1"
```

Expected: prints an absolute path to a terminal and exits 0.

- [ ] **Step 3: Point the run backend at it**

In `RunBackend.qml`, delete the `property string terminal: ""` declaration, the whole `probeScript()` function, and the entire `Process { id: terminalProbe ... }` block, and add the import:

```qml
import qs
```

Then replace the body of `search()` so it reads from the shared singleton:

```qml
    function search(q, limit) {
        const command = (q || "").trim();
        if (command.length === 0)
            return [];

        const terminalName = Terminal.path.replace(/^.*\//, "");
        const hasTerminal = terminalName.length > 0;

        const action = hasTerminal
            ? {
                "label": "Run in terminal",
                "execute": function () {
                    Terminal.run(["/bin/sh", "-lc", command]);
                }
            }
            : {
                "label": "Copy command",
                "execute": function () {
                    Quickshell.execDetached(["wl-copy", command]);
                }
            };

        return [
            {
                "id": "run:" + command,
                "title": command,
                "subtitle": hasTerminal ? ("OPENS IN " + terminalName.toUpperCase()) : "NO TERMINAL FOUND",
                "hint": hasTerminal ? "[ENTER] RUN IN TERMINAL" : "[ENTER] COPY COMMAND",
                "icon": "",
                "previewSource": null,
                "badge": "RUN",
                "delegate": runBackend.cardDelegate,
                "actions": [action]
            }
        ];
    }
```

and add a re-emit so the reel refreshes once the probe finishes:

```qml
    Connections {
        target: Terminal

        function onResolved() {
            if (BackendManager.activeMode === "run")
                runBackend.resultsReady(Launcher.query, runBackend.search(Launcher.query, Config.maxResults));
        }
    }
```

- [ ] **Step 4: Verify the run mode still loads**

Run: `timeout 8 qs -p quickshell/keyshell 2>&1 | head -20`
Expected: `Configuration Loaded`, no QML errors, and no `ReferenceError: Terminal is not defined`.

- [ ] **Step 5: Commit (only if authorised)**

```bash
git add quickshell/keyshell/Terminal.qml quickshell/keyshell/RunBackend.qml
git commit -m "Share terminal resolution between the run backend and plugins"
```

---

### Task 5: The generic plugin card

**Files:**
- Create: `quickshell/keyshell/components/PluginCardDelegate.qml`

**Interfaces:**
- Consumes: the item fields `glyph`, `stateBadge`, `title`, `subtitle`, `hint`.
- Produces: a delegate for plugin items.

- [ ] **Step 1: Create the delegate**

Create `quickshell/keyshell/components/PluginCardDelegate.qml`, following the house idiom (`FileCardDelegate.qml` for structure, `Theme.heavy()` for type, inverted colours when active):

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
        id: topRow
        anchors.top: parent.top
        anchors.left: parent.left
        anchors.right: parent.right
        height: 18

        Rectangle {
            id: statePill
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            height: 18
            width: Math.min(statePillText.implicitWidth + 10, parent.width * 0.5)
            color: delegate.active ? Theme.slab : Theme.accent
            radius: 2
            visible: Boolean(delegate.itemData && delegate.itemData.stateBadge)

            Text {
                id: statePillText
                anchors.centerIn: parent
                width: parent.width - 8
                horizontalAlignment: Text.AlignHCenter
                elide: Text.ElideRight
                text: delegate.itemData && delegate.itemData.stateBadge ? delegate.itemData.stateBadge : ""
                font: Theme.heavy(9)
                color: delegate.active ? Theme.accent : Theme.slab
                renderType: Text.QtRendering
            }
        }
    }

    Item {
        id: middleRow
        anchors.top: topRow.bottom
        anchors.bottom: footerText.top
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.topMargin: 6
        anchors.bottomMargin: 6

        Text {
            id: glyphText
            anchors.left: parent.left
            anchors.verticalCenter: parent.verticalCenter
            width: 44
            horizontalAlignment: Text.AlignHCenter
            elide: Text.ElideRight
            text: delegate.itemData && delegate.itemData.glyph ? delegate.itemData.glyph : "🧩"
            font: Theme.heavy(24)
            color: delegate.active ? Theme.slab : Theme.ink
            renderType: Text.QtRendering
        }

        Column {
            anchors.left: glyphText.right
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            anchors.leftMargin: 8
            spacing: 2

            Text {
                width: parent.width
                text: delegate.itemData ? delegate.itemData.title : ""
                font: Theme.heavy(13)
                color: delegate.active ? Theme.slab : Theme.ink
                elide: Text.ElideMiddle
                maximumLineCount: 1
                renderType: Text.QtRendering
            }

            Text {
                width: parent.width
                text: delegate.itemData ? delegate.itemData.subtitle : ""
                font: Theme.heavy(10)
                color: delegate.active ? Theme.slab : Theme.muted
                elide: Text.ElideMiddle
                maximumLineCount: 1
                renderType: Text.QtRendering
            }
        }
    }

    Text {
        id: footerText
        anchors.bottom: parent.bottom
        anchors.left: parent.left
        anchors.right: parent.right
        text: (delegate.itemData && delegate.itemData.hint) ? delegate.itemData.hint : "[ENTER] RUN"
        font: Theme.heavy(9)
        color: delegate.active ? Theme.slab : Theme.muted
        horizontalAlignment: Text.AlignHCenter
        elide: Text.ElideRight
        renderType: Text.QtRendering
    }
}
```

- [ ] **Step 2: Verify it parses**

Run: `qmllint quickshell/keyshell/components/PluginCardDelegate.qml; echo "exit=$?"`
Expected: `exit=0`.

- [ ] **Step 3: Commit (only if authorised)**

```bash
git add quickshell/keyshell/components/PluginCardDelegate.qml
git commit -m "Add a generic card for plugin items"
```

---

### Task 6: The plugin backend component

**Files:**
- Create: `quickshell/keyshell/PluginBackend.qml`

**Interfaces:**
- Consumes: `PluginProto.parseLine` (Task 3), `Terminal` (Task 4), `PluginCardDelegate` (Task 5), `Config`.
- Produces: a component implementing the backend contract — `id`, `name`, `badge`, `prefix`, `searchable`, `trackable`, `cardDelegate`, `search(query, limit) -> null`, `refresh()`, `signal resultsReady(string query, var items)` — plus `stop()` and the writable `pluginId` used at creation.

- [ ] **Step 1: Create the component**

Create `quickshell/keyshell/PluginBackend.qml`:

```qml
import QtQuick
import Quickshell
import Quickshell.Io
import qs.components
import "plugin-proto.js" as PluginProto

QtObject {
    id: backend

    property string pluginId: ""
    property string name: ""
    property string badge: ""
    property string prefix: ""
    property bool searchable: false
    property bool trackable: false
    property string glyph: ""
    property var command: []

    readonly property string id: backend.pluginId
    readonly property Component cardDelegate: PluginCardDelegate {}

    property string pendingQuery: ""
    property int pendingLimit: Config.maxPluginResults
    property bool waiting: false
    property bool unavailable: false
    property var collected: []

    property Process pluginProcess: Process {
        command: backend.command
        running: false
        stdinEnabled: true

        stdout: SplitParser {
            onRead: (line) => backend.handleLine(line)
        }

        onExited: (exitCode, exitStatus) => {
            if (backend.waiting) {
                backend.waiting = false;
                backend.finish();
            }
            if (exitCode !== 0)
                backend.unavailable = true;
        }
    }

    property Timer timeoutTimer: Timer {
        interval: Config.pluginTimeoutMs
        repeat: false
        onTriggered: {
            if (!backend.waiting)
                return;
            backend.waiting = false;
            backend.finish();
        }
    }

    signal resultsReady(string query, var items)

    function start() {
        if (pluginProcess.running)
            return;
        pluginProcess.running = true;
    }

    function stop() {
        timeoutTimer.stop();
        waiting = false;
        collected = [];
        if (pluginProcess.running)
            pluginProcess.running = false;
    }

    function request(query, limit) {
        pendingQuery = query || "";
        pendingLimit = limit || Config.maxPluginResults;
        collected = [];
        waiting = true;
        start();

        if (!pluginProcess.running) {
            waiting = false;
            finish();
            return;
        }

        pluginProcess.write(JSON.stringify({ "query": pendingQuery, "limit": pendingLimit }) + "\n");
        timeoutTimer.restart();
    }

    function handleLine(line) {
        if (!waiting)
            return;

        const parsed = PluginProto.parseLine(line);
        if (parsed === null)
            return;

        if (parsed.done) {
            timeoutTimer.stop();
            waiting = false;
            finish();
            return;
        }

        const items = collected.slice();
        items.push(parsed.item);
        collected = items;
    }

    function finish() {
        const source = collected;
        const items = [];
        for (let i = 0; i < source.length && items.length < pendingLimit; i++)
            items.push(toDisplayItem(source[i]));
        resultsReady(pendingQuery, items);
    }

    function toDisplayItem(raw) {
        const actions = [];
        for (let i = 0; i < raw.actions.length; i++)
            actions.push(toAction(raw.actions[i]));

        return {
            "id": raw.id,
            "title": raw.title,
            "subtitle": raw.subtitle,
            "glyph": raw.glyph.length > 0 ? raw.glyph : backend.glyph,
            "stateBadge": raw.badge,
            "hint": "[ENTER] RUN",
            "icon": "",
            "previewSource": null,
            "badge": backend.badge,
            "delegate": backend.cardDelegate,
            "actions": actions
        };
    }

    function toAction(action) {
        if (action.type === "copy") {
            return {
                "label": "Copy",
                "execute": function () {
                    Quickshell.execDetached(["wl-copy", action.text]);
                }
            };
        }
        if (action.type === "open-url") {
            return {
                "label": "Open",
                "execute": function () {
                    Quickshell.execDetached(["xdg-open", action.url]);
                }
            };
        }
        if (action.type === "set-query") {
            return {
                "label": "Search",
                "execute": function () {
                    Launcher.setQuery(action.query);
                }
            };
        }
        if (action.type === "exec-terminal") {
            return {
                "label": "Run in terminal",
                "execute": function () {
                    if (!Terminal.run(action.argv))
                        Quickshell.execDetached(["wl-copy"].concat(action.argv));
                }
            };
        }
        return {
            "label": "Run",
            "execute": function () {
                Quickshell.execDetached(action.argv);
            }
        };
    }

    function search(query, limit) {
        backend.request(query, limit);
        return null;
    }

    function refresh() {
    }
}
```

- [ ] **Step 2: Verify it parses and the whole configuration still loads**

Run: `qmllint quickshell/keyshell/PluginBackend.qml; echo "exit=$?"`
Expected: `exit=0`.

Then run: `timeout 10 qs -p quickshell/keyshell 2>&1 | tail -6`
Expected: `Configuration Loaded` and no `Type PluginBackend unavailable` or `Cannot assign to non-existent default property`. Linting alone does not catch this: Quickshell type-checks every type in the synthesized `qmldir`, so a component that cannot be created breaks the whole configuration.

- [ ] **Step 3: Commit (only if authorised)**

```bash
git add quickshell/keyshell/PluginBackend.qml
git commit -m "Speak a line-delimited JSON protocol to plugin executables"
```

---

### Task 7: Plugin discovery

**Files:**
- Create: `quickshell/keyshell/PluginRegistry.qml`

**Interfaces:**
- Consumes: `Config.pluginSearchDirs`, `Config.minTriggerLength`, `PluginProto.parseManifests` and `MANIFEST_SEPARATOR` (Task 3), the `PluginBackend` component (Task 6).
- Produces: `PluginRegistry.backends` (a live array, change-notifying) and `PluginRegistry.stopAll()`.

- [ ] **Step 1: Create the singleton**

Create `quickshell/keyshell/PluginRegistry.qml`:

```qml
pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Io
import "plugin-proto.js" as PluginProto

Singleton {
    id: registry

    property var backends: []

    readonly property Component backendComponent: PluginBackend {}

    function discoveryScript() {
        let script = "";
        for (let i = 0; i < Config.pluginSearchDirs.length; i++) {
            const dir = String(Config.pluginSearchDirs[i]).replace(/'/g, "");
            if (dir.length === 0)
                continue;
            script += "[ -d '" + dir + "' ] && for m in '" + dir + "'/*/plugin.json; do [ -f \"$m\" ] && { printf '%s\\n' '" + PluginProto.MANIFEST_SEPARATOR + "'; cat \"$m\"; }; done; ";
        }
        return script + "exit 0";
    }

    Process {
        id: discovery
        command: ["sh", "-c", registry.discoveryScript()]
        running: true

        stdout: StdioCollector {
            id: discoveryOutput
        }

        onExited: (exitCode, exitStatus) => registry.adopt(discoveryOutput.text)
    }

    function adopt(text) {
        const manifests = PluginProto.parseManifests(text, Config.minTriggerLength);
        const created = [];
        for (let i = 0; i < manifests.length; i++) {
            const manifest = manifests[i];
            const instance = backendComponent.createObject(registry, {
                "pluginId": manifest.id,
                "name": manifest.name,
                "badge": manifest.badge,
                "prefix": manifest.trigger,
                "searchable": manifest.searchable,
                "trackable": manifest.trackable,
                "glyph": manifest.glyph,
                "command": manifest.command
            });
            if (instance)
                created.push(instance);
        }
        backends = created;
    }

    function stopAll() {
        for (let i = 0; i < backends.length; i++) {
            if (backends[i].stop)
                backends[i].stop();
        }
    }
}
```

Discovery runs once at startup; a plugin added later appears after a restart, which is what the spec scopes.

- [ ] **Step 2: Verify it loads with no plugins installed**

Run: `timeout 8 qs -p quickshell/keyshell 2>&1 | head -20`
Expected: `Configuration Loaded`, no QML errors, and no `createObject` warnings.

- [ ] **Step 3: Commit (only if authorised)**

```bash
git add quickshell/keyshell/PluginRegistry.qml
git commit -m "Discover plugins from manifest files on disk"
```

---

### Task 8: Dynamic backend registry and trigger dispatch

**Files:**
- Modify: `quickshell/keyshell/BackendManager.qml`

**Interfaces:**
- Consumes: `PluginRegistry.backends` (Task 7), `PluginProto.matchTrigger` (Task 2).
- Produces: `BackendManager.backends` including plugins, `BackendManager.tryTrigger(prospectiveQuery) -> bool`, and a `stop()` call on the outgoing mode.

- [ ] **Step 1: Make the backend list dynamic**

Add the import at the top of `BackendManager.qml`:

```qml
import "plugin-proto.js" as PluginProto
```

and replace the fixed `backends` array with a built-ins list plus the registry's live list:

```qml
    readonly property var builtinBackends: [AppBackend, WindowBackend, FileBackend, AudioBackend, MediaBackend, TrayBackend, BluetoothBackend, PowerBackend, NetworkBackend, CalculatorBackend, RunBackend]
    readonly property var backends: builtinBackends.concat(PluginRegistry.backends)
```

- [ ] **Step 2: Stop the outgoing mode's process**

In `setMode()`, immediately after the two existing `stopDiscovery`/`stopScan` guards, add:

```qml
        const previous = getBackend(activeMode);
        if (previous && previous.stop)
            previous.stop();
```

- [ ] **Step 3: Replace single-character prefix matching**

Delete `checkPrefixTrigger()` entirely and add:

```qml
    function triggerList() {
        const triggers = [];
        for (let i = 0; i < backends.length; i++) {
            const b = backends[i];
            if (b.prefix && b.prefix.length > 0)
                triggers.push(b.prefix);
        }
        return triggers;
    }

    function backendForPrefix(prefix) {
        for (let i = 0; i < backends.length; i++) {
            if (backends[i].prefix === prefix)
                return backends[i];
        }
        return null;
    }

    function tryTrigger(prospectiveQuery) {
        const match = PluginProto.matchTrigger(prospectiveQuery, triggerList());
        if (match === null)
            return false;
        const backend = backendForPrefix(match.trigger);
        if (!backend || backend.id === activeMode)
            return false;
        setMode(backend.id);
        Launcher.setQuery(match.remainder);
        return true;
    }
```

The `backend.id === activeMode` guard is what stops a matched trigger from resetting the query on every subsequent keystroke.

- [ ] **Step 4: Verify the built-in prefixes still work**

Run: `timeout 8 qs -p quickshell/keyshell 2>&1 | head -20`
Expected: `Configuration Loaded` with no reference to `checkPrefixTrigger`.

- [ ] **Step 5: Commit (only if authorised)**

```bash
git add quickshell/keyshell/BackendManager.qml
git commit -m "Let discovered plugins register as launcher modes"
```

---

### Task 9: Whole-query trigger path

**Files:**
- Modify: `quickshell/keyshell/components/LauncherWindow.qml:537-540`

**Interfaces:**
- Consumes: `BackendManager.tryTrigger` (Task 8).
- Produces: no new interface; changes which keystrokes switch modes.

- [ ] **Step 1: Offer the whole prospective query**

Replace:

```qml
                if (Launcher.query === "" && BackendManager.checkPrefixTrigger(event.text)) {
                    event.accepted = true;
                    return;
                }
```

with:

```qml
                if (BackendManager.tryTrigger(Launcher.query + event.text)) {
                    event.accepted = true;
                    return;
                }
```

The `Launcher.query === ""` guard has to go, because a multi-character trigger is typed progressively.

- [ ] **Step 2: Verify single-character sigils still switch mode**

Run: `timeout 60 qs -p quickshell/keyshell` in the background, then:

```bash
qs -p quickshell/keyshell ipc call keyshell open
sleep 2
wdotool mousemove 1700 90; wdotool click 1; sleep 1
printf 'SENTINEL' | wl-copy
wdotool key equal; sleep 0.4; wdotool key 7; sleep 0.4; wdotool key asterisk; sleep 0.4; wdotool key 6; sleep 0.5
wdotool key Return; sleep 2
wl-paste
```

Expected: `42`. This proves the whole-query path still opens the calculator and evaluates.

- [ ] **Step 3: Commit (only if authorised)**

```bash
git add quickshell/keyshell/components/LauncherWindow.qml
git commit -m "Match mode triggers against the whole query"
```

---

### Task 10: Plugin process cleanup

**Files:**
- Modify: `quickshell/keyshell/Launcher.qml`

**Interfaces:**
- Consumes: `PluginRegistry.stopAll()` (Task 7).
- Produces: no new interface; enforces the resource-cleanup rule.

- [ ] **Step 1: Stop plugins on dismiss**

In `dismiss()`, beside the existing `BluetoothBackend.stopDiscovery();` and `NetworkBackend.stopScan();` lines, add:

```qml
        PluginRegistry.stopAll();
```

- [ ] **Step 2: Verify dismissal still works**

Run: `timeout 30 qs -p quickshell/keyshell` in the background, then:

```bash
qs -p quickshell/keyshell ipc call keyshell open >/dev/null
sleep 2
qs -p quickshell/keyshell ipc call keyshell isOpen
qs -p quickshell/keyshell ipc call keyshell close >/dev/null
sleep 1
qs -p quickshell/keyshell ipc call keyshell isOpen
```

Expected: `true` then `false`, with no QML errors about `PluginRegistry`.

- [ ] **Step 3: Commit (only if authorised)**

```bash
git add quickshell/keyshell/Launcher.qml
git commit -m "Stop plugin processes when the launcher is dismissed"
```

---

### Task 11: Fixture plugin and end-to-end verification

**Files:**
- Create: `tests/fixtures/fixture-plugin.sh`
- Create: `tests/fixtures/plugin.json`

**Interfaces:**
- Consumes: everything above.
- Produces: a repeatable proof that discovery, the protocol and action execution work.

- [ ] **Step 1: Write the fixture plugin**

Create `tests/fixtures/fixture-plugin.sh`:

```sh
#!/bin/sh
# Minimal keyshell plugin: one JSON request per line in, items plus a
# terminator out. The first item writes a marker file so a test can observe
# both that a request was answered and that an action ran, without depending
# on the clipboard.
while IFS= read -r line; do
    printf '%s\n' '{"id":"one","title":"Fixture One","subtitle":"writes a marker file","glyph":"1","badge":"TEST","actions":[{"type":"exec","argv":["/bin/sh","-c","echo activated > /tmp/keyshell-fixture-activated"]}]}'
    printf '%s\n' '{"id":"two","title":"Fixture Two","subtitle":"sets the query","glyph":"2","actions":[{"type":"set-query","query":"fixture"}]}'
    printf '%s\n' '{"done":true}'
done
```

Then: `chmod +x tests/fixtures/fixture-plugin.sh`

- [ ] **Step 2: Write the fixture manifest**

Create `tests/fixtures/plugin.json` (the `command` path is rewritten to the absolute repo path by the verification step below):

```json
{
  "name": "Fixture",
  "badge": "FIXTURE",
  "trigger": ",fx",
  "glyph": "🧪",
  "trackable": false,
  "searchable": false,
  "description": "Test fixture plugin",
  "command": ["/ABSOLUTE/PATH/TO/tests/fixtures/fixture-plugin.sh"]
}
```

The trigger `,fx` is deliberately unshifted so the whole path is drivable by `wdotool key`.

- [ ] **Step 3: Install it into the config plugin directory**

```bash
mkdir -p "${XDG_CONFIG_HOME:-$HOME/.config}/keyshell/plugins/fixture"
sed "s#/ABSOLUTE/PATH/TO#$(pwd)#" tests/fixtures/plugin.json > "${XDG_CONFIG_HOME:-$HOME/.config}/keyshell/plugins/fixture/plugin.json"
cat "${XDG_CONFIG_HOME:-$HOME/.config}/keyshell/plugins/fixture/plugin.json"
```

Expected: the manifest prints with an absolute `command` path.

- [ ] **Step 4: Drive the plugin from the overlay**

Start `timeout 120 qs -p quickshell/keyshell` in the background, then:

```bash
rm -f /tmp/keyshell-fixture-activated
C="qs -p quickshell/keyshell ipc call keyshell"
$C open >/dev/null; sleep 4
wdotool mousemove 1700 90; wdotool click 1; sleep 1.5
for k in comma f x; do wdotool key "$k" >/dev/null 2>&1; sleep 0.5; done
sleep 1.5
echo "marker before Enter: $(ls /tmp/keyshell-fixture-activated 2>/dev/null || echo absent)"
wdotool key Return >/dev/null 2>&1; sleep 3
echo "after Enter: isOpen=$($C isOpen)"
echo "marker after Enter: $(cat /tmp/keyshell-fixture-activated 2>/dev/null || echo absent)"
```

Expected: `marker after Enter: activated` and `after Enter: isOpen=false`, with `marker before Enter: absent`. That single observation proves discovery, manifest parsing, trigger matching, the JSON protocol, item normalisation, activation and declarative action execution, and that nothing runs before Enter.

Note: prefer a file-writing `exec` action over a `copy` action as the oracle. `wl-copy` and `wl-paste` can block in a headless or ownerless-clipboard situation, which stalls the verification script.

- [ ] **Step 5: Check the log for errors**

```bash
LOG=$(ls -t /run/user/1000/quickshell/by-id/*/log.qslog | head -1)
grep -aoE "(TypeError|ReferenceError|SyntaxError|is not a function)[^\"]{0,90}" "$LOG" | sort -u
```

Expected: no output.

- [ ] **Step 6: Remove the installed fixture**

```bash
rm -rf "${XDG_CONFIG_HOME:-$HOME/.config}/keyshell/plugins/fixture"
```

- [ ] **Step 7: Commit (only if authorised)**

```bash
git add tests/fixtures
git commit -m "Add a fixture plugin for end-to-end verification"
```

---

### Task 12: Documentation

**Files:**
- Modify: `AGENTS.md`
- Modify: `README.md`

**Interfaces:**
- Consumes: everything above.
- Produces: no code interface.

- [ ] **Step 1: Document the contract in `AGENTS.md`**

Add `PluginRegistry.qml`, `PluginBackend.qml`, `Terminal.qml` and `plugin-proto.js` to the architecture tree, add `PluginCardDelegate.qml` to the component list, and add a subsection after "Adding a New Backend Mode" covering: the manifest location and fields, the minimum trigger length of two, the `{"query", "limit"}` request and `{"done": true}` terminator, the five allowed action types, and the rule that a plugin never runs code inside the shell process.

- [ ] **Step 2: Document usage in `README.md`**

Add a "Plugins" section covering: where to put a plugin (`$XDG_CONFIG_HOME/keyshell/plugins/<name>/plugin.json`), the manifest example, the trigger syntax with the `,fx` example, and the fact that a plugin's mode is reachable by its trigger or by `Tab`.

- [ ] **Step 3: Run the unit harness one final time**

Run: `node tests/run.js`
Expected: PASS, prints `all assertions passed`.

- [ ] **Step 4: Commit (only if authorised)**

```bash
git add AGENTS.md README.md
git commit -m "Explain how to write a keyshell plugin"
```

---

## Self-Review

**Spec coverage**

| Spec section | Task |
| --- | --- |
| 3 Component boundaries | Tasks 4, 5, 6, 7 |
| 4 Discovery and manifest | Tasks 1, 3, 7 |
| 5 Protocol | Tasks 2, 3, 6 |
| 6 Action model | Task 6 |
| 7 Trigger matching | Tasks 2, 8, 9 |
| 8 Merged search | Task 6 (`search()` with a null return, so the aggregator collects it) and Task 8 (registry list) |
| 9 Lifecycle and errors | Tasks 6, 8, 10 |
| 10 Config additions | Task 1 |
| 11 Testing | Tasks 2, 3, 5, 9, 10, 11 |
| 12 Out of scope | No task adds rescans, plugin QML, sandboxing, management commands, or non-JSON protocols |

**Placeholder scan** — every step carries real code or a concrete command with an expected result. The one deliberate placeholder is `/ABSOLUTE/PATH/TO` in the fixture manifest, which Step 3 of Task 11 rewrites with `sed` and then prints for confirmation.

**Type consistency** — `matchTrigger`, `parseLine`, `normalizeItem` and `parseManifests` keep one signature across Tasks 2, 3, 6 and 7. `Terminal.path`/`Terminal.run` are consumed identically in Tasks 4 and 6. `PluginBackend` exposes exactly the backend contract that `BackendManager` and `SearchAggregator` already read (`id`, `badge`, `prefix`, `searchable`, `trackable`, `cardDelegate`, `search`, `refresh`, `resultsReady`), plus `stop()` which Task 8 calls and Task 10 reaches through `stopAll()`. `pluginId` is set only by `PluginRegistry.adopt` and read only through the `id` alias, so no QML declaration ever assigns a property named `id`.