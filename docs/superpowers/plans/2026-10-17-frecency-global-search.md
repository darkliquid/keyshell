# Frecency & Global Search Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the no-prefix query search every backend that opts in, ranked by match quality with learned usage as the tiebreak, backed by a persisted frecency store.

**Architecture:** `BackendManager` stays a pure registry and mode dispatcher; a new `SearchAggregator` singleton owns fan-out, collection and merge; a new `Frecency` singleton owns the on-disk usage store; the scoring math lives in a dependency-free `rank.js` module so it can be unit-tested outside QML. Backends opt in with two declared flags, `searchable` and `trackable`.

**Tech Stack:** Quickshell 0.3.2+ (Qt 6 QML), `Quickshell.Io` (`FileView`, `Process`), plain JavaScript for the scoring math, `node`/`bun` for the unit harness.

**Spec:** `docs/superpowers/specs/2026-10-17-frecency-global-search-design.md`

## Global Constraints

- Quickshell 0.3.2+ floor; target niri, degrade gracefully on other Wayland compositors.
- No hardcoded magic numbers: every duration, weight, cap and path belongs in `Config.qml`.
- No inline hex colours; use `Theme.slab`, `Theme.accent`, etc.
- Every global manager or backend is a `pragma Singleton` at line 1, instantiated as `Singleton { ... }`.
- Never use em dashes in source files.
- Stop timers and async processes when the launcher is dismissed; discard stale async results with a `generation` counter.
- Commit steps: create the commit shown **only** when the user has authorised committing. If they have not, stop after the verification step and ask. Never push to a remote.
- Before implementing `Frecency.qml`, confirm the `Quickshell.Io` `FileView` API (`path`, `text()`, `setText()`, `onLoaded`, `onLoadFailed`) and `Quickshell.env(name)` against the installed Quickshell 0.3.2 docs. If `FileView` cannot create the parent directory, the `mkdir -p` process in Task 5 must run before the first write.

---

### Task 1: Scoring math — match tiers

**Files:**
- Create: `quickshell/keyshell/rank.js`
- Create: `tests/run.js`

**Interfaces:**
- Consumes: nothing.
- Produces: `matchTier(query: string, item: object) -> number` where the return is `-1` for no match, `0` prefix, `1` word start, `2` substring, `3` subtitle or keyword. `item` fields read: `title`, `subtitle`, `keywords` (string or array of strings).

- [ ] **Step 1: Write the failing test**

Create `tests/run.js`:

```javascript
const assert = require("assert");
const Rank = require("../quickshell/keyshell/rank.js");

function item(title, subtitle, keywords) {
    return { "id": title, "title": title, "subtitle": subtitle || "", "keywords": keywords || "" };
}

// --- matchTier ---
assert.strictEqual(Rank.matchTier("fir", item("Firefox")), 0, "prefix -> tier 0");
assert.strictEqual(Rank.matchTier("editor", item("Code Editor")), 1, "word start -> tier 1");
assert.strictEqual(Rank.matchTier("fox", item("Firefox")), 2, "substring -> tier 2");
assert.strictEqual(Rank.matchTier("fire", item("Code Editor", "firefox profile")), 3, "subtitle -> tier 3");
assert.strictEqual(Rank.matchTier("fire", item("Code Editor", "", ["firefox"])), 3, "keyword -> tier 3");
assert.strictEqual(Rank.matchTier("zzz", item("Firefox")), -1, "no match -> -1");
assert.strictEqual(Rank.matchTier("", item("Firefox")), 0, "empty query -> tier 0");

console.log("all assertions passed");
```

- [ ] **Step 2: Run it to make sure it fails**

Run: `node tests/run.js`
Expected: FAIL with `Cannot find module '../quickshell/keyshell/rank.js'`.

- [ ] **Step 3: Write the minimal implementation**

Create `quickshell/keyshell/rank.js`:

```javascript
// Pure ranking math. Imported by QML via `import "rank.js" as Rank` and by
// tests via `require`. No QML or Quickshell imports.

function normalizeQuery(q) {
    return (q || "").trim().toLowerCase();
}

function keywordText(raw) {
    if (Array.isArray(raw))
        return raw.join(" ").toLowerCase();
    return (raw || "").toLowerCase();
}

function matchTier(query, item) {
    const q = normalizeQuery(query);
    if (q === "")
        return 0;

    const title = (item.title || "").toLowerCase();
    if (title.indexOf(q) === 0)
        return 0;

    const parts = title.split(/[\s\-_.]+/);
    for (let i = 0; i < parts.length; i++) {
        if (parts[i].length > 0 && parts[i].indexOf(q) === 0)
            return 1;
    }

    if (title.indexOf(q) !== -1)
        return 2;

    const subtitle = (item.subtitle || "").toLowerCase();
    if (subtitle.indexOf(q) !== -1)
        return 3;
    if (keywordText(item.keywords).indexOf(q) !== -1)
        return 3;

    return -1;
}

if (typeof module !== "undefined" && module.exports)
    module.exports = { matchTier: matchTier };
```

