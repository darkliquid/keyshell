import QtQuick
import Quickshell
import Quickshell.Wayland
import Quickshell.Widgets
import qs

Item {
    id: delegate

    readonly property var card: parent ? parent.card : null
    readonly property var itemData: card ? card.entry : null
    readonly property bool active: card ? card.active : false

    readonly property bool canCaptureToplevel: {
        const desktop = String(Quickshell.env("XDG_CURRENT_DESKTOP") || "");
        const hyprSig = String(Quickshell.env("HYPRLAND_INSTANCE_SIGNATURE") || "");
        return hyprSig.length > 0 || desktop.indexOf("Hyprland") !== -1;
    }

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
            color: delegate.active ? Theme.slab : Theme.accentAlt
            radius: 2

            Text {
                id: categoryBadgeText
                anchors.centerIn: parent
                width: parent.width - 8
                horizontalAlignment: Text.AlignHCenter
                elide: Text.ElideRight
                text: "WINDOW"
                font: Theme.heavy(9)
                color: delegate.active ? Theme.accentAlt : Theme.slab
                renderType: Text.QtRendering
            }
        }

        Rectangle {
            id: wsBadge
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            height: 18
            width: Math.min(wsText.implicitWidth + 10, parent.width * 0.45)
            color: delegate.active ? Theme.slab : Theme.accent
            radius: 2
            visible: Boolean(delegate.itemData && delegate.itemData.subtitle)

            Text {
                id: wsText
                anchors.centerIn: parent
                width: parent.width - 8
                horizontalAlignment: Text.AlignHCenter
                elide: Text.ElideRight
                text: delegate.itemData ? delegate.itemData.subtitle : ""
                font: Theme.heavy(9)
                color: delegate.active ? Theme.accent : Theme.slab
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
            id: previewBox
            anchors.left: parent.left
            anchors.verticalCenter: parent.verticalCenter
            width: 76
            height: 46
            color: delegate.active ? Theme.surfaceAlt : Theme.slab
            border.width: 1.5
            border.color: delegate.active ? Theme.slab : Theme.accentAlt
            radius: 3
            clip: true

            ScreencopyView {
                id: toplevelPreview
                anchors.fill: parent
                anchors.margins: 2
                captureSource: (Launcher.open && BackendManager.activeMode === "windows" && delegate.canCaptureToplevel && delegate.itemData) ? delegate.itemData.previewSource : null
                live: Launcher.open && BackendManager.activeMode === "windows" && delegate.canCaptureToplevel
                paintCursor: false
                visible: hasContent
            }

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
                visible: !toplevelPreview.hasContent && status === Image.Ready
            }

            Text {
                anchors.centerIn: parent
                visible: !toplevelPreview.hasContent && iconImg.status !== Image.Ready
                text: "🗔"
                font.pixelSize: 20
                color: delegate.active ? Theme.accent : Theme.muted
                renderType: Text.QtRendering
            }

            Rectangle {
                anchors.right: parent.right
                anchors.bottom: parent.bottom
                anchors.margins: 2
                width: 16
                height: 16
                radius: 2
                color: Theme.slab
                border.width: 1
                border.color: Theme.accentAlt
                visible: toplevelPreview.hasContent && cornerIcon.status === Image.Ready

                IconImage {
                    id: cornerIcon
                    anchors.centerIn: parent
                    width: 12
                    height: 12
                    source: iconImg.source
                }
            }
        }

        Column {
            anchors.left: previewBox.right
            anchors.leftMargin: 10
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            spacing: 2

            Text {
                width: parent.width
                text: delegate.itemData ? delegate.itemData.title.toUpperCase() : "UNTITLED WINDOW"
                font: Theme.heavy(13)
                color: delegate.active ? Theme.slab : Theme.ink
                elide: Text.ElideRight
                maximumLineCount: 1
                renderType: Text.QtRendering
            }

            Text {
                width: parent.width
                text: delegate.itemData && delegate.itemData.icon ? delegate.itemData.icon.toUpperCase() : "APPLICATION"
                font: Theme.heavy(10)
                color: delegate.active ? Theme.slab : Theme.muted
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
        text: "[ENTER] FOCUS WINDOW"
        font: Theme.heavy(9)
        color: delegate.active ? Theme.slab : Theme.muted
        horizontalAlignment: Text.AlignHCenter
        elide: Text.ElideRight
        renderType: Text.QtRendering
    }
}
