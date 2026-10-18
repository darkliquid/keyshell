import QtQuick
import Quickshell
import qs

Item {
    id: delegate

    readonly property var card: parent ? parent.card : null
    readonly property var itemData: card ? card.entry : null
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
            id: statePill
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            height: 18
            width: Math.min(statePillText.implicitWidth + 10, parent.width * 0.5)
            color: delegate.active ? Theme.slab : Theme.accent
            radius: 2
            visible: Boolean(delegate.itemData && delegate.itemData.stateBadge)

            Text {
                id: statePillText
                anchors.centerIn: parent
                width: parent.width - 8
                horizontalAlignment: Text.AlignHCenter
                elide: Text.ElideRight
                text: delegate.itemData && delegate.itemData.stateBadge ? delegate.itemData.stateBadge : ""
                font: Theme.heavy(9)
                color: delegate.active ? Theme.accent : Theme.slab
                renderType: Text.QtRendering
            }
        }
    }

    Item {
        id: middleRow
        anchors.top: topRow.bottom
        anchors.bottom: bottomZone.top
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.topMargin: 6
        anchors.bottomMargin: 6

        Text {
            id: glyphText
            anchors.left: parent.left
            anchors.verticalCenter: parent.verticalCenter
            width: 44
            horizontalAlignment: Text.AlignHCenter
            elide: Text.ElideRight
            text: delegate.itemData && delegate.itemData.glyph ? delegate.itemData.glyph : "🧩"
            font: Theme.heavy(24)
            color: delegate.active ? Theme.slab : Theme.ink
            renderType: Text.QtRendering
        }

        Column {
            anchors.left: glyphText.right
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            anchors.leftMargin: 8
            spacing: 2

            Text {
                width: parent.width
                text: delegate.itemData ? delegate.itemData.title : ""
                font: Theme.heavy(13)
                color: delegate.active ? Theme.slab : Theme.ink
                elide: Text.ElideMiddle
                maximumLineCount: 1
                renderType: Text.QtRendering
            }

            Text {
                width: parent.width
                text: delegate.itemData ? delegate.itemData.subtitle : ""
                font: Theme.heavy(10)
                color: delegate.active ? Theme.slab : Theme.muted
                elide: Text.ElideMiddle
                maximumLineCount: 1
                renderType: Text.QtRendering
            }
        }
    }

    Item {
        id: bottomZone
        anchors.bottom: parent.bottom
        anchors.left: parent.left
        anchors.right: parent.right
        height: 16

        readonly property bool holding: Boolean(delegate.itemData && delegate.itemData.isDestructive) && delegate.active && (Launcher.powerHoldProgress > 0)

        Row {
            anchors.centerIn: parent
            spacing: 6
            visible: bottomZone.holding

            Text {
                anchors.verticalCenter: parent.verticalCenter
                text: "HOLD " + Math.round((Launcher.powerHoldProgress || 0.0) * 100) + "%"
                font: Theme.heavy(9)
                color: delegate.active ? Theme.slab : Theme.ink
                renderType: Text.QtRendering
            }

            Rectangle {
                anchors.verticalCenter: parent.verticalCenter
                width: ticksRow.width + 6
                height: 10
                color: Theme.slab
                radius: 2

                Row {
                    id: ticksRow
                    anchors.centerIn: parent
                    spacing: 2

                    Repeater {
                        model: 10

                        delegate: Rectangle {
                            required property int index
                            width: 4
                            height: 6
                            radius: 1
                            transform: Matrix4x4 {
                                matrix: Qt.matrix4x4(
                                    1, -0.25, 0, 0,
                                    0,     1, 0, 0,
                                    0,     0, 1, 0,
                                    0,     0, 0, 1
                                )
                            }
                            color: ((index + 1) * 0.1 <= (Launcher.powerHoldProgress || 0.0) + 0.01) ? Theme.accent : Theme.surfaceAlt
                        }
                    }
                }
            }
        }

        Text {
            anchors.fill: parent
            verticalAlignment: Text.AlignVCenter
            visible: !bottomZone.holding
            text: (delegate.itemData && delegate.itemData.hint) ? delegate.itemData.hint : "[ENTER] RUN"
            font: Theme.heavy(9)
            color: delegate.active ? Theme.slab : Theme.muted
            horizontalAlignment: Text.AlignHCenter
            elide: Text.ElideRight
            renderType: Text.QtRendering
        }
    }
}