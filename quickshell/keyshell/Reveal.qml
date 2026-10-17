pragma Singleton
import Quickshell

Singleton {
    id: reveal

    property real yLeft: 0
    property real yRight: 0
    property real hLeft: Config.bandHeightPx
    property real hRight: Config.bandHeightPx
    property real angleRad: 0
    property real shearFactor: 0
    property real yc: 0

    readonly property real degToRad: Math.PI / 180

    function resample(w, h) {
        if (!(w > 0) || !(h > 0))
            return;

        const yMid = h * Config.bandCenterFactor;
        const spread = h * Config.centerSpreadFactor;
        const minDelta = w * Math.tan(Config.minTiltDeg * degToRad);

        let yL = yMid + (Math.random() * 2 - 1) * spread;
        let yR = yMid + (Math.random() * 2 - 1) * spread;

        if (Math.abs(yR - yL) < minDelta) {
            const mid = (yL + yR) / 2;
            const dir = yR >= yL ? 1 : -1;
            yL = mid - (dir * minDelta) / 2;
            yR = mid + (dir * minDelta) / 2;
        }

        const angle = Math.atan2(yR - yL, w);
        const center = (yL + yR) / 2;

        const baseH = Config.bandHeightPx;
        const taperL = 1 + (Math.random() * 2 - 1) * Config.taperMaxFactor;
        const taperR = 1 + (Math.random() * 2 - 1) * Config.taperMaxFactor;

        const leftH = Math.max(Config.effectiveMinEndHeightPx, baseH * taperL);
        const rightH = Math.max(Config.effectiveMinEndHeightPx, baseH * taperR);

        const jitter = (Math.random() * 2 - 1) * Config.shearJitterDeg;
        const raw = Config.shearCoupling * Math.abs(angle / degToRad) + jitter;
        const shearDeg = Math.max(Config.shearMinDeg, Math.min(Config.shearMaxDeg, raw));
        const sign = angle < 0 ? -1 : 1;

        yLeft = yL;
        yRight = yR;
        hLeft = leftH;
        hRight = rightH;
        angleRad = angle;
        yc = center;
        shearFactor = Math.tan(shearDeg * degToRad) * sign;
    }
}