- [ ] **Step 4: Run the tests and make sure they pass**

Run: `node tests/run.js`
Expected: PASS, prints `all assertions passed`.

- [ ] **Step 5: Commit (only if authorised)**

```bash
git add tests/run.js quickshell/keyshell/rank.js
git commit -m "Add match-tier scoring for launcher result ranking"
```

---

### Task 2: Scoring math — frecency decay

**Files:**
- Modify: `quickshell/keyshell/rank.js`
- Modify: `tests/run.js`

**Interfaces:**
- Consumes: nothing from Task 1.
- Produces: `frecencyScore(entry: object|null, nowMs: number, buckets: Array<{maxAgeMs:number,weight:number}>, countCap: number) -> number` returning a normalised `0..1`. `entry` is `{ count: number, lastUsed: number }` or null.

- [ ] **Step 1: Write the failing test**

Append to `tests/run.js`, before the final `console.log`:

```javascript
// --- frecencyScore ---
const buckets = [
    { "maxAgeMs": 3600000, "weight": 4 },
    { "maxAgeMs": 86400000, "weight": 3 },
    { "maxAgeMs": 604800000, "weight": 2 },
    { "maxAgeMs": 2592000000, "weight": 1 }
];
const now = 1000000000000;
const freshMany = Rank.frecencyScore({ count: 10, lastUsed: now - 1000 }, now, buckets, 10);
const freshOne = Rank.frecencyScore({ count: 1, lastUsed: now - 1000 }, now, buckets, 10);
const oldMany = Rank.frecencyScore({ count: 10, lastUsed: now - 900000000 }, now, buckets, 10);

assert.ok(freshMany > freshOne, "more hits scores higher in the same bucket");
assert.ok(freshOne > oldMany, "recent beats old at equal count");
assert.strictEqual(Rank.frecencyScore(null, now, buckets, 10), 0, "no entry -> 0");
assert.strictEqual(Rank.frecencyScore({ count: 5, lastUsed: now - 99999999999 }, now, buckets, 10), 0, "beyond oldest bucket -> 0");
```

Also add `frecencyScore: frecencyScore` to the `module.exports` object in `rank.js` name list at this step (see Step 3).

- [ ] **Step 2: Run it to make sure it fails**

Run: `node tests/run.js`
Expected: FAIL with `Rank.frecencyScore is not a function`.

- [ ] **Step 3: Write the minimal implementation**

In `quickshell/keyshell/rank.js`, add before the `module.exports` block:

```javascript
function frecencyScore(entry, nowMs, buckets, countCap) {
    if (!entry || !entry.lastUsed)
        return 0;

    const age = nowMs - entry.lastUsed;
    if (age < 0)
        return 0;

    let weight = 0;
    for (let i = 0; i < buckets.length; i++) {
        if (age <= buckets[i].maxAgeMs) {
            weight = buckets[i].weight;
            break;
        }
    }
    if (weight === 0)
        return 0;

    const top = buckets[0].weight;
    const cap = countCap || 10;
    const countFactor = Math.min(1, Math.log1p(entry.count || 1) / Math.log1p(cap));
    const score = (weight / top) * countFactor;
    return Math.max(0, Math.min(1, score));
}
```

And update the export line:

```javascript
if (typeof module !== "undefined" && module.exports)
    module.exports = { matchTier: matchTier, frecencyScore: frecencyScore };
```

- [ ] **Step 4: Run the tests and make sure they pass**

Run: `node tests/run.js`
Expected: PASS, prints `all assertions passed`.

- [ ] **Step 5: Commit (only if authorised)**

```bash
git add tests/run.js quickshell/keyshell/rank.js
git commit -m "Score recent and frequent launcher items for ranking"
```

---

### Task 3: Scoring math — merge and sort

**Files:**
- Modify: `quickshell/keyshell/rank.js`
- Modify: `tests/run.js`

**Interfaces:**
- Consumes: `matchTier` (Task 1), `frecencyScore` (Task 2).
- Produces: `mergeAndSort(query: string, groups: Array<{source:string, items:Array}>, entries: object, nowMs: number, options: {buckets, countCap, limit}) -> Array`. Each returned item is a shallow copy carrying `source`, `tier` and `frecency`.

- [ ] **Step 1: Write the failing test**

Append to `tests/run.js`, before the final `console.log`:

