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

    property string queuedQuery: ""
    property int queuedLimit: Config.maxPluginResults
    property bool queuedMerged: false
    property bool hasQueued: false

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
        hasQueued = false;
        collected = [];
        if (pluginProcess.running)
            pluginProcess.running = false;
    }

    function request(query, limit) {
        const wantedQuery = query || "";
        const wantedLimit = limit || Config.maxPluginResults;
        const wantedMerged = BackendManager.isMerged();

        if (waiting) {
            queuedQuery = wantedQuery;
            queuedLimit = wantedLimit;
            queuedMerged = wantedMerged;
            hasQueued = true;
            return;
        }

        send(wantedQuery, wantedLimit, wantedMerged);
    }

    function send(query, limit, merged) {
        pendingQuery = query;
        pendingLimit = limit;
        collected = [];
        waiting = true;
        start();

        if (!pluginProcess.running) {
            waiting = false;
            emitItems();
            return;
        }

        pluginProcess.write(JSON.stringify({
            "query": query,
            "limit": limit,
            "merged": merged
        }) + "\n");
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

    function emitItems() {
        const source = collected;
        const items = [];
        for (let i = 0; i < source.length && items.length < pendingLimit; i++)
            items.push(toDisplayItem(source[i]));
        resultsReady(pendingQuery, items);
    }

    function finish() {
        emitItems();

        if (hasQueued) {
            const nextQuery = queuedQuery;
            const nextLimit = queuedLimit;
            const nextMerged = queuedMerged;
            hasQueued = false;
            send(nextQuery, nextLimit, nextMerged);
        }
    }

    function holdHint() {
        return "HOLD [ENTER] " + (Config.powerHoldDurationMs / 1000).toFixed(1) + "s  //  DOUBLE-TAP";
    }

    function actionHint(actionType) {
        if (actionType === "copy")
            return "[ENTER] COPY";
        if (actionType === "open-url")
            return "[ENTER] OPEN";
        if (actionType === "set-query")
            return "[ENTER] SEARCH";
        if (actionType === "exec-terminal")
            return "[ENTER] RUN IN TERMINAL";
        return "[ENTER] RUN";
    }

    function toDisplayItem(raw) {
        const actions = [];
        for (let i = 0; i < raw.actions.length; i++)
            actions.push(toAction(raw.actions[i]));

        const destructive = raw.destructive === true;

        return {
            "id": raw.id,
            "title": raw.title,
            "subtitle": raw.subtitle,
            "glyph": raw.glyph.length > 0 ? raw.glyph : backend.glyph,
            "stateBadge": raw.badge,
            "isDestructive": destructive,
            "hint": destructive ? holdHint() : actionHint(raw.actions[0].type),
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