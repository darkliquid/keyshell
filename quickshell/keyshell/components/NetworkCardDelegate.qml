import QtQuick
import Quickshell
import Quickshell.Widgets
import qs

Item {
    id: root

    readonly property var card: (parent && parent.card) ? parent.card : (parent ? parent.parent : null)
    readonly property var entry: (card && card.entry) ? card.entry : (modelData || null)
    property var modelData: null
    readonly property bool active: card ? card.active : false

    anchors.fill: parent
    anchors.margins: 10

    Item {
        id: topRow
        anchors.top: parent.top
        anchors.left: parent.left
        anchors.right: parent.right
        height: 18

        Rectangle {
            id: categoryBadge
            anchors.left: parent.left
            anchors.verticalCenter: parent.verticalCenter
            height: 18
            width: Math.min(categoryBadgeText.implicitWidth + 10, parent.width * 0.5)
            color: root.active ? Theme.slab : Theme.accentAlt
            radius: 2

            Text {
                id: categoryBadgeText
                anchors.centerIn: parent
                width: parent.width - 8
                horizontalAlignment: Text.AlignHCenter
                elide: Text.ElideRight
                text: root.entry && root.entry.badge ? root.entry.badge : "NETWORK"
                font: Theme.heavy(9)
                color: root.active ? Theme.accentAlt : Theme.slab
                renderType: Text.QtRendering
            }
        }

        Rectangle {
            id: statusBadge
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            height: 18
            width: Math.min(statusBadgeText.implicitWidth + 10, parent.width * 0.45)
            color: {
                const stat = root.entry && root.entry.statusBadge ? root.entry.statusBadge : "OPEN";
                if (stat === "CONNECTED")
                    return root.active ? Theme.slab : Theme.accent;
                if (stat === "CONNECTING")
                    return root.active ? Theme.slab : Theme.accentAlt;
                return root.active ? Theme.slab : Theme.surfaceAlt;
            }
            radius: 2

            Text {
                id: statusBadgeText
                anchors.centerIn: parent
                width: parent.width - 8
                horizontalAlignment: Text.AlignHCenter
                elide: Text.ElideRight
                text: root.entry && root.entry.statusBadge ? root.entry.statusBadge : "OPEN"
                font: Theme.heavy(9)
                color: {
                    const stat = root.entry && root.entry.statusBadge ? root.entry.statusBadge : "OPEN";
                    if (stat === "CONNECTED")
                        return root.active ? Theme.accent : Theme.ink;
                    if (stat === "CONNECTING")
                        return root.active ? Theme.accentAlt : Theme.slab;
                    if (stat === "SAVED")
                        return Theme.ink;
                    return root.active ? Theme.ink : Theme.muted;
                }
                renderType: Text.QtRendering
            }
        }
    }

    Item {
        id: middleRow
        anchors.top: topRow.bottom
        anchors.bottom: footerText.top
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.topMargin: 6
        anchors.bottomMargin: 6

        Rectangle {
            id: iconBox
            anchors.left: parent.left
            anchors.verticalCenter: parent.verticalCenter
            width: 44
            height: 44
            color: root.active ? Theme.surfaceAlt : Theme.slab
            border.width: 2
            border.color: root.active ? Theme.slab : Theme.accentAlt
            radius: 4

            IconImage {
                id: iconImg
                anchors.centerIn: parent
                width: 28
                height: 28
                source: {
                    const raw = (root.entry && root.entry.icon) ? String(root.entry.icon) : "";
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
                text: (root.entry && root.entry.badge === "ETHERNET") ? "🌐" : "📶"
                font.pixelSize: 22
                color: root.active ? Theme.accent : Theme.muted
                renderType: Text.QtRendering
            }
        }

        Column {
            anchors.left: iconBox.right
            anchors.leftMargin: 10
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            spacing: 2

            Text {
                width: parent.width
                text: root.entry && root.entry.title ? root.entry.title.toUpperCase() : "NETWORK"
                font: Theme.heavy(13)
                color: root.active ? Theme.slab : Theme.ink
                elide: Text.ElideRight
                maximumLineCount: 1
                renderType: Text.QtRendering
            }

            Item {
                id: subtitleRow
                width: parent.width
                height: 14

                Row {
                    id: signalRow
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    height: 12
                    spacing: 2
                    visible: Boolean(root.entry && root.entry.isWifi && root.entry.signalStrength !== undefined)

                    Repeater {
                        model: 5

                        delegate: Rectangle {
                            required property int index
                            width: 4
                            height: [4, 6, 8, 10, 12][index]
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
                                const strength = (root.entry && root.entry.signalStrength !== undefined) ? root.entry.signalStrength : 0.0;
                                const normStrength = strength > 1.0 ? strength / 100.0 : strength;
                                const filled = (index + 1) * 0.2 <= normStrength;
                                if (!filled)
                                    return root.active ? Theme.slab : Theme.surfaceAlt;
                                return Theme.accentAlt;
                            }
                        }
                    }
                }

                Text {
                    anchors.left: parent.left
                    anchors.right: signalRow.visible ? signalRow.left : parent.right
                    anchors.rightMargin: signalRow.visible ? 6 : 0
                    anchors.verticalCenter: parent.verticalCenter
                    text: root.entry && root.entry.subtitle ? root.entry.subtitle.toUpperCase() : ""
                    font: Theme.heavy(9)
                    color: root.active ? Theme.slab : Theme.muted
                    elide: Text.ElideRight
                    maximumLineCount: 1
                    renderType: Text.QtRendering
                }
            }
        }
    }

    Text {
        id: footerText
        anchors.bottom: parent.bottom
        anchors.left: parent.left
        anchors.right: parent.right
        text: (root.entry && root.entry.connected) ? "[ENTER] DISCONNECT  //  [SPACE] CONSOLE" : "[ENTER] CONNECT  //  [SPACE] CONSOLE"
        font: Theme.heavy(9)
        color: root.active ? Theme.slab : Theme.muted
        horizontalAlignment: Text.AlignHCenter
        elide: Text.ElideRight
        renderType: Text.QtRendering
    }
}