```javascript
// --- mergeAndSort ---
const groups = [
    { source: "apps", items: [ item("Firefox"), item("Files") ] },
    { source: "windows", items: [ item("Firefox - Tab") ] }
];
const entries = { "apps:Firefox": { count: 10, lastUsed: now - 1000 } };
const opts = { buckets: buckets, countCap: 10, limit: 10 };

const merged = Rank.mergeAndSort("fir", groups, entries, now, opts);
assert.strictEqual(merged.length, 2, "only matching items survive");
assert.strictEqual(merged[0].source, "apps", "frecency boosts the app hit");
assert.strictEqual(merged[1].source, "windows", "lower frecency sorts second");
assert.ok(merged.every(function (m) { return typeof m.tier === "number" && typeof m.frecency === "number"; }), "tier and frecency attached");

assert.strictEqual(Rank.mergeAndSort("zzz", groups, entries, now, opts).length, 0, "non-matching dropped");
assert.strictEqual(Rank.mergeAndSort("", groups, {}, now, opts).length, 3, "empty query keeps everything");
```

- [ ] **Step 2: Run it to make sure it fails**

Run: `node tests/run.js`
Expected: FAIL with `Rank.mergeAndSort is not a function`.

- [ ] **Step 3: Write the minimal implementation**

In `quickshell/keyshell/rank.js`, add before the `module.exports` block:

```javascript
function mergeAndSort(query, groups, entries, nowMs, options) {
    const opts = options || {};
    const buckets = opts.buckets || [];
    const countCap = opts.countCap || 10;
    const limit = opts.limit || 60;

    const merged = [];
    for (let g = 0; g < groups.length; g++) {
        const source = groups[g].source;
        const items = groups[g].items || [];
        for (let i = 0; i < items.length; i++) {
            const item = items[i];
            const tier = matchTier(query, item);
            if (tier < 0)
                continue;

            const key = source + ":" + item.id;
            const entry = entries ? entries[key] : null;
            const copy = Object.assign({}, item);
            copy.source = source;
            copy.tier = tier;
            copy.frecency = frecencyScore(entry, nowMs, buckets, countCap);
            merged.push(copy);
        }
    }

    merged.sort(function (a, b) {
        if (a.tier !== b.tier)
            return a.tier - b.tier;
        if (a.frecency !== b.frecency)
            return b.frecency - a.frecency;
        const al = (a.title || "").length;
        const bl = (b.title || "").length;
        if (al !== bl)
            return al - bl;
        const at = (a.title || "").toLowerCase();
        const bt = (b.title || "").toLowerCase();
        return at < bt ? -1 : (at > bt ? 1 : 0);
    });

    return merged.slice(0, limit);
}
```

And update the export line:

```javascript
if (typeof module !== "undefined" && module.exports)
    module.exports = { matchTier: matchTier, frecencyScore: frecencyScore, mergeAndSort: mergeAndSort };
```

- [ ] **Step 4: Run the tests and make sure they pass**

Run: `node tests/run.js`
Expected: PASS, prints `all assertions passed`.

- [ ] **Step 5: Commit (only if authorised)**

```bash
git add tests/run.js quickshell/keyshell/rank.js
git commit -m "Merge results from several launcher sources into one ranked list"
```

---

### Task 4: Config tunables and backend opt-in flags

**Files:**
- Modify: `quickshell/keyshell/Config.qml`
- Modify: `quickshell/keyshell/AppBackend.qml`
- Modify: `quickshell/keyshell/WindowBackend.qml`
- Modify: `quickshell/keyshell/FileBackend.qml`

**Interfaces:**
- Consumes: nothing.
- Produces: `Config.mergedModeId`, `Config.mergeSettleMs`, `Config.maxMergedResults`, `Config.frecencyBuckets`, `Config.frecencyCountCap`, `Config.frecencyMaxAgeDays`, `Config.frecencyFlushDebounceMs`; and the backend property `searchable: bool` / `trackable: bool`.

- [ ] **Step 1: Add the tunables to `Config.qml`**

Insert after line 53 (`readonly property int revealMaxAttempts: 32;`):

```qml
    readonly property string mergedModeId: "all"
    readonly property int mergeSettleMs: 120
    readonly property int maxMergedResults: 60

    readonly property int frecencyCountCap: 10
    readonly property int frecencyMaxAgeDays: 90
    readonly property int frecencyFlushDebounceMs: 2000
    readonly property var frecencyBuckets: [
        { "maxAgeMs": 3600000, "weight": 4 },
        { "maxAgeMs": 86400000, "weight": 3 },
        { "maxAgeMs": 604800000, "weight": 2 },
        { "maxAgeMs": 2592000000, "weight": 1 }
    ]
```

- [ ] **Step 2: Flag the opt-in backends**

In `AppBackend.qml`, after line 13 (`readonly property Component cardDelegate: AppCardDelegate {}`):

```qml
    readonly property bool searchable: true
    readonly property bool trackable: true
```

In `WindowBackend.qml`, after line 15 (`readonly property Component cardDelegate: WindowCardDelegate {}`):

