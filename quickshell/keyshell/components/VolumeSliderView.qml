import QtQuick
import Quickshell
import Quickshell.Services.Pipewire
import qs

Item {
    id: root

    property var targetNode: Launcher.sliderTargetNode

    readonly property var audio: targetNode ? targetNode.audio : null
    readonly property bool isMuted: audio ? Boolean(audio.muted) : false
    readonly property real volume: (audio && audio.volume !== undefined && isFinite(audio.volume)) ? audio.volume : 0.0
    readonly property bool isSink: targetNode ? targetNode.isSink : true
    readonly property bool isDefault: {
        if (!targetNode)
            return false;
        if (targetNode.isSink)
            return Pipewire.defaultAudioSink === targetNode;
        return Pipewire.defaultAudioSource === targetNode;
    }

    anchors.fill: parent

    Column {
        anchors.fill: parent
        anchors.leftMargin: 48
        anchors.rightMargin: 48
        anchors.topMargin: 16
        anchors.bottomMargin: 16
        spacing: 12

        Item {
            id: headerItem
            width: parent.width
            height: 36

            Row {
                id: badgesRow
                anchors.left: parent.left
                anchors.right: percentText.left
                anchors.rightMargin: 16
                anchors.verticalCenter: parent.verticalCenter
                spacing: 8

                Rectangle {
                    id: typeBadge
                    anchors.verticalCenter: parent.verticalCenter
                    height: 24
                    width: typeBadgeText.implicitWidth + 14
                    color: root.isSink ? Theme.accent : Theme.accentAlt
                    radius: 2

                    Text {
                        id: typeBadgeText
                        anchors.centerIn: parent
                        text: root.isSink ? "OUTPUT SINK" : "INPUT SOURCE"
                        font: Theme.heavy(11)
                        color: Theme.slab
                        renderType: Text.QtRendering
                    }
                }

                Rectangle {
                    id: defaultBadge
                    anchors.verticalCenter: parent.verticalCenter
                    height: 24
                    width: defaultTagText.implicitWidth + 14
                    color: Theme.accentAlt
                    radius: 2
                    visible: root.isDefault

                    Text {
                        id: defaultTagText
                        anchors.centerIn: parent
                        text: "DEFAULT"
                        font: Theme.heavy(11)
                        color: Theme.slab
                        renderType: Text.QtRendering
                    }
                }

                Text {
                    id: titleText
                    anchors.verticalCenter: parent.verticalCenter
                    width: Math.max(0, badgesRow.width - typeBadge.width - (root.isDefault ? (defaultBadge.width + badgesRow.spacing) : 0) - badgesRow.spacing)
                    text: root.targetNode ? (root.targetNode.description || root.targetNode.name || "Audio Device") : ""
                    font: Theme.heavy(20)
                    color: Theme.ink
                    elide: Text.ElideRight
                    renderType: Text.QtRendering
                }
            }

            Text {
                id: percentText
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                text: root.isMuted ? "MUTED" : Math.round(root.volume * 100) + "%"
                font: Theme.heavy(32)
                color: root.isMuted ? Theme.accentAlt : Theme.accent
                renderType: Text.QtRendering
            }
        }

        Item {
            width: parent.width
            height: 40

            Rectangle {
                id: trackBg
                anchors.fill: parent
                color: Theme.surfaceAlt
                border.color: root.isMuted ? Theme.accentAlt : Theme.muted
                border.width: root.isMuted ? 2 : 1
                transform: Matrix4x4 {
                    property real shear: Reveal.shearFactor
                    matrix: Qt.matrix4x4(
                        1, shear, 0, 0,
                        0, 1, 0, 0,
                        0, 0, 1, 0,
                        0, 0, 0, 1
                    )
                }

                Row {
                    id: ticksRow
                    anchors.fill: parent
                    anchors.margins: 6
                    spacing: 4

                    Repeater {
                        model: Config.sliderTickCount

                        Rectangle {
                            required property int index

                            width: Math.max(0, (ticksRow.width - (Config.sliderTickCount - 1) * ticksRow.spacing) / Config.sliderTickCount)
                            height: ticksRow.height

                            color: {
                                if (root.isMuted)
                                    return Theme.slab;

                                const percent = (index + 1) / 20.0;
                                if (root.volume >= percent) {
                                    if (index >= 20)
                                        return Theme.accentAlt;
                                    return Theme.accent;
                                }
                                return Theme.slab;
                            }
                        }
                    }
                }
            }
        }

        Item {
            width: parent.width
            height: 18

            Text {
                anchors.left: parent.left
                anchors.verticalCenter: parent.verticalCenter
                text: "[LEFT / RIGHT] ADJUST VOLUME  //  [M] MUTE  //  [D] SET DEFAULT"
                font: Theme.heavy(11)
                color: Theme.muted
                renderType: Text.QtRendering
            }

            Text {
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                text: "[ESC / ENTER] RETURN"
                font: Theme.heavy(11)
                color: Theme.muted
                renderType: Text.QtRendering
            }
        }
    }
}
