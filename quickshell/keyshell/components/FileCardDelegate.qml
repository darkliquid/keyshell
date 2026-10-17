import QtQuick
import Quickshell
import Quickshell.Widgets
import qs

Item {
    id: delegate

    readonly property var card: parent ? parent.card : null
    readonly property var itemData: card ? card.entry : null
    readonly property bool active: card ? card.active : false
    readonly property bool isImageFile: Boolean(itemData && itemData.isImage && itemData.fileUrl)

    function fallbackGlyph() {
        const kind = itemData && itemData.fileKind ? itemData.fileKind : "FILE";
        if (kind === "FOLDER")
            return "📁";
        if (kind === "IMAGE")
            return "🖼";
        if (kind === "VIDEO")
            return "🎬";
        if (kind === "AUDIO")
            return "🎵";
        if (kind === "ARCHIVE")
            return "📦";
        if (kind === "CODE")
            return "⌨";
        if (kind === "PDF" || kind === "DOCUMENT")
            return "📄";
        return "📄";
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
            id: kindBadge
            anchors.left: parent.left
            anchors.verticalCenter: parent.verticalCenter
            height: 18
            width: Math.min(kindBadgeText.implicitWidth + 10, parent.width * 0.5)
            color: delegate.active ? Theme.slab : Theme.accentAlt
            radius: 2

            Text {
                id: kindBadgeText
                anchors.centerIn: parent
                width: parent.width - 8
                horizontalAlignment: Text.AlignHCenter
                elide: Text.ElideRight
                text: (delegate.itemData && delegate.itemData.fileKind) ? delegate.itemData.fileKind : "FILE"
                font: Theme.heavy(9)
                color: delegate.active ? Theme.accentAlt : Theme.slab
                renderType: Text.QtRendering
            }
        }

        Rectangle {
            id: extPill
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            height: 18
            width: Math.min(extPillText.implicitWidth + 10, parent.width * 0.45)
            color: delegate.active ? Theme.slab : Theme.accent
            radius: 2
            visible: Boolean(delegate.itemData && delegate.itemData.extBadge && delegate.itemData.extBadge !== delegate.itemData.fileKind)

            Text {
                id: extPillText
                anchors.centerIn: parent
                width: parent.width - 8
                horizontalAlignment: Text.AlignHCenter
                elide: Text.ElideRight
                text: delegate.itemData && delegate.itemData.extBadge ? delegate.itemData.extBadge : ""
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
            width: delegate.isImageFile ? 68 : 44
            height: 44
            color: delegate.active ? Theme.surfaceAlt : Theme.slab
            border.width: 1.5
            border.color: delegate.active ? Theme.slab : Theme.accentAlt
            radius: 3
            clip: true

            Image {
                id: thumbImg
                anchors.fill: parent
                anchors.margins: 2
                source: (Launcher.open && BackendManager.activeMode === "files" && delegate.isImageFile) ? delegate.itemData.fileUrl : ""
                sourceSize.width: 160
                sourceSize.height: 120
                fillMode: Image.PreserveAspectCrop
                asynchronous: true
                cache: false
                visible: delegate.isImageFile && status === Image.Ready
            }

            IconImage {
                id: mimeIcon
                anchors.centerIn: parent
                width: 26
                height: 26
                source: {
                    const raw = (delegate.itemData && delegate.itemData.icon) ? String(delegate.itemData.icon) : "text-x-generic";
                    if (raw.indexOf("://") !== -1 || raw.indexOf("/") === 0)
                        return raw;
                    return Quickshell.iconPath(raw, true);
                }
                visible: (!delegate.isImageFile || thumbImg.status !== Image.Ready) && status === Image.Ready
            }

            Text {
                anchors.centerIn: parent
                visible: (!delegate.isImageFile || thumbImg.status !== Image.Ready) && mimeIcon.status !== Image.Ready
                text: delegate.fallbackGlyph()
                font.pixelSize: 20
                color: delegate.active ? Theme.accent : Theme.accentAlt
                renderType: Text.QtRendering
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

    Text {
        id: footerText
        anchors.bottom: parent.bottom
        anchors.left: parent.left
        anchors.right: parent.right
        text: "[ENTER] OPEN WITH DEFAULT APP"
        font: Theme.heavy(9)
        color: delegate.active ? Theme.slab : Theme.muted
        horizontalAlignment: Text.AlignHCenter
        elide: Text.ElideRight
        renderType: Text.QtRendering
    }
}
