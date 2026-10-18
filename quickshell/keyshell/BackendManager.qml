pragma Singleton
import QtQuick
import QtQml.Models
import Quickshell
import "plugin-proto.js" as PluginProto

Singleton {
    id: backendManager

    readonly property string mergedMode: Config.mergedModeId
    property string activeMode: Config.mergedModeId

    readonly property var builtinBackends: [AppBackend, WindowBackend, FileBackend, AudioBackend, MediaBackend, TrayBackend, BluetoothBackend, PowerBackend, NetworkBackend, CalculatorBackend, RunBackend]
    readonly property var backends: builtinBackends.concat(PluginRegistry.backends)

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
        const previous = getBackend(activeMode);
        if (previous && previous.stop)
            previous.stop();
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

    function triggerList() {
        const triggers = [];
        for (let i = 0; i < backends.length; i++) {
            const b = backends[i];
            if (b.prefix && b.prefix.length > 0)
                triggers.push(b.prefix);
        }
        return triggers;
    }

    function backendForPrefix(prefix) {
        for (let i = 0; i < backends.length; i++) {
            if (backends[i].prefix === prefix)
                return backends[i];
        }
        return null;
    }

    function tryTrigger(prospectiveQuery) {
        const match = PluginProto.matchTrigger(prospectiveQuery, triggerList());
        if (match === null)
            return false;
        const backend = backendForPrefix(match.trigger);
        if (!backend || backend.id === activeMode)
            return false;
        Launcher.setQuery(match.remainder);
        setMode(backend.id);
        return true;
    }

    function search(query, limit) {
        const b = getBackend(activeMode);
        if (!b)
            return;
        const syncResults = b.search(query, limit);
        if (syncResults !== null && syncResults !== undefined)
            resultsReady(tagSource(syncResults, activeMode));
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
            resultsReady(tagSource(items, sourceId));
    }

    function tagSource(items, sourceId) {
        if (!items)
            return items;
        const tagged = [];
        for (let i = 0; i < items.length; i++) {
            const copy = Object.assign({}, items[i]);
            copy.source = sourceId;
            tagged.push(copy);
        }
        return tagged;
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