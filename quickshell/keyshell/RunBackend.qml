pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Io
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

    property string terminal: ""

    signal resultsReady(string query, var items)

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
        id: terminalProbe
        command: ["sh", "-c", runBackend.probeScript()]
        running: true

        stdout: StdioCollector {
            id: terminalOutput
        }

        onExited: (exitCode, exitStatus) => {
            runBackend.terminal = (exitCode === 0) ? terminalOutput.text.trim() : "";
            if (BackendManager.activeMode === "run")
                runBackend.resultsReady(Launcher.query, runBackend.search(Launcher.query, Config.maxResults));
        }
    }

    function search(q, limit) {
        const command = (q || "").trim();
        if (command.length === 0)
            return [];

        const terminalName = runBackend.terminal.replace(/^.*\//, "");
        const hasTerminal = terminalName.length > 0;

        const action = hasTerminal
            ? {
                "label": "Run in terminal",
                "execute": function () {
                    Quickshell.execDetached([runBackend.terminal, "-e", "/bin/sh", "-lc", command]);
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