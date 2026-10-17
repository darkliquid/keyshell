pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Io
import qs.components

Singleton {
    id: fileBackend

    readonly property string id: "files"
    readonly property string name: "Files"
    readonly property string badge: "FILES"
    readonly property string prefix: "/"
    readonly property Component cardDelegate: FileCardDelegate {}

    property int generation: 0
    property string pendingQuery: ""
    property int pendingLimit: Config.maxFileResults

    signal resultsReady(string query, var items)

    Timer {
        id: debounceTimer
        interval: Config.fileDebounceMs
        repeat: false
        onTriggered: {
            fileBackend.executeSearch();
        }
    }

    Process {
        id: dsearchProcess
        property int queryGeneration: 0
        property string activeQuery: ""

        stdout: StdioCollector {
            id: dsearchStdout
        }

        onExited: (exitCode, exitStatus) => {
            if (dsearchProcess.queryGeneration !== fileBackend.generation)
                return;

            try {
                const parsed = JSON.parse(dsearchStdout.text);
                const hits = parsed.hits || [];
                const items = [];
                for (let i = 0; i < hits.length; i++) {
                    const hit = hits[i];
                    const fullPath = hit.id || "";
                    if (fullPath.length === 0)
                        continue;

                    const parts = fullPath.split("/");
                    const filename = parts[parts.length - 1];
                    const parentPath = parts.slice(0, parts.length - 1).join("/").replace(/^\/home\/[^\/]+/, "~");
                    const isDir = (hit.type === "dir") || (hit.fields && hit.fields.type === "dir");
                    const isImage = !isDir && Boolean(filename.match(/\.(png|jpg|jpeg|svg|webp|gif|bmp|ico|avif)$/i));

                    let iconName = "text-x-generic";
                    let fileKind = "FILE";
                    if (isDir) {
                        iconName = "folder";
                        fileKind = "FOLDER";
                    } else if (isImage) {
                        iconName = "image-x-generic";
                        fileKind = "IMAGE";
                    } else if (filename.match(/\.(mp4|mkv|webm|avi|mov|m4v|flv)$/i)) {
                        iconName = "video-x-generic";
                        fileKind = "VIDEO";
                    } else if (filename.match(/\.(mp3|flac|wav|ogg|m4a|aac|opus)$/i)) {
                        iconName = "audio-x-generic";
                        fileKind = "AUDIO";
                    } else if (filename.match(/\.pdf$/i)) {
                        iconName = "application-pdf";
                        fileKind = "PDF";
                    } else if (filename.match(/\.(zip|tar|gz|xz|bz2|zst|7z|rar|deb|rpm|iso)$/i)) {
                        iconName = "package-x-generic";
                        fileKind = "ARCHIVE";
                    } else if (filename.match(/\.(doc|docx|odt|rtf)$/i)) {
                        iconName = "x-office-document";
                        fileKind = "DOCUMENT";
                    } else if (filename.match(/\.(xls|xlsx|ods|csv)$/i)) {
                        iconName = "x-office-spreadsheet";
                        fileKind = "SHEET";
                    } else if (filename.match(/\.(ppt|pptx|odp)$/i)) {
                        iconName = "x-office-presentation";
                        fileKind = "SLIDES";
                    } else if (filename.match(/\.(qml|js|ts|jsx|tsx|py|go|rs|c|cpp|h|hpp|java|rb|php|sh|zsh|bash|fish|lua|zig)$/i)) {
                        iconName = "text-x-script";
                        fileKind = "CODE";
                    } else if (filename.match(/\.(json|yaml|yml|toml|ini|conf|cfg|xml|html|css|scss|md|txt|log)$/i)) {
                        iconName = "text-x-generic";
                        fileKind = "TEXT";
                    } else {
                        iconName = "application-x-Generic";
                    }

                    const extMatch = !isDir ? filename.match(/\.([a-zA-Z0-9]{1,6})$/) : null;
                    const extBadge = extMatch ? extMatch[1].toUpperCase() : fileKind;

                    items.push({
                        "id": fullPath,
                        "title": filename,
                        "subtitle": parentPath,
                        "icon": iconName,
                        "isImage": isImage,
                        "isDir": isDir,
                        "fileKind": fileKind,
                        "extBadge": extBadge,
                        "fileUrl": "file://" + fullPath,
                        "previewSource": null,
                        "badge": "FILES",
                        "delegate": fileBackend.cardDelegate,
                        "actions": [
                            {
                                "label": "Open",
                                "execute": function () {
                                    Quickshell.execDetached(["xdg-open", fullPath]);
                                }
                            }
                        ]
                    });
                }
                fileBackend.resultsReady(dsearchProcess.activeQuery, items);
            } catch (e) {
            }
        }
    }

    function executeSearch() {
        if (dsearchProcess.running)
            dsearchProcess.running = false;

        const currentGen = generation;
        dsearchProcess.queryGeneration = currentGen;
        dsearchProcess.activeQuery = pendingQuery;

        const trimmed = pendingQuery.trim();
        const args = ["search", "--json", "--limit", pendingLimit.toString()];
        if (trimmed !== "") {
            args.push("--fuzzy");
            args.push("--");
            args.push(trimmed);
        } else {
            args.push("--sort");
            args.push("mtime");
            args.push("--");
            args.push("*");
        }

        dsearchProcess.command = ["dsearch"].concat(args);
        dsearchProcess.running = true;
    }

    function search(q, limit) {
        generation++;
        pendingQuery = q || "";
        pendingLimit = limit || Config.maxFileResults;
        debounceTimer.restart();
        return null;
    }

    function refresh() {
        pendingQuery = "";
        executeSearch();
    }
}
