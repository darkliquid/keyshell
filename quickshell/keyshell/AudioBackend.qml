pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Services.Pipewire
import qs.components

Singleton {
    id: audioBackend

    readonly property string id: "audio"
    readonly property string name: "Audio"
    readonly property string badge: "AUDIO"
    readonly property string prefix: "!"
    readonly property Component cardDelegate: AudioCardDelegate {}

    signal resultsReady(string query, var items)

    function mapFormFactorIcon(node) {
        if (!node)
            return "audio-card";
        const props = node.properties || {};
        const formFactor = props["device.form-factor"] || "";
        const iconName = props["device.icon-name"] || "";

        if (iconName.length > 0)
            return iconName;

        if (formFactor === "headset" || formFactor === "headphones")
            return "audio-headset";
        if (formFactor === "speaker")
            return "audio-speakers";
        if (formFactor === "microphone" || !node.isSink)
            return "audio-input-microphone";
        return "audio-card";
    }

    function formatSubtitle(node) {
        if (!node)
            return "";
        const props = node.properties || {};
        const api = props["device.api"] || "";
        const bus = props["device.bus"] || "";
        if (bus.length > 0)
            return bus.toUpperCase() + " AUDIO";
        if (api.length > 0)
            return api.toUpperCase();
        return node.isSink ? "AUDIO OUTPUT" : "AUDIO INPUT";
    }

    function buildItems() {
        const nodes = Pipewire.nodes.values || [];
        const items = [];

        for (let i = 0; i < nodes.length; i++) {
            const node = nodes[i];
            if (!node || !node.audio || node.isStream)
                continue;

            const isSink = node.isSink;
            const isDefault = isSink
                ? (Pipewire.defaultAudioSink === node)
                : (Pipewire.defaultAudioSource === node);

            items.push({
                "id": "pw-node-" + node.id,
                "title": node.description || node.name || "Audio Device",
                "subtitle": formatSubtitle(node),
                "icon": mapFormFactorIcon(node),
                "previewSource": null,
                "badge": isSink ? "OUTPUT" : "INPUT",
                "isSink": isSink,
                "isDefault": isDefault,
                "node": node,
                "delegate": audioBackend.cardDelegate,
                "actions": [
                    {
                        "label": isDefault ? "Adjust Volume" : "Set as Default",
                        "execute": function () {
                            if (isDefault)
                                Launcher.openSlider(node);
                            else
                                audioBackend.setDefault(node);
                        }
                    },
                    {
                        "label": "Adjust Volume",
                        "execute": function () {
                            Launcher.openSlider(node);
                        }
                    },
                    {
                        "label": "Toggle Mute",
                        "execute": function () {
                            audioBackend.toggleMute(node);
                        }
                    }
                ]
            });
        }

        items.sort(function (a, b) {
            const aRank = a.isSink ? (a.isDefault ? 0 : 1) : (a.isDefault ? 2 : 3);
            const bRank = b.isSink ? (b.isDefault ? 0 : 1) : (b.isDefault ? 2 : 3);
            if (aRank !== bRank)
                return aRank - bRank;
            return a.title.localeCompare(b.title);
        });

        return items;
    }

    function search(query, limit) {
        const q = (query || "").trim().toLowerCase();
        const maxCount = limit || Config.maxResults;
        const allItems = buildItems();

        if (q === "")
            return allItems.slice(0, maxCount);

        const filtered = [];
        for (let i = 0; i < allItems.length; i++) {
            const item = allItems[i];
            const titleMatch = item.title.toLowerCase().indexOf(q) !== -1;
            const subtitleMatch = item.subtitle.toLowerCase().indexOf(q) !== -1;
            if (titleMatch || subtitleMatch) {
                filtered.push(item);
                if (filtered.length >= maxCount)
                    break;
            }
        }
        return filtered;
    }

    function refresh() {
        resultsReady(Launcher.query, search(Launcher.query, Config.maxResults));
    }

    PwObjectTracker {
        objects: Pipewire.nodes.values
    }

    function setDefault(node) {
        if (!node)
            return;
        if (node.isSink)
            Pipewire.preferredDefaultAudioSink = node;
        else
            Pipewire.preferredDefaultAudioSource = node;
        refresh();
    }

    function toggleMute(node) {
        if (!node || !node.audio)
            return;
        node.audio.muted = !node.audio.muted;
    }

    function adjustVolume(node, delta) {
        if (!node || !node.audio)
            return;
        const raw = node.audio.volume;
        const current = (raw !== undefined && isFinite(raw)) ? raw : 0.0;
        const target = Math.max(0.0, Math.min(Config.volumeMaxBoost, current + delta));
        node.audio.volume = target;
    }

    Connections {
        target: Pipewire
        function onDefaultAudioSinkChanged() { audioBackend.refresh(); }
        function onDefaultAudioSourceChanged() { audioBackend.refresh(); }
        function onReadyChanged() { audioBackend.refresh(); }
    }

    Connections {
        target: Pipewire.nodes
        function onValuesChanged() { audioBackend.refresh(); }
    }
}
