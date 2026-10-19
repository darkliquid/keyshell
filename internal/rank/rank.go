// Package rank ports the QML ranking math: match tiers, frecency scoring, and
// merge-and-sort. It is pure and has no dependencies on the launcher or the UI.
package rank

import (
	"math"
	"strings"

	"github.com/darkliquid/keyshell/internal/backend"
)

// Matcher scores a candidate against a query. The default scorer is a simple
// word-aware scorer; a stronger scorer (e.g. fzf's algorithm) can replace it.
type Matcher interface {
	Score(query, candidate string) float64
}

// DefaultMatcher is the basic fuzzy scorer.
type DefaultMatcher struct{}

// Score returns a higher value when candidate contains query's characters in
// order. This is a stub tuned for the tier model; ranking primarily uses the
// tier, with this only as a tiebreak.
func (DefaultMatcher) Score(query, candidate string) float64 {
	q := strings.ToLower(query)
	c := strings.ToLower(candidate)
	if q == "" {
		return 1
	}
	// Simple subsequence match score.
	score := 0.0
	qi := 0
	for i := 0; i < len(c) && qi < len(q); i++ {
		if c[i] == q[qi] {
			score += 1.0 / float64(i+1)
			qi++
		}
	}
	if qi < len(q) {
		return 0
	}
	return score
}

// Match is one ranked result.
type Match struct {
	Item     backend.Item
	Source   string
	Tier     int
	Frecency float64
	Score    float64
}

// Group is a source with its items, mirroring rank.js's groups.
type Group struct {
	Source string
	Items  []backend.Item
}

// Options controls ranking.
type Options struct {
	Buckets  []Bucket
	CountCap int
	Limit    int
	Matcher  Matcher
}

// Bucket is an age/weight pair.
type Bucket struct {
	MaxAgeMs int64
	Weight   int
}

// Hist is the frecency lookup for a score.
type Hist interface {
	Score(source string, id backend.ResultID) float64
}

// Rank takes groups, ranks their items, and returns the top matches.
func Rank(query string, groups []Group, hist Hist, opts Options) []Match {
	if opts.CountCap == 0 {
		opts.CountCap = 10
	}
	if opts.Limit == 0 {
		opts.Limit = 60
	}
	if opts.Matcher == nil {
		opts.Matcher = DefaultMatcher{}
	}

	merged := []Match{}
	for _, g := range groups {
		for _, item := range g.Items {
			tier := tier(query, item)
			if tier < 0 {
				continue
			}
			var freSc float64
			if hist != nil {
				freSc = hist.Score(g.Source, item.ID)
			}
			merged = append(merged, Match{
				Item:     item,
				Source:   g.Source,
				Tier:     tier,
				Frecency: freSc,
				Score:    opts.Matcher.Score(query, item.Title),
			})
		}
	}

	sortMatches(merged)

	if len(merged) > opts.Limit {
		merged = merged[:opts.Limit]
	}
	return merged
}

// RankOne ranks a single group.
func RankOne(query string, source string, items []backend.Item, hist Hist, opts Options) []Match {
	return Rank(query, []Group{{Source: source, Items: items}}, hist, opts)
}

// sortMatches sorts by tier, then frecency, then length, then title.
func sortMatches(m []Match) {
	for i := 1; i < len(m); i++ {
		for j := i; j > 0; j-- {
			if less(m[j], m[j-1]) {
				m[j], m[j-1] = m[j-1], m[j]
			} else {
				break
			}
		}
	}
}

func less(a, b Match) bool {
	if a.Tier != b.Tier {
		return a.Tier < b.Tier
	}
	if a.Frecency != b.Frecency {
		return a.Frecency > b.Frecency
	}
	if len(a.Item.Title) != len(b.Item.Title) {
		return len(a.Item.Title) < len(b.Item.Title)
	}
	return strings.ToLower(a.Item.Title) < strings.ToLower(b.Item.Title)
}

// tier returns 0..3 for the match tier, or -1 if no tier matches.
func tier(query string, item backend.Item) int {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return 0
	}
	title := strings.ToLower(item.Title)
	if strings.HasPrefix(title, q) {
		return 0
	}
	for _, part := range splitWords(title) {
		if len(part) > 0 && strings.HasPrefix(part, q) {
			return 1
		}
	}
	if strings.Contains(title, q) {
		return 2
	}
	if strings.Contains(strings.ToLower(item.Subtitle), q) {
		return 3
	}
	return -1
}

// splitWords splits on spaces, dashes, underscores, and dots.
func splitWords(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool {
		return r == ' ' || r == '-' || r == '_' || r == '.'
	})
}

// FrecencyScore replicates the QML frecencyScore.
func FrecencyScore(entry *Entry, nowMs int64, buckets []Bucket, countCap int) float64 {
	if entry == nil || entry.LastUsed == 0 {
		return 0
	}
	age := nowMs - entry.LastUsed
	if age < 0 {
		return 0
	}

	weight := 0
	for _, b := range buckets {
		if age <= b.MaxAgeMs {
			weight = b.Weight
			break
		}
	}
	if weight == 0 {
		return 0
	}
	top := buckets[0].Weight
	cap := countCap
	if cap == 0 {
		cap = 10
	}
	count := entry.Count
	if count < 1 {
		count = 1
	}
	countFactor := mathMin(1, mathLog1p(float64(count))/mathLog1p(float64(cap)))
	score := (float64(weight) / float64(top)) * countFactor
	if score < 0 {
		return 0
	}
	if score > 1 {
		return 1
	}
	return score
}

func mathMin(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func mathLog1p(x float64) float64 {
	return math.Log1p(x)
}
