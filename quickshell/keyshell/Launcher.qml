pragma Singleton
import QtQuick
import Quickshell

Singleton {
    id: launcher

    property bool open: false
    property string query: ""
    property var results: []
    property var displayResults: []
    property int selectedIndex: 0
    property int displayIndex: 0
    property string selectedItemId: ""
    property string selectedSource: ""
    property var sliderTargetNode: null
    property var mediaConsolePlayer: null
    property var trayMenuItem: null
    property var trayMenuEntries: []
    property int trayMenuIndex: 0
    property var bluetoothTargetDevice: null
    property bool powerConsoleOpen: false
    property real powerHoldProgress: 0.0
    property var networkTargetItem: null
    property var networkPasswordTarget: null
    property string networkPasswordBuffer: ""

    signal revealed()
    signal dismissed()
    signal modelReset()
    signal sliderOpened()
    signal sliderClosed()
    signal mediaConsoleOpened()
    signal mediaConsoleClosed()
    signal trayMenuOpened()
    signal trayMenuClosed()
    signal bluetoothConsoleOpened()
    signal bluetoothConsoleClosed()
    signal powerConsoleOpened()
    signal powerConsoleClosed()
    signal networkConsoleOpened()
    signal networkConsoleClosed()
    signal networkPasswordOpened()
    signal networkPasswordClosed()

    function buildDisplayResults() {
        const n = results.length;
        if (n === 0) {
            displayResults = [];
            displayIndex = 0;
            return;
        }

        const target = Config.cyclicBufferTarget;
        let repeatFactor = Math.max(21, Math.ceil(target / n));
        if (repeatFactor % 2 === 0)
            repeatFactor += 1;

        let list = [];
        for (let r = 0; r < repeatFactor; r++) {
            for (let i = 0; i < n; i++)
                list.push(results[i]);
        }

        const midCycle = Math.floor(repeatFactor / 2);
        displayResults = list;
        displayIndex = midCycle * n + selectedIndex;
    }

    function refresh() {
        if (BackendManager.isMerged())
            SearchAggregator.searchAll(query, Config.maxMergedResults);
        else
            BackendManager.search(query, Config.maxResults);
    }

    function toggle() {
        if (open)
            dismiss();
        else
            show();
    }

    function show() {
        query = "";
        selectedIndex = 0;
        selectedItemId = "";
        selectedSource = "";
        sliderTargetNode = null;
        mediaConsolePlayer = null;
        trayMenuItem = null;
        trayMenuEntries = [];
        trayMenuIndex = 0;
        bluetoothTargetDevice = null;
        powerConsoleOpen = false;
        powerHoldProgress = 0.0;
        networkTargetItem = null;
        networkPasswordTarget = null;
        networkPasswordBuffer = "";
        refresh();
        open = true;
        revealed();
    }

    function dismiss() {
        if (!open)
            return;
        open = false;
        sliderTargetNode = null;
        mediaConsolePlayer = null;
        trayMenuItem = null;
        trayMenuEntries = [];
        trayMenuIndex = 0;
        bluetoothTargetDevice = null;
        powerConsoleOpen = false;
        powerHoldProgress = 0.0;
        networkTargetItem = null;
        networkPasswordTarget = null;
        networkPasswordBuffer = "";
        BluetoothBackend.stopDiscovery();
        NetworkBackend.stopScan();
        PluginRegistry.stopAll();
        Frecency.flush();
        dismissed();
    }

    function openSlider(node) {
        if (!node)
            return;
        mediaConsolePlayer = null;
        trayMenuItem = null;
        trayMenuEntries = [];
        bluetoothTargetDevice = null;
        powerConsoleOpen = false;
        powerHoldProgress = 0.0;
        networkTargetItem = null;
        networkPasswordTarget = null;
        networkPasswordBuffer = "";
        sliderTargetNode = node;
        sliderOpened();
    }

    function closeSlider() {
        if (!sliderTargetNode)
            return;
        sliderTargetNode = null;
        sliderClosed();
    }

    function openMediaConsole(player) {
        if (!player)
            return;
        sliderTargetNode = null;
        trayMenuItem = null;
        trayMenuEntries = [];
        bluetoothTargetDevice = null;
        powerConsoleOpen = false;
        powerHoldProgress = 0.0;
        networkTargetItem = null;
        networkPasswordTarget = null;
        networkPasswordBuffer = "";
        mediaConsolePlayer = player;
        mediaConsoleOpened();
    }

    function closeMediaConsole() {
        if (!mediaConsolePlayer)
            return;
        mediaConsolePlayer = null;
        mediaConsoleClosed();
    }

    function openTrayMenu(item) {
        if (!item)
            return;
        sliderTargetNode = null;
        mediaConsolePlayer = null;
        bluetoothTargetDevice = null;
        powerConsoleOpen = false;
        powerHoldProgress = 0.0;
        networkTargetItem = null;
        networkPasswordTarget = null;
        networkPasswordBuffer = "";
        trayMenuItem = item;
        trayMenuEntries = TrayBackend.extractMenuItems(item);
        trayMenuIndex = 0;
        trayMenuOpened();
    }

    function closeTrayMenu() {
        if (!trayMenuItem)
            return;
        trayMenuItem = null;
        trayMenuEntries = [];
        trayMenuIndex = 0;
        trayMenuClosed();
    }

    function moveTrayMenuSelection(delta) {
        const n = trayMenuEntries.length;
        if (n === 0)
            return;
        trayMenuIndex = ((trayMenuIndex + delta) % n + n) % n;
    }

    function openBluetoothConsole(dev) {
        if (!dev)
            return;
        sliderTargetNode = null;
        mediaConsolePlayer = null;
        trayMenuItem = null;
        trayMenuEntries = [];
        powerConsoleOpen = false;
        powerHoldProgress = 0.0;
        networkTargetItem = null;
        networkPasswordTarget = null;
        networkPasswordBuffer = "";
        bluetoothTargetDevice = dev;
        bluetoothConsoleOpened();
    }

    function closeBluetoothConsole() {
        if (!bluetoothTargetDevice)
            return;
        bluetoothTargetDevice = null;
        bluetoothConsoleClosed();
    }

    function openPowerConsole() {
        sliderTargetNode = null;
        mediaConsolePlayer = null;
        trayMenuItem = null;
        trayMenuEntries = [];
        bluetoothTargetDevice = null;
        networkTargetItem = null;
        networkPasswordTarget = null;
        networkPasswordBuffer = "";
        powerConsoleOpen = true;
        powerHoldProgress = 0.0;
        powerConsoleOpened();
    }

    function closePowerConsole() {
        if (!powerConsoleOpen)
            return;
        powerConsoleOpen = false;
        powerHoldProgress = 0.0;
        powerConsoleClosed();
    }

    function setPowerHoldProgress(val) {
        powerHoldProgress = Math.max(0.0, Math.min(1.0, val));
    }

    function openNetworkConsole(item) {
        if (!item)
            return;
        sliderTargetNode = null;
        mediaConsolePlayer = null;
        trayMenuItem = null;
        trayMenuEntries = [];
        bluetoothTargetDevice = null;
        powerConsoleOpen = false;
        powerHoldProgress = 0.0;
        networkPasswordTarget = null;
        networkPasswordBuffer = "";
        networkTargetItem = item;
        networkConsoleOpened();
    }

    function closeNetworkConsole() {
        if (!networkTargetItem)
            return;
        networkTargetItem = null;
        networkConsoleClosed();
    }

    function openNetworkPassword(item) {
        if (!item)
            return;
        sliderTargetNode = null;
        mediaConsolePlayer = null;
        trayMenuItem = null;
        trayMenuEntries = [];
        bluetoothTargetDevice = null;
        powerConsoleOpen = false;
        powerHoldProgress = 0.0;
        networkTargetItem = null;
        networkPasswordTarget = item;
        networkPasswordBuffer = "";
        networkPasswordOpened();
    }

    function closeNetworkPassword() {
        if (!networkPasswordTarget)
            return;
        networkPasswordTarget = null;
        networkPasswordBuffer = "";
        networkPasswordClosed();
    }

    function setNetworkPasswordBuffer(buf) {
        networkPasswordBuffer = buf;
    }

    function appendNetworkPasswordChar(ch) {
        networkPasswordBuffer += ch;
    }

    function deleteNetworkPasswordChar() {
        if (networkPasswordBuffer.length > 0)
            networkPasswordBuffer = networkPasswordBuffer.slice(0, -1);
    }

    function setQuery(text) {
        query = text;
        selectedIndex = 0;
        selectedItemId = "";
        selectedSource = "";
        refresh();
    }

    function moveSelection(delta) {
        const n = results.length;
        if (n === 0)
            return;
        selectedIndex = ((selectedIndex + delta) % n + n) % n;
        const entry = results[selectedIndex];
        if (entry) {
            selectedItemId = entry.id || "";
            selectedSource = entry.source || "";
        }
        const nextDisplay = displayIndex + delta;
        if (nextDisplay >= 0 && nextDisplay < displayResults.length)
            displayIndex = nextDisplay;
    }

    function activate() {
        if (results.length === 0)
            return;
        const entry = results[selectedIndex];
        if (entry && entry.actions && entry.actions.length > 0 && entry.actions[0].execute) {
            SearchAggregator.recordActivation(entry);
            entry.actions[0].execute();
            if (sliderTargetNode === null && mediaConsolePlayer === null && trayMenuItem === null && bluetoothTargetDevice === null && !powerConsoleOpen && networkTargetItem === null && networkPasswordTarget === null)
                dismiss();
        }
    }

    function installResults(items) {
        const previous = selectedIndex;
        results = items || [];

        let idx = -1;
        if (selectedItemId !== "") {
            for (let i = 0; i < results.length; i++) {
                const entry = results[i];
                if (entry.id === selectedItemId && (entry.source || "") === selectedSource) {
                    idx = i;
                    break;
                }
            }
        }
        if (idx < 0)
            idx = Math.min(previous, results.length > 0 ? results.length - 1 : 0);

        selectedIndex = idx;
        buildDisplayResults();
        modelReset();
    }

    Connections {
        target: BackendManager
        function onResultsReady(items) {
            launcher.installResults(items);
        }
    }

    Connections {
        target: SearchAggregator
        function onResultsReady(items) {
            if (BackendManager.isMerged())
                launcher.installResults(items);
        }
    }
}
