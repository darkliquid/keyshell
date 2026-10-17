pragma Singleton
import QtQuick
import Quickshell

Singleton {
    readonly property color slab: "#0B0B0F"
    readonly property color surfaceAlt: "#121218"
    readonly property color ink: "#F2F2F7"
    readonly property color accent: "#FF2E63"
    readonly property color accentAlt: "#08F7FE"
    readonly property color muted: "#5A5A66"

    readonly property string fontFamily: "Red Hat Display"
    readonly property real shadowOffset: 4

    function heavy(pixelSize) {
        return Qt.font({
            "family": fontFamily,
            "pixelSize": pixelSize,
            "weight": Font.Black,
            "italic": true,
            "capitalization": Font.AllUppercase,
            "hintingPreference": Font.PreferNoHinting
        });
    }
}
