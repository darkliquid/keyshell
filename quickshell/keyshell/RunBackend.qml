pragma Singleton
import QtQuick
import Quickshell
import qs.components

Singleton {
    id: runBackend

    readonly property string id: "run"
    readonly property string name: "Run Command"
    readonly property string badge: "RUN"
    readonly property string prefix: ":"
    readonly property bool searchable: false
    readonly property bool trackable: false
    readonly property Component cardDelegate: RunCardDelegate {}

    signal resultsReady(string query, var items)

    Connections {
        target: Terminal

        function onResolved() {
            if (BackendManager.activeMode === "run")
                runBackend.resultsReady(Launcher.query, runBackend.search(Launcher.query, Config.maxResults));
        }
    }

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

    function refresh() {
    }
}