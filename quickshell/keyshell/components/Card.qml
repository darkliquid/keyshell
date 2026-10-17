import QtQuick
import Quickshell
import qs

Item {
    id: card

    readonly property var view: ListView.view
    readonly property var entry: modelData
    readonly property bool active: ListView.isCurrentItem
    readonly property real iconSize: Math.max(36, Math.min(52, Math.round(card.height * 0.40)))

    readonly property real shearOffset: -Reveal.shearFactor * card.height * 0.5
    readonly property real shearInset: Math.ceil(Math.abs(Reveal.shearFactor) * card.height * 0.5) + 4

    width: view ? view.cardWidth : Config.cardWidthPx
    height: view ? Math.max(90, view.height - Theme.shadowOffset - 4) : 130

    Rectangle {
        anchors.fill: parent
        x: Theme.shadowOffset
        y: Theme.shadowOffset
        color: Theme.accentAlt
        opacity: card.active ? 0 : 0.7
        antialiasing: true
        layer.enabled: true
        layer.smooth: true
        layer.samples: 4
        transform: Matrix4x4 {
            matrix: Qt.matrix4x4(1, Reveal.shearFactor, 0, card.shearOffset,
                                 0, 1, 0, 0,
                                 0, 0, 1, 0,
                                 0, 0, 0, 1)
        }
    }

    Rectangle {
        anchors.fill: parent
        color: card.active ? Theme.accent : Theme.slab
        border.color: card.active ? Theme.accent : Theme.muted
        border.width: 3
        antialiasing: true
        layer.enabled: true
        layer.smooth: true
        layer.samples: 4
        transform: Matrix4x4 {
            matrix: Qt.matrix4x4(1, Reveal.shearFactor, 0, card.shearOffset,
                                 0, 1, 0, 0,
                                 0, 0, 1, 0,
                                 0, 0, 0, 1)
        }
    }

    Loader {
        id: delegateLoader
        anchors.fill: parent
        anchors.leftMargin: card.shearInset
        anchors.rightMargin: card.shearInset
        clip: true
        sourceComponent: (card.entry && card.entry.delegate) ? card.entry.delegate : defaultDelegate
        property var card: card
    }

    Component {
        id: defaultDelegate
        AppCardDelegate {}
    }
}
