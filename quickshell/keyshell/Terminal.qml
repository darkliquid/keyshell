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