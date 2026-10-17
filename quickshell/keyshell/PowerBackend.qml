pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Io
import Quickshell.Services.UPower
import qs.components

Singleton {
    id: powerBackend

    readonly property string id: "power"
    readonly property string name: "Power"
    readonly property string badge: "POWER"
    readonly property string prefix: "%"
    readonly property Component cardDelegate: PowerCardDelegate {}

    property bool keepAwakeActive: false
    property var items: []

    signal resultsReady(string query, var items)

    Process {
        id: inhibitorProcess
        command: ["systemd-inhibit", "--what=idle", "--who=keyshell", "--why=Keep Awake", "sleep", "infinity"]
        onRunningChanged: {
            powerBackend.keepAwakeActive = running;
        }
    }

    function lockSession() {
        Quickshell.execDetached(["loginctl", "lock-session"]);
        Launcher.dismiss();
    }

    function suspendSession() {
        Quickshell.execDetached(["systemctl", "suspend"]);
        Launcher.dismiss();
    }

    function rebootSystem() {
        Quickshell.execDetached(["systemctl", "reboot"]);
        Launcher.dismiss();
    }

    function poweroffSystem() {
        Quickshell.execDetached(["systemctl", "poweroff"]);
        Launcher.dismiss();
    }

    function hibernateSystem() {
        Quickshell.execDetached(["systemctl", "hibernate"]);
        Launcher.dismiss();
    }

    function logoutSession() {
        Quickshell.execDetached(["niri", "msg", "action", "quit", "--skip-confirmation"]);
        Launcher.dismiss();
    }

    function toggleKeepAwake() {
        if (inhibitorProcess.running) {
            inhibitorProcess.running = false;
            keepAwakeActive = false;
        } else {
            inhibitorProcess.running = true;
            keepAwakeActive = true;
        }
        updateItems();
    }

    function setPowerProfile(prof) {
        if (!PowerProfiles)
            return;
        if (prof === "power-saver")
            PowerProfiles.profile = PowerProfile.PowerSaver;
        else if (prof === "balanced")
            PowerProfiles.profile = PowerProfile.Balanced;
        else if (prof === "performance" && PowerProfiles.hasPerformanceProfile)
            PowerProfiles.profile = PowerProfile.Performance;
        updateItems();
    }

    function executeAction(item) {
        if (!item)
            return;
        if (item.actions && item.actions.length > 0 && item.actions[0].execute)
            item.actions[0].execute();
    }

    function updateItems() {
        items = [
            {
                "id": "power-lock",
                "title": "Lock Screen",
                "subtitle": "Lock Wayland session",
                "icon": "system-lock-screen",
                "badge": "SESSION",
                "isDestructive": false,
                "isActive": false,
                "delegate": powerBackend.cardDelegate,
                "actions": [
                    {
                        "label": "Lock",
                        "execute": lockSession
                    }
                ]
            },
            {
                "id": "power-suspend",
                "title": "Suspend",
                "subtitle": "Suspend system to RAM",
                "icon": "system-suspend",
                "badge": "POWER",
                "isDestructive": false,
                "isActive": false,
                "delegate": powerBackend.cardDelegate,
                "actions": [
                    {
                        "label": "Suspend",
                        "execute": suspendSession
                    }
                ]
            },
            {
                "id": "power-reboot",
                "title": "Reboot",
                "subtitle": "Reboot operating system",
                "icon": "system-reboot",
                "badge": "POWER",
                "isDestructive": true,
                "isActive": false,
                "delegate": powerBackend.cardDelegate,
                "actions": [
                    {
                        "label": "Reboot",
                        "execute": rebootSystem
                    }
                ]
            },
            {
                "id": "power-poweroff",
                "title": "Power Off",
                "subtitle": "Shutdown machine hardware",
                "icon": "system-shutdown",
                "badge": "POWER",
                "isDestructive": true,
                "isActive": false,
                "delegate": powerBackend.cardDelegate,
                "actions": [
                    {
                        "label": "Power Off",
                        "execute": poweroffSystem
                    }
                ]
            },
            {
                "id": "power-hibernate",
                "title": "Hibernate",
                "subtitle": "Suspend system to disk",
                "icon": "system-suspend-to-disk",
                "badge": "POWER",
                "isDestructive": true,
                "isActive": false,
                "delegate": powerBackend.cardDelegate,
                "actions": [
                    {
                        "label": "Hibernate",
                        "execute": hibernateSystem
                    }
                ]
            },
            {
                "id": "power-logout",
                "title": "Log Out",
                "subtitle": "Terminate compositor session",
                "icon": "system-log-out",
                "badge": "SESSION",
                "isDestructive": true,
                "isActive": false,
                "delegate": powerBackend.cardDelegate,
                "actions": [
                    {
                        "label": "Log Out",
                        "execute": logoutSession
                    }
                ]
            },
            {
                "id": "power-keepawake",
                "title": "Keep Awake",
                "subtitle": keepAwakeActive ? "Sleep inhibitor active" : "Idle sleep allowed",
                "icon": "alarm-symbolic",
                "badge": "INHIBITOR",
                "isDestructive": false,
                "isActive": keepAwakeActive,
                "delegate": powerBackend.cardDelegate,
                "actions": [
                    {
                        "label": "Toggle",
                        "execute": toggleKeepAwake
                    }
                ]
            },
            {
                "id": "power-profile-powersaver",
                "title": "Power Saver",
                "subtitle": (PowerProfiles && PowerProfiles.profile === PowerProfile.PowerSaver) ? "Active energy saver" : "Energy conservation profile",
                "icon": "power-profile-power-saver",
                "badge": "PROFILE",
                "isDestructive": false,
                "isActive": Boolean(PowerProfiles && PowerProfiles.profile === PowerProfile.PowerSaver),
                "delegate": powerBackend.cardDelegate,
                "actions": [
                    {
                        "label": "Select",
                        "execute": function () {
                            setPowerProfile("power-saver");
                        }
                    }
                ]
            },
            {
                "id": "power-profile-balanced",
                "title": "Balanced",
                "subtitle": (PowerProfiles && PowerProfiles.profile === PowerProfile.Balanced) ? "Active balanced profile" : "Standard efficiency profile",
                "icon": "power-profile-balanced",
                "badge": "PROFILE",
                "isDestructive": false,
                "isActive": Boolean(PowerProfiles && PowerProfiles.profile === PowerProfile.Balanced),
                "delegate": powerBackend.cardDelegate,
                "actions": [
                    {
                        "label": "Select",
                        "execute": function () {
                            setPowerProfile("balanced");
                        }
                    }
                ]
            },
            {
                "id": "power-profile-performance",
                "title": "Performance",
                "subtitle": (PowerProfiles && PowerProfiles.profile === PowerProfile.Performance) ? "Active performance profile" : ((PowerProfiles && PowerProfiles.hasPerformanceProfile) ? "Maximum power profile" : "Unsupported on this device"),
                "icon": "power-profile-performance",
                "badge": "PROFILE",
                "isDestructive": false,
                "isActive": Boolean(PowerProfiles && PowerProfiles.profile === PowerProfile.Performance),
                "delegate": powerBackend.cardDelegate,
                "actions": [
                    {
                        "label": "Select",
                        "execute": function () {
                            setPowerProfile("performance");
                        }
                    }
                ]
            }
        ];
    }

    function search(query, maxCount) {
        updateItems();
        const q = (query || "").toLowerCase().trim();
        const limit = maxCount || Config.maxResults;
        let filtered = [];
        if (q === "") {
            filtered = items.slice();
        } else {
            for (let i = 0; i < items.length; i++) {
                const it = items[i];
                const titleMatch = it.title.toLowerCase().indexOf(q) !== -1;
                const subMatch = it.subtitle.toLowerCase().indexOf(q) !== -1;
                const badgeMatch = it.badge.toLowerCase().indexOf(q) !== -1;
                if (titleMatch || subMatch || badgeMatch)
                    filtered.push(it);
            }
        }
        const res = filtered.slice(0, limit);
        resultsReady(query, res);
        return res;
    }

    function refresh() {
        updateItems();
        search(Launcher.query, Config.maxResults);
    }

    Connections {
        target: PowerProfiles
        function onProfileChanged() {
            powerBackend.refresh();
        }
    }

    Component.onCompleted: {
        updateItems();
    }
}
