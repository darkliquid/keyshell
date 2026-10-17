import QtQuick
import Quickshell
import qs

ListView {
    id: reel

    property real cardWidth: Config.cardWidthPx

    readonly property real pitch: cardWidth + spacing

    orientation: ListView.Horizontal
    clip: true
    spacing: 24
    cacheBuffer: 2000
    model: Launcher.displayResults
    delegate: Card {}
    currentIndex: Launcher.displayIndex
    highlightMoveDuration: Config.scrollDurationMs
    snapMode: ListView.SnapOneItem
    highlightRangeMode: ListView.StrictlyEnforceRange
    preferredHighlightBegin: width / 2 - pitch / 2
    preferredHighlightEnd: width / 2 + pitch / 2

    Connections {
        target: Launcher
        function onModelReset() {
            reel.positionViewAtIndex(Launcher.displayIndex, ListView.Center);
        }
        function onRevealed() {
            reel.positionViewAtIndex(Launcher.displayIndex, ListView.Center);
        }
    }
}
