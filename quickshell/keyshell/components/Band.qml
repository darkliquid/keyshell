import QtQuick
import QtQuick.Shapes
import Quickshell.Bluetooth
import Quickshell.Networking
import qs

Item {
    id: band

    property real expandProgress: 0.0
    property real contentOpacity: 0.0

    function resetView() {
        if (reel.visible)
            reel.positionViewAtIndex(Launcher.displayIndex, ListView.Center);
    }

    anchors.fill: parent

    layer.enabled: true
    layer.smooth: true
    layer.samples: 4

    Shape {
        id: slabShape
        anchors.fill: parent
        visible: band.expandProgress > 0

        ShapePath {
            strokeWidth: 0
            strokeColor: "transparent"
            fillColor: Theme.slab

            startX: 0
            startY: Reveal.yLeft - (Reveal.hLeft / 2) * band.expandProgress

            PathLine {
                x: band.width
                y: Reveal.yRight - (Reveal.hRight / 2) * band.expandProgress
            }
            PathLine {
                x: band.width
                y: Reveal.yRight + (Reveal.hRight / 2) * band.expandProgress
            }
            PathLine {
                x: 0
                y: Reveal.yLeft + (Reveal.hLeft / 2) * band.expandProgress
            }
            PathLine {
                x: 0
                y: Reveal.yLeft - (Reveal.hLeft / 2) * band.expandProgress
            }
        }
    }

    Item {
        id: contentContainer
        width: Math.sqrt(band.width * band.width + (Reveal.yRight - Reveal.yLeft) * (Reveal.yRight - Reveal.yLeft))
        height: Math.min(Reveal.hLeft, Reveal.hRight) * Math.cos(Reveal.angleRad)
        x: (band.width - width) / 2
        y: Reveal.yc - height / 2
        rotation: Reveal.angleRad * 180 / Math.PI
        transformOrigin: Item.Center
        opacity: band.contentOpacity
        visible: opacity > 0

        readonly property bool isReelActive: Launcher.sliderTargetNode === null && Launcher.mediaConsolePlayer === null && Launcher.trayMenuItem === null && Launcher.bluetoothTargetDevice === null && !Launcher.powerConsoleOpen && Launcher.networkTargetItem === null && Launcher.networkPasswordTarget === null

        SearchField {
            id: searchField
            anchors.horizontalCenter: parent.horizontalCenter
            anchors.top: parent.top
            anchors.topMargin: Config.contentPaddingPx
            width: Math.min(parent.width * 0.8, 1200)
            height: Config.searchHeightPx
            visible: contentContainer.isReelActive
        }

        Reel {
            id: reel
            anchors.top: searchField.bottom
            anchors.topMargin: 8
            anchors.bottom: parent.bottom
            anchors.bottomMargin: Config.contentPaddingPx
            anchors.left: parent.left
            anchors.right: parent.right
            cardWidth: Math.min(Config.cardWidthMaxPx, band.width / 3)
            visible: contentContainer.isReelActive && Launcher.results.length > 0
        }

        Item {
            id: emptyStateView
            anchors.top: searchField.bottom
            anchors.topMargin: 8
            anchors.bottom: parent.bottom
            anchors.bottomMargin: Config.contentPaddingPx
            anchors.left: parent.left
            anchors.right: parent.right
            visible: contentContainer.isReelActive && Launcher.results.length === 0

            Item {
                id: emptyPlaque
                anchors.centerIn: parent
                width: Math.min(460, parent.width * 0.8)
                height: Math.max(96, parent.height - Theme.shadowOffset - 8)

                Rectangle {
                    anchors.fill: parent
                    x: Theme.shadowOffset
                    y: Theme.shadowOffset
                    color: Theme.accentAlt
                    opacity: 0.65
                    antialiasing: true
                    transform: Matrix4x4 {
                        matrix: Qt.matrix4x4(1, Reveal.shearFactor, 0, -Reveal.shearFactor * emptyPlaque.height * 0.5,
                                             0, 1, 0, 0,
                                             0, 0, 1, 0,
                                             0, 0, 0, 1)
                    }
                }

                Rectangle {
                    anchors.fill: parent
                    color: Theme.surfaceAlt
                    border.color: Theme.accentAlt
                    border.width: 2
                    antialiasing: true
                    transform: Matrix4x4 {
                        matrix: Qt.matrix4x4(1, Reveal.shearFactor, 0, -Reveal.shearFactor * emptyPlaque.height * 0.5,
                                             0, 1, 0, 0,
                                             0, 0, 1, 0,
                                             0, 0, 0, 1)
                    }
                }

                Column {
                    anchors.centerIn: parent
                    width: parent.width - Math.ceil(Math.abs(Reveal.shearFactor) * emptyPlaque.height) - 28
                    spacing: 6

                    Text {
                        width: parent.width
                        horizontalAlignment: Text.AlignHCenter
                        elide: Text.ElideRight
                        font: Theme.heavy(16)
                        color: Theme.ink
                        renderType: Text.QtRendering
                        text: {
                            const mode = BackendManager.activeMode;
                            const q = Launcher.query.trim();
                            if (mode === "files" && q.length === 0)
                                return "TYPE TO SEARCH FILES";
                            if (mode === "media" && q.length === 0)
                                return "NO ACTIVE MEDIA PLAYERS";
                            if (mode === "windows" && q.length === 0)
                                return "NO OPEN WINDOWS";
                            if (mode === "tray" && q.length === 0)
                                return "NO SYSTEM TRAY ITEMS";
                            if (mode === "audio" && q.length === 0)
                                return "NO AUDIO DEVICES";
                            if (mode === "bluetooth") {
                                const adapter = Bluetooth.defaultAdapter;
                                if (!adapter)
                                    return "NO BLUETOOTH ADAPTER";
                                if (!adapter.enabled)
                                    return "BLUETOOTH RADIO DISABLED";
                                if (q.length === 0)
                                    return adapter.discovering ? "SCANNING FOR DEVICES..." : "NO BLUETOOTH DEVICES";
                            }
                            if (mode === "network") {
                                if (!Networking.wifiHardwareEnabled)
                                    return "WI-FI HARDWARE BLOCKED";
                                if (!Networking.wifiEnabled)
                                    return "WI-FI RADIO DISABLED";
                                if (q.length === 0)
                                    return NetworkBackend.scanning ? "SCANNING FOR NETWORKS..." : "NO NETWORKS FOUND";
                            }
                            return "NO MATCHES FOR \"" + q.toUpperCase() + "\"";
                        }
                    }

                    Text {
                        width: parent.width
                        horizontalAlignment: Text.AlignHCenter
                        elide: Text.ElideRight
                        font: Theme.heavy(10)
                        color: Theme.accentAlt
                        renderType: Text.QtRendering
                        text: {
                            const mode = BackendManager.activeMode;
                            const q = Launcher.query.trim();
                            if (q.length > 0)
                                return "[BACKSPACE] CLEAR FILTER  //  [TAB] SWITCH MODE";
                            if (mode === "files")
                                return "START TYPING FILENAME OR PATH  //  [TAB] SWITCH MODE";
                            if (mode === "bluetooth") {
                                const adapter = Bluetooth.defaultAdapter;
                                if (adapter && !adapter.enabled)
                                    return "[ENTER / E] ENABLE BLUETOOTH  //  [TAB] SWITCH MODE";
                                return "[S] TOGGLE SCAN  //  [E] POWER  //  [TAB] SWITCH MODE";
                            }
                            if (mode === "network") {
                                if (!Networking.wifiEnabled)
                                    return "[W] ENABLE WI-FI  //  [TAB] SWITCH MODE";
                                return "[S] SCAN NETWORKS  //  [W] TOGGLE WI-FI  //  [TAB] SWITCH MODE";
                            }
                            return "[TAB] SWITCH MODE  //  [ESC] DISMISS";
                        }
                    }
                }
            }
        }

        VolumeSliderView {
            id: sliderView
            anchors.fill: parent
            visible: Launcher.sliderTargetNode !== null
        }

        MediaConsoleView {
            id: mediaConsoleView
            anchors.fill: parent
            visible: Launcher.mediaConsolePlayer !== null
        }

        TrayMenuView {
            id: trayMenuView
            anchors.fill: parent
            visible: Launcher.trayMenuItem !== null
        }

        BluetoothConsoleView {
            id: bluetoothConsoleView
            anchors.fill: parent
            visible: Launcher.bluetoothTargetDevice !== null
        }

        PowerConsoleView {
            id: powerConsoleView
            anchors.fill: parent
            visible: Launcher.powerConsoleOpen
        }

        NetworkConsoleView {
            id: networkConsoleView
            anchors.fill: parent
            visible: Launcher.networkTargetItem !== null
        }

        NetworkPasswordView {
            id: networkPasswordView
            anchors.fill: parent
            visible: Launcher.networkPasswordTarget !== null
        }
    }
}
