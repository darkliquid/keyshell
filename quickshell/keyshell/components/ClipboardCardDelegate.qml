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

        Text {
            id: glyphText
            anchors.left: parent.left
            anchors.top: parent.top
            anchors.topMargin: 2
            width: 32
            horizontalAlignment: Text.AlignHCenter
            text: delegate.itemData && delegate.itemData.glyph ? delegate.itemData.glyph : "📋"
            font: Theme.heavy(20)
            color: delegate.active ? Theme.slab : Theme.ink
            renderType: Text.QtRendering
        }

        Text {
            id: entryText
            anchors.left: glyphText.right
            anchors.right: parent.right
            anchors.top: parent.top
            anchors.bottom: ageText.top
            anchors.leftMargin: 8
            anchors.bottomMargin: 4
            wrapMode: Text.WrapAtWordBoundaryOrAnywhere
            maximumLineCount: 3
            elide: Text.ElideRight
            verticalAlignment: Text.AlignTop
            text: delegate.itemData ? delegate.itemData.title : ""
            font: Theme.heavy(11)
            color: delegate.active ? Theme.slab : Theme.ink
            renderType: Text.QtRendering
        }

        Text {
            id: ageText
            anchors.left: glyphText.right
            anchors.right: parent.right
            anchors.bottom: parent.bottom
            anchors.leftMargin: 8
            maximumLineCount: 1
            elide: Text.ElideRight
            text: delegate.itemData ? delegate.itemData.subtitle : ""
            font: Theme.heavy(9)
            color: delegate.active ? Theme.slab : Theme.muted
            renderType: Text.QtRendering
        }
    }

    Text {
        id: footerText
        anchors.bottom: parent.bottom
        anchors.left: parent.left
        anchors.right: parent.right
        text: "[ENTER] COPY  //  [X] DELETE"
        font: Theme.heavy(9)
        color: delegate.active ? Theme.slab : Theme.muted
        horizontalAlignment: Text.AlignHCenter
        elide: Text.ElideRight
        renderType: Text.QtRendering
    }
}