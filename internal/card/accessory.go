package card

import (
	"sort"

	"github.com/darkliquid/keyshell/internal/backend"
)

// AccessoryRows is the hard cap on accessory widgets a card shows. The band
// geometry allows more, but two is what a card reads as before it becomes a
// table.
const AccessoryRows = 2

const (
	rankPinned = iota
	rankPrimary
	rankAnomaly
	rankPlain
)

type candidate struct {
	part Part
	rank int
	ord  int
}

// accessories picks up to AccessoryRows control-backed widgets and reports how
// many eligible controls did not fit.
func accessories(item backend.Item) ([]Part, int) {
	var cands []candidate
	for i, c := range item.Controls {
		part, ok := accessoryPart(i, c)
		if !ok {
			continue
		}
		cands = append(cands, candidate{part: part, rank: rankOf(c), ord: i})
	}
	sort.SliceStable(cands, func(i, j int) bool {
		if cands[i].rank != cands[j].rank {
			return cands[i].rank < cands[j].rank
		}
		return cands[i].ord < cands[j].ord
	})

	out := make([]Part, 0, AccessoryRows)
	shown := make(map[int]bool, AccessoryRows)
	for _, c := range cands {
		if len(out) == AccessoryRows {
			break
		}
		out = append(out, c.part)
		shown[c.ord] = true
	}
	hidden := 0
	for _, c := range cands {
		if !shown[c.ord] {
			hidden++
		}
	}
	return out, hidden
}

// accessoryPart maps one control to its card form. The kind is the first
// filter: text never reaches a card and a choice needs an explicit pin.
func accessoryPart(i int, c backend.Control) (Part, bool) {
	if c.Display == backend.DisplayExpanded {
		return Part{}, false
	}
	p := Part{Control: i, Text: c.Text, Label: c.Label}
	switch c.Kind {
	case backend.ControlRange:
		p.Kind = backend.PartMeter
		p.Ratio = ratio(c)
		if ratioLike(c) {
			p.Ticks = 10
		}
	case backend.ControlProgress:
		p.Kind = backend.PartTimeline
		p.Ratio = ratio(c)
	case backend.ControlToggle:
		if !c.Bool {
			return Part{}, false
		}
		p.Kind = backend.PartNote
	case backend.ControlChoice:
		if c.Display != backend.DisplayCard {
			return Part{}, false
		}
		p.Kind = backend.PartNote
	default:
		return Part{}, false
	}
	return p, true
}

func rankOf(c backend.Control) int {
	switch {
	case c.Display == backend.DisplayCard:
		return rankPinned
	case c.Primary:
		return rankPrimary
	case anomalous(c):
		return rankAnomaly
	}
	return rankPlain
}

// anomalous reports a value in a state worth naming: a true toggle, or a ratio
// below a fifth of its range. Healthy state stays silent.
func anomalous(c backend.Control) bool {
	switch c.Kind {
	case backend.ControlToggle:
		return c.Bool
	case backend.ControlRange, backend.ControlProgress:
		return ratio(c) < 0.2
	}
	return false
}

func ratio(c backend.Control) float64 {
	if c.Max <= c.Min {
		return 0
	}
	r := (c.Value - c.Min) / (c.Max - c.Min)
	if r < 0 {
		return 0
	}
	if r > 1 {
		return 1
	}
	return r
}

func ratioLike(c backend.Control) bool {
	return c.Min <= 0 && c.Max > 0 && c.Max <= 1
}
