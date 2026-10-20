package ksl

import (
	"strings"
	"testing"
)

// tokDump renders a token compactly for table tests.
func tokDump(toks []token) []string {
	out := make([]string, len(toks))
	for i, t := range toks {
		switch t.kind {
		case tScalar:
			out[i] = t.text + "/" + t.unit
		default:
			out[i] = t.text
		}
	}
	return out
}

func lexAll(t *testing.T, src string) []token {
	t.Helper()
	toks, errs := lex("test.ksl", []byte(src))
	if len(errs) > 0 {
		t.Fatalf("lex() errors: %v", errs)
	}
	return toks
}

func TestLexTokens(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []string
	}{
		{"ident and prop", "gap: 24", []string{"gap", ":", "24/", "tEOF"}},
		{"units attach", "rotate: rand(-4, 4) * 1deg, y: 40%", []string{
			"rotate", ":", "rand", "(", "-", "4/", ",", "4/", ")", "*", "1deg/deg", ",", "y", ":", "40%/%", "tEOF"}},
		{"ref and member", "$results.len", []string{"results", ".", "len", "tEOF"}},
		{"ref with dash", "$accent-alt", []string{"accent-alt", "tEOF"}},
		{"hex colour", "#FF2E63", []string{"FF2E63", "tEOF"}},
		{"id marker", "Box#card { }", []string{"Box", "card", "{", "}", "tEOF"}},
		{"string escapes", `"a\"b\n"`, []string{"a\"b\n", "tEOF"}},
		{"two-char ops", "a == b && c <= d", []string{"a", "==", "b", "&&", "c", "<=", "d", "tEOF"}},
		{"at and percent punct", "@keyframes", []string{"@", "keyframes", "tEOF"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			toks := lexAll(t, tt.src)
			got := tokDump(toks)
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for i := range tt.want {
				if tt.want[i] == "tEOF" {
					if toks[i].kind != tEOF {
						t.Errorf("token %d = %q, want EOF", i, got[i])
					}
					continue
				}
				if got[i] != tt.want[i] {
					t.Errorf("token %d = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestLexNewlinesAndComments(t *testing.T) {
	toks := lexAll(t, "a: 1 // note\nb: 2")
	kinds := []tokenKind{tIdent, tPunct, tScalar, tNewline, tIdent, tPunct, tScalar, tEOF}
	for i, want := range kinds {
		if toks[i].kind != want {
			t.Errorf("token %d kind = %v, want %v", i, toks[i].kind, want)
		}
	}
	if toks[4].pos.Line != 2 {
		t.Errorf("token after comment starts on line %d, want 2", toks[4].pos.Line)
	}
}

func TestLexErrors(t *testing.T) {
	tests := []struct{ name, src, msg string }{
		{"stray percent", "y: 40 %", "stray '%'"},
		{"bad suffix", "y: 12qubits", "invalid number suffix"},
		{"unterminated string", `x: "oops`, "unterminated string"},
		{"unknown char", "x: 1 & 2", "unexpected character"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, errs := lex("test.ksl", []byte(tt.src))
			if len(errs) == 0 || !strings.Contains(errs[0].Msg, tt.msg) {
				t.Errorf("lex() errors = %v, want message containing %q", errs, tt.msg)
			}
		})
	}
}
