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
        dirty = false;
        prune();
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