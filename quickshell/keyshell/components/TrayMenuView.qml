import QtQuick
import Quickshell
import Quickshell.Widgets
import qs

Item {
    id: root

    readonly property var item: Launcher.trayMenuItem
    readonly property var entries: Launcher.trayMenuEntries
    readonly property int selectedIdx: Launcher.trayMenuIndex

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
                spacing: 10

                Rectangle {
                    width: 24
                    height: 24
                    anchors.verticalCenter: parent.verticalCenter
                    color: Theme.slab
                    border.width: 1
                    border.color: Theme.accentAlt
                    radius: 2

                    IconImage {
                        id: trayIconImg
                        anchors.centerIn: parent
                        width: 18
                        height: 18
                        source: {
                            const raw = (root.item && root.item.icon) ? String(root.item.icon) : "";
                            if (raw.length === 0)
                                return "";
                            if (raw.indexOf("://") !== -1 || raw.indexOf("/") === 0)
                                return raw;
                            return Quickshell.iconPath(raw, true);
                        }
                        visible: status === Image.Ready
                    }

                    Text {
                        anchors.centerIn: parent
                        visible: trayIconImg.status !== Image.Ready
                        text: "⚙"
                        font.pixelSize: 12
                        color: Theme.accentAlt
                        renderType: Text.QtRendering
                    }
                }

                Text {
                    anchors.verticalCenter: parent.verticalCenter
                    width: Math.min(implicitWidth, parent.width - 320)
                    text: {
                        const base = root.item && root.item.title ? root.item.title.toUpperCase() : "SYSTEM ITEM";
                        if (TrayBackend.subMenuHandle && TrayBackend.subMenuHandle.text)
                            return base + " ▸ " + TrayBackend.cleanMenuText(TrayBackend.subMenuHandle.text).toUpperCase();
                        return base;
                    }
                    font: Theme.heavy(16)
                    color: Theme.ink
                    elide: Text.ElideRight
                    renderType: Text.QtRendering
                }

                Rectangle {
                    anchors.verticalCenter: parent.verticalCenter
                    height: 20
                    width: menuBadgeText.implicitWidth + 10
                    color: TrayBackend.subMenuHandle ? Theme.accent : Theme.accentAlt
                    radius: 2

                    Text {
                        id: menuBadgeText
                        anchors.centerIn: parent
                        text: TrayBackend.subMenuHandle ? "SUBMENU" : "DBUS MENU"
                        font: Theme.heavy(10)
                        color: Theme.slab
                        renderType: Text.QtRendering
                    }
                }

                Item {
                    width: 1
                    height: 1
                }

                Text {
                    anchors.verticalCenter: parent.verticalCenter
                    text: TrayBackend.subMenuHandle ? "[ESC / BACKSPACE] BACK" : "[ESC] RETURN TO REEL"
                    font: Theme.heavy(10)
                    color: Theme.muted
                    renderType: Text.QtRendering
                }
            }

            Item {
                width: parent.width
                height: 64

                Text {
                    anchors.centerIn: parent
                    visible: !root.entries || root.entries.length === 0
                    text: (root.item && root.item.hasMenu) ? "LOADING MENU..." : "NO MENU ACTIONS AVAILABLE"
                    font: Theme.heavy(14)
                    color: Theme.muted
                    renderType: Text.QtRendering
                }

                Flow {
                    anchors.fill: parent
                    spacing: 8
                    visible: root.entries && root.entries.length > 0

                    Repeater {
                        model: root.entries ? root.entries : []

                        delegate: Rectangle {
                            id: chip
                            readonly property bool isSelected: index === root.selectedIdx
                            width: Math.min(260, chipText.implicitWidth + numText.implicitWidth + 24)
                            height: 28
                            color: isSelected ? Theme.accent : Theme.slab
                            border.width: 1
                            border.color: isSelected ? Theme.accent : Theme.surfaceAlt
                            radius: 2

                            MouseArea {
                                anchors.fill: parent
                                cursorShape: Qt.PointingHandCursor
                                onClicked: {
                                    Launcher.trayMenuIndex = index;
                                    const entryObj = modelData ? modelData.entry : null;
                                    const openedSubmenu = TrayBackend.triggerMenuEntry(entryObj);
                                    if (!openedSubmenu)
                                        Qt.callLater(function() { Launcher.dismiss(); });
                                }
                            }

                            Row {
                                anchors.centerIn: parent
                                spacing: 6

                                Text {
                                    id: numText
                                    text: "[" + (index + 1) + "]"
                                    font: Theme.heavy(10)
                                    color: chip.isSelected ? Theme.slab : Theme.accentAlt
                                    renderType: Text.QtRendering
                                }

                                Text {
                                    id: chipText
                                    width: Math.min(implicitWidth, chip.width - numText.implicitWidth - 24)
                                    text: modelData.text ? modelData.text.toUpperCase() : "ACTION"
                                    font: Theme.heavy(11)
                                    color: chip.isSelected ? Theme.slab : Theme.ink
                                    elide: Text.ElideRight
                                    renderType: Text.QtRendering
                                }
                            }
                        }
                    }
                }
            }

            Text {
                width: parent.width
                text: "[1-8] QUICK TRIGGER  //  [←/→] NAVIGATE  //  [ENTER] EXECUTE  //  [ESC] RETURN"
                font: Theme.heavy(10)
                color: Theme.muted
                elide: Text.ElideRight
                renderType: Text.QtRendering
            }
        }
    }
}
