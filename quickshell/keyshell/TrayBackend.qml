pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Services.SystemTray
import Quickshell.DBusMenu
import qs.components

Singleton {
    id: trayBackend

    readonly property string id: "tray"
    readonly property string name: "Tray"
    readonly property string badge: "TRAY"
    readonly property string prefix: "^"
    readonly property Component cardDelegate: TrayCardDelegate {}

    property var items: []
    property var rootMenuHandle: null
    property var subMenuHandle: null
    property var subMenuStack: []
    readonly property var activeChildren: trayBackend.subMenuHandle ? subMenuOpener.children : rootMenuOpener.children

    signal resultsReady(string query, var items)

    QsMenuOpener {
        id: rootMenuOpener
        menu: trayBackend.rootMenuHandle

        onChildrenChanged: {
            if (Launcher.trayMenuItem && !trayBackend.subMenuHandle)
                Launcher.trayMenuEntries = extractMenuItems(Launcher.trayMenuItem);
        }
    }

    QsMenuOpener {
        id: subMenuOpener
        menu: trayBackend.subMenuHandle

        onChildrenChanged: {
            if (Launcher.trayMenuItem && trayBackend.subMenuHandle)
                Launcher.trayMenuEntries = extractMenuItems(Launcher.trayMenuItem);
        }
    }

    Connections {
        target: Launcher
        function onTrayMenuItemChanged() {
            const it = Launcher.trayMenuItem;
            if (it) {
                releaseMenuTimer.stop();
                trayBackend.subMenuStack = [];
                trayBackend.subMenuHandle = null;
                trayBackend.rootMenuHandle = it.menu || (it.item ? it.item.menu : null);
                Launcher.trayMenuEntries = trayBackend.extractMenuItems(it);
                menuRefreshTimer.restart();
            } else {
                menuRefreshTimer.stop();
                releaseMenuTimer.restart();
            }
        }
    }

    Timer {
        id: menuRefreshTimer
        interval: 60
        repeat: true
        running: Launcher.open && Launcher.trayMenuItem !== null
        onTriggered: {
            if (Launcher.trayMenuItem) {
                if (trayBackend.subMenuHandle && trayBackend.subMenuHandle.updateLayout && (!Launcher.trayMenuEntries || Launcher.trayMenuEntries.length === 0))
                    trayBackend.subMenuHandle.updateLayout();
                const updated = trayBackend.extractMenuItems(Launcher.trayMenuItem);
                if (updated.length !== Launcher.trayMenuEntries.length) {
                    Launcher.trayMenuEntries = updated;
                } else {
                    for (let i = 0; i < updated.length; i++) {
                        if (updated[i].text !== Launcher.trayMenuEntries[i].text || updated[i].hasChildren !== Launcher.trayMenuEntries[i].hasChildren) {
                            Launcher.trayMenuEntries = updated;
                            break;
                        }
                    }
                }
            }
        }
    }

    Timer {
        id: releaseMenuTimer
        interval: 250
        repeat: false
        onTriggered: {
            if (Launcher.trayMenuItem === null) {
                trayBackend.subMenuStack = [];
                trayBackend.subMenuHandle = null;
                trayBackend.rootMenuHandle = null;
            }
        }
    }

    Connections {
        target: trayBackend.activeChildren
        ignoreUnknownSignals: true
        function onValuesChanged() {
            if (Launcher.trayMenuItem)
                Launcher.trayMenuEntries = extractMenuItems(Launcher.trayMenuItem);
        }
    }

    function cleanMenuText(str) {
        if (!str)
            return "";
        return str.replace(/&/g, "").replace(/_/g, "").trim();
    }

    function extractMenuItems(item) {
        const rawItem = item ? (item.item || item) : null;
        const childrenModel = trayBackend.activeChildren;
        if (!rawItem || !rawItem.hasMenu || !childrenModel)
            return [];
        let list = [];
        const n = childrenModel.count !== undefined ? childrenModel.count : (childrenModel.values ? childrenModel.values.length : 0);
        for (let i = 0; i < n; i++) {
            const entry = childrenModel.values ? childrenModel.values[i] : childrenModel.get(i);
            if (entry && !entry.isSeparator && entry.enabled) {
                const label = cleanMenuText(entry.text);
                if (label.length > 0) {
                    list.push({
                        text: label + (entry.hasChildren ? " ▸" : ""),
                        icon: entry.icon || "",
                        hasChildren: Boolean(entry.hasChildren),
                        entry: entry
                    });
                    if (list.length >= Config.maxTrayMenuItems)
                        break;
                }
            }
        }
        return list;
    }

    function formatCategory(cat) {
        if (cat === Category.Hardware)
            return "HARDWARE";
        if (cat === Category.SystemServices)
            return "SYSTEM";
        if (cat === Category.ApplicationStatus)
            return "APP";
        if (cat === Category.Communications)
            return "COMMUNICATION";
        return "TRAY";
    }

    function formatStatus(stat) {
        if (stat === Status.NeedsAttention)
            return "ATTENTION";
        if (stat === Status.Active)
            return "ACTIVE";
        return "PASSIVE";
    }

    function updateItems() {
        const trayModel = SystemTray.items;
        if (!trayModel) {
            items = [];
            return;
        }

        let raw = [];
        const n = trayModel.count !== undefined ? trayModel.count : (trayModel.values ? trayModel.values.length : 0);
        for (let i = 0; i < n; i++) {
            const item = trayModel.values ? trayModel.values[i] : trayModel.get(i);
            if (item)
                raw.push(item);
        }

        raw.sort(function(a, b) {
            const aAttn = (a.status === Status.NeedsAttention) ? 2 : ((a.status === Status.Active) ? 1 : 0);
            const bAttn = (b.status === Status.NeedsAttention) ? 2 : ((b.status === Status.Active) ? 1 : 0);
            if (aAttn !== bAttn)
                return bAttn - aAttn;
            const aTitle = a.title || a.tooltipTitle || a.id || "";
            const bTitle = b.title || b.tooltipTitle || b.id || "";
            return aTitle.localeCompare(bTitle);
        });

        let list = [];
        for (let i = 0; i < raw.length; i++) {
            const it = raw[i];
            const title = it.title || it.tooltipTitle || it.id || "System Item";
            const sub = it.tooltipDescription || it.tooltipTitle || formatCategory(it.category);
            const catBadge = formatCategory(it.category);
            const statBadge = formatStatus(it.status);

            list.push({
                id: "sni-" + (it.id || i),
                title: title,
                subtitle: sub,
                icon: it.icon || "application-x-executable",
                previewSource: null,
                badge: catBadge,
                statusBadge: statBadge,
                status: it.status,
                category: it.category,
                hasMenu: it.hasMenu,
                menu: it.menu,
                item: it,
                delegate: trayBackend.cardDelegate,
                actions: [
                    {
                        label: it.hasMenu ? "Open Menu" : "Activate",
                        execute: function() {
                            if (it.hasMenu)
                                Launcher.openTrayMenu(it);
                            else {
                                it.activate();
                                Launcher.dismiss();
                            }
                        }
                    }
                ]
            });
        }

        items = list;
    }

    function search(query, maxCount) {
        updateItems();
        const limit = (maxCount !== undefined && maxCount > 0) ? maxCount : Config.maxResults;
        const q = (query || "").trim().toLowerCase();

        if (q.length === 0) {
            const res = items.slice(0, limit);
            resultsReady(query, res);
            return res;
        }

        let filtered = [];
        for (let i = 0; i < items.length; i++) {
            const item = items[i];
            const titleMatch = item.title.toLowerCase().indexOf(q) !== -1;
            const subMatch = item.subtitle.toLowerCase().indexOf(q) !== -1;
            const badgeMatch = item.badge.toLowerCase().indexOf(q) !== -1;
            if (titleMatch || subMatch || badgeMatch)
                filtered.push(item);
        }

        const res = filtered.slice(0, limit);
        resultsReady(query, res);
        return res;
    }

    function refresh() {
        updateItems();
        search(Launcher.query, Config.maxResults);
    }

    function activate(item) {
        if (!item)
            return;
        item.activate();
    }

    function secondaryActivate(item) {
        if (!item)
            return;
        item.secondaryActivate();
    }

    function triggerMenuEntry(entry) {
        if (!entry)
            return false;
        if (entry.hasChildren) {
            releaseMenuTimer.stop();
            let stack = trayBackend.subMenuStack.slice();
            stack.push(trayBackend.subMenuHandle);
            trayBackend.subMenuStack = stack;
            trayBackend.subMenuHandle = entry;
            if (entry.sendOpened)
                entry.sendOpened();
            if (entry.updateLayout)
                entry.updateLayout();
            Launcher.trayMenuIndex = 0;
            Launcher.trayMenuEntries = extractMenuItems(Launcher.trayMenuItem);
            menuRefreshTimer.restart();
            return true;
        }
        if (entry.triggered)
            entry.triggered();
        else if (entry.sendTriggered)
            entry.sendTriggered();
        return false;
    }

    function popSubMenu() {
        if (!trayBackend.subMenuHandle && trayBackend.subMenuStack.length === 0)
            return false;
        let stack = trayBackend.subMenuStack.slice();
        const prev = stack.length > 0 ? stack.pop() : null;
        trayBackend.subMenuStack = stack;
        trayBackend.subMenuHandle = prev;
        Launcher.trayMenuIndex = 0;
        Launcher.trayMenuEntries = extractMenuItems(Launcher.trayMenuItem);
        menuRefreshTimer.restart();
        return true;
    }

    Connections {
        target: SystemTray.items
        ignoreUnknownSignals: true
        function onValuesChanged() {
            trayBackend.updateItems();
            if (BackendManager.activeMode === "tray")
                trayBackend.search(Launcher.query, Config.maxResults);
        }
    }

    Component.onCompleted: {
        updateItems();
    }
}