```qml
    readonly property bool searchable: true
    readonly property bool trackable: true
```

In `FileBackend.qml`, after line 14 (`readonly property Component cardDelegate: FileCardDelegate {}`):

```qml
    readonly property bool searchable: true
    readonly property bool trackable: false
```

The other six backends are left untouched: the flags default to `false`, so they stay out of merged search and frecency.

- [ ] **Step 3: Verify the shell still loads**

Run: `qs -p quickshell/keyshell`
Expected: the overlay loads with no QML errors in the console; `qs ipc call keyshell open` shows the reel.

- [ ] **Step 4: Commit (only if authorised)**

```bash
git add quickshell/keyshell/Config.qml quickshell/keyshell/AppBackend.qml quickshell/keyshell/WindowBackend.qml quickshell/keyshell/FileBackend.qml
git commit -m "Let backends declare whether they join merged search and usage tracking"
```

---

### Task 5: The frecency store

**Files:**
- Create: `quickshell/keyshell/Frecency.qml`

**Interfaces:**
- Consumes: `Config.frecencyBuckets`, `Config.frecencyCountCap`, `Config.frecencyMaxAgeDays`, `Config.frecencyFlushDebounceMs`; `Rank.frecencyScore` (Task 2).
- Produces: `Frecency.record(source, itemId)`, `Frecency.score(source, itemId) -> number`, `Frecency.snapshot() -> object`, `Frecency.flush()`.

- [ ] **Step 1: Create the singleton**

Create `quickshell/keyshell/Frecency.qml`:

```qml
pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Io
import "rank.js" as Rank

Singleton {
    id: frecency

    readonly property string storeDir: (Quickshell.env("XDG_STATE_HOME") || (Quickshell.env("HOME") + "/.local/state")) + "/keyshell"
    readonly property string storePath: storeDir + "/frecency.json"

    property var entries: ({})
    property bool dirty: false

    Process {
        id: ensureDir
        command: ["mkdir", "-p", frecency.storeDir]
        running: true
    }

    FileView {
        id: storeFile
        path: frecency.storePath
        blockLoading: true
        onLoaded: frecency.adopt(storeFile.text())
        onLoadFailed: {
            frecency.entries = ({});
            frecency.dirty = false;
        }
    }

    Timer {
        id: flushTimer
        interval: Config.frecencyFlushDebounceMs
        repeat: false
        onTriggered: frecency.flush()
    }

    function adopt(raw) {
        try {
            const parsed = JSON.parse(raw);
            entries = (parsed && typeof parsed === "object") ? parsed : ({});
        } catch (e) {
            console.warn("keyshell: frecency store unreadable, starting empty");
            entries = ({});
        }
        prune();
        dirty = false;
    }

    function keyFor(source, itemId) {
        return source + ":" + itemId;
    }

    function record(source, itemId) {
        if (!source || !itemId)
            return;
        const next = {};
        for (const existing in entries)
            next[existing] = entries[existing];
        const k = keyFor(source, itemId);
        const previous = next[k];
        next[k] = {
            "count": (previous ? previous.count : 0) + 1,
            "lastUsed": Date.now()
        };
        entries = next;
        dirty = true;
        flushTimer.restart();
    }

    function score(source, itemId) {
        const entry = entries[keyFor(source, itemId)];
        return Rank.frecencyScore(entry, Date.now(), Config.frecencyBuckets, Config.frecencyCountCap);
    }

    function snapshot() {
        return entries;
    }

    function prune() {
        const cutoff = Date.now() - Config.frecencyMaxAgeDays * 86400000;
        const next = {};
        let removed = 0;
        for (const k in entries) {
            const entry = entries[k];
            if (entry && entry.lastUsed && entry.lastUsed >= cutoff)
                next[k] = entry;
            else
                removed++;
        }
        if (removed > 0) {
            entries = next;
            dirty = true;
        }
    }

    function flush() {
        if (!dirty)
            return;
        storeFile.setText(JSON.stringify(entries));
        dirty = false;
    }
}
```

- [ ] **Step 2: Verify the store is created and empty**

Run: `qs -p quickshell/keyshell` then `qs ipc call keyshell open` then `qs ipc call keyshell toggle`.
Then run: `cat "${XDG_STATE_HOME:-$HOME/.local/state}/keyshell/frecency.json"`
Expected: either no file yet (nothing recorded) or `{}`. No QML errors about `FileView` or `Quickshell.env`.

- [ ] **Step 3: Verify persistence round-trips**

In a temporary scratch check, call `Frecency.record` twice by activating an app twice (see Task 8 for the wiring), or temporarily evaluate on load. Then confirm the file contains one key of the form `apps:<id>` with `count` and `lastUsed`.

- [ ] **Step 4: Commit (only if authorised)**

```bash
git add quickshell/keyshell/Frecency.qml
git commit -m "Persist launcher usage history so favourite items can rank higher"
```

