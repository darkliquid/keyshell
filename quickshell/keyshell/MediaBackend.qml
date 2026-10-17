pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Services.Mpris
import qs.components

Singleton {
    id: mediaBackend

    readonly property string id: "media"
    readonly property string name: "Media"
    readonly property string badge: "MEDIA"
    readonly property string prefix: ">"
    readonly property Component cardDelegate: MediaCardDelegate {}

    property var items: []

    signal resultsReady(string query, var items)

    Timer {
        id: pollTimer
        interval: Config.mediaPositionPollIntervalMs
        repeat: true
        running: Launcher.open && (BackendManager.activeMode === "media" || Launcher.mediaConsolePlayer !== null)
        onTriggered: {
            if (BackendManager.activeMode === "media")
                mediaBackend.refresh();
            else
                mediaBackend.updateItems();
        }
    }

    function formatDuration(sec) {
        if (sec === undefined || sec === null || sec < 0 || !isFinite(sec))
            return "00:00";
        const total = Math.floor(sec);
        const m = Math.floor(total / 60);
        const s = total % 60;
        return (m < 10 ? "0" + m : m) + ":" + (s < 10 ? "0" + s : s);
    }

    function updateItems() {
        const playerModel = Mpris.players;
        if (!playerModel) {
            items = [];
            return;
        }

        let raw = [];
        const n = playerModel.count !== undefined ? playerModel.count : (playerModel.values ? playerModel.values.length : 0);
        for (let i = 0; i < n; i++) {
            const player = playerModel.values ? playerModel.values[i] : playerModel.get(i);
            if (player)
                raw.push(player);
        }

        raw.sort(function(a, b) {
            const aPlaying = a.isPlaying ? 1 : 0;
            const bPlaying = b.isPlaying ? 1 : 0;
            if (aPlaying !== bPlaying)
                return bPlaying - aPlaying;
            const aPaused = (a.playbackState === MprisPlaybackState.Paused) ? 1 : 0;
            const bPaused = (b.playbackState === MprisPlaybackState.Paused) ? 1 : 0;
            if (aPaused !== bPaused)
                return bPaused - aPaused;
            const aName = a.identity || "";
            const bName = b.identity || "";
            return aName.localeCompare(bName);
        });

        let list = [];
        for (let i = 0; i < raw.length; i++) {
            const p = raw[i];
            const identity = p.identity || "Media Player";
            const title = p.trackTitle || "No Track";
            const artist = p.trackArtist || "";
            const album = p.trackAlbum || "";
            let sub = "";
            if (artist.length > 0 && album.length > 0)
                sub = artist + " — " + album;
            else if (artist.length > 0)
                sub = artist;
            else if (album.length > 0)
                sub = album;
            else
                sub = identity;

            list.push({
                id: "mpris-" + (p.desktopEntry || identity || i),
                title: title,
                subtitle: sub,
                icon: p.desktopEntry || "audio-x-generic",
                artUrl: p.trackArtUrl || "",
                previewSource: null,
                badge: identity.toUpperCase(),
                playbackState: p.playbackState,
                isPlaying: p.isPlaying,
                player: p,
                delegate: mediaBackend.cardDelegate,
                actions: [
                    {
                        label: "Open Console",
                        execute: function() {
                            Launcher.openMediaConsole(p);
                        }
                    }
                ]
            });
        }

        items = list;
    }

    function search(query, maxCount) {
        updateItems();
        const limit = (maxCount !== undefined && maxCount > 0) ? maxCount : Config.maxResults;
        const q = (query || "").trim().toLowerCase();

        if (q.length === 0) {
            const res = items.slice(0, limit);
            resultsReady(query, res);
            return res;
        }

        let filtered = [];
        for (let i = 0; i < items.length; i++) {
            const item = items[i];
            const titleMatch = item.title.toLowerCase().indexOf(q) !== -1;
            const subMatch = item.subtitle.toLowerCase().indexOf(q) !== -1;
            const badgeMatch = item.badge.toLowerCase().indexOf(q) !== -1;
            if (titleMatch || subMatch || badgeMatch)
                filtered.push(item);
        }

        const res = filtered.slice(0, limit);
        resultsReady(query, res);
        return res;
    }

    function refresh() {
        updateItems();
        search(Launcher.query, Config.maxResults);
    }

    function togglePlay(player) {
        if (!player)
            return;
        if (player.canTogglePlaying)
            player.togglePlaying();
        else if (player.isPlaying && player.canPause)
            player.pause();
        else if (!player.isPlaying && player.canPlay)
            player.play();
    }

    function next(player) {
        if (!player)
            return;
        if (player.canGoNext)
            player.next();
    }

    function previous(player) {
        if (!player)
            return;
        if (player.canGoPrevious)
            player.previous();
    }

    function seek(player, offsetSec) {
        if (!player || !player.canSeek)
            return;
        player.seek(offsetSec);
    }

    function primaryPlayer() {
        updateItems();
        if (items.length === 0)
            return null;
        for (let i = 0; i < items.length; i++) {
            if (items[i].player && items[i].player.isPlaying)
                return items[i].player;
        }
        return items[0].player;
    }

    Connections {
        target: Mpris.players
        ignoreUnknownSignals: true
        function onValuesChanged() {
            mediaBackend.updateItems();
            if (BackendManager.activeMode === "media")
                mediaBackend.search(Launcher.query, Config.maxResults);
        }
    }

    Component.onCompleted: {
        updateItems();
    }
}
