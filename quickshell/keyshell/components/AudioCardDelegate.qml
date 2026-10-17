import QtQuick
import Quickshell
import Quickshell.Widgets
import qs

Item {
    id: delegate

    readonly property var card: (parent && parent.card) ? parent.card : null
    readonly property var itemData: card ? card.entry : (modelData || null)
    property var modelData: null
    readonly property bool active: card ? card.active : false
    readonly property var node: itemData ? itemData.node : null
    readonly property var audio: node ? node.audio : null
    readonly property bool isMuted: audio ? Boolean(audio.muted) : false
    readonly property real volume: (audio && audio.volume !== undefined && isFinite(audio.volume)) ? audio.volume : 0.0
    readonly property bool isDefault: itemData ? itemData.isDefault : false
    readonly property bool isSink: itemData ? (itemData.badge === "OUTPUT") : true

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
            width: Math.min(categoryText.implicitWidth + 10, parent.width * 0.5)
            color: delegate.active ? Theme.slab : (delegate.isSink ? Theme.accent : Theme.accentAlt)
            radius: 2

            Text {
                id: categoryText
                anchors.centerIn: parent
                width: parent.width - 8
                horizontalAlignment: Text.AlignHCenter
                elide: Text.ElideRight
                text: delegate.itemData ? delegate.itemData.badge : "AUDIO"
                font: Theme.heavy(9)
                color: delegate.active ? (delegate.isSink ? Theme.accent : Theme.accentAlt) : Theme.slab
                renderType: Text.QtRendering
            }
        }

        Rectangle {
            id: defaultBadge
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            height: 18
            width: Math.min(defaultText.implicitWidth + 10, parent.width * 0.45)
            color: delegate.active ? Theme.slab : Theme.accentAlt
            radius: 2
            visible: delegate.isDefault

            Text {
                id: defaultText
                anchors.centerIn: parent
                width: parent.width - 8
                horizontalAlignment: Text.AlignHCenter
                elide: Text.ElideRight
                text: "DEFAULT"
                font: Theme.heavy(9)
                color: delegate.active ? Theme.accentAlt : Theme.slab
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
            color: delegate.active ? Theme.surfaceAlt : Theme.slab
            border.width: 2
            border.color: delegate.active ? Theme.slab : (delegate.isSink ? Theme.accent : Theme.accentAlt)
            radius: 4

            IconImage {
                id: iconImg
                anchors.centerIn: parent
                width: 26
                height: 26
                source: {
                    const raw = (delegate.itemData && delegate.itemData.icon) ? String(delegate.itemData.icon) : "";
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
                text: delegate.isSink ? "🔊" : "🎙"
                font.pixelSize: 20
                color: delegate.active ? Theme.accent : Theme.muted
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
                text: delegate.itemData ? delegate.itemData.title.toUpperCase() : "AUDIO DEVICE"
                font: Theme.heavy(12)
                color: delegate.active ? Theme.slab : Theme.ink
                elide: Text.ElideRight
                maximumLineCount: 1
                renderType: Text.QtRendering
            }

            Text {
                width: parent.width
                text: delegate.itemData ? delegate.itemData.subtitle.toUpperCase() : ""
                font: Theme.heavy(9)
                color: delegate.active ? Theme.slab : Theme.muted
                elide: Text.ElideRight
                maximumLineCount: 1
                renderType: Text.QtRendering
            }

            Item {
                width: parent.width
                height: 14

                Rectangle {
                    anchors.fill: parent
                    color: delegate.active ? Theme.slab : Theme.surfaceAlt
                    border.color: delegate.active ? Theme.slab : Theme.accent
                    border.width: 1
                    radius: 2
                    visible: delegate.isMuted

                    Text {
                        anchors.centerIn: parent
                        text: "[ MUTED ]"
                        font: Theme.heavy(8)
                        color: delegate.active ? Theme.accent : Theme.accentAlt
                        renderType: Text.QtRendering
                    }
                }

                Item {
                    anchors.fill: parent
                    visible: !delegate.isMuted

                    Text {
                        id: volLabel
                        anchors.left: parent.left
                        anchors.verticalCenter: parent.verticalCenter
                        text: "VOL"
                        font: Theme.heavy(9)
                        color: delegate.active ? Theme.slab : Theme.muted
                        renderType: Text.QtRendering
                    }

                    Text {
                        id: pctLabel
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        text: Math.round(delegate.volume * 100) + "%"
                        font: Theme.heavy(9)
                        color: delegate.active ? Theme.slab : (delegate.isSink ? Theme.accent : Theme.accentAlt)
                        renderType: Text.QtRendering
                    }

                    Row {
                        id: volTicks
                        anchors.left: volLabel.right
                        anchors.leftMargin: 5
                        anchors.right: pctLabel.left
                        anchors.rightMargin: 5
                        anchors.verticalCenter: parent.verticalCenter
                        spacing: 2

                        Repeater {
                            model: 10

                            Rectangle {
                                required property int index
                                width: Math.max(2, Math.floor((volTicks.width - 18) / 10))
                                height: 7
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
                                    const activeCount = Math.round(delegate.volume * 10);
                                    if (index < activeCount)
                                        return delegate.active ? Theme.slab : (delegate.isSink ? Theme.accent : Theme.accentAlt);
                                    return delegate.active ? Theme.surfaceAlt : Theme.surfaceAlt;
                                }
                            }
                        }
                    }
                }
            }
        }
    }

    Text {
        id: footerText
        anchors.bottom: parent.bottom
        anchors.left: parent.left
        anchors.right: parent.right
        text: delegate.isDefault ? "[SPACE] ADJUST VOLUME  //  [M] MUTE" : "[ENTER] DEFAULT  //  [SPACE] VOLUME"
        font: Theme.heavy(9)
        color: delegate.active ? Theme.slab : Theme.muted
        horizontalAlignment: Text.AlignHCenter
        elide: Text.ElideRight
        renderType: Text.QtRendering
    }
}
