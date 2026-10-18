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