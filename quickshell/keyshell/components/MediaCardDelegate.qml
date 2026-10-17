import QtQuick
import qs

Item {
    id: root

    readonly property var card: (parent && parent.card) ? parent.card : (parent ? parent.parent : null)
    readonly property var entry: (card && card.entry) ? card.entry : (modelData || null)
    property var modelData: null
    readonly property bool active: card ? card.active : false
    readonly property var player: entry ? entry.player : null

    function formatDuration(sec) {
        if (sec === undefined || sec === null || sec < 0 || !isFinite(sec))
            return "00:00";
        const total = Math.floor(sec);
        const m = Math.floor(total / 60);
        const s = total % 60;
        return (m < 10 ? "0" + m : m) + ":" + (s < 10 ? "0" + s : s);
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
            width: Math.min(badgeText.implicitWidth + 10, parent.width * 0.5)
            color: root.active ? Theme.slab : Theme.accentAlt
            radius: 2

            Text {
                id: badgeText
                anchors.centerIn: parent
                width: parent.width - 8
                horizontalAlignment: Text.AlignHCenter
                elide: Text.ElideRight
                text: root.entry && root.entry.badge ? root.entry.badge : "MEDIA"
                font: Theme.heavy(9)
                color: root.active ? Theme.accentAlt : Theme.slab
                renderType: Text.QtRendering
            }
        }

        Rectangle {
            id: statusBadge
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            height: 18
            width: Math.min(statusText.implicitWidth + 10, parent.width * 0.45)
            color: root.player && root.player.isPlaying
                   ? (root.active ? Theme.slab : Theme.accent)
                   : (root.active ? Theme.slab : Theme.surfaceAlt)
            radius: 2

            Text {
                id: statusText
                anchors.centerIn: parent
                width: parent.width - 8
                horizontalAlignment: Text.AlignHCenter
                elide: Text.ElideRight
                text: root.player ? (root.player.isPlaying ? "PLAYING" : "PAUSED") : "READY"
                font: Theme.heavy(9)
                color: root.player && root.player.isPlaying
                       ? (root.active ? Theme.accent : Theme.ink)
                       : (root.active ? Theme.ink : Theme.muted)
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
            id: artBox
            anchors.left: parent.left
            anchors.verticalCenter: parent.verticalCenter
            width: 44
            height: 44
            color: root.active ? Theme.surfaceAlt : Theme.slab
            border.width: 2
            border.color: root.active ? Theme.slab : Theme.accentAlt
            radius: 4
            clip: true

            Image {
                id: artImage
                anchors.fill: parent
                source: (root.player && root.player.trackArtUrl) ? root.player.trackArtUrl : ""
                fillMode: Image.PreserveAspectCrop
                visible: status === Image.Ready
                asynchronous: true
            }

            Text {
                anchors.centerIn: parent
                visible: artImage.status !== Image.Ready
                text: "🎵"
                font.pixelSize: 22
                color: root.active ? Theme.accent : Theme.muted
                renderType: Text.QtRendering
            }
        }

        Column {
            anchors.left: artBox.right
            anchors.leftMargin: 10
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            spacing: 2

            Text {
                width: parent.width
                text: root.player && root.player.trackTitle ? root.player.trackTitle.toUpperCase() : "NO TRACK PLAYING"
                font: Theme.heavy(12)
                color: root.active ? Theme.slab : Theme.ink
                elide: Text.ElideRight
                maximumLineCount: 1
                renderType: Text.QtRendering
            }

            Text {
                width: parent.width
                text: {
                    if (!root.player)
                        return "UNKNOWN ARTIST";
                    const artist = root.player.trackArtist || "";
                    const album = root.player.trackAlbum || "";
                    if (artist.length > 0 && album.length > 0)
                        return (artist + " — " + album).toUpperCase();
                    if (artist.length > 0)
                        return artist.toUpperCase();
                    if (album.length > 0)
                        return album.toUpperCase();
                    return "UNKNOWN ARTIST";
                }
                font: Theme.heavy(9)
                color: root.active ? Theme.slab : Theme.muted
                elide: Text.ElideRight
                maximumLineCount: 1
                renderType: Text.QtRendering
            }

            Item {
                width: parent.width
                height: 12

                Text {
                    id: timeText
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    text: {
                        const cur = root.player ? root.player.position : 0;
                        const len = root.player ? root.player.length : 0;
                        return root.formatDuration(cur) + "/" + root.formatDuration(len);
                    }
                    font: Theme.heavy(8)
                    color: root.active ? Theme.slab : Theme.accentAlt
                    renderType: Text.QtRendering
                }

                Rectangle {
                    anchors.left: parent.left
                    anchors.right: timeText.left
                    anchors.rightMargin: 6
                    anchors.verticalCenter: parent.verticalCenter
                    height: 5
                    radius: 1
                    color: root.active ? Theme.slab : Theme.surfaceAlt

                    Rectangle {
                        anchors.left: parent.left
                        anchors.top: parent.top
                        anchors.bottom: parent.bottom
                        radius: 1
                        width: {
                            const cur = root.player ? root.player.position : 0;
                            const len = root.player ? root.player.length : 0;
                            if (!isFinite(len) || len <= 0 || !isFinite(cur) || cur <= 0)
                                return 0;
                            const ratio = Math.min(1.0, cur / len);
                            return parent.width * ratio;
                        }
                        color: root.active ? Theme.accentAlt : Theme.accent
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
        text: "[SPACE] PLAY  //  [ENTER] CONSOLE"
        font: Theme.heavy(9)
        color: root.active ? Theme.slab : Theme.muted
        horizontalAlignment: Text.AlignHCenter
        elide: Text.ElideRight
        renderType: Text.QtRendering
    }
}
