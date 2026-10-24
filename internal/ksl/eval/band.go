package eval

import (
	"fmt"
	"math"
	"math/rand"
)

// callBandHelper generates geometry for a slanted band.
func callBandHelper(env *Env, args []any) (any, error) {
	st := env.State()

	var w, h float64
	var seed int64
	if st != nil {
		w, h = st.Width, st.Height
		seed = int64(st.Seed) ^ int64(st.Summon)
	}
	if w <= 0 {
		w = 1920
	}
	if h <= 0 {
		h = 1080
	}

	height := getFloat(args, 0, 290.0)
	minTiltDeg := getFloat(args, 1, 5.0)
	centerSpread := getFloat(args, 2, 0.10)
	taperMax := getFloat(args, 3, 0.15)
	shearMinDeg := getFloat(args, 4, 6.0)
	shearMaxDeg := getFloat(args, 5, 14.0)

	rng := rand.New(rand.NewSource(seed))

	const degToRad = math.Pi / 180
	yMid := h * 0.5
	spread := h * centerSpread
	minDelta := w * math.Tan(minTiltDeg*degToRad)

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

	taperL := 1 + (rng.Float64()*2-1)*taperMax
	taperR := 1 + (rng.Float64()*2-1)*taperMax
	hLeft := math.Max(200.0, height*taperL)
	hRight := math.Max(200.0, height*taperR)

	jitter := (rng.Float64()*2 - 1) * 2.0
	raw := 1.0*math.Abs(angle/degToRad) + jitter
	shearDeg := math.Max(shearMinDeg, math.Min(shearMaxDeg, raw))
	sign := 1.0
	if angle < 0 {
		sign = -1
	}
	shear := math.Tan(shearDeg*degToRad) * sign

	halfL := hLeft / 2
	halfR := hRight / 2
	d := fmt.Sprintf("M 0 %g L %g %g L %g %g L 0 %g Z",
		yL-halfL, w, yR-halfR, w, yR+halfR, yL+halfL)

	cw := math.Sqrt(w*w + (yR-yL)*(yR-yL))
	ch := math.Min(hLeft, hRight) * math.Cos(angle)

	return map[string]any{
		"d":         d,
		"angle":     Length{V: angle * 180 / math.Pi, Unit: "deg"},
		"shear":     shear,
		"cardShear": shear,
		"cw":        cw,
		"ch":        ch,
		"frameX":    (w - cw) / 2,
		"frameY":    center - ch/2,
		"y":         center,
		"h":         ch,
		"hLeft":     hLeft,
		"hRight":    hRight,
		"yLeft":     yL,
		"yRight":    yR,
	}, nil
}

func getFloat(args []any, idx int, def float64) float64 {
	if idx < len(args) && args[idx] != nil {
		switch v := args[idx].(type) {
		case float64:
			return v
		case float32:
			return float64(v)
		case int:
			return float64(v)
		case int64:
			return float64(v)
		case int32:
			return float64(v)
		case Num:
			return float64(v)
		case Length:
			return v.V
		}
	}
	return def
}
