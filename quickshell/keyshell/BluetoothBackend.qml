pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Bluetooth
import qs.components

Singleton {
    id: bluetoothBackend

    readonly property string id: "bluetooth"
    readonly property string name: "Bluetooth"
    readonly property string badge: "BLUETOOTH"
    readonly property string prefix: "*"
    readonly property Component cardDelegate: BluetoothCardDelegate {}

    property var items: []

    signal resultsReady(string query, var items)

    Timer {
        id: scanTimer
        interval: Config.bluetoothScanTimeoutMs
        repeat: false
        onTriggered: {
            stopDiscovery();
        }
    }

    function formatCategory(icon) {
        if (!icon)
            return "DEVICE";
        if (icon.indexOf("head") !== -1 || icon.indexOf("audio") !== -1)
            return "AUDIO";
        if (icon.indexOf("keyboard") !== -1)
            return "KEYBOARD";
        if (icon.indexOf("mouse") !== -1 || icon.indexOf("tablet") !== -1)
            return "MOUSE";
        if (icon.indexOf("gamepad") !== -1 || icon.indexOf("gaming") !== -1)
            return "GAMEPAD";
        if (icon.indexOf("phone") !== -1)
            return "PHONE";
        return "DEVICE";
    }

    function formatStatus(dev) {
        if (!dev)
            return "DISCOVERED";
        if (dev.connected)
            return "CONNECTED";
        if (dev.state === BluetoothDeviceState.Connecting || dev.pairing)
            return "CONNECTING";
        if (dev.paired)
            return "PAIRED";
        return "DISCOVERED";
    }

    function updateItems() {
        const adapter = Bluetooth.defaultAdapter;
        const devModel = adapter ? adapter.devices : Bluetooth.devices;
        if (!devModel) {
            items = [];
            return;
        }

        let raw = [];
        const n = devModel.count !== undefined ? devModel.count : (devModel.values ? devModel.values.length : 0);
        for (let i = 0; i < n; i++) {
            const dev = devModel.values ? devModel.values[i] : devModel.get(i);
            if (dev)
                raw.push(dev);
        }

        raw.sort(function(a, b) {
            const aTier = (a.connected || a.state === BluetoothDeviceState.Connecting || a.pairing) ? 0 : (a.paired ? 1 : 2);
            const bTier = (b.connected || b.state === BluetoothDeviceState.Connecting || b.pairing) ? 0 : (b.paired ? 1 : 2);
            if (aTier !== bTier)
                return aTier - bTier;
            const aName = a.name || a.deviceName || a.address || "";
            const bName = b.name || b.deviceName || b.address || "";
            return aName.localeCompare(bName);
        });

        let list = [];
        for (let i = 0; i < raw.length; i++) {
            const dev = raw[i];
            const hasName = (dev.name && dev.name.length > 0) || (dev.deviceName && dev.deviceName.length > 0);
            const title = dev.name || dev.deviceName || dev.address || "Bluetooth Device";
            const catBadge = formatCategory(dev.icon);
            const statBadge = formatStatus(dev);

            let sub = statBadge;
            if (dev.batteryAvailable) {
                const rawBat = dev.battery;
                const pct = Math.round(rawBat > 1.0 ? rawBat : rawBat * 100);
                sub = "BATTERY " + pct + "% // " + (dev.trusted ? "TRUSTED" : sub);
            } else if (dev.trusted) {
                sub = sub + " // TRUSTED";
            }

            list.push({
                id: "bt-" + (dev.address || i),
                title: title,
                subtitle: sub,
                icon: dev.icon || "preferences-system-bluetooth",
                previewSource: null,
                badge: catBadge,
                statusBadge: statBadge,
                address: dev.address || "",
                hasResolvedName: hasName,
                device: dev,
                delegate: bluetoothBackend.cardDelegate,
                actions: [
                    {
                        label: dev.connected ? "Disconnect" : "Connect",
                        execute: function() {
                            if (dev.connected)
                                dev.disconnect();
                            else
                                dev.connect();
                            Launcher.dismiss();
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

        let filtered = [];
        for (let i = 0; i < items.length; i++) {
            const it = items[i];
            if (q.length === 0) {
                if (it.hasResolvedName || it.device.connected || it.device.paired)
                    filtered.push(it);
            } else {
                const titleMatch = it.title.toLowerCase().indexOf(q) !== -1;
                const subMatch = it.subtitle.toLowerCase().indexOf(q) !== -1;
                const addrMatch = it.address.toLowerCase().indexOf(q) !== -1;
                const badgeMatch = it.badge.toLowerCase().indexOf(q) !== -1;
                if (titleMatch || subMatch || addrMatch || badgeMatch)
                    filtered.push(it);
            }
        }

        const res = filtered.slice(0, limit);
        resultsReady(query, res);
        return res;
    }

    function refresh() {
        updateItems();
        search(Launcher.query, Config.maxResults);
    }

    function toggleDiscovery() {
        const adapter = Bluetooth.defaultAdapter;
        if (!adapter)
            return;
        if (adapter.discovering)
            stopDiscovery();
        else
            startDiscovery();
    }

    function startDiscovery() {
        const adapter = Bluetooth.defaultAdapter;
        if (!adapter)
            return;
        adapter.discovering = true;
        scanTimer.restart();
    }

    function stopDiscovery() {
        scanTimer.stop();
        const adapter = Bluetooth.defaultAdapter;
        if (adapter && adapter.discovering)
            adapter.discovering = false;
    }

    function togglePower() {
        const adapter = Bluetooth.defaultAdapter;
        if (!adapter)
            return;
        adapter.enabled = !adapter.enabled;
    }

    Component.onCompleted: {
        updateItems();
    }
}
