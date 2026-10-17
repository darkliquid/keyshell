import QtQuick
import qs

Item {
    id: root
    anchors.fill: parent

    Item {
        anchors.centerIn: parent
        width: Math.min(parent.width * 0.9, 1100)
        height: 136

        Column {
            anchors.fill: parent
            spacing: 12

            Row {
                width: parent.width
                height: 24

                Text {
                    id: headerText
                    text: "CONNECT TO ENCRYPTED WI-FI // " + ((Launcher.networkPasswordTarget && Launcher.networkPasswordTarget.title) ? Launcher.networkPasswordTarget.title.toUpperCase() : "WI-FI NETWORK") + ((Launcher.networkPasswordTarget && Launcher.networkPasswordTarget.security) ? (" [" + Launcher.networkPasswordTarget.security + "]") : "")
                    font: Theme.heavy(11)
                    color: Theme.accentAlt
                    renderType: Text.QtRendering
                }
            }

            Row {
                width: parent.width
                height: 48
                spacing: 12

                Text {
                    anchors.verticalCenter: parent.verticalCenter
                    text: "PASSWORD:"
                    font: Theme.heavy(16)
                    color: Theme.ink
                    renderType: Text.QtRendering
                }

                Rectangle {
                    anchors.verticalCenter: parent.verticalCenter
                    height: 42
                    width: Math.min(parent.width - 150, 600)
                    color: Theme.slab
                    border.width: 2
                    border.color: Theme.accent
                    radius: 2
                    antialiasing: true

                    transform: Matrix4x4 {
                        matrix: Qt.matrix4x4(
                            1, -0.25, 0, 0,
                            0,     1, 0, 0,
                            0,     0, 1, 0,
                            0,     0, 0, 1
                        )
                    }

                    Row {
                        anchors.left: parent.left
                        anchors.leftMargin: 16
                        anchors.verticalCenter: parent.verticalCenter
                        spacing: 4

                        Text {
                            text: "\u2022".repeat(Launcher.networkPasswordBuffer ? Launcher.networkPasswordBuffer.length : 0)
                            font: Theme.heavy(20)
                            color: Theme.accent
                            renderType: Text.QtRendering
                        }

                        Rectangle {
                            width: 3
                            height: 22
                            color: Theme.accentAlt
                            visible: Launcher.networkPasswordTarget !== null

                            SequentialAnimation on opacity {
                                loops: Animation.Infinite
                                running: Launcher.open && Launcher.networkPasswordTarget !== null
                                NumberAnimation { to: 0.0; duration: 400; easing.type: Easing.InOutQuad }
                                NumberAnimation { to: 1.0; duration: 400; easing.type: Easing.InOutQuad }
                            }
                        }
                    }
                }
            }

            Item {
                width: 1
                height: 1
            }

            Text {
                width: parent.width
                text: "[ENTER] SUBMIT & CONNECT  //  [ESC] CANCEL & RETURN TO REEL"
                font: Theme.heavy(10)
                color: Theme.muted
                renderType: Text.QtRendering
            }
        }
    }
}
