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