pragma Singleton
import QtQuick
import QtQml.Models
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
                BackendManager.backendResults(modelData.id, query, items);
            }
        }
    }
}