---

### Task 6: Registry-driven result collection and the merged sentinel mode

**Files:**
- Modify: `quickshell/keyshell/BackendManager.qml` (replace the file)
- Modify: `quickshell/keyshell/components/SearchField.qml:23`

**Interfaces:**
- Consumes: `Config.mergedModeId`, the backend `searchable` flag.
- Produces: `BackendManager.mergedMode` (string), `BackendManager.isMerged() -> bool`, `BackendManager.activeBadge` (string), `BackendManager.getBackend(modeId)`, `BackendManager.backendResults(sourceId, query, items)`, and the signal `sourceResults(sourceId, query, items)`.

- [ ] **Step 1: Replace `BackendManager.qml`**

Replace the whole file with:

```qml
pragma Singleton
import QtQuick
import Quickshell

Singleton {
    id: backendManager

    readonly property string mergedMode: Config.mergedModeId
    property string activeMode: Config.mergedModeId

    readonly property var backends: [AppBackend, WindowBackend, FileBackend, AudioBackend, MediaBackend, TrayBackend, BluetoothBackend, PowerBackend, NetworkBackend]

    readonly property var activeBackend: getBackend(activeMode)
    readonly property string activeBadge: isMerged() ? "SEARCH" : (activeBackend ? activeBackend.badge : "SEARCH")

    signal resultsReady(var items)
    signal sourceResults(string sourceId, string query, var items)

    function isMerged() {
        return activeMode === mergedMode;
    }

    function getBackend(modeId) {
        for (let i = 0; i < backends.length; i++) {
            if (backends[i].id === modeId)
                return backends[i];
        }
        return null;
    }

    function setMode(modeId) {
        if (activeMode === modeId)
            return;
        if (activeMode === "bluetooth" && modeId !== "bluetooth")
            BluetoothBackend.stopDiscovery();
        if (activeMode === "network" && modeId !== "network")
            NetworkBackend.stopScan();
        if (modeId === mergedMode) {
            activeMode = modeId;
            return;
        }
        if (getBackend(modeId)) {
            activeMode = modeId;
            refresh();
        }
    }

    function canCycleTo(backend) {
        if (!backend)
            return false;
        if (backend.id === "media")
            return MediaBackend.items && MediaBackend.items.length > 0;
        if (backend.id === "tray")
            return TrayBackend.items && TrayBackend.items.length > 0;
        return true;
    }

    function canCycleToMode(modeId) {
        if (modeId === mergedMode)
            return true;
        return canCycleTo(getBackend(modeId));
    }

    function nextCycleMode(delta) {
        const modes = [mergedMode];
        for (let i = 0; i < backends.length; i++)
            modes.push(backends[i].id);

        const step = delta >= 0 ? 1 : -1;
        let currentIndex = 0;
        for (let i = 0; i < modes.length; i++) {
            if (modes[i] === activeMode) {
                currentIndex = i;
                break;
            }
        }
        for (let offset = 1; offset <= modes.length; offset++) {
            const idx = ((currentIndex + step * offset) % modes.length + modes.length) % modes.length;
            if (canCycleToMode(modes[idx]))
                return modes[idx];
        }
        return mergedMode;
    }

    function nextCycleModeBadge() {
        const modeId = nextCycleMode(1);
        if (modeId === mergedMode)
            return "SEARCH";
        const b = getBackend(modeId);
        return b ? b.badge : "SEARCH";
    }

    function cycleMode(delta) {
        setMode(nextCycleMode(delta));
    }

    function checkPrefixTrigger(text) {
        if (!text || text.length === 0)
            return false;
        const first = text.charAt(0);
        for (let i = 0; i < backends.length; i++) {
            const b = backends[i];
            if (b.prefix.length > 0 && b.prefix === first) {
                setMode(b.id);
                Launcher.setQuery(text.slice(1).trimStart());
                return true;
            }
        }
        return false;
    }

    function search(query, limit) {
        const b = getBackend(activeMode);
        if (!b)
            return;
        const syncResults = b.search(query, limit);
        if (syncResults !== null && syncResults !== undefined)
            resultsReady(syncResults);
    }

    function refresh() {
        if (isMerged())
            return;
        const b = activeBackend;
        if (b && b.refresh)
            b.refresh();
        search(Launcher.query, Config.maxResults);
    }

    function backendResults(sourceId, query, items) {
        sourceResults(sourceId, query, items);
        if (!isMerged() && activeMode === sourceId && query === Launcher.query)
            resultsReady(items);
    }

    Instantiator {
        model: backendManager.backends

        delegate: Connections {
            required property var modelData

            target: modelData

            function onResultsReady(query, items) {
                backendManager.backendResults(modelData.id, query, items);
            }
        }
    }
}
```

