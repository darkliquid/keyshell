pragma Singleton
import QtQuick
import Quickshell
import "rank.js" as Rank

Singleton {
    id: aggregator

    property int generation: 0
    property string pendingQuery: ""
    property int pendingLimit: Config.maxMergedResults
    property var pendingGroups: ({})
    property bool collecting: false

    signal resultsReady(var items)

    Timer {
        id: settleTimer
        interval: Config.mergeSettleMs
        repeat: false
        onTriggered: aggregator.emitMerged()
    }

    Connections {
        target: BackendManager

        function onSourceResults(sourceId, query, items) {
            aggregator.collect(sourceId, query, items);
        }
    }

    function searchableBackends() {
        const out = [];
        for (let i = 0; i < BackendManager.backends.length; i++) {
            if (BackendManager.backends[i].searchable)
                out.push(BackendManager.backends[i]);
        }
        return out;
    }

    function searchAll(query, limit) {
        generation++;
        pendingQuery = query || "";
        pendingLimit = limit || Config.maxMergedResults;
        pendingGroups = ({});
        collecting = true;

        const list = searchableBackends();
        for (let i = 0; i < list.length; i++) {
            const b = list[i];
            const sync = b.search(pendingQuery, Config.maxResults);
            if (sync !== null && sync !== undefined)
                pendingGroups[b.id] = sync;
        }
        settleTimer.restart();
    }

    function collect(sourceId, query, items) {
        if (!collecting)
            return;
        if (query !== pendingQuery)
            return;
        const backend = BackendManager.getBackend(sourceId);
        if (!backend || !backend.searchable)
            return;
        pendingGroups[sourceId] = items || [];
    }

    function emitMerged() {
        collecting = false;
        const groups = [];
        for (const sourceId in pendingGroups)
            groups.push({ "source": sourceId, "items": pendingGroups[sourceId] });
        const merged = Rank.mergeAndSort(pendingQuery, groups, Frecency.snapshot(), Date.now(), {
            "buckets": Config.frecencyBuckets,
            "countCap": Config.frecencyCountCap,
            "limit": pendingLimit
        });
        resultsReady(merged);
    }

    function isTrackable(sourceId) {
        const backend = BackendManager.getBackend(sourceId);
        return Boolean(backend && backend.trackable);
    }

    function recordActivation(item) {
        if (!item || !item.source || !item.id)
            return;
        if (!isTrackable(item.source))
            return;
        Frecency.record(item.source, item.id);
    }
}