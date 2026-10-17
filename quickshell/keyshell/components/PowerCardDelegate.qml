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
                text: root.entry && root.entry.badge ? root.entry.badge : "POWER"
                font: Theme.heavy(9)
                color: root.active ? Theme.accentAlt : Theme.slab
                renderType: Text.QtRendering
            }
        }

        Rectangle {
            id: safetyBadge
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            height: 18
            width: Math.min(safetyBadgeText.implicitWidth + 10, parent.width * 0.45)
            radius: 2
            color: {
                if (root.entry && root.entry.isDestructive)
                    return root.active ? Theme.slab : Theme.accent;
                if (root.entry && root.entry.isActive)
                    return root.active ? Theme.slab : Theme.accentAlt;
                return root.active ? Theme.slab : Theme.surfaceAlt;
            }

            Text {
                id: safetyBadgeText
                anchors.centerIn: parent
                width: parent.width - 8
                horizontalAlignment: Text.AlignHCenter
                elide: Text.ElideRight
                text: {
                    if (root.entry && root.entry.isDestructive)
                        return "DESTRUCTIVE";
                    if (root.entry && root.entry.isActive)
                        return "ACTIVE";
                    return "SAFE";
                }
                font: Theme.heavy(9)
                color: {
                    if (root.entry && root.entry.isDestructive)
                        return root.active ? Theme.accent : Theme.slab;
                    if (root.entry && root.entry.isActive)
                        return root.active ? Theme.accentAlt : Theme.slab;
                    return root.active ? Theme.ink : Theme.muted;
                }
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

        Rectangle {
            id: iconBox
            anchors.left: parent.left
            anchors.verticalCenter: parent.verticalCenter
            width: 44
            height: 44
            color: root.active ? Theme.surfaceAlt : Theme.slab
            border.width: 2
            border.color: root.active ? Theme.slab : ((root.entry && root.entry.isDestructive) ? Theme.accent : Theme.accentAlt)
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
                text: {
                    const id = root.entry && root.entry.id ? root.entry.id : "";
                    const title = root.entry && root.entry.title ? root.entry.title.toLowerCase() : "";
                    const icon = root.entry && root.entry.icon ? root.entry.icon : "";
                    if (id.indexOf("lock") !== -1 || title.indexOf("lock") !== -1 || icon.indexOf("lock") !== -1)
                        return "🔒";
                    if (id.indexOf("suspend") !== -1 || title.indexOf("suspend") !== -1 || icon.indexOf("suspend") !== -1)
                        return "💤";
                    if (id.indexOf("reboot") !== -1 || title.indexOf("reboot") !== -1 || icon.indexOf("reboot") !== -1)
                        return "🔄";
                    if (id.indexOf("poweroff") !== -1 || id.indexOf("shutdown") !== -1 || title.indexOf("power off") !== -1 || title.indexOf("shutdown") !== -1 || icon.indexOf("shutdown") !== -1)
                        return "⏻";
                    if (id.indexOf("hibernate") !== -1 || title.indexOf("hibernate") !== -1 || icon.indexOf("hibernate") !== -1)
                        return "💾";
                    if (id.indexOf("logout") !== -1 || id.indexOf("log-out") !== -1 || title.indexOf("log out") !== -1 || icon.indexOf("logout") !== -1 || icon.indexOf("log-out") !== -1)
                        return "🚪";
                    if (id.indexOf("awake") !== -1 || title.indexOf("awake") !== -1 || icon.indexOf("alarm") !== -1)
                        return "☕";
                    if (id.indexOf("profile") !== -1 || title.indexOf("profile") !== -1 || (root.entry && root.entry.badge === "PROFILE"))
                        return "⚡";
                    return "⏻";
                }
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
                text: root.entry && root.entry.title ? root.entry.title.toUpperCase() : "ACTION"
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

    Item {
        id: bottomZone
        anchors.bottom: parent.bottom
        anchors.left: parent.left
        anchors.right: parent.right
        height: 16

        Row {
            anchors.centerIn: parent
            spacing: 6
            visible: Boolean((root.entry && root.entry.isDestructive) && root.active && (Launcher.powerHoldProgress > 0))

            Text {
                anchors.verticalCenter: parent.verticalCenter
                text: "HOLD " + Math.round((Launcher.powerHoldProgress || 0.0) * 100) + "%"
                font: Theme.heavy(9)
                color: root.active ? Theme.slab : Theme.ink
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
            visible: !Boolean((root.entry && root.entry.isDestructive) && root.active && (Launcher.powerHoldProgress > 0))
            text: (root.entry && root.entry.isDestructive) ? "HOLD [ENTER] 1.0s  //  [SPACE] CONSOLE" : "[ENTER] EXECUTE  //  [SPACE] CONSOLE"
            font: Theme.heavy(9)
            color: root.active ? Theme.slab : Theme.muted
            horizontalAlignment: Text.AlignHCenter
            elide: Text.ElideRight
            renderType: Text.QtRendering
        }
    }
}
