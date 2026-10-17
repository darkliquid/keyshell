import QtQuick
import Quickshell
import qs

Item {
    id: delegate

    readonly property var card: parent ? parent.card : null
    readonly property var itemData: card ? card.entry : null
    readonly property bool active: card ? card.active : false
    readonly property real iconSize: card ? card.iconSize : 48

    anchors.fill: parent
    anchors.margins: 10

    Image {
        id: appIcon
        anchors.horizontalCenter: parent.horizontalCenter
        anchors.top: parent.top
        anchors.topMargin: 4
        width: delegate.iconSize
        height: delegate.iconSize
        source: delegate.itemData && delegate.itemData.icon ? Quickshell.iconPath(delegate.itemData.icon, true) : ""
    }

    Text {
        anchors.horizontalCenter: parent.horizontalCenter
        anchors.top: appIcon.bottom
        anchors.topMargin: 6
        anchors.bottom: parent.bottom
        width: parent.width - 8
        horizontalAlignment: Text.AlignHCenter
        verticalAlignment: Text.AlignVCenter
        wrapMode: Text.WrapAtWordBoundaryOrAnywhere
        maximumLineCount: 2
        elide: Text.ElideRight
        text: delegate.itemData ? delegate.itemData.title : ""
        font: Theme.heavy(Math.max(12, Math.min(16, Math.round(parent.height * 0.14))))
        color: delegate.active ? Theme.slab : Theme.ink
        renderType: Text.QtRendering
    }
}
