//go:build !race

package render

func isRaceEnabled() bool {
	return false
}
