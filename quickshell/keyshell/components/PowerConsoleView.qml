import QtQuick
import Quickshell.Widgets
import Quickshell.Services.UPower
import qs

Item {
    id: root

    anchors.fill: parent

    function formatSecs(secs) {
        if (!secs || secs <= 0)
            return "";
        const m = Math.round(secs / 60);
        const hrs = Math.floor(m / 60);
        const mins = m % 60;
        if (hrs > 0)
            return hrs + "h " + mins + "m";
        return mins + "m";
    }

    Item {
        anchors.centerIn: parent
        width: Math.min(parent.width * 0.9, 1100)
        height: 136

        Column {
            anchors.fill: parent
            spacing: 10

            Item {
                width: parent.width
                height: 38

                Row {
                    anchors.left: parent.left
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: 12

                    Text {
                        anchors.verticalCenter: parent.verticalCenter
                        text: {
                            if (UPower.displayDevice && UPower.displayDevice.isPresent) {
                                const raw = (UPower.displayDevice && !isNaN(UPower.displayDevice.percentage)) ? UPower.displayDevice.percentage : 0;
                                const pct = Math.round(raw > 1.0 ? raw : raw * 100);
                                return pct + "%";
                            }
                            return "AC";
                        }
                        font: Theme.heavy(28)
                        color: Theme.accentAlt
                        renderType: Text.QtRendering
                    }

                    Column {
                        anchors.verticalCenter: parent.verticalCenter
                        spacing: 2

                        Text {
                            text: {
                                if (!UPower.displayDevice || !UPower.displayDevice.isPresent)
                                    return "AC LINE POWER CONNECTED // DESKTOP SYSTEM";
                                const stateStr = UPowerDeviceState.toString(UPower.displayDevice.state).toUpperCase();
                                let secs = 0;
                                if (UPower.displayDevice.state === UPowerDeviceState.Charging)
                                    secs = UPower.displayDevice.timeToFull;
                                else if (UPower.displayDevice.state === UPowerDeviceState.Discharging)
                                    secs = UPower.displayDevice.timeToEmpty;
                                const formatted = root.formatSecs(secs);
                                if (formatted !== "") {
                                    if (UPower.displayDevice.state === UPowerDeviceState.Charging)
                                        return stateStr + " (" + formatted + " TO FULL)";
                                    return stateStr + " (" + formatted + " REMAINING)";
                                }
                                return stateStr;
                            }
                            font: Theme.heavy(11)
                            color: Theme.ink
                            renderType: Text.QtRendering
                        }

                        Text {
                            text: {
                                if (!UPower.displayDevice || !UPower.displayDevice.isPresent)
                                    return "DIRECT HARDWARE POWER // UNLIMITED RUNTIME";
                                const rawHealth = UPower.displayDevice ? UPower.displayDevice.healthPercentage : 0;
                                const healthPct = (!rawHealth || isNaN(rawHealth)) ? 100 : Math.round(rawHealth > 1.0 ? rawHealth : rawHealth * 100);
                                const rawRate = UPower.displayDevice ? UPower.displayDevice.changeRate : 0;
                                const rate = (!rawRate || isNaN(rawRate)) ? "0.0" : Math.abs(rawRate).toFixed(1);
                                return "HEALTH " + healthPct + "% // " + rate + "W RATE";
                            }
                            font: Theme.heavy(10)
                            color: Theme.muted
                            renderType: Text.QtRendering
                        }
                    }

                    Row {
                        anchors.verticalCenter: parent.verticalCenter
                        spacing: 3
                        visible: Boolean(UPower.displayDevice && UPower.displayDevice.isPresent)

                        Repeater {
                            model: 10
                            delegate: Rectangle {
                                required property int index
                                width: 5
                                height: 10
                                radius: 1
                                transform: Matrix4x4 {
                                    matrix: Qt.matrix4x4(
                                        1, -0.25, 0, 0,
                                        0,     1, 0, 0,
                                        0,     0, 1, 0,
                                        0,     0, 0, 1
                                    )
                                }
                                color: {
                                    const raw = (UPower.displayDevice && !isNaN(UPower.displayDevice.percentage)) ? UPower.displayDevice.percentage : 0;
                                    const pct = Math.round(raw > 1.0 ? raw : raw * 100);
                                    return ((index + 1) * 10 <= pct) ? Theme.accentAlt : Theme.surfaceAlt;
                                }
                            }
                        }
                    }
                }

                Text {
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    text: "[ESC / ENTER] RETURN TO REEL"
                    font: Theme.heavy(10)
                    color: Theme.muted
                    renderType: Text.QtRendering
                }
            }

            Item {
                width: parent.width
                height: 48

                Row {
                    anchors.centerIn: parent
                    spacing: 12

                    Rectangle {
                        id: chip1
                        height: 32
                        width: chip1Row.implicitWidth + 24
                        radius: 2
                        readonly property bool active: Boolean(PowerProfiles && PowerProfiles.profile === PowerProfile.PowerSaver)
                        color: active ? Theme.accentAlt : Theme.slab
                        border.width: 1
                        border.color: active ? Theme.accentAlt : Theme.surfaceAlt

                        MouseArea {
                            anchors.fill: parent
                            cursorShape: Qt.PointingHandCursor
                            onClicked: {
                                if (PowerProfiles)
                                    PowerProfiles.profile = PowerProfile.PowerSaver;
                            }
                        }

                        Row {
                            id: chip1Row
                            anchors.centerIn: parent
                            spacing: 6

                            Text {
                                text: "[1]"
                                font: Theme.heavy(10)
                                color: chip1.active ? Theme.slab : Theme.accentAlt
                                renderType: Text.QtRendering
                            }

                            Text {
                                text: "POWER SAVER"
                                font: Theme.heavy(11)
                                color: chip1.active ? Theme.slab : Theme.ink
                                renderType: Text.QtRendering
                            }
                        }
                    }

                    Rectangle {
                        id: chip2
                        height: 32
                        width: chip2Row.implicitWidth + 24
                        radius: 2
                        readonly property bool active: Boolean(PowerProfiles && PowerProfiles.profile === PowerProfile.Balanced)
                        color: active ? Theme.accentAlt : Theme.slab
                        border.width: 1
                        border.color: active ? Theme.accentAlt : Theme.surfaceAlt

                        MouseArea {
                            anchors.fill: parent
                            cursorShape: Qt.PointingHandCursor
                            onClicked: {
                                if (PowerProfiles)
                                    PowerProfiles.profile = PowerProfile.Balanced;
                            }
                        }

                        Row {
                            id: chip2Row
                            anchors.centerIn: parent
                            spacing: 6

                            Text {
                                text: "[2]"
                                font: Theme.heavy(10)
                                color: chip2.active ? Theme.slab : Theme.accentAlt
                                renderType: Text.QtRendering
                            }

                            Text {
                                text: "BALANCED"
                                font: Theme.heavy(11)
                                color: chip2.active ? Theme.slab : Theme.ink
                                renderType: Text.QtRendering
                            }
                        }
                    }

                    Rectangle {
                        id: chip3
                        height: 32
                        width: chip3Row.implicitWidth + 24
                        radius: 2
                        readonly property bool supported: Boolean(PowerProfiles && PowerProfiles.hasPerformanceProfile)
                        readonly property bool active: Boolean(PowerProfiles && PowerProfiles.profile === PowerProfile.Performance)
                        color: active ? Theme.accent : Theme.slab
                        border.width: 1
                        border.color: active ? Theme.accent : Theme.surfaceAlt
                        opacity: supported ? 1.0 : 0.4

                        MouseArea {
                            anchors.fill: parent
                            cursorShape: chip3.supported ? Qt.PointingHandCursor : Qt.ArrowCursor
                            enabled: chip3.supported
                            onClicked: {
                                if (PowerProfiles && PowerProfiles.hasPerformanceProfile)
                                    PowerProfiles.profile = PowerProfile.Performance;
                            }
                        }

                        Row {
                            id: chip3Row
                            anchors.centerIn: parent
                            spacing: 6

                            Text {
                                text: "[3]"
                                font: Theme.heavy(10)
                                color: chip3.active ? Theme.slab : Theme.accent
                                renderType: Text.QtRendering
                            }

                            Text {
                                text: "PERFORMANCE"
                                font: Theme.heavy(11)
                                color: chip3.active ? Theme.slab : (chip3.supported ? Theme.ink : Theme.muted)
                                renderType: Text.QtRendering
                            }
                        }
                    }

                    Rectangle {
                        id: chip4
                        height: 32
                        width: chip4Row.implicitWidth + 24
                        radius: 2
                        readonly property bool active: PowerBackend.keepAwakeActive
                        color: active ? Theme.accentAlt : Theme.slab
                        border.width: 1
                        border.color: active ? Theme.accentAlt : Theme.surfaceAlt

                        MouseArea {
                            anchors.fill: parent
                            cursorShape: Qt.PointingHandCursor
                            onClicked: {
                                PowerBackend.toggleKeepAwake();
                            }
                        }

                        Row {
                            id: chip4Row
                            anchors.centerIn: parent
                            spacing: 6

                            Text {
                                text: "[4]"
                                font: Theme.heavy(10)
                                color: chip4.active ? Theme.slab : Theme.accentAlt
                                renderType: Text.QtRendering
                            }

                            Text {
                                text: "KEEP AWAKE: " + (PowerBackend.keepAwakeActive ? "ON" : "OFF")
                                font: Theme.heavy(11)
                                color: chip4.active ? Theme.slab : Theme.ink
                                renderType: Text.QtRendering
                            }
                        }
                    }
                }
            }

            Text {
                width: parent.width
                text: "[1-3] SELECT POWER PROFILE  //  [4] TOGGLE KEEP AWAKE  //  [ESC / ENTER] RETURN"
                font: Theme.heavy(10)
                color: Theme.muted
                horizontalAlignment: Text.AlignHCenter
                renderType: Text.QtRendering
            }
        }
    }
}
