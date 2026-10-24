//go:build !darwin && !windows

package display

import "testing"

func TestParseSize(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantW     int
		wantH     int
		wantFound bool
	}{
		// Valid cases
		{name: "simple resolution", input: "1920x1080", wantW: 1920, wantH: 1080, wantFound: true},
		{name: "with refresh rate", input: "  3840x2160@60Hz ", wantW: 3840, wantH: 2160, wantFound: true},
		{name: "with mode prefix and px", input: "modes: 2560x1440 px, 144Hz", wantW: 2560, wantH: 1440, wantFound: true},

		// Invalid cases
		{name: "not a resolution", input: "notaresolution", wantFound: false},
		{name: "missing height", input: "600x", wantFound: false},
		{name: "missing width", input: "x1080", wantFound: false},
		{name: "negative dimension", input: "-100x200", wantFound: false},
		{name: "zero dimension", input: "0x0", wantFound: false},
		{name: "empty string", input: "", wantFound: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, h, ok := parseSize(tt.input)
			if ok != tt.wantFound {
				t.Fatalf("parseSize(%q) ok = %v, want %v", tt.input, ok, tt.wantFound)
			}
			if ok {
				if w != tt.wantW || h != tt.wantH {
					t.Errorf("parseSize(%q) = (%d, %d), want (%d, %d)", tt.input, w, h, tt.wantW, tt.wantH)
				}
			}
		})
	}
}

func TestPhysicalSizeFiltering(t *testing.T) {
	tests := []struct {
		name       string
		line       string
		isPhysical bool
		wantW      int
		wantH      int
	}{
		{
			name:       "physical size label with mm",
			line:       "Physical size: 597x336 mm",
			isPhysical: true,
		},
		{
			name:       "lowercase physical keyword",
			line:       "  physical: 1920x1080",
			isPhysical: true,
		},
		{
			name:       "ending with mm",
			line:       "dimensions 597x336 mm",
			isPhysical: true,
		},
		{
			name:       "ending with mm and trailing whitespace",
			line:       "597x336 mm  ",
			isPhysical: true,
		},
		{
			name:       "normal resolution line",
			line:       "1920x1080",
			isPhysical: false,
			wantW:      1920,
			wantH:      1080,
		},
		{
			name:       "mode with refresh rate",
			line:       "3840x2160@60Hz",
			isPhysical: false,
			wantW:      3840,
			wantH:      2160,
		},
		{
			name:       "mode line",
			line:       "modes: 2560x1440 px, 144Hz",
			isPhysical: false,
			wantW:      2560,
			wantH:      1440,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPhysicalLine(tt.line)
			if got != tt.isPhysical {
				t.Errorf("isPhysicalLine(%q) = %v, want %v", tt.line, got, tt.isPhysical)
			}

			w, h, ok := parseOutputLine(tt.line)
			if tt.isPhysical {
				if ok {
					t.Errorf("parseOutputLine(%q) returned (%d, %d, true), expected physical line to be skipped", tt.line, w, h)
				}
			} else {
				if !ok {
					t.Errorf("parseOutputLine(%q) returned false, expected it to parse", tt.line)
				} else if w != tt.wantW || h != tt.wantH {
					t.Errorf("parseOutputLine(%q) = (%d, %d), want (%d, %d)", tt.line, w, h, tt.wantW, tt.wantH)
				}
			}
		})
	}
}
