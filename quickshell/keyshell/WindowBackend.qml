pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Io
import Quickshell.Wayland
import qs.components

Singleton {
    id: windowBackend

    readonly property string id: "windows"
    readonly property string name: "Windows"
    readonly property string badge: "WINDOWS"
    readonly property string prefix: "@"
    readonly property Component cardDelegate: WindowCardDelegate {}

    property var wmWindows: ({})

    signal resultsReady(string query, var items)

    Process {
        id: wmQuery
        command: [
            "sh",
            "-c",
            "if [ -n \"$NIRI_SOCKET\" ] && command -v niri >/dev/null 2>&1; then printf '{\"wm\":\"niri\",\"windows\":'; niri msg -j windows 2>/dev/null || printf '[]'; printf ',\"workspaces\":'; niri msg -j workspaces 2>/dev/null || printf '[]'; printf '}'; elif [ -n \"$SWAYSOCK\" ] && command -v swaymsg >/dev/null 2>&1; then printf '{\"wm\":\"sway\",\"tree\":'; swaymsg -t get_tree 2>/dev/null || printf '{}'; printf '}'; else printf '{}'; fi"
        ]
        stdout: SplitParser {
            splitMarker: ""
            onRead: (data) => {
                try {
                    const payload = JSON.parse(data);
                    const map = {};
                    const addEntry = (key, entry) => {
                        if (!map[key])
                            map[key] = [];
                        map[key].push(entry);
                    };

                    if (payload.wm === "niri" && Array.isArray(payload.windows)) {
                        const wsById = {};
                        const workspaces = Array.isArray(payload.workspaces) ? payload.workspaces : [];
                        for (let i = 0; i < workspaces.length; i++) {
                            const ws = workspaces[i];
                            if (ws && ws.id !== undefined)
                                wsById[ws.id] = ws;
                        }

                        for (let i = 0; i < payload.windows.length; i++) {
                            const win = payload.windows[i];
                            if (!win)
                                continue;
                            const appId = win.app_id || "";
                            const title = win.title || "Untitled Window";
                            const ws = (win.workspace_id !== undefined && wsById[win.workspace_id]) ? wsById[win.workspace_id] : null;
                            const wsTag = ws
                                ? (ws.name ? ws.name.toUpperCase() : ("WS " + (ws.idx !== undefined ? ws.idx : ws.id)))
                                : (win.workspace_id ? ("WS " + win.workspace_id) : "");
                            const outputName = ws && ws.output ? String(ws.output) : "";
                            const entry = {
                                wsTag: wsTag,
                                outputName: outputName
                            };
                            addEntry(appId + "::" + title, entry);
                            if (appId.length > 0)
                                addEntry(appId, entry);
                        }
                    } else if (payload.wm === "sway" && payload.tree) {
                        const walkSway = (node, currentOutput, currentWsName) => {
                            if (!node)
                                return;
                            let outName = currentOutput;
                            let wsName = currentWsName;
                            if (node.type === "output" && node.name && node.name !== "__i3")
                                outName = String(node.name);
                            if (node.type === "workspace")
                                wsName = node.name ? ("WS " + String(node.name).toUpperCase()) : "WS";
                            const isLeaf = (node.type === "con" || node.type === "floating_con") && (!node.nodes || node.nodes.length === 0) && (!node.floating_nodes || node.floating_nodes.length === 0) && (node.app_id || node.window_properties || node.name);
                            if (isLeaf) {
                                const appId = node.app_id || (node.window_properties && node.window_properties.class) || "";
                                const title = node.name || "Untitled Window";
                                const entry = {
                                    wsTag: wsName || "",
                                    outputName: outName || ""
                                };
                                addEntry(appId + "::" + title, entry);
                                if (appId.length > 0)
                                    addEntry(appId, entry);
                            }
                            const children = (node.nodes || []).concat(node.floating_nodes || []);
                            for (let i = 0; i < children.length; i++)
                                walkSway(children[i], outName, wsName);
                        };
                        walkSway(payload.tree, "", "");
                    }

                    windowBackend.wmWindows = map;
                    if (BackendManager.activeMode === "windows")
                        windowBackend.resultsReady(Launcher.query, windowBackend.search(Launcher.query, Config.maxWindowResults));
                } catch (e) {
                }
            }
        }
    }

    Connections {
        target: ToplevelManager.toplevels
        function onValuesChanged() {
            if (BackendManager.activeMode === "windows")
                windowBackend.resultsReady(Launcher.query, windowBackend.search(Launcher.query, Config.maxWindowResults));
        }
    }

    function toItem(tl, usedCounts) {
        const titleStr = tl.title || "Untitled Window";
        const appIdStr = tl.appId || "";
        const lookupKey = appIdStr + "::" + titleStr;
        const candidates = windowBackend.wmWindows[lookupKey] || windowBackend.wmWindows[appIdStr] || [];
        const usedIdx = usedCounts[lookupKey] || 0;
        usedCounts[lookupKey] = usedIdx + 1;
        const wmEntry = candidates.length > 0 ? candidates[Math.min(usedIdx, candidates.length - 1)] : null;
        const wsTag = (wmEntry && wmEntry.wsTag) ? wmEntry.wsTag : (tl.activated ? "ACTIVE" : "");
        const outputName = (wmEntry && wmEntry.outputName) ? wmEntry.outputName : "";

        return {
            "id": appIdStr + "::" + titleStr + "::" + usedIdx,
            "title": titleStr,
            "subtitle": wsTag,
            "icon": appIdStr,
            "previewSource": tl,
            "outputName": outputName,
            "badge": "WINDOWS",
            "delegate": windowBackend.cardDelegate,
            "actions": [
                {
                    "label": "Focus",
                    "execute": function () {
                        tl.activate();
                    }
                }
            ]
        };
    }

    function search(q, limit) {
        const maxCount = (limit !== undefined && limit > 0) ? limit : Config.maxWindowResults;
        if (limit !== undefined && limit <= 0)
            return [];

        const query = (q || "").toLowerCase();
        const toplevels = ToplevelManager.toplevels.values || [];
        const results = [];
        const usedCounts = {};

        for (let i = 0; i < toplevels.length; i++) {
            const tl = toplevels[i];
            if (!tl)
                continue;

            const title = (tl.title || "").toLowerCase();
            const appId = (tl.appId || "").toLowerCase();

            if (query === "" || title.indexOf(query) !== -1 || appId.indexOf(query) !== -1)
                results.push(toItem(tl, usedCounts));

            if (results.length >= maxCount)
                break;
        }

        return results;
    }

    function refresh() {
        if (!wmQuery.running)
            wmQuery.running = true;
        resultsReady(Launcher.query, search(Launcher.query, Config.maxWindowResults));
    }

    Component.onCompleted: {
        const _ = ToplevelManager.toplevels.values;
        refresh();
    }
}