- [ ] **Step 2: Use the merged badge in `SearchField.qml`**

Change line 23 from:

```qml
            text: BackendManager.activeBackend ? BackendManager.activeBackend.badge : "APPS"
```

to:

```qml
            text: BackendManager.activeBadge
```

- [ ] **Step 3: Verify cycling and explicit modes still work**

Run: `qs -p quickshell/keyshell`, then `qs ipc call keyshell openMode "windows"`.
Expected: the badge reads `WINDOWS`, window results appear. Press Tab repeatedly (or click the badge) and confirm the cycle includes `SEARCH` first and skips empty media/tray as before. `qs ipc call keyshell openMode "audio"` still shows audio devices.

- [ ] **Step 4: Commit (only if authorised)**

```bash
git add quickshell/keyshell/BackendManager.qml quickshell/keyshell/components/SearchField.qml
git commit -m "Register backends generically and add a merged search mode"
```

---

### Task 7: The aggregator and merged routing

**Files:**
- Create: `quickshell/keyshell/SearchAggregator.qml`
- Modify: `quickshell/keyshell/Launcher.qml:68-70` (`refresh`), `:79-96` (`show`), `:333-342` (the results `Connections`)

**Interfaces:**
- Consumes: `BackendManager.backends`, `BackendManager.sourceResults`, `BackendManager.getBackend`, `Rank.mergeAndSort` (Task 3), `Frecency.snapshot()` (Task 5).
- Produces: `SearchAggregator.searchAll(query, limit)`, `SearchAggregator.resultsReady(items)`, `SearchAggregator.recordActivation(item)`, `SearchAggregator.isTrackable(sourceId)`.

- [ ] **Step 1: Create the aggregator**

Create `quickshell/keyshell/SearchAggregator.qml`:

```qml
pragma Singleton
import QtQuick
import Quickshell
import "rank.js" as Rank

Singleton {
    id: aggregator

    property int generation: 0
    property string pendingQuery: ""
    property int pendingLimit: Config.maxMergedResults
    property var pendingGroups: ({})
    property bool collecting: false

    signal resultsReady(var items)

    Timer {
        id: settleTimer
        interval: Config.mergeSettleMs
        repeat: false
        onTriggered: aggregator.emitMerged()
    }

    Connections {
        target: BackendManager

        function onSourceResults(sourceId, query, items) {
            aggregator.collect(sourceId, query, items);
        }
    }

    function searchableBackends() {
        const out = [];
        for (let i = 0; i < BackendManager.backends.length; i++) {
            if (BackendManager.backends[i].searchable)
                out.push(BackendManager.backends[i]);
        }
        return out;
    }

    function searchAll(query, limit) {
        generation++;
        pendingQuery = query || "";
        pendingLimit = limit || Config.maxMergedResults;
        pendingGroups = ({});
        collecting = true;

        const list = searchableBackends();
        for (let i = 0; i < list.length; i++) {
            const b = list[i];
            const sync = b.search(pendingQuery, Config.maxResults);
            if (sync !== null && sync !== undefined)
                pendingGroups[b.id] = sync;
        }
        settleTimer.restart();
    }

    function collect(sourceId, query, items) {
        if (!collecting)
            return;
        if (query !== pendingQuery)
            return;
        const backend = BackendManager.getBackend(sourceId);
        if (!backend || !backend.searchable)
            return;
        pendingGroups[sourceId] = items || [];
    }

    function emitMerged() {
        collecting = false;
        const groups = [];
        for (const sourceId in pendingGroups)
            groups.push({ "source": sourceId, "items": pendingGroups[sourceId] });
        const merged = Rank.mergeAndSort(pendingQuery, groups, Frecency.snapshot(), Date.now(), {
            "buckets": Config.frecencyBuckets,
            "countCap": Config.frecencyCountCap,
            "limit": pendingLimit
        });
        resultsReady(merged);
    }

    function isTrackable(sourceId) {
        const backend = BackendManager.getBackend(sourceId);
        return Boolean(backend && backend.trackable);
    }

    function recordActivation(item) {
        if (!item || !item.source || !item.id)
            return;
        if (!isTrackable(item.source))
            return;
        Frecency.record(item.source, item.id);
    }
}
```

- [ ] **Step 2: Route `Launcher.refresh()` by mode**

Replace `Launcher.qml:68-70`:

```qml
    function refresh() {
        BackendManager.search(query, Config.maxResults);
    }
```

with:

```qml
    function refresh() {
        if (BackendManager.isMerged())
            SearchAggregator.searchAll(query, Config.maxMergedResults);
        else
            BackendManager.search(query, Config.maxResults);
    }
```

- [ ] **Step 3: Make `show()` use the routed refresh**

In `Launcher.qml:93`, change:

```qml
        BackendManager.refresh();
```

to:

```qml
        refresh();
```

