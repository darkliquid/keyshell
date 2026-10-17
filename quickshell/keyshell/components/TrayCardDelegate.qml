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
    readonly property var item: entry ? entry.item : null

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
                text: root.entry && root.entry.badge ? root.entry.badge : "TRAY"
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
                const isAttn = root.entry && root.entry.statusBadge === "ATTENTION";
                if (isAttn)
                    return root.active ? Theme.slab : Theme.accent;
                return root.active ? Theme.slab : Theme.surfaceAlt;
            }
            radius: 2

            Text {
                id: statusBadgeText
                anchors.centerIn: parent
                width: parent.width - 8
                horizontalAlignment: Text.AlignHCenter
                elide: Text.ElideRight
                text: root.entry && root.entry.statusBadge ? root.entry.statusBadge : "ACTIVE"
                font: Theme.heavy(9)
                color: {
                    const isAttn = root.entry && root.entry.statusBadge === "ATTENTION";
                    if (isAttn)
                        return root.active ? Theme.accent : Theme.ink;
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
                text: "⚙"
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
                text: root.entry && root.entry.title ? root.entry.title.toUpperCase() : "SYSTEM ITEM"
                font: Theme.heavy(13)
                color: root.active ? Theme.slab : Theme.ink
                elide: Text.ElideRight
                maximumLineCount: 1
                renderType: Text.QtRendering
            }

            Text {
                width: parent.width
                text: root.entry && root.entry.subtitle ? root.entry.subtitle.toUpperCase() : ""
                font: Theme.heavy(10)
                color: root.active ? Theme.slab : Theme.muted
                elide: Text.ElideRight
                maximumLineCount: 1
                renderType: Text.QtRendering
            }
        }
    }

    Text {
        id: footerText
        anchors.bottom: parent.bottom
        anchors.left: parent.left
        anchors.right: parent.right
        text: (root.item && root.item.hasMenu) ? "[SPACE] ACTIVATE  //  [ENTER] MENU" : "[ENTER] ACTIVATE"
        font: Theme.heavy(9)
        color: root.active ? Theme.slab : Theme.muted
        horizontalAlignment: Text.AlignHCenter
        elide: Text.ElideRight
        renderType: Text.QtRendering
    }
}
