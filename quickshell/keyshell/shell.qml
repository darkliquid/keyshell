import QtQuick
import Quickshell
import Quickshell.Io
import Quickshell.Services.Pipewire
import Quickshell.Services.Mpris
import Quickshell.Services.SystemTray
import Quickshell.DBusMenu
import Quickshell.Bluetooth
import Quickshell.Services.UPower
import Quickshell.Networking
import qs
import qs.components

ShellRoot {
    id: root

    Variants {
        model: Quickshell.screens

        LauncherWindow {}
    }

    Timer {
        id: hudTimer
        interval: Config.volumeHudTimeoutMs
        repeat: false
        onTriggered: {
            if (Launcher.open)
                Launcher.dismiss();
        }
    }

    IpcHandler {
        target: "keyshell"

        function toggle(): void {
            hudTimer.stop();
            Launcher.toggle();
        }

        function toggleMode(mode: string): void {
            hudTimer.stop();
            if (mode && mode.length > 0)
                BackendManager.setMode(mode);
            Launcher.toggle();
        }

        function open(): void {
            hudTimer.stop();
            if (!Launcher.open)
                Launcher.show();
        }

        function openMode(mode: string): void {
            hudTimer.stop();
            if (mode && mode.length > 0)
                BackendManager.setMode(mode);
            if (!Launcher.open)
                Launcher.show();
        }

        function openVolume(): void {
            hudTimer.stop();
            BackendManager.setMode("audio");
            if (!Launcher.open)
                Launcher.show();
            if (Pipewire.defaultAudioSink)
                Launcher.openSlider(Pipewire.defaultAudioSink);
        }

        function volumeUp(step: real): void {
            const delta = (step > 0.0) ? step : Config.audioVolumeStep;
            if (Pipewire.defaultAudioSink) {
                AudioBackend.adjustVolume(Pipewire.defaultAudioSink, delta);
                BackendManager.setMode("audio");
                if (!Launcher.open)
                    Launcher.show();
                Launcher.openSlider(Pipewire.defaultAudioSink);
                hudTimer.restart();
            }
        }

        function volumeDown(step: real): void {
            const delta = (step > 0.0) ? step : Config.audioVolumeStep;
            if (Pipewire.defaultAudioSink) {
                AudioBackend.adjustVolume(Pipewire.defaultAudioSink, -delta);
                BackendManager.setMode("audio");
                if (!Launcher.open)
                    Launcher.show();
                Launcher.openSlider(Pipewire.defaultAudioSink);
                hudTimer.restart();
            }
        }

        function toggleMute(): void {
            if (Pipewire.defaultAudioSink) {
                AudioBackend.toggleMute(Pipewire.defaultAudioSink);
                BackendManager.setMode("audio");
                if (!Launcher.open)
                    Launcher.show();
                Launcher.openSlider(Pipewire.defaultAudioSink);
                hudTimer.restart();
            }
        }

        function openMedia(): void {
            hudTimer.stop();
            BackendManager.setMode("media");
            if (!Launcher.open)
                Launcher.show();
            const player = MediaBackend.primaryPlayer();
            if (player)
                Launcher.openMediaConsole(player);
        }

        function playPause(): void {
            const player = MediaBackend.primaryPlayer();
            if (player)
                MediaBackend.togglePlay(player);
        }

        function nextTrack(): void {
            const player = MediaBackend.primaryPlayer();
            if (player)
                MediaBackend.next(player);
        }

        function previousTrack(): void {
            const player = MediaBackend.primaryPlayer();
            if (player)
                MediaBackend.previous(player);
        }

        function openTray(): void {
            hudTimer.stop();
            BackendManager.setMode("tray");
            if (!Launcher.open)
                Launcher.show();
        }

        function openBluetooth(): void {
            hudTimer.stop();
            BackendManager.setMode("bluetooth");
            if (!Launcher.open)
                Launcher.show();
        }

        function toggleBluetoothPower(): void {
            BluetoothBackend.togglePower();
        }

        function toggleBluetoothScan(): void {
            BluetoothBackend.toggleDiscovery();
        }

        function openPower(): void {
            hudTimer.stop();
            BackendManager.setMode("power");
            if (!Launcher.open)
                Launcher.show();
        }

        function openPowerConsole(): void {
            hudTimer.stop();
            BackendManager.setMode("power");
            if (!Launcher.open)
                Launcher.show();
            Launcher.openPowerConsole();
        }

        function toggleKeepAwake(): void {
            PowerBackend.toggleKeepAwake();
        }

        function setPowerProfile(profile: string): void {
            if (profile && profile.length > 0)
                PowerBackend.setPowerProfile(profile);
        }

        function lockSession(): void {
            PowerBackend.lockSession();
        }

        function suspendSession(): void {
            PowerBackend.suspendSession();
        }

        function openNetwork(): void {
            hudTimer.stop();
            BackendManager.setMode("network");
            if (!Launcher.open)
                Launcher.show();
        }

        function openNetworkConsole(): void {
            hudTimer.stop();
            BackendManager.setMode("network");
            if (!Launcher.open)
                Launcher.show();
            const items = NetworkBackend.items;
            if (items && items.length > 0)
                Launcher.openNetworkConsole(items[0]);
        }

        function toggleWifi(): void {
            NetworkBackend.toggleWifi();
        }

        function scanWifi(): void {
            NetworkBackend.startScan();
        }

        function disconnectNetwork(): void {
            const items = NetworkBackend.items;
            if (items) {
                for (let i = 0; i < items.length; i++) {
                    if (items[i].connected) {
                        NetworkBackend.disconnectNetwork(items[i]);
                        break;
                    }
                }
            }
        }

        function close(): void {
            hudTimer.stop();
            if (Launcher.open)
                Launcher.dismiss();
        }

        function isOpen(): bool {
            return Launcher.open;
        }

        function setMode(mode: string): void {
            if (mode && mode.length > 0)
                BackendManager.setMode(mode);
        }
    }
}