- [ ] **Step 4: Install results through one path and consume the aggregator**

Replace the results `Connections` block at `Launcher.qml:333-342`:

```qml
    Connections {
        target: BackendManager
        function onResultsReady(items) {
            launcher.results = items || [];
            if (launcher.selectedIndex >= launcher.results.length)
                launcher.selectedIndex = launcher.results.length > 0 ? launcher.results.length - 1 : 0;
            launcher.buildDisplayResults();
            launcher.modelReset();
        }
    }
```

with:

```qml
    function installResults(items) {
        results = items || [];
        if (selectedIndex >= results.length)
            selectedIndex = results.length > 0 ? results.length - 1 : 0;
        buildDisplayResults();
        modelReset();
    }

    Connections {
        target: BackendManager
        function onResultsReady(items) {
            launcher.installResults(items);
        }
    }

    Connections {
        target: SearchAggregator
        function onResultsReady(items) {
            if (BackendManager.isMerged())
                launcher.installResults(items);
        }
    }
```

- [ ] **Step 5: Verify merged search works**

Run: `qs -p quickshell/keyshell`, `qs ipc call keyshell open`.
Type `fir`.
Expected: the badge shows `SEARCH`; Firefox (an app) and any window whose title contains "fir" both appear; non-matching items are absent. Clear the query and confirm the reel shows a mix of apps and windows. Press `Tab` to leave merged mode and confirm explicit modes are unchanged.

- [ ] **Step 6: Verify late async results do not error**

Run: `qs -v -p quickshell/keyshell` and type a file-ish query (for example `report`) in merged mode.
Expected: file hits land after the settle window without QML warnings; the console shows no `TypeError` from `pendingGroups`.

- [ ] **Step 7: Commit (only if authorised)**

```bash
git add quickshell/keyshell/SearchAggregator.qml quickshell/keyshell/Launcher.qml
git commit -m "Search all opted-in backends at once from the default query"
```

---

### Task 8: Selection stability and usage recording

**Files:**
- Modify: `quickshell/keyshell/Launcher.qml` (properties block, `show`, `setQuery`, `moveSelection`, `installResults`, `activate`, `dismiss`)

**Interfaces:**
- Consumes: `SearchAggregator.recordActivation` (Task 7), `Frecency.flush` (Task 5).
- Produces: `Launcher.selectedItemId`, `Launcher.selectedSource`; `activate()` now records before executing.

- [ ] **Step 1: Track the selected item by identity**

In the `Launcher.qml` properties block, after line 13 (`property int displayIndex: 0`):

```qml
    property string selectedItemId: ""
    property string selectedSource: ""
```

- [ ] **Step 2: Reset identity when the query or overlay changes**

In `show()` (near line 81) and in `setQuery()` (near line 308), add alongside the existing `selectedIndex = 0;`:

```qml
        selectedItemId = "";
        selectedSource = "";
```

- [ ] **Step 3: Record the identity on keyboard movement**

Replace `moveSelection` at `Launcher.qml:312-320`:

```qml
    function moveSelection(delta) {
        const n = results.length;
        if (n === 0)
            return;
        selectedIndex = ((selectedIndex + delta) % n + n) % n;
        const entry = results[selectedIndex];
        if (entry) {
            selectedItemId = entry.id || "";
            selectedSource = entry.source || "";
        }
        const nextDisplay = displayIndex + delta;
        if (nextDisplay >= 0 && nextDisplay < displayResults.length)
            displayIndex = nextDisplay;
    }
```

- [ ] **Step 4: Preserve the highlight across late merges**

Replace the `installResults` helper written in Task 7 with:

```qml
    function installResults(items) {
        const previous = selectedIndex;
        results = items || [];

        let idx = -1;
        if (selectedItemId !== "") {
            for (let i = 0; i < results.length; i++) {
                const entry = results[i];
                if (entry.id === selectedItemId && (entry.source || "") === selectedSource) {
                    idx = i;
                    break;
                }
            }
        }
        if (idx < 0)
            idx = Math.min(previous, results.length > 0 ? results.length - 1 : 0);

        selectedIndex = idx;
        buildDisplayResults();
        modelReset();
    }
```

- [ ] **Step 5: Record an activation before executing**

Replace `activate` at `Launcher.qml:322-331`:

```qml
    function activate() {
        if (results.length === 0)
            return;
        const entry = results[selectedIndex];
        if (entry && entry.actions && entry.actions.length > 0 && entry.actions[0].execute) {
            SearchAggregator.recordActivation(entry);
            entry.actions[0].execute();
            if (sliderTargetNode === null && mediaConsolePlayer === null && trayMenuItem === null && bluetoothTargetDevice === null && !powerConsoleOpen && networkTargetItem === null && networkPasswordTarget === null)
                dismiss();
        }
    }
```

- [ ] **Step 6: Flush the store on dismissal**

