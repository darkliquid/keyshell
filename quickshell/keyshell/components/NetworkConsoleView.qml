import QtQuick
import Quickshell
import Quickshell.Widgets
import Quickshell.Networking
import qs

Item {
    id: root

    readonly property var item: Launcher.networkTargetItem

    anchors.fill: parent

    Item {
        anchors.centerIn: parent
        width: Math.min(parent.width * 0.9, 1100)
        height: 136

        Column {
            anchors.fill: parent
            spacing: 12

            Row {
                width: parent.width
                spacing: 10

                Rectangle {
                    width: 24
                    height: 24
                    anchors.verticalCenter: parent.verticalCenter
                    color: Theme.slab
                    border.width: 1
                    border.color: Theme.accentAlt
                    radius: 2

                    IconImage {
                        id: iconImg
                        anchors.centerIn: parent
                        width: 18
                        height: 18
                        source: {
                            const raw = (root.item && root.item.icon) ? String(root.item.icon) : "";
                            if (raw.length === 0)
                                return "";
                            if (raw.indexOf("://") !== -1 || raw.indexOf("/") === 0)
                                return raw;
                            return Quickshell.iconPath(raw, true);
                        }
                        visible: status === Image.Ready
                    }

                    Text {
                        anchors.centerIn: parent
                        visible: iconImg.status !== Image.Ready
                        text: (root.item && root.item.badge === "ETHERNET") ? "🌐" : "📶"
                        font.pixelSize: 12
                        color: Theme.accentAlt
                        renderType: Text.QtRendering
                    }
                }

                Text {
                    anchors.verticalCenter: parent.verticalCenter
                    width: Math.min(implicitWidth, parent.width - 480)
                    text: (root.item && root.item.title) ? root.item.title.toUpperCase() : "NETWORK"
                    font: Theme.heavy(16)
                    color: Theme.ink
                    elide: Text.ElideRight
                    renderType: Text.QtRendering
                }

                Rectangle {
                    anchors.verticalCenter: parent.verticalCenter
                    height: 20
                    width: statusBadgeText.implicitWidth + 10
                    color: (root.item && root.item.connected) ? Theme.accent : Theme.surfaceAlt
                    radius: 2

                    Text {
                        id: statusBadgeText
                        anchors.centerIn: parent
                        text: (root.item && root.item.connected) ? "CONNECTED" : ((root.item && root.item.statusBadge) ? root.item.statusBadge : "DISCONNECTED")
                        font: Theme.heavy(10)
                        color: (root.item && root.item.connected) ? Theme.slab : Theme.muted
                        renderType: Text.QtRendering
                    }
                }

                Rectangle {
                    anchors.verticalCenter: parent.verticalCenter
                    height: 20
                    width: detailsTagText.implicitWidth + 10
                    color: Theme.surfaceAlt
                    radius: 2

                    Text {
                        id: detailsTagText
                        anchors.centerIn: parent
                        text: (root.item && root.item.security) ? root.item.security.toUpperCase() : ((root.item && root.item.speed) ? root.item.speed.toUpperCase() : "ETHERNET")
                        font.family: "monospace"
                        font.pixelSize: 10
                        font.weight: Font.Bold
                        color: Theme.accentAlt
                        renderType: Text.QtRendering
                    }
                }

                Rectangle {
                    anchors.verticalCenter: parent.verticalCenter
                    height: 20
                    width: ifaceTagText.implicitWidth + 10
                    color: Theme.surfaceAlt
                    radius: 2
                    visible: Boolean(root.item && root.item.interfaceName)

                    Text {
                        id: ifaceTagText
                        anchors.centerIn: parent
                        text: (root.item && root.item.interfaceName) ? root.item.interfaceName : ""
                        font.family: "monospace"
                        font.pixelSize: 10
                        font.weight: Font.Bold
                        color: Theme.accentAlt
                        renderType: Text.QtRendering
                    }
                }

                Rectangle {
                    anchors.verticalCenter: parent.verticalCenter
                    height: 20
                    width: macTagText.implicitWidth + 10
                    color: Theme.surfaceAlt
                    radius: 2
                    visible: Boolean(root.item && root.item.address)

                    Text {
                        id: macTagText
                        anchors.centerIn: parent
                        text: (root.item && root.item.address) ? root.item.address : ""
                        font.family: "monospace"
                        font.pixelSize: 10
                        font.weight: Font.Bold
                        color: Theme.muted
                        renderType: Text.QtRendering
                    }
                }

                Rectangle {
                    anchors.verticalCenter: parent.verticalCenter
                    height: 20
                    width: connectivityPillText.implicitWidth + 10
                    color: Theme.surfaceAlt
                    radius: 2

                    Text {
                        id: connectivityPillText
                        anchors.centerIn: parent
                        text: {
                            const conn = Networking.connectivity;
                            if (conn === NetworkConnectivity.Full) return "FULL INTERNET";
                            if (conn === NetworkConnectivity.Portal) return "CAPTIVE PORTAL";
                            if (conn === NetworkConnectivity.Limited) return "LIMITED ACCESS";
                            return "OFFLINE";
                        }
                        font: Theme.heavy(10)
                        color: Networking.connectivity === NetworkConnectivity.Full ? Theme.accentAlt : Theme.muted
                        renderType: Text.QtRendering
                    }
                }

                Row {
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: 6
                    visible: Boolean(root.item && root.item.isWifi && root.item.signalStrength !== undefined)

                    Text {
                        anchors.verticalCenter: parent.verticalCenter
                        text: {
                            if (!root.item || root.item.signalStrength === undefined)
                                return "";
                            const raw = root.item.signalStrength;
                            const pct = Math.round(raw > 1.0 ? raw : raw * 100);
                            return pct + "%";
                        }
                        font: Theme.heavy(10)
                        color: Theme.accentAlt
                        renderType: Text.QtRendering
                    }

                    Row {
                        height: 14
                        anchors.verticalCenter: parent.verticalCenter
                        spacing: 2

                        Repeater {
                            model: 5

                            delegate: Rectangle {
                                required property int index
                                width: 4
                                height: [6, 8, 10, 12, 14][index]
                                radius: 1
                                anchors.bottom: parent.bottom
                                transform: Matrix4x4 {
                                    matrix: Qt.matrix4x4(
                                        1, -0.25, 0, 0,
                                        0,     1, 0, 0,
                                        0,     0, 1, 0,
                                        0,     0, 0, 1
                                    )
                                }
                                color: {
                                    const strength = (root.item && root.item.signalStrength !== undefined) ? root.item.signalStrength : 0.0;
                                    const normStrength = strength > 1.0 ? strength / 100.0 : strength;
                                    const filled = (index + 1) * 0.2 <= normStrength;
                                    return filled ? Theme.accentAlt : Theme.surfaceAlt;
                                }
                            }
                        }
                    }
                }

                Rectangle {
                    anchors.verticalCenter: parent.verticalCenter
                    height: 20
                    width: ethernetLinkText.implicitWidth + 10
                    color: Theme.surfaceAlt
                    radius: 2
                    visible: Boolean(root.item && root.item.badge === "ETHERNET")

                    Text {
                        id: ethernetLinkText
                        anchors.centerIn: parent
                        text: (root.item && root.item.connected) ? "LINK UP" : "NO CARRIER"
                        font: Theme.heavy(10)
                        color: (root.item && root.item.connected) ? Theme.accent : Theme.muted
                        renderType: Text.QtRendering
                    }
                }

                Item {
                    width: 1
                    height: 1
                }

                Text {
                    anchors.verticalCenter: parent.verticalCenter
                    text: "[ESC / ENTER] RETURN"
                    font: Theme.heavy(10)
                    color: Theme.muted
                    renderType: Text.QtRendering
                }
            }

            Item {
                width: parent.width
                height: 48

                Row {
                    anchors.centerIn: parent
                    spacing: 12

                    Rectangle {
                        id: chip1
                        height: 32
                        width: chip1Row.implicitWidth + 24
                        color: (root.item && root.item.connected) ? Theme.surfaceAlt : Theme.accent
                        border.width: 1
                        border.color: Theme.accent
                        radius: 2

                        MouseArea {
                            anchors.fill: parent
                            cursorShape: Qt.PointingHandCursor
                            onClicked: {
                                if (root.item) {
                                    if (root.item.connected)
                                        NetworkBackend.disconnectNetwork(root.item);
                                    else
                                        NetworkBackend.connectNetwork(root.item);
                                }
                            }
                        }

                        Row {
                            id: chip1Row
                            anchors.centerIn: parent
                            spacing: 6

                            Text {
                                text: "[1]"
                                font: Theme.heavy(10)
                                color: (root.item && root.item.connected) ? Theme.accentAlt : Theme.slab
                                renderType: Text.QtRendering
                            }

                            Text {
                                text: (root.item && root.item.connected) ? "DISCONNECT" : "CONNECT"
                                font: Theme.heavy(11)
                                color: (root.item && root.item.connected) ? Theme.ink : Theme.slab
                                renderType: Text.QtRendering
                            }
                        }
                    }

                    Rectangle {
                        id: chip2
                        height: 32
                        width: chip2Row.implicitWidth + 24
                        color: Theme.slab
                        border.width: 1
                        border.color: Theme.accentAlt
                        radius: 2
                        visible: Boolean(root.item && root.item.isWifi)

                        MouseArea {
                            anchors.fill: parent
                            cursorShape: Qt.PointingHandCursor
                            onClicked: {
                                if (root.item && root.item.isWifi)
                                    Launcher.openNetworkPassword(root.item);
                            }
                        }

                        Row {
                            id: chip2Row
                            anchors.centerIn: parent
                            spacing: 6

                            Text {
                                text: "[2]"
                                font: Theme.heavy(10)
                                color: Theme.accentAlt
                                renderType: Text.QtRendering
                            }

                            Text {
                                text: "ENTER PSK"
                                font: Theme.heavy(11)
                                color: Theme.ink
                                renderType: Text.QtRendering
                            }
                        }
                    }

                    Rectangle {
                        id: chip3
                        height: 32
                        width: chip3Row.implicitWidth + 24
                        color: Theme.slab
                        border.width: 1
                        border.color: Theme.surfaceAlt
                        radius: 2
                        visible: Boolean(root.item && root.item.isWifi)

                        MouseArea {
                            anchors.fill: parent
                            cursorShape: Qt.PointingHandCursor
                            onClicked: {
                                if (root.item && root.item.isWifi) {
                                    NetworkBackend.forgetNetwork(root.item);
                                    Launcher.closeNetworkConsole();
                                }
                            }
                        }

                        Row {
                            id: chip3Row
                            anchors.centerIn: parent
                            spacing: 6

                            Text {
                                text: "[3]"
                                font: Theme.heavy(10)
                                color: Theme.accent
                                renderType: Text.QtRendering
                            }

                            Text {
                                text: "FORGET"
                                font: Theme.heavy(11)
                                color: Theme.ink
                                renderType: Text.QtRendering
                            }
                        }
                    }

                    Rectangle {
                        id: chip4
                        height: 32
                        width: chip4Row.implicitWidth + 24
                        color: Theme.slab
                        border.width: 1
                        border.color: Networking.wifiEnabled ? Theme.accentAlt : Theme.surfaceAlt
                        radius: 2

                        MouseArea {
                            anchors.fill: parent
                            cursorShape: Qt.PointingHandCursor
                            onClicked: {
                                NetworkBackend.toggleWifi();
                            }
                        }

                        Row {
                            id: chip4Row
                            anchors.centerIn: parent
                            spacing: 6

                            Text {
                                text: "[4]"
                                font: Theme.heavy(10)
                                color: Theme.accentAlt
                                renderType: Text.QtRendering
                            }

                            Text {
                                text: Networking.wifiEnabled ? "WI-FI: ON" : "WI-FI: OFF"
                                font: Theme.heavy(11)
                                color: Theme.ink
                                renderType: Text.QtRendering
                            }
                        }
                    }
                }
            }

            Text {
                width: parent.width
                text: "[1-4] QUICK ACTION  //  [S] SCAN FOR NETWORKS  //  [ESC / ENTER] RETURN"
                font: Theme.heavy(10)
                color: Theme.muted
                elide: Text.ElideRight
                renderType: Text.QtRendering
            }
        }
    }
}
