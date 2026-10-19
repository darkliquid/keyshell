package render

import (
	"math"
	"math/rand"

	"github.com/darkliquid/keyshell/internal/config"
)

// Reveal is the band's per-summon geometry, ported from the QML Reveal.qml.
// The band is a four-point trapezoid: each end has its own centre line and
// height, and the content inside it is rotated by the tilt angle.
type Reveal struct {
	YLeft  float64 // centre y at the left edge
	YRight float64 // centre y at the right edge
	HLeft  float64 // band height at the left edge
	HRight float64 // band height at the right edge
	Angle  float64 // tilt in radians
	Shear  float64 // card shear factor (tan of the shear angle, signed)
	Yc     float64 // band centre line y
}

const degToRad = math.Pi / 180

// Resample randomises the band geometry within the configured ranges. It is
// called once per summon, not per frame.
func Resample(w, h float64, cfg config.Reveal, rng *rand.Rand) Reveal {
	if w <= 0 || h <= 0 {
		return Reveal{}
	}
	yMid := h * cfg.BandCenterFactor
	spread := h * cfg.CenterSpreadFactor
	minDelta := w * math.Tan(cfg.MinTiltDeg*degToRad)

	yL := yMid + (rng.Float64()*2-1)*spread
	yR := yMid + (rng.Float64()*2-1)*spread
	if math.Abs(yR-yL) < minDelta {
		mid := (yL + yR) / 2
		dir := 1.0
		if yR < yL {
			dir = -1
		}
		yL = mid - dir*minDelta/2
		yR = mid + dir*minDelta/2
	}

	angle := math.Atan2(yR-yL, w)
	center := (yL + yR) / 2

	taperL := 1 + (rng.Float64()*2-1)*cfg.TaperMaxFactor
	taperR := 1 + (rng.Float64()*2-1)*cfg.TaperMaxFactor
	leftH := math.Max(cfg.MinEndHeightPx, cfg.BandHeightPx*taperL)
	rightH := math.Max(cfg.MinEndHeightPx, cfg.BandHeightPx*taperR)

	jitter := (rng.Float64()*2 - 1) * cfg.ShearJitterDeg
	raw := cfg.ShearCoupling*math.Abs(angle/degToRad) + jitter
	shearDeg := math.Max(cfg.ShearMinDeg, math.Min(cfg.ShearMaxDeg, raw))
	sign := 1.0
	if angle < 0 {
		sign = -1
	}

	return Reveal{
		YLeft: yL, YRight: yR,
		HLeft: leftH, HRight: rightH,
		Angle: angle, Yc: center,
		Shear: math.Tan(shearDeg*degToRad) * sign,
	}
}