In `dismiss()`, after `BluetoothBackend.stopDiscovery();` (near line 113), add:

```qml
        Frecency.flush();
```

- [ ] **Step 7: Verify frecency actually ranks**

Run: `qs -p quickshell/keyshell`, open, type `fir`, select Firefox, press Enter. Repeat twice.
Then run: `cat "${XDG_STATE_HOME:-$HOME/.local/state}/keyshell/frecency.json"`
Expected: a key `apps:<firefox id>` with `"count": 3`.

Re-open the launcher, type `f` and confirm Firefox sorts above other equally-tier-0 apps. Type `zzz` and confirm no results.

- [ ] **Step 8: Verify late hits do not move the highlight**

Open merged mode, type a query that also matches files, use arrow keys to highlight a specific app, then wait for file results to arrive.
Expected: the highlighted item stays the same item; the reel does not jump.

- [ ] **Step 9: Commit (only if authorised)**

```bash
git add quickshell/keyshell/Launcher.qml
git commit -m "Keep the highlighted item steady while results stream in and learn from activations"
```

---

### Task 9: Documentation and final verification

**Files:**
- Modify: `AGENTS.md`
- Modify: `README.md`

**Interfaces:**
- Consumes: everything above.
- Produces: no code interface; the guide future contributors and plugins read.

- [ ] **Step 1: Document the new architecture in `AGENTS.md`**

In the architecture tree, add under the backends and components the three new files and the test harness:

```text
        ├── Frecency.qml               # Singleton: persisted usage scores (XDG state)
        ├── SearchAggregator.qml       # Singleton: merged fan-out, ranking, settle timer
        ├── rank.js                    # Plain JS: match tiers, frecency decay, merge/sort
```

Add a short subsection under "Adding a New Backend Mode" stating: a backend that should appear in the merged default query must declare `readonly property bool searchable: true`, and one whose activations should be remembered must declare `readonly property bool trackable: true`; both default to `false`.

- [ ] **Step 2: Document the behaviour in `README.md`**

Add a section covering: typing with no prefix searches applications, open windows and files at once; frequently used items rise to the top; history is stored at `${XDG_STATE_HOME:-~/.local/state}/keyshell/frecency.json` and can be deleted to reset; explicit prefixes (`@`, `/`, and the rest) still jump straight to a single mode.

- [ ] **Step 3: Run the unit harness one final time**

Run: `node tests/run.js`
Expected: PASS, prints `all assertions passed`.

- [ ] **Step 4: Run the manual matrix**

Run: `qs -v -p quickshell/keyshell`, then for each case:

| Case | Action | Expected |
| --- | --- | --- |
| Merged ordering, no history | delete the store, open, type `fir` | apps and windows mixed, tier order holds |
| Merged ordering, with history | activate Firefox 3x, type `f` | Firefox sorts above other tie-tier apps |
| Persistence across restart | kill `qs`, relaunch, type `f` | Firefox still ranks first |
| Highlight stability | highlight an app, wait for file hits | highlight unchanged |
| Corrupt store | write `not json` to the store, relaunch | one console warning, empty start, no crash |
| Missing store | delete the store, relaunch | empty start, no error |
| Flags absent | `qs ipc call keyshell openMode "audio"` | audio mode still populates |
| Prefix jump | type `@` with an empty query | jumps to WINDOWS mode |

- [ ] **Step 5: Commit (only if authorised)**

```bash
git add AGENTS.md README.md
git commit -m "Explain merged search and usage ranking to contributors and users"
```

---

## Self-Review

**Spec coverage**

| Spec section | Task |
| --- | --- |
| 3 Component boundaries | Tasks 5, 6, 7 |
| 4 Backend contract flags | Task 4 |
| 5 Merged query data flow | Task 7 |
| 6 Ranking | Tasks 1, 3 |
| 7 Frecency store | Tasks 2, 5 |
| 8 Selection stability | Task 8 |
| 9 Errors and degradation | Task 5 (corrupt/missing), Task 6 (registry), Task 7 (generation guard, settle timer) |
| 10 Config additions | Task 4 |
| 11 Testing | Tasks 1-3, Task 9 |
| 12 Out of scope | respected: no pinning, no plugin backend, no clipboard/calculator |

**Placeholder scan** — every step carries real code or a concrete command and expected result; no TBD or "add error handling" steps.

**Type consistency** — `matchTier`, `frecencyScore`, `mergeAndSort` keep one signature across Tasks 1-3 and their call sites in Tasks 5 and 7. `Frecency.record/score/snapshot/flush` match between Task 5 and callers in Tasks 7 and 8. `BackendManager.getBackend` returns null for unknown ids in Task 6 and every caller (Tasks 6, 7) null-checks it. `installResults` is defined once in Task 7 and replaced wholesale in Task 8 rather than duplicated.