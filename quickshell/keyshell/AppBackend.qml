pragma Singleton
import QtQuick
import Quickshell
import qs.components

Singleton {
    id: appBackend

    readonly property string id: "apps"
    readonly property string name: "Applications"
    readonly property string badge: "APPS"
    readonly property string prefix: ""
    readonly property Component cardDelegate: AppCardDelegate {}

    property var index: []
    property bool ready: false

    signal resultsReady(string query, var items)

    function build() {
        const rows = [];
        const list = DesktopEntries.applications.values;
        for (let i = 0; i < list.length; i++) {
            const e = list[i];
            rows.push({
                "entry": e,
                "name": e.name || "",
                "lname": (e.name || "").toLowerCase(),
                "lgeneric": (e.genericName || "").toLowerCase(),
                "lkeywords": e.keywords ? e.keywords.join(" ").toLowerCase() : ""
            });
        }
        index = rows;
        ready = true;
    }

    function ensureBuilt() {
        if (index.length === 0 && DesktopEntries.applications.values.length > 0)
            build();
    }

    function tierOf(row, q) {
        const n = row.lname;
        if (n.startsWith(q))
            return 0;
        const parts = n.split(/[\s\-_.]+/);
        for (let i = 0; i < parts.length; i++) {
            if (parts[i].length > 0 && parts[i].startsWith(q))
                return 1;
        }
        if (n.indexOf(q) !== -1)
            return 2;
        if (row.lgeneric.indexOf(q) !== -1 || row.lkeywords.indexOf(q) !== -1)
            return 3;
        return -1;
    }

    function byName(a, b) {
        return a.lname < b.lname ? -1 : (a.lname > b.lname ? 1 : 0);
    }

    function toItem(e) {
        return {
            "id": e.id || e.name,
            "title": e.name || "",
            "subtitle": e.genericName || "",
            "icon": e.icon || "",
            "previewSource": null,
            "badge": "APPS",
            "delegate": appBackend.cardDelegate,
            "actions": [
                {
                    "label": "Launch",
                    "execute": function () {
                        if (e.execute)
                            e.execute();
                    }
                }
            ]
        };
    }

    function search(q, limit) {
        ensureBuilt();
        const query = (q || "").toLowerCase();
        let matched = [];
        if (query === "") {
            const all = index.slice();
            all.sort(byName);
            matched = all.slice(0, limit).map(function (r) { return r.entry; });
        } else {
            const hits = [];
            for (let i = 0; i < index.length; i++) {
                const t = tierOf(index[i], query);
                if (t >= 0)
                    hits.push({ "row": index[i], "tier": t });
            }
            hits.sort(function (a, b) {
                if (a.tier !== b.tier)
                    return a.tier - b.tier;
                if (a.row.lname.length !== b.row.lname.length)
                    return a.row.lname.length - b.row.lname.length;
                return a.row.lname < b.row.lname ? -1 : 1;
            });
            matched = hits.slice(0, limit).map(function (h) { return h.row.entry; });
        }
        return matched.map(toItem);
    }

    function refresh() {
        build();
    }

    Connections {
        target: DesktopEntries
        function onApplicationsChanged() {
            appBackend.build();
        }
    }
}
