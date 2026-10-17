import QtQuick
import Quickshell.Bluetooth
import Quickshell.Networking
import Quickshell.Services.UPower
import qs

Row {
    id: root

    spacing: 12

    Rectangle {
        id: modeBadge
        anchors.verticalCenter: parent.verticalCenter
        height: 28
        width: modeBadgeText.implicitWidth + 16
        color: Theme.accent
        radius: 2

        Text {
            id: modeBadgeText
            anchors.centerIn: parent
            text: BackendManager.activeBackend ? BackendManager.activeBackend.badge : "APPS"
            font: Theme.heavy(12)
            color: Theme.slab
            renderType: Text.QtRendering
        }

        MouseArea {
            anchors.fill: parent
            cursorShape: Qt.PointingHandCursor
            onClicked: {
                BackendManager.cycleMode(1);
            }
        }
    }

    Row {
        id: queryRow
        anchors.verticalCenter: parent.verticalCenter
        spacing: 4

        Text {
            id: queryText
            width: Math.min(implicitWidth, Math.max(60, root.width - modeBadge.width - matchCountText.implicitWidth - 48))
            text: Launcher.query.toUpperCase()
            font: Theme.heavy(22)
            color: Theme.ink
            elide: Text.ElideLeft
            renderType: Text.QtRendering
        }

        Rectangle {
            id: cursorBar
            width: 3
            height: 24
            color: Theme.accent
            visible: Launcher.open

            SequentialAnimation on opacity {
                loops: Animation.Infinite
                running: Launcher.open
                NumberAnimation { to: 0.0; duration: 400; easing.type: Easing.InOutQuad }
                NumberAnimation { to: 1.0; duration: 400; easing.type: Easing.InOutQuad }
            }
        }
    }

    Item {
        width: 1
        height: 1
    }

    Text {
        id: matchCountText
        anchors.verticalCenter: parent.verticalCenter
        font: Theme.heavy(14)
        color: Theme.muted
        renderType: Text.QtRendering

        text: {
            const count = Launcher.results.length;
            const mode = BackendManager.activeMode;
            let label = "";

            if (mode === "windows") {
                if (count === 0)
                    label = "NO WINDOWS";
            } else if (mode === "files") {
                if (count === 0)
                    label = "NO FILES";
            } else if (mode === "audio") {
                if (count === 0)
                    label = "NO AUDIO DEVICES";
            } else if (mode === "media") {
                if (count === 0)
                    label = "NO MEDIA PLAYERS";
            } else if (mode === "tray") {
                if (count === 0)
                    label = "NO TRAY ITEMS";
            } else if (mode === "bluetooth") {
                const adapter = Bluetooth.defaultAdapter;
                if (!adapter)
                    label = "NO BLUETOOTH ADAPTER";
                else if (!adapter.enabled)
                    label = "BLUETOOTH DISABLED  //  [ENTER] ENABLE";
                else if (adapter.discovering)
                    label = "📡 SCANNING...";
                else if (count === 0)
                    label = "NO BLUETOOTH DEVICES";
            } else if (mode === "power") {
                const dev = UPower.displayDevice;
                if (dev && dev.isPresent) {
                    const raw = (!dev || isNaN(dev.percentage)) ? 0 : dev.percentage;
                    const pct = Math.round(raw > 1.0 ? raw : raw * 100);
                    const stateStr = UPowerDeviceState.toString(dev.state).toUpperCase();
                    let rem = "";
                    if (dev.state === UPowerDeviceState.Discharging && dev.timeToEmpty > 0) {
                        const m = Math.round(dev.timeToEmpty / 60);
                        const hrs = Math.floor(m / 60);
                        const mins = m % 60;
                        rem = hrs > 0 ? (" (" + hrs + "h " + mins + "m)") : (" (" + mins + "m)");
                    } else if (dev.state === UPowerDeviceState.Charging && dev.timeToFull > 0) {
                        const m = Math.round(dev.timeToFull / 60);
                        const hrs = Math.floor(m / 60);
                        const mins = m % 60;
                        rem = hrs > 0 ? (" (" + hrs + "h " + mins + "m)") : (" (" + mins + "m)");
                    }
                    label = "BATTERY " + pct + "% // " + stateStr + rem;
                } else if (count === 0) {
                    label = "NO ACTIONS";
                }
            } else if (mode === "network") {
                if (!Networking.wifiHardwareEnabled)
                    label = "WI-FI HARDWARE BLOCKED";
                else if (!Networking.wifiEnabled)
                    label = "WI-FI DISABLED  //  [W] ENABLE";
                else if (NetworkBackend.scanning)
                    label = "📡 SCANNING...";
                else if (count === 0)
                    label = "NO NETWORKS";
            } else {
                if (count === 0)
                    label = "NO MATCHES";
            }

            const nextMode = BackendManager.nextCycleModeBadge();
            const hint = "[TAB -> " + nextMode + "]";

            return label.length > 0 ? (label + "  //  " + hint) : hint;
        }
    }
}
