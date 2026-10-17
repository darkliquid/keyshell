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
        id: middleRow
        anchors.top: parent.top
        anchors.bottom: footerText.top
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.bottomMargin: 6

        Column {
            anchors.centerIn: parent
            width: parent.width
            spacing: 6

            Text {
                width: parent.width
                horizontalAlignment: Text.AlignHCenter
                text: delegate.itemData ? delegate.itemData.title : ""
                font: Theme.heavy(16)
                color: delegate.active ? Theme.slab : Theme.ink
                elide: Text.ElideRight
                maximumLineCount: 2
                wrapMode: Text.WrapAtWordBoundaryOrAnywhere
                renderType: Text.QtRendering
            }

            Text {
                width: parent.width
                horizontalAlignment: Text.AlignHCenter
                text: delegate.itemData ? delegate.itemData.subtitle : ""
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
        text: (delegate.itemData && delegate.itemData.hint) ? delegate.itemData.hint : "[ENTER] RUN"
        font: Theme.heavy(9)
        color: delegate.active ? Theme.slab : Theme.muted
        horizontalAlignment: Text.AlignHCenter
        elide: Text.ElideRight
        renderType: Text.QtRendering
    }
}