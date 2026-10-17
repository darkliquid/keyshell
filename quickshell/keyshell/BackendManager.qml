pragma Singleton
import QtQuick
import Quickshell

Singleton {
    id: backendManager

    property string activeMode: "apps"

    readonly property var backends: [AppBackend, WindowBackend, FileBackend, AudioBackend, MediaBackend, TrayBackend, BluetoothBackend, PowerBackend, NetworkBackend]

    readonly property var activeBackend: getBackend(activeMode)

    signal resultsReady(var items)

    function getBackend(modeId) {
        for (let i = 0; i < backends.length; i++) {
            if (backends[i].id === modeId)
                return backends[i];
        }
        return AppBackend;
    }

    function setMode(modeId) {
        if (activeMode === modeId)
            return;
        if (activeMode === "bluetooth" && modeId !== "bluetooth")
            BluetoothBackend.stopDiscovery();
        if (activeMode === "network" && modeId !== "network")
            NetworkBackend.stopScan();
        for (let i = 0; i < backends.length; i++) {
            if (backends[i].id === modeId) {
                activeMode = modeId;
                refresh();
                return;
            }
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

    function nextCycleBackend(delta) {
        const step = delta >= 0 ? 1 : -1;
        let currentIndex = 0;
        for (let i = 0; i < backends.length; i++) {
            if (backends[i].id === activeMode) {
                currentIndex = i;
                break;
            }
        }
        for (let offset = 1; offset <= backends.length; offset++) {
            const idx = ((currentIndex + step * offset) % backends.length + backends.length) % backends.length;
            const candidate = backends[idx];
            if (canCycleTo(candidate))
                return candidate;
        }
        return AppBackend;
    }

    function nextCycleModeBadge() {
        const b = nextCycleBackend(1);
        return b ? b.badge : "APPS";
    }

    function cycleMode(delta) {
        const nextBackend = nextCycleBackend(delta);
        if (nextBackend)
            setMode(nextBackend.id);
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
        const b = activeBackend;
        if (!b)
            return;
        const syncResults = b.search(query, limit);
        if (syncResults !== null && syncResults !== undefined)
            resultsReady(syncResults);
    }

    function refresh() {
        if (activeBackend && activeBackend.refresh)
            activeBackend.refresh();
        search(Launcher.query, Config.maxResults);
    }

    Component.onCompleted: {
        for (let i = 0; i < backends.length; i++) {
            const _ = backends[i].id;
        }
    }

    Connections {
        target: AppBackend
        function onResultsReady(query, items) {
            if (backendManager.activeMode === "apps" && query === Launcher.query)
                backendManager.resultsReady(items);
        }
    }

    Connections {
        target: WindowBackend
        function onResultsReady(query, items) {
            if (backendManager.activeMode === "windows" && query === Launcher.query)
                backendManager.resultsReady(items);
        }
    }

    Connections {
        target: FileBackend
        function onResultsReady(query, items) {
            if (backendManager.activeMode === "files" && query === Launcher.query)
                backendManager.resultsReady(items);
        }
    }

    Connections {
        target: AudioBackend
        function onResultsReady(query, items) {
            if (backendManager.activeMode === "audio" && query === Launcher.query)
                backendManager.resultsReady(items);
        }
    }

    Connections {
        target: MediaBackend
        function onResultsReady(query, items) {
            if (backendManager.activeMode === "media" && query === Launcher.query)
                backendManager.resultsReady(items);
        }
    }

    Connections {
        target: TrayBackend
        function onResultsReady(query, items) {
            if (backendManager.activeMode === "tray" && query === Launcher.query)
                backendManager.resultsReady(items);
        }
    }

    Connections {
        target: BluetoothBackend
        function onResultsReady(query, items) {
            if (backendManager.activeMode === "bluetooth" && query === Launcher.query)
                backendManager.resultsReady(items);
        }
    }

    Connections {
        target: PowerBackend
        function onResultsReady(query, items) {
            if (backendManager.activeMode === "power" && query === Launcher.query)
                backendManager.resultsReady(items);
        }
    }

    Connections {
        target: NetworkBackend
        function onResultsReady(query, items) {
            if (backendManager.activeMode === "network" && query === Launcher.query)
                backendManager.resultsReady(items);
        }
    }
}
