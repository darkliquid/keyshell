package calc

import (
	"math"
	"testing"
)

// TestEvalPortsFixtures mirrors the evaluation cases in the QML test suite.
func TestEvalPortsFixtures(t *testing.T) {
	tests := []struct {
		expr string
		want string
		ok   bool
	}{
		{"2+3*4", "14", true},
		{"(2+3)*4", "20", true},
		{"-5+2", "-3", true},
		{"3*-2", "-6", true},
		{"2^10", "1,024", true},
		{"2^3^2", "512", true}, // right-associative
		{"10%3", "1", true},
		{" 2 + 2 ", "4", true},
		{"sqrt(16)", "4", true},
		{"max(1,5,3)", "5", true},
		{"min(4,2)", "2", true},
		{"abs(-3)", "3", true},
		{"round(2.6)", "3", true},
		{"floor(2.9)", "2", true},
		{"ceil(2.1)", "3", true},
		{"pow(2,8)", "256", true},
		{"pi", formatValue(math.Pi), true},
		{"e", formatValue(math.E), true},
		{"1/0", "", false},
		{"0/0", "", false},
		{"2+", "", false},
		{"((", "", false},
		{"", "", false},
		{"abc", "", false},
		{"2+*3", "", false},
		{"sqrt()", "", false},
		{"nope(1)", "", false},
	}
	for _, tt := range tests {
		got, ok := Eval(tt.expr)
		if ok != tt.ok {
			t.Errorf("Eval(%q) ok = %v, want %v", tt.expr, ok, tt.ok)
			continue
		}
		if tt.ok && got != tt.want {
			t.Errorf("Eval(%q) = %q, want %q", tt.expr, got, tt.want)
		}
	}
}

// TestFormatPortsFixtures mirrors the formatting cases from the QML suite.
func TestFormatPortsFixtures(t *testing.T) {
	tests := []struct {
		value float64
		dec   int
		want  string
	}{
		{14, 10, "14"},
		{0.1 + 0.2, 10, "0.3"},
		{1234567, 10, "1,234,567"},
		{-42, 10, "-42"},
		{1.0 / 3.0, 4, "0.3333"},
		{1e21, 10, "1e+21"},
	}
	for _, tt := range tests {
		if got := format(tt.value, tt.dec); got != tt.want {
			t.Errorf("format(%v,%d) = %q, want %q", tt.value, tt.dec, got, tt.want)
		}
	}
}

// formatValue formats a number using the package's formatter, for constants.
func formatValue(v float64) string {
	return format(v, 10)
}
