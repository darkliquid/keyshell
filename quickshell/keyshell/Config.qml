pragma Singleton
import QtQuick
import Quickshell

Singleton {
    readonly property real minTiltDeg: 5.0
    readonly property real centerSpreadFactor: 0.10
    readonly property real taperMaxFactor: 0.15

    readonly property real shearCoupling: 1.0
    readonly property real shearJitterDeg: 2
    readonly property real shearMinDeg: 6
    readonly property real shearMaxDeg: 14

    readonly property real bandHeightPx: 290
    readonly property real bandCenterFactor: 0.5
    readonly property real minEndHeightPx: 200

    readonly property real cardWidthPx: 320
    readonly property real cardWidthMaxPx: 360
    readonly property real searchHeightPx: 48
    readonly property real contentPaddingPx: 12
    readonly property int cyclicBufferTarget: 600

    readonly property int expandDurationMs: 90
    readonly property int scrollDurationMs: 70

    readonly property int fileDebounceMs: 100
    readonly property int maxResults: 60
    readonly property int maxWindowResults: 60
    readonly property int maxFileResults: 60

    readonly property real audioVolumeStep: 0.05
    readonly property real audioVolumeFineStep: 0.01
    readonly property real volumeMaxBoost: 1.5
    readonly property int volumeHudTimeoutMs: 1200
    readonly property int sliderTickCount: 25

    readonly property int mediaPositionPollIntervalMs: 500
    readonly property real mediaSeekStepSec: 5.0
    readonly property real mediaSeekFineStepSec: 1.0
    readonly property real mediaSeekCoarseStepSec: 15.0

    readonly property int maxTrayMenuItems: 8

    readonly property int bluetoothScanTimeoutMs: 20000

    readonly property int powerHoldDurationMs: 1000
    readonly property int powerDoubleTapTimeoutMs: 400

    readonly property int wifiScanTimeoutMs: 20000

    readonly property int revealMaxAttempts: 32

    readonly property string mergedModeId: "all"
    readonly property int mergeSettleMs: 120
    readonly property int maxMergedResults: 60

    readonly property int frecencyCountCap: 10
    readonly property int frecencyMaxAgeDays: 90
    readonly property int frecencyFlushDebounceMs: 2000
    readonly property var frecencyBuckets: [
        { "maxAgeMs": 3600000, "weight": 4 },
        { "maxAgeMs": 86400000, "weight": 3 },
        { "maxAgeMs": 604800000, "weight": 2 },
        { "maxAgeMs": 2592000000, "weight": 1 }
    ]

    readonly property real effectiveMinEndHeightPx: Math.min(minEndHeightPx, bandHeightPx)

    readonly property int maxExpressionLength: 120
    readonly property int calcMaxDecimals: 10
    readonly property var terminalCandidates: ["ghostty", "alacritty"]

    Component.onCompleted: {
        if (minEndHeightPx > bandHeightPx)
            console.warn("keyshell: minEndHeightPx exceeds bandHeightPx; clamped to " + effectiveMinEndHeightPx);
    }
}
