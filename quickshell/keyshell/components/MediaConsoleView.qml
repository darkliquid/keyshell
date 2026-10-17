import QtQuick
import qs

Item {
    id: root

    readonly property var player: Launcher.mediaConsolePlayer

    function formatDuration(sec) {
        if (sec === undefined || sec === null || sec < 0 || !isFinite(sec))
            return "00:00";
        const total = Math.floor(sec);
        const m = Math.floor(total / 60);
        const s = total % 60;
        return (m < 10 ? "0" + m : m) + ":" + (s < 10 ? "0" + s : s);
    }

    anchors.fill: parent

    Item {
        anchors.centerIn: parent
        width: Math.min(parent.width * 0.9, 1100)
        height: 136

        Row {
            anchors.fill: parent
            spacing: 20

            Rectangle {
                id: artBlock
                width: 120
                height: 120
                anchors.verticalCenter: parent.verticalCenter
                color: Theme.slab
                border.width: 2
                border.color: Theme.accentAlt
                clip: true

                Image {
                    id: consoleArt
                    anchors.fill: parent
                    source: (root.player && root.player.trackArtUrl) ? root.player.trackArtUrl : ""
                    fillMode: Image.PreserveAspectCrop
                    visible: status === Image.Ready
                    asynchronous: true
                }

                Text {
                    anchors.centerIn: parent
                    visible: consoleArt.status !== Image.Ready
                    text: "🎵"
                    font.pixelSize: 48
                    color: Theme.muted
                    renderType: Text.QtRendering
                }
            }

            Column {
                anchors.verticalCenter: parent.verticalCenter
                width: parent.width - artBlock.width - 20
                spacing: 6

                Row {
                    spacing: 8

                    Rectangle {
                        height: 18
                        width: consoleBadgeText.implicitWidth + 10
                        color: Theme.accent
                        radius: 2

                        Text {
                            id: consoleBadgeText
                            anchors.centerIn: parent
                            text: "MEDIA CONSOLE"
                            font: Theme.heavy(10)
                            color: Theme.slab
                            renderType: Text.QtRendering
                        }
                    }

                    Rectangle {
                        height: 18
                        width: identityBadgeText.implicitWidth + 10
                        color: Theme.accentAlt
                        radius: 2

                        Text {
                            id: identityBadgeText
                            anchors.centerIn: parent
                            text: root.player && root.player.identity ? root.player.identity.toUpperCase() : "PLAYER"
                            font: Theme.heavy(10)
                            color: Theme.slab
                            renderType: Text.QtRendering
                        }
                    }

                    Rectangle {
                        height: 18
                        width: playStateBadgeText.implicitWidth + 10
                        color: root.player && root.player.isPlaying ? Theme.accent : Theme.surfaceAlt
                        radius: 2

                        Text {
                            id: playStateBadgeText
                            anchors.centerIn: parent
                            text: root.player ? (root.player.isPlaying ? "PLAYING" : "PAUSED") : "READY"
                            font: Theme.heavy(10)
                            color: root.player && root.player.isPlaying ? Theme.slab : Theme.muted
                            renderType: Text.QtRendering
                        }
                    }
                }

                Text {
                    width: parent.width
                    text: root.player && root.player.trackTitle ? root.player.trackTitle.toUpperCase() : "NO TRACK PLAYING"
                    font: Theme.heavy(20)
                    color: Theme.ink
                    elide: Text.ElideRight
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
                    font: Theme.heavy(11)
                    color: Theme.accentAlt
                    elide: Text.ElideRight
                    renderType: Text.QtRendering
                }

                Row {
                    width: parent.width
                    spacing: 12

                    Text {
                        id: elapsedLabel
                        anchors.verticalCenter: parent.verticalCenter
                        text: root.formatDuration(root.player ? root.player.position : 0)
                        font: Theme.heavy(12)
                        color: Theme.accentAlt
                        renderType: Text.QtRendering
                    }

                    Rectangle {
                        id: trackBar
                        anchors.verticalCenter: parent.verticalCenter
                        width: Math.max(0, parent.width - elapsedLabel.implicitWidth - totalLabel.implicitWidth - 24)
                        height: 10
                        color: Theme.surfaceAlt
                        radius: 2

                        Rectangle {
                            anchors.left: parent.left
                            anchors.top: parent.top
                            anchors.bottom: parent.bottom
                            width: {
                                const cur = root.player ? root.player.position : 0;
                                const len = root.player ? root.player.length : 0;
                                if (!isFinite(len) || len <= 0 || !isFinite(cur) || cur <= 0)
                                    return 0;
                                const ratio = Math.min(1.0, cur / len);
                                return parent.width * ratio;
                            }
                            color: Theme.accent
                            radius: 2
                        }
                    }

                    Text {
                        id: totalLabel
                        anchors.verticalCenter: parent.verticalCenter
                        text: root.formatDuration(root.player ? root.player.length : 0)
                        font: Theme.heavy(12)
                        color: Theme.muted
                        renderType: Text.QtRendering
                    }
                }

                Text {
                    width: parent.width
                    text: "[SPACE] PLAY/PAUSE  //  [N] NEXT  //  [P] PREV  //  [←/→] SEEK 5s  //  [ESC] RETURN"
                    font: Theme.heavy(10)
                    color: Theme.muted
                    elide: Text.ElideRight
                    renderType: Text.QtRendering
                }
            }
        }
    }
}
