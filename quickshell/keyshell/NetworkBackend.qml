pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Networking
import qs.components

Singleton {
    id: networkBackend

    readonly property string id: "network"
    readonly property string name: "Network"
    readonly property string badge: "NETWORK"
    readonly property string prefix: "&"
    readonly property Component cardDelegate: NetworkCardDelegate {}

    property bool scanning: false
    property var items: []

    signal resultsReady(string query, var items)

    Timer {
        id: scanTimer
        interval: Config.wifiScanTimeoutMs
        repeat: false
        onTriggered: {
            stopScan();
        }
    }

    function formatSecurity(sec) {
        if (sec === undefined || sec === null)
            return "OPEN";
        if (sec === WifiSecurityType.Open)
            return "OPEN";
        if (sec === WifiSecurityType.Sae || sec === WifiSecurityType.Wpa3SuiteB192)
            return "WPA3-SAE";
        if (sec === WifiSecurityType.Wpa2Psk || sec === WifiSecurityType.Wpa2Eap)
            return "WPA2-PSK";
        if (sec === WifiSecurityType.WpaPsk || sec === WifiSecurityType.WpaEap)
            return "WPA-PSK";
        if (sec === WifiSecurityType.Owe)
            return "OWE";
        return "SECURED";
    }

    function formatConnectivity(conn) {
        if (conn === NetworkConnectivity.Full)
            return "FULL INTERNET";
        if (conn === NetworkConnectivity.Portal)
            return "CAPTIVE PORTAL";
        if (conn === NetworkConnectivity.Limited)
            return "LIMITED ACCESS";
        return "OFFLINE";
    }

    function toggleScan() {
        if (scanning)
            stopScan();
        else
            startScan();
    }

    function startScan() {
        scanning = true;
        const devModel = Networking.devices;
        if (devModel) {
            const n = devModel.count !== undefined ? devModel.count : (devModel.values ? devModel.values.length : 0);
            for (let i = 0; i < n; i++) {
                const dev = devModel.values ? devModel.values[i] : devModel.get(i);
                if (dev && (dev.type === DeviceType.Wifi || dev.scannerEnabled !== undefined))
                    dev.scannerEnabled = true;
            }
        }
        scanTimer.restart();
    }

    function stopScan() {
        scanTimer.stop();
        scanning = false;
        const devModel = Networking.devices;
        if (devModel) {
            const n = devModel.count !== undefined ? devModel.count : (devModel.values ? devModel.values.length : 0);
            for (let i = 0; i < n; i++) {
                const dev = devModel.values ? devModel.values[i] : devModel.get(i);
                if (dev && (dev.type === DeviceType.Wifi || dev.scannerEnabled !== undefined))
                    dev.scannerEnabled = false;
            }
        }
    }

    function toggleWifi() {
        Networking.wifiEnabled = !Networking.wifiEnabled;
    }

    function connectNetwork(item) {
        if (!item)
            return;
        if (item.isWifi && item.network) {
            if (item.network.known || item.network.security === WifiSecurityType.Open || item.network.security === WifiSecurityType.Owe) {
                item.network.connect();
                Launcher.dismiss();
            } else {
                if (Launcher.openNetworkPassword)
                    Launcher.openNetworkPassword(item);
            }
        }
    }

    function connectWithPsk(network, psk) {
        if (!network)
            return;
        network.connectWithPsk(psk);
        Launcher.dismiss();
    }

    function disconnectNetwork(item) {
        if (!item)
            return;
        if (item.isWifi && item.network)
            item.network.disconnect();
    }

    function forgetNetwork(item) {
        if (!item)
            return;
        if (item.isWifi && item.network)
            item.network.forget();
    }

    function updateItems() {
        const devModel = Networking.devices;
        if (!devModel) {
            items = [];
            return;
        }

        let rawList = [];
        const devCount = devModel.count !== undefined ? devModel.count : (devModel.values ? devModel.values.length : 0);

        for (let i = 0; i < devCount; i++) {
            const dev = devModel.values ? devModel.values[i] : devModel.get(i);
            if (!dev)
                continue;

            const isWiredDev = (dev.type === DeviceType.Wired) || (dev.hasLink !== undefined);
            const isWifiDev = (dev.type === DeviceType.Wifi) || (dev.scannerEnabled !== undefined);

            if (isWiredDev) {
                const isWiredConn = Boolean(dev.connected);
                const hasLink = Boolean(dev.hasLink);
                const speedStr = dev.linkSpeed ? (dev.linkSpeed + "M") : "ETHERNET";
                const wiredStatBadge = isWiredConn ? "CONNECTED" : (hasLink ? "READY" : "DISCONNECTED");
                const wiredItem = {
                    id: "net-wired-" + (dev.name || dev.address || i),
                    isWired: true,
                    isWifi: false,
                    title: "WIRED (" + (dev.name || "ETH") + ")",
                    hasResolvedName: true,
                    security: "WIRED",
                    signalStrength: 0.0,
                    connected: isWiredConn,
                    link: hasLink,
                    speed: speedStr,
                    statusBadge: wiredStatBadge,
                    subtitle: hasLink ? (dev.linkSpeed ? (dev.linkSpeed + " MBPS") : "LINK UP") : "NO CARRIER",
                    icon: "network-wired",
                    badge: "ETHERNET",
                    address: dev.address || "",
                    interfaceName: dev.name || "",
                    device: dev,
                    network: null,
                    delegate: networkBackend.cardDelegate,
                    actions: []
                };
                wiredItem.actions = [
                    {
                        label: isWiredConn ? "Disconnect" : "Connect",
                        execute: function() {
                            if (isWiredConn && dev.disconnect)
                                dev.disconnect();
                            Launcher.dismiss();
                        }
                    }
                ];
                rawList.push(wiredItem);
            }

            if (isWifiDev) {
                let bestWifiNets = [];
                let wifiByName = {};
                const netModel = dev.networks;
                if (netModel) {
                    const netCount = netModel.count !== undefined ? netModel.count : (netModel.values ? netModel.values.length : 0);
                    for (let j = 0; j < netCount; j++) {
                        const net = netModel.values ? netModel.values[j] : netModel.get(j);
                        if (!net)
                            continue;
                        const name = net.name || "";
                        if (name.length > 0) {
                            if (!wifiByName[name]) {
                                wifiByName[name] = net;
                            } else {
                                const prev = wifiByName[name];
                                if (net.connected && !prev.connected) {
                                    wifiByName[name] = net;
                                } else if (!net.connected && prev.connected) {
                                } else {
                                    const prevSig = prev.signalStrength !== undefined ? prev.signalStrength : 0.0;
                                    const curSig = net.signalStrength !== undefined ? net.signalStrength : 0.0;
                                    if (curSig > prevSig)
                                        wifiByName[name] = net;
                                }
                            }
                        } else {
                            bestWifiNets.push(net);
                        }
                    }
                }

                for (const name in wifiByName) {
                    bestWifiNets.push(wifiByName[name]);
                }

                for (let k = 0; k < bestWifiNets.length; k++) {
                    const net = bestWifiNets[k];
                    const hasName = Boolean(net.name && net.name.length > 0);
                    const title = hasName ? net.name : "<HIDDEN NETWORK>";
                    const secStr = formatSecurity(net.security);
                    const rawSig = net.signalStrength !== undefined ? net.signalStrength : 0.0;
                    const pct = Math.round(rawSig > 1.0 ? rawSig : rawSig * 100);
                    const isConn = Boolean(net.connected);
                    const isKnown = Boolean(net.known);
                    const statBadge = isConn ? "CONNECTED" : (net.state === ConnectionState.Connecting ? "CONNECTING" : (isKnown ? "SAVED" : (net.security === WifiSecurityType.Open ? "OPEN" : "SECURED")));

                    const wifiItem = {
                        id: "net-wifi-" + (net.name || (i + "-" + k)),
                        isWired: false,
                        isWifi: true,
                        title: title,
                        hasResolvedName: hasName,
                        security: secStr,
                        signalStrength: rawSig,
                        connected: isConn,
                        known: isKnown,
                        statusBadge: statBadge,
                        subtitle: secStr + " // " + pct + "% SIGNAL",
                        icon: "network-wireless",
                        badge: "WIFI",
                        address: dev.address || "",
                        interfaceName: dev.name || "",
                        network: net,
                        device: dev,
                        delegate: networkBackend.cardDelegate,
                        actions: []
                    };
                    wifiItem.actions = [
                        {
                            label: isConn ? "Disconnect" : "Connect",
                            execute: function() {
                                if (wifiItem.connected) {
                                    disconnectNetwork(wifiItem);
                                    Launcher.dismiss();
                                } else {
                                    connectNetwork(wifiItem);
                                }
                            }
                        }
                    ];
                    rawList.push(wifiItem);
                }
            }
        }

        rawList.sort(function(a, b) {
            const aTier = (a.isWired && a.connected) || (a.isWifi && a.connected) ? 0 : (a.isWifi && a.known ? 1 : (a.isWifi ? 2 : 3));
            const bTier = (b.isWired && b.connected) || (b.isWifi && b.connected) ? 0 : (b.isWifi && b.known ? 1 : (b.isWifi ? 2 : 3));
            if (aTier !== bTier)
                return aTier - bTier;

            if (aTier === 0) {
                if (a.isWired && !b.isWired)
                    return -1;
                if (!a.isWired && b.isWired)
                    return 1;
            }

            const aSig = a.signalStrength !== undefined ? a.signalStrength : 0.0;
            const bSig = b.signalStrength !== undefined ? b.signalStrength : 0.0;
            if (Math.abs(bSig - aSig) > 0.001)
                return bSig - aSig;

            const aTitle = a.title || "";
            const bTitle = b.title || "";
            return aTitle.localeCompare(bTitle);
        });

        items = rawList;
    }

    function search(query, maxCount) {
        updateItems();
        const limit = (maxCount !== undefined && maxCount > 0) ? maxCount : Config.maxResults;
        const q = (query || "").trim().toLowerCase();

        let filtered = [];
        for (let i = 0; i < items.length; i++) {
            const it = items[i];
            if (q.length === 0) {
                if (it.hasResolvedName || it.connected || it.known || it.isWired)
                    filtered.push(it);
            } else {
                const titleMatch = (it.title || "").toLowerCase().indexOf(q) !== -1;
                const subMatch = (it.subtitle || "").toLowerCase().indexOf(q) !== -1;
                const secMatch = (it.security || "").toLowerCase().indexOf(q) !== -1;
                const addrMatch = (it.address || "").toLowerCase().indexOf(q) !== -1;
                const badgeMatch = (it.badge || "").toLowerCase().indexOf(q) !== -1;
                const ifaceMatch = (it.interfaceName || "").toLowerCase().indexOf(q) !== -1;
                if (titleMatch || subMatch || secMatch || addrMatch || badgeMatch || ifaceMatch)
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

    Component.onCompleted: {
        updateItems();
    }
}
