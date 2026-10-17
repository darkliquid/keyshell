import QtQuick
import Quickshell
import Quickshell.Widgets
import qs

Item {
    id: root

    readonly property var device: Launcher.bluetoothTargetDevice

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
                        id: btIconImg
                        anchors.centerIn: parent
                        width: 18
                        height: 18
                        source: {
                            const raw = (root.device && root.device.icon) ? String(root.device.icon) : "";
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
                        visible: btIconImg.status !== Image.Ready
                        text: "⚡"
                        font.pixelSize: 12
                        color: Theme.accentAlt
                        renderType: Text.QtRendering
                    }
                }

                Text {
                    anchors.verticalCenter: parent.verticalCenter
                    width: Math.min(implicitWidth, parent.width - 360)
                    text: root.device && (root.device.name || root.device.deviceName) ? (root.device.name || root.device.deviceName).toUpperCase() : "BLUETOOTH DEVICE"
                    font: Theme.heavy(16)
                    color: Theme.ink
                    elide: Text.ElideRight
                    renderType: Text.QtRendering
                }

                Rectangle {
                    anchors.verticalCenter: parent.verticalCenter
                    height: 20
                    width: macText.implicitWidth + 10
                    color: Theme.surfaceAlt
                    radius: 2

                    Text {
                        id: macText
                        anchors.centerIn: parent
                        text: root.device && root.device.address ? root.device.address : "00:00:00:00:00:00"
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
                    width: statusText.implicitWidth + 10
                    color: (root.device && root.device.connected) ? Theme.accent : Theme.surfaceAlt
                    radius: 2

                    Text {
                        id: statusText
                        anchors.centerIn: parent
                        text: (root.device && root.device.connected) ? "CONNECTED" : "DISCONNECTED"
                        font: Theme.heavy(10)
                        color: (root.device && root.device.connected) ? Theme.slab : Theme.muted
                        renderType: Text.QtRendering
                    }
                }

                Row {
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: 6
                    visible: root.device && root.device.batteryAvailable

                    Text {
                        anchors.verticalCenter: parent.verticalCenter
                        text: {
                            if (!root.device || !root.device.batteryAvailable)
                                return "";
                            const raw = root.device.battery;
                            const pct = Math.round(raw > 1.0 ? raw : raw * 100);
                            return pct + "% BATTERY";
                        }
                        font: Theme.heavy(10)
                        color: Theme.accentAlt
                        renderType: Text.QtRendering
                    }

                    Row {
                        anchors.verticalCenter: parent.verticalCenter
                        spacing: 2

                        Repeater {
                            model: 10
                            delegate: Rectangle {
                                required property int index
                                width: 4
                                height: 10
                                radius: 1
                                transform: Matrix4x4 {
                                    matrix: Qt.matrix4x4(
                                        1, -0.25, 0, 0,
                                        0,     1, 0, 0,
                                        0,     0, 1, 0,
                                        0,     0, 0, 1
                                    )
                                }
                                color: {
                                    const raw = root.device ? root.device.battery : 0.0;
                                    const pct = Math.round(raw > 1.0 ? raw : raw * 100);
                                    const filled = (index + 1) * 10 <= pct;
                                    return filled ? Theme.accentAlt : Theme.surfaceAlt;
                                }
                            }
                        }
                    }
                }

                Item {
                    width: 1
                    height: 1
                }

                Text {
                    anchors.verticalCenter: parent.verticalCenter
                    text: "[ESC] RETURN TO REEL"
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
                        color: (root.device && root.device.connected) ? Theme.surfaceAlt : Theme.accent
                        border.width: 1
                        border.color: Theme.accent
                        radius: 2

                        MouseArea {
                            anchors.fill: parent
                            cursorShape: Qt.PointingHandCursor
                            onClicked: {
                                if (root.device) {
                                    if (root.device.connected)
                                        root.device.disconnect();
                                    else
                                        root.device.connect();
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
                                color: (root.device && root.device.connected) ? Theme.accentAlt : Theme.slab
                                renderType: Text.QtRendering
                            }

                            Text {
                                text: (root.device && root.device.connected) ? "DISCONNECT" : "CONNECT"
                                font: Theme.heavy(11)
                                color: (root.device && root.device.connected) ? Theme.ink : Theme.slab
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

                        MouseArea {
                            anchors.fill: parent
                            cursorShape: Qt.PointingHandCursor
                            onClicked: {
                                if (root.device) {
                                    if (root.device.pairing)
                                        root.device.cancelPair();
                                    else if (!root.device.paired)
                                        root.device.pair();
                                }
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
                                text: (root.device && root.device.pairing) ? "CANCEL PAIR" : ((root.device && root.device.paired) ? "PAIRED" : "PAIR")
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
                        border.color: (root.device && root.device.trusted) ? Theme.accent : Theme.surfaceAlt
                        radius: 2

                        MouseArea {
                            anchors.fill: parent
                            cursorShape: Qt.PointingHandCursor
                            onClicked: {
                                if (root.device)
                                    root.device.setTrusted(!root.device.trusted);
                            }
                        }

                        Row {
                            id: chip3Row
                            anchors.centerIn: parent
                            spacing: 6

                            Text {
                                text: "[3]"
                                font: Theme.heavy(10)
                                color: Theme.accentAlt
                                renderType: Text.QtRendering
                            }

                            Text {
                                text: (root.device && root.device.trusted) ? "UNTRUST" : "TRUST"
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
                        border.color: Theme.surfaceAlt
                        radius: 2

                        MouseArea {
                            anchors.fill: parent
                            cursorShape: Qt.PointingHandCursor
                            onClicked: {
                                if (root.device) {
                                    root.device.forget();
                                    Launcher.closeBluetoothConsole();
                                }
                            }
                        }

                        Row {
                            id: chip4Row
                            anchors.centerIn: parent
                            spacing: 6

                            Text {
                                text: "[4]"
                                font: Theme.heavy(10)
                                color: Theme.accent
                                renderType: Text.QtRendering
                            }

                            Text {
                                text: "FORGET"
                                font: Theme.heavy(11)
                                color: Theme.accent
                                renderType: Text.QtRendering
                            }
                        }
                    }
                }
            }

            Text {
                width: parent.width
                text: "[1-4] QUICK ACTION  //  [S] TOGGLE SCAN  //  [ESC] RETURN"
                font: Theme.heavy(10)
                color: Theme.muted
                elide: Text.ElideRight
                renderType: Text.QtRendering
            }
        }
    }
}
