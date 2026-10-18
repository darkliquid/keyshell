import QtQuick
import Quickshell
import Quickshell.Wayland
import Quickshell.Bluetooth
import qs

PanelWindow {
    id: win

    property var modelData
    property real lastEnterPressTime: 0
    property bool holdingDestructive: false

    Timer {
        id: holdStepTimer
        interval: 20
        repeat: true
        running: false
        property real startTime: 0
        onTriggered: {
            const elapsed = Date.now() - startTime;
            const progress = Math.min(1.0, elapsed / Config.powerHoldDurationMs);
            Launcher.setPowerHoldProgress(progress);
            if (progress >= 1.0) {
                holdStepTimer.stop();
                win.holdingDestructive = false;
                Launcher.setPowerHoldProgress(0.0);
                Launcher.activate();
            }
        }
    }

    screen: modelData
    implicitWidth: screen ? screen.width : 0
    implicitHeight: screen ? screen.height : 0
    anchors { top: true; bottom: true; left: true; right: true }
    color: "transparent"
    visible: Launcher.open

    WlrLayershell.layer: WlrLayer.Overlay
    WlrLayershell.exclusionMode: ExclusionMode.Ignore
    WlrLayershell.namespace: "keyshell"
    WlrLayershell.keyboardFocus: Launcher.open ? WlrKeyboardFocus.OnDemand : WlrKeyboardFocus.None

    SequentialAnimation {
        id: revealAnim

        ScriptAction {
            script: {
                band.expandProgress = 0;
                band.contentOpacity = 0;
                const targetW = (win.width > 0 && win.width !== 500) ? win.width : (win.screen ? win.screen.width : win.width);
                const targetH = (win.height > 0 && win.height !== 500) ? win.height : (win.screen ? win.screen.height : win.height);
                Reveal.resample(targetW, targetH);
                band.resetView();
                WindowBackend.refresh();
                scope.forceActiveFocus();
            }
        }

        PauseAnimation {
            duration: 35
        }

        ParallelAnimation {
            NumberAnimation {
                target: band
                property: "expandProgress"
                from: 0.0
                to: 1.0
                duration: Config.expandDurationMs
                easing.type: Easing.OutCubic
            }
            NumberAnimation {
                target: band
                property: "contentOpacity"
                from: 0.0
                to: 1.0
                duration: Config.expandDurationMs
                easing.type: Easing.OutCubic
            }
        }
    }

    Connections {
        target: Launcher
        function onRevealed() {
            win.lastEnterPressTime = 0;
            holdStepTimer.stop();
            win.holdingDestructive = false;
            Launcher.setPowerHoldProgress(0.0);
            revealAnim.restart();
        }
        function onDismissed() {
            win.lastEnterPressTime = 0;
            holdStepTimer.stop();
            win.holdingDestructive = false;
            Launcher.setPowerHoldProgress(0.0);
            revealAnim.stop();
            band.expandProgress = 0.0;
            band.contentOpacity = 0.0;
        }
        function onSelectedIndexChanged() {
            win.lastEnterPressTime = 0;
            if (win.holdingDestructive) {
                holdStepTimer.stop();
                win.holdingDestructive = false;
                Launcher.setPowerHoldProgress(0.0);
            }
        }
        function onPowerConsoleOpenChanged() {
            win.lastEnterPressTime = 0;
            if (win.holdingDestructive) {
                holdStepTimer.stop();
                win.holdingDestructive = false;
                Launcher.setPowerHoldProgress(0.0);
            }
        }
    }

    Connections {
        target: BackendManager
        function onActiveModeChanged() {
            win.lastEnterPressTime = 0;
            if (win.holdingDestructive) {
                holdStepTimer.stop();
                win.holdingDestructive = false;
                Launcher.setPowerHoldProgress(0.0);
            }
        }
    }

    FocusScope {
        id: scope
        anchors.fill: parent
        focus: Launcher.open

        Keys.onReleased: (event) => {
            if (event.isAutoRepeat)
                return;
            if (event.key === Qt.Key_Return || event.key === Qt.Key_Enter) {
                if (win.holdingDestructive) {
                    holdStepTimer.stop();
                    win.holdingDestructive = false;
                    Launcher.setPowerHoldProgress(0.0);
                    event.accepted = true;
                    return;
                }
            }
        }

        Keys.onPressed: (event) => {
            if (!Launcher.open)
                return;

            if (Launcher.networkPasswordTarget !== null) {
                if (event.key === Qt.Key_Escape) {
                    Launcher.closeNetworkPassword();
                    event.accepted = true;
                    return;
                }
                if (event.key === Qt.Key_Return || event.key === Qt.Key_Enter) {
                    if (Launcher.networkPasswordTarget && Launcher.networkPasswordTarget.network)
                        NetworkBackend.connectWithPsk(Launcher.networkPasswordTarget.network, Launcher.networkPasswordBuffer);
                    event.accepted = true;
                    return;
                }
                if (event.key === Qt.Key_Backspace) {
                    Launcher.deleteNetworkPasswordChar();
                    event.accepted = true;
                    return;
                }
                const blocked = Qt.ControlModifier | Qt.AltModifier | Qt.MetaModifier;
                if ((event.modifiers & blocked) === 0 && event.text.length === 1 && event.text.charCodeAt(0) >= 0x20 && event.key !== Qt.Key_Tab && event.key !== Qt.Key_Backtab) {
                    Launcher.appendNetworkPasswordChar(event.text);
                    event.accepted = true;
                    return;
                }
                event.accepted = true;
                return;
            }

            if (Launcher.networkTargetItem !== null) {
                if (event.key === Qt.Key_1) {
                    if (Launcher.networkTargetItem.connected)
                        NetworkBackend.disconnectNetwork(Launcher.networkTargetItem);
                    else
                        NetworkBackend.connectNetwork(Launcher.networkTargetItem);
                    event.accepted = true;
                    return;
                }
                if (event.key === Qt.Key_2) {
                    if (Launcher.networkTargetItem && Launcher.networkTargetItem.isWifi)
                        Launcher.openNetworkPassword(Launcher.networkTargetItem);
                    event.accepted = true;
                    return;
                }
                if (event.key === Qt.Key_3) {
                    if (Launcher.networkTargetItem && Launcher.networkTargetItem.isWifi) {
                        NetworkBackend.forgetNetwork(Launcher.networkTargetItem);
                        Launcher.closeNetworkConsole();
                    }
                    event.accepted = true;
                    return;
                }
                if (event.key === Qt.Key_4) {
                    NetworkBackend.toggleWifi();
                    event.accepted = true;
                    return;
                }
                if (event.key === Qt.Key_S) {
                    NetworkBackend.toggleScan();
                    event.accepted = true;
                    return;
                }
                if (event.key === Qt.Key_Escape || event.key === Qt.Key_Return || event.key === Qt.Key_Enter) {
                    Launcher.closeNetworkConsole();
                    event.accepted = true;
                    return;
                }
                event.accepted = true;
                return;
            }

            if (Launcher.bluetoothTargetDevice !== null) {
                if (event.key === Qt.Key_1) {
                    if (Launcher.bluetoothTargetDevice.connected)
                        Launcher.bluetoothTargetDevice.disconnect();
                    else
                        Launcher.bluetoothTargetDevice.connect();
                    event.accepted = true;
                    return;
                }
                if (event.key === Qt.Key_2) {
                    if (Launcher.bluetoothTargetDevice.pairing)
                        Launcher.bluetoothTargetDevice.cancelPair();
                    else if (!Launcher.bluetoothTargetDevice.paired)
                        Launcher.bluetoothTargetDevice.pair();
                    event.accepted = true;
                    return;
                }
                if (event.key === Qt.Key_3) {
                    Launcher.bluetoothTargetDevice.setTrusted(!Launcher.bluetoothTargetDevice.trusted);
                    event.accepted = true;
                    return;
                }
                if (event.key === Qt.Key_4) {
                    Launcher.bluetoothTargetDevice.forget();
                    Launcher.closeBluetoothConsole();
                    event.accepted = true;
                    return;
                }
                if (event.key === Qt.Key_S) {
                    BluetoothBackend.toggleDiscovery();
                    event.accepted = true;
                    return;
                }
                if (event.key === Qt.Key_Escape || event.key === Qt.Key_Return || event.key === Qt.Key_Enter) {
                    Launcher.closeBluetoothConsole();
                    event.accepted = true;
                    return;
                }
                event.accepted = true;
                return;
            }

            if (Launcher.powerConsoleOpen) {
                if (event.key === Qt.Key_1) {
                    PowerBackend.setPowerProfile("power-saver");
                    event.accepted = true;
                    return;
                }
                if (event.key === Qt.Key_2) {
                    PowerBackend.setPowerProfile("balanced");
                    event.accepted = true;
                    return;
                }
                if (event.key === Qt.Key_3) {
                    PowerBackend.setPowerProfile("performance");
                    event.accepted = true;
                    return;
                }
                if (event.key === Qt.Key_4) {
                    PowerBackend.toggleKeepAwake();
                    event.accepted = true;
                    return;
                }
                if (event.key === Qt.Key_Escape || event.key === Qt.Key_Return || event.key === Qt.Key_Enter) {
                    Launcher.closePowerConsole();
                    event.accepted = true;
                    return;
                }
                event.accepted = true;
                return;
            }

            if (Launcher.trayMenuItem !== null) {
                if (event.key >= Qt.Key_1 && event.key <= Qt.Key_9) {
                    const numIdx = event.key - Qt.Key_1;
                    if (numIdx >= 0 && numIdx < Launcher.trayMenuEntries.length) {
                        const openedSubmenu = TrayBackend.triggerMenuEntry(Launcher.trayMenuEntries[numIdx].entry);
                        if (!openedSubmenu)
                            Launcher.dismiss();
                        event.accepted = true;
                        return;
                    }
                }
                if (event.key === Qt.Key_Left || event.key === Qt.Key_Up) {
                    Launcher.moveTrayMenuSelection(-1);
                    event.accepted = true;
                    return;
                }
                if (event.key === Qt.Key_Right || event.key === Qt.Key_Down) {
                    Launcher.moveTrayMenuSelection(1);
                    event.accepted = true;
                    return;
                }
                if (event.key === Qt.Key_Return || event.key === Qt.Key_Enter || event.key === Qt.Key_Space) {
                    if (Launcher.trayMenuEntries.length > 0 && Launcher.trayMenuIndex < Launcher.trayMenuEntries.length) {
                        const openedSubmenu = TrayBackend.triggerMenuEntry(Launcher.trayMenuEntries[Launcher.trayMenuIndex].entry);
                        if (!openedSubmenu)
                            Launcher.dismiss();
                    }
                    event.accepted = true;
                    return;
                }
                if (event.key === Qt.Key_Backspace) {
                    if (!TrayBackend.popSubMenu())
                        Launcher.closeTrayMenu();
                    event.accepted = true;
                    return;
                }
                if (event.key === Qt.Key_Escape) {
                    if (!TrayBackend.popSubMenu())
                        Launcher.closeTrayMenu();
                    event.accepted = true;
                    return;
                }
                event.accepted = true;
                return;
            }

            if (Launcher.mediaConsolePlayer !== null) {
                const step = (event.modifiers & Qt.ShiftModifier) ? Config.mediaSeekFineStepSec : Config.mediaSeekStepSec;
                if (event.key === Qt.Key_Left || event.key === Qt.Key_Down) {
                    MediaBackend.seek(Launcher.mediaConsolePlayer, -step);
                    event.accepted = true;
                    return;
                }
                if (event.key === Qt.Key_Right || event.key === Qt.Key_Up) {
                    MediaBackend.seek(Launcher.mediaConsolePlayer, step);
                    event.accepted = true;
                    return;
                }
                if (event.key === Qt.Key_Space) {
                    MediaBackend.togglePlay(Launcher.mediaConsolePlayer);
                    event.accepted = true;
                    return;
                }
                if (event.key === Qt.Key_N) {
                    MediaBackend.next(Launcher.mediaConsolePlayer);
                    event.accepted = true;
                    return;
                }
                if (event.key === Qt.Key_P) {
                    MediaBackend.previous(Launcher.mediaConsolePlayer);
                    event.accepted = true;
                    return;
                }
                if (event.key === Qt.Key_R) {
                    if (Launcher.mediaConsolePlayer.canRaise)
                        Launcher.mediaConsolePlayer.raise();
                    Launcher.dismiss();
                    event.accepted = true;
                    return;
                }
                if (event.key === Qt.Key_Escape || event.key === Qt.Key_Return || event.key === Qt.Key_Enter) {
                    Launcher.closeMediaConsole();
                    event.accepted = true;
                    return;
                }
                event.accepted = true;
                return;
            }

            if (Launcher.sliderTargetNode !== null) {
                const step = (event.modifiers & Qt.ShiftModifier) ? Config.audioVolumeFineStep : Config.audioVolumeStep;
                switch (event.key) {
                case Qt.Key_Left:
                case Qt.Key_Down:
                    AudioBackend.adjustVolume(Launcher.sliderTargetNode, -step);
                    event.accepted = true;
                    return;
                case Qt.Key_Right:
                case Qt.Key_Up:
                    AudioBackend.adjustVolume(Launcher.sliderTargetNode, step);
                    event.accepted = true;
                    return;
                case Qt.Key_M:
                case Qt.Key_Space:
                    AudioBackend.toggleMute(Launcher.sliderTargetNode);
                    event.accepted = true;
                    return;
                case Qt.Key_D:
                    AudioBackend.setDefault(Launcher.sliderTargetNode);
                    event.accepted = true;
                    return;
                case Qt.Key_Escape:
                case Qt.Key_Return:
                case Qt.Key_Enter:
                    Launcher.closeSlider();
                    event.accepted = true;
                    return;
                default:
                    event.accepted = true;
                    return;
                }
            }

            if (BackendManager.activeMode === "audio" && Launcher.results.length > 0) {
                const currentEntry = Launcher.results[Launcher.selectedIndex];
                const currentNode = currentEntry ? currentEntry.node : null;

                if (event.key === Qt.Key_Space && currentNode) {
                    Launcher.openSlider(currentNode);
                    event.accepted = true;
                    return;
                }
                if (event.key === Qt.Key_D && currentNode) {
                    AudioBackend.setDefault(currentNode);
                    event.accepted = true;
                    return;
                }
                if (event.key === Qt.Key_M && currentNode) {
                    AudioBackend.toggleMute(currentNode);
                    event.accepted = true;
                    return;
                }
            }

            if (BackendManager.activeMode === "media" && Launcher.results.length > 0) {
                const curMedia = Launcher.results[Launcher.selectedIndex];
                if (curMedia && curMedia.player) {
                    if (event.key === Qt.Key_Space) {
                        MediaBackend.togglePlay(curMedia.player);
                        event.accepted = true;
                        return;
                    }
                    if (event.key === Qt.Key_N) {
                        MediaBackend.next(curMedia.player);
                        event.accepted = true;
                        return;
                    }
                    if (event.key === Qt.Key_P) {
                        MediaBackend.previous(curMedia.player);
                        event.accepted = true;
                        return;
                    }
                    if (event.key === Qt.Key_R) {
                        if (curMedia.player.canRaise)
                            curMedia.player.raise();
                        Launcher.dismiss();
                        event.accepted = true;
                        return;
                    }
                }
            }

            if (BackendManager.activeMode === "tray" && Launcher.results.length > 0) {
                const curTray = Launcher.results[Launcher.selectedIndex];
                if (curTray && curTray.item) {
                    if (event.key === Qt.Key_Space) {
                        if (event.modifiers & Qt.ShiftModifier)
                            TrayBackend.secondaryActivate(curTray.item);
                        else
                            TrayBackend.activate(curTray.item);
                        event.accepted = true;
                        return;
                    }
                }
            }

            if (BackendManager.activeMode === "bluetooth") {
                if (event.key === Qt.Key_E) {
                    BluetoothBackend.togglePower();
                    event.accepted = true;
                    return;
                }
                if (event.key === Qt.Key_S) {
                    BluetoothBackend.toggleDiscovery();
                    event.accepted = true;
                    return;
                }
                if (event.key === Qt.Key_Space && Launcher.results.length > 0) {
                    const curBt = Launcher.results[Launcher.selectedIndex];
                    if (curBt && curBt.device) {
                        Launcher.openBluetoothConsole(curBt.device);
                        event.accepted = true;
                        return;
                    }
                }
            }

            if (BackendManager.activeMode === "power") {
                if (event.key === Qt.Key_Space) {
                    Launcher.openPowerConsole();
                    event.accepted = true;
                    return;
                }
            }

            if (BackendManager.activeMode === "network") {
                if (event.key === Qt.Key_W) {
                    NetworkBackend.toggleWifi();
                    event.accepted = true;
                    return;
                }
                if (event.key === Qt.Key_S) {
                    NetworkBackend.toggleScan();
                    event.accepted = true;
                    return;
                }
                if (event.key === Qt.Key_Space && Launcher.results.length > 0) {
                    const curNet = (Launcher.selectedIndex >= 0 && Launcher.selectedIndex < Launcher.results.length) ? Launcher.results[Launcher.selectedIndex] : null;
                    if (curNet) {
                        Launcher.openNetworkConsole(curNet);
                        event.accepted = true;
                        return;
                    }
                }
            }

            if (BackendManager.activeMode === "clipboard") {
                if (event.key === Qt.Key_P) {
                    ClipboardBackend.togglePause();
                    event.accepted = true;
                    return;
                }
                if (event.key === Qt.Key_C) {
                    ClipboardBackend.clearHistory();
                    event.accepted = true;
                    return;
                }
                if (event.key === Qt.Key_X) {
                    const curClip = (Launcher.selectedIndex >= 0 && Launcher.selectedIndex < Launcher.results.length) ? Launcher.results[Launcher.selectedIndex] : null;
                    if (curClip)
                        ClipboardBackend.deleteEntry(curClip.id);
                    event.accepted = true;
                    return;
                }
            }

            const blocked = Qt.ControlModifier | Qt.AltModifier | Qt.MetaModifier;
            if ((event.modifiers & blocked) === 0
                    && event.text.length === 1
                    && event.text.charCodeAt(0) >= 0x20) {
                if (BackendManager.tryTrigger(Launcher.query + event.text)) {
                    event.accepted = true;
                    return;
                }
                Launcher.setQuery(Launcher.query + event.text);
                event.accepted = true;
                return;
            }
            switch (event.key) {
            case Qt.Key_Escape:
                Launcher.dismiss();
                event.accepted = true;
                break;
            case Qt.Key_Return:
            case Qt.Key_Enter:
                if (BackendManager.activeMode === "network" && Launcher.results.length > 0) {
                    const curNet = (Launcher.selectedIndex >= 0 && Launcher.selectedIndex < Launcher.results.length) ? Launcher.results[Launcher.selectedIndex] : null;
                    if (curNet) {
                        if (curNet.connected)
                            NetworkBackend.disconnectNetwork(curNet);
                        else
                            NetworkBackend.connectNetwork(curNet);
                        event.accepted = true;
                        return;
                    }
                }
                if (BackendManager.activeMode === "bluetooth") {
                    const adapter = Bluetooth.defaultAdapter;
                    if (adapter && !adapter.enabled) {
                        BluetoothBackend.togglePower();
                        event.accepted = true;
                        return;
                    }
                }
                if (Launcher.results.length > 0) {
                    const curItem = (Launcher.selectedIndex >= 0 && Launcher.selectedIndex < Launcher.results.length) ? Launcher.results[Launcher.selectedIndex] : null;
                    if (curItem && curItem.isDestructive) {
                        if (!event.isAutoRepeat) {
                            const now = Date.now();
                            if (now - win.lastEnterPressTime < Config.powerDoubleTapTimeoutMs) {
                                win.lastEnterPressTime = 0;
                                holdStepTimer.stop();
                                win.holdingDestructive = false;
                                Launcher.setPowerHoldProgress(0.0);
                                Launcher.activate();
                                event.accepted = true;
                                return;
                            }
                            win.lastEnterPressTime = now;
                            win.holdingDestructive = true;
                            holdStepTimer.startTime = now;
                            holdStepTimer.start();
                        }
                        event.accepted = true;
                        return;
                    }
                }
                Launcher.activate();
                event.accepted = true;
                break;
            case Qt.Key_Backspace:
                Launcher.setQuery(Launcher.query.slice(0, -1));
                event.accepted = true;
                break;
            case Qt.Key_Tab:
                BackendManager.cycleMode(1);
                event.accepted = true;
                break;
            case Qt.Key_Backtab:
                BackendManager.cycleMode(-1);
                event.accepted = true;
                break;
            case Qt.Key_Left:
                Launcher.moveSelection(-1);
                event.accepted = true;
                break;
            case Qt.Key_Right:
                Launcher.moveSelection(1);
                event.accepted = true;
                break;
            default:
                break;
            }
        }

        Band {
            id: band
        }
    }
}
