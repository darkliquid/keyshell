# Generic Card Composition Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the ported per-delegate card bodies with one card the host composes from typed item fields, budgeted to two accessory widgets with an explicit override and an overflow hint.

**Architecture:** A new pure package `internal/card` turns a `backend.Item` into a `Plan` (layout, ordered parts, overflow count) with no theme, no measurement and no drawing. `internal/render` draws that plan inside the existing sheared card quad, measuring text and eliding to fit. New optional item fields carry status, artwork, prose and hints over both the in-process and plugin seams.

**Tech Stack:** Go 1.27 (mise), `github.com/gogpu/gg` (existing), `github.com/gogpu/gg/text` (existing), line-delimited JSON plugin protocol v1.

**Spec:** `docs/superpowers/specs/2026-10-20-card-composition-design.md`

## Global Constraints

- `CGO_ENABLED=0`; every Go command runs as `mise exec -- go …`, and the full gate is `mise run check` (gofmt -l, go vet, go test) from the repo root.
- No new module dependencies.
- The plugin protocol stays at version 1; every new field is optional and decodes to a zero value when absent.
- Composition lives only in `internal/card`. The renderer measures and draws; it never chooses parts.
- `card.AccessoryRows` is hard-coded at 2. Do not make it configurable.
- `text` controls never reach a card. `Display == "expanded"` always excludes a control from the card, even when pinned.
- No em dashes in source, comments, strings or docs.
- Card visuals keep the existing identity: sheared quad, 3px border, accent fill when selected, cyan offset shadow when not.
- No new comments explaining what code does; comment only non-obvious why (matches the repo's existing style).

---

### Task 1: Extend the item and control schema

**Files:**
- Modify: `internal/backend/backend.go`
- Modify: `internal/plugin/proto/proto.go`
- Modify: `internal/plugin/sdk/sdk.go`
- Modify: `internal/plugin/host.go`
- Test: `internal/plugin/proto/proto_test.go`, `internal/plugin/host_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces: `backend.Tone` (constants `ToneAccent`, `ToneNeutral`, `ToneInfo`, `TonePositive`, `ToneWarn`, `ToneAlert`, `ToneAttention`), `backend.Display` (`DisplayAuto`, `DisplayCard`, `DisplayExpanded`), `backend.PartKind` (`PartBadge`, `PartStatus`, `PartVisual`, `PartTitle`, `PartSubtitle`, `PartDetail`, `PartMeter`, `PartTimeline`, `PartNote`, `PartHold`, `PartFooter`), `backend.CardPart{Kind, Control, Text, Tone}`, `backend.ControlKind` value `ControlProgress`, and new fields `Item.Icon`, `Item.Image`, `Item.Preview`, `Item.Status`, `Item.StatusTone`, `Item.Detail`, `Item.Hint`, `Item.Card`, `Control.Display`, `Control.Primary`.

- [ ] **Step 1: Write the failing wire test**

Add to `internal/plugin/proto/proto_test.go`:

```go
// TestItemCardFieldsUsePinnedWireNames locks the JSON names of the card fields
// so a rename cannot silently break a plugin written against them.
func TestItemCardFieldsUsePinnedWireNames(t *testing.T) {
	msg := Message{Item: &Item{
		ID: "i", Title: "Speakers", Status: "DEFAULT", StatusTone: "positive",
		Icon: "audio", Image: "file:///art.png", Preview: "handle:1",
		Detail: "body", Hint: "ENTER COPY",
		Controls: []ControlSpec{{ID: "volume", Kind: "range", Display: "card", Primary: true}},
		Card:     []CardPartSpec{{Kind: "meter", Control: "volume", Text: "VOL", Tone: "info"}},
	}}
	b, err := Encode(msg)
	if err != nil {
		t.Fatalf("Encode = err %v", err)
	}
	var raw struct {
		Item map[string]json.RawMessage `json:"item"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("Unmarshal = err %v", err)
	}
	for _, key := range []string{"status", "status_tone", "icon", "image", "preview", "detail", "hint", "card"} {
		if _, ok := raw.Item[key]; !ok {
			t.Errorf("item JSON missing key %q", key)
		}
	}
	got, err := Decode(b)
	if err != nil {
		t.Fatalf("Decode = err %v", err)
	}
	if got.Item == nil || got.Item.Controls[0].Display != "card" || !got.Item.Controls[0].Primary {
		t.Errorf("control hints lost: %+v", got.Item)
	}
	if got.Item.Card[0].Kind != "meter" || got.Item.Card[0].Control != "volume" {
		t.Errorf("card override lost: %+v", got.Item.Card)
	}
}
```

- [ ] **Step 2: Run it to confirm it fails**

Run: `mise exec -- go test ./internal/plugin/proto/ -run TestItemCardFieldsUsePinnedWireNames -v`
Expected: FAIL, compile errors `undefined: Item.Status` / `Item.StatusTone` / `CardPartSpec`.

- [ ] **Step 3: Add the backend types and fields**

In `internal/backend/backend.go`, after `ControlKind`:

```go
const ControlProgress ControlKind = "progress"

// Tone is the semantic colour of a chip. The theme maps each tone to colours.
type Tone string

const (
	ToneAccent    Tone = "accent"
	ToneNeutral   Tone = "neutral"
	ToneInfo      Tone = "info"
	TonePositive  Tone = "positive"
	ToneWarn      Tone = "warn"
	ToneAlert     Tone = "alert"
	ToneAttention Tone = "attention"
)

// Display steers card composition for one control.
type Display string

const (
	DisplayAuto     Display = ""
	DisplayCard     Display = "card"
	DisplayExpanded Display = "expanded"
)

// PartKind names one display widget a card can use.
type PartKind string

const (
	PartBadge    PartKind = "badge"
	PartStatus   PartKind = "status"
	PartVisual   PartKind = "visual"
	PartTitle    PartKind = "title"
	PartSubtitle PartKind = "subtitle"
	PartDetail   PartKind = "detail"
	PartMeter    PartKind = "meter"
	PartTimeline PartKind = "timeline"
	PartNote     PartKind = "note"
	PartHold     PartKind = "hold"
	PartFooter   PartKind = "footer"
)

// CardPart is one explicit card part. A CardPart list on an item overrides the
// derived body; the header and footer still come from the derive rules.
type CardPart struct {
	Kind    PartKind
	Control string
	Text    string
	Tone    Tone
}
```

Add to `Control` (after `Options`):

```go
	Display Display
	Primary bool
```

Add to `Item` (after `Expandable`):

```go
	Icon       string
	Image      string
	Preview    string
	Status     string
	StatusTone Tone
	Detail     string
	Hint       string
	Card       []CardPart
```

- [ ] **Step 4: Mirror the fields in the protocol**

In `internal/plugin/proto/proto.go`, add to `Item`:

```go
	Icon       string          `json:"icon,omitempty"`
	Image      string          `json:"image,omitempty"`
	Preview    string          `json:"preview,omitempty"`
	Status     string          `json:"status,omitempty"`
	StatusTone string          `json:"status_tone,omitempty"`
	Detail     string          `json:"detail,omitempty"`
	Hint       string          `json:"hint,omitempty"`
	Card       []CardPartSpec  `json:"card,omitempty"`
```

Add to `ControlSpec`:

```go
	Display string `json:"display,omitempty"`
	Primary bool   `json:"primary,omitempty"`
```

Add the new spec type after `ControlSpec`:

```go
// CardPartSpec is one explicit card part in an override list.
type CardPartSpec struct {
	Kind    string `json:"kind"`
	Control string `json:"control,omitempty"`
	Text    string `json:"text,omitempty"`
	Tone    string `json:"tone,omitempty"`
}
```

- [ ] **Step 5: Run the wire test**

Run: `mise exec -- go test ./internal/plugin/proto/ -v`
Expected: PASS.

- [ ] **Step 6: Write the failing conversion test**

Add to `internal/plugin/host_test.go`:

```go
// TestFromItemCarriesCardFields checks the host copies every card field off the
// wire, including the control hints and the override list.
func TestFromItemCarriesCardFields(t *testing.T) {
	got := fromItem(proto.Item{
		ID: "i", Title: "Speakers", Status: "DEFAULT", StatusTone: "positive",
		Icon: "audio", Image: "img", Preview: "preview", Detail: "body", Hint: "ENTER",
		Controls: []proto.ControlSpec{{ID: "volume", Kind: "progress", Display: "card", Primary: true}},
		Card:     []proto.CardPartSpec{{Kind: "meter", Control: "volume", Text: "VOL", Tone: "info"}},
	})
	if got.Status != "DEFAULT" || got.StatusTone != backend.TonePositive {
		t.Errorf("status = %q/%q", got.Status, got.StatusTone)
	}
	if got.Icon != "audio" || got.Image != "img" || got.Preview != "preview" {
		t.Errorf("visual fields = %q/%q/%q", got.Icon, got.Image, got.Preview)
	}
	if got.Detail != "body" || got.Hint != "ENTER" {
		t.Errorf("text fields = %q/%q", got.Detail, got.Hint)
	}
	if len(got.Controls) != 1 || got.Controls[0].Display != backend.DisplayCard || !got.Controls[0].Primary {
		t.Errorf("controls = %+v", got.Controls)
	}
	if len(got.Card) != 1 || got.Card[0].Kind != backend.PartMeter || got.Card[0].Control != "volume" {
		t.Errorf("card = %+v", got.Card)
	}
}
```

Imports: `internal/plugin/proto` must be added to the test file.

- [ ] **Step 7: Run it to confirm it fails**

Run: `mise exec -- go test ./internal/plugin/ -run TestFromItemCarriesCardFields -v`
Expected: FAIL, compile errors on the missing fields.

- [ ] **Step 8: Update both conversion directions**

In `internal/plugin/host.go` `fromItem`, extend the returned `backend.Item`:

```go
		Icon: p.Icon, Image: p.Image, Preview: p.Preview,
		Status: p.Status, StatusTone: backend.Tone(p.StatusTone),
		Detail: p.Detail, Hint: p.Hint, Card: cardParts(p.Card),
```

and the control loop:

```go
		controls = append(controls, backend.Control{
			ID: c.ID, Kind: backend.ControlKind(c.Kind), Label: c.Label,
			Value: c.Value, Min: c.Min, Max: c.Max, Step: c.Step,
			Bool: c.Bool, Text: c.Text, Options: c.Options,
			Display: backend.Display(c.Display), Primary: c.Primary,
		})
```

Add beside `fromItem`:

```go
func cardParts(specs []proto.CardPartSpec) []backend.CardPart {
	if len(specs) == 0 {
		return nil
	}
	out := make([]backend.CardPart, 0, len(specs))
	for _, s := range specs {
		out = append(out, backend.CardPart{
			Kind: backend.PartKind(s.Kind), Control: s.Control,
			Text: s.Text, Tone: backend.Tone(s.Tone),
		})
	}
	return out
}
```

In `internal/plugin/sdk/sdk.go`, mirror the fields on `ItemShape`, `ControlSpecShape`, add `CardPartShape{Kind, Control, Text, Tone string}`, add `Card []CardPartShape` to `ItemShape`, `Display string` and `Primary bool` to `ControlSpecShape`, and extend `toItem`:

```go
		Icon: it.Icon, Image: it.Image, Preview: it.Preview,
		Status: it.Status, StatusTone: it.StatusTone,
		Detail: it.Detail, Hint: it.Hint, Card: toCardParts(it.Card),
```

with:

```go
func toCardParts(parts []CardPartShape) []proto.CardPartSpec {
	if len(parts) == 0 {
		return nil
	}
	out := make([]proto.CardPartSpec, 0, len(parts))
	for _, p := range parts {
		out = append(out, proto.CardPartSpec{Kind: p.Kind, Control: p.Control, Text: p.Text, Tone: p.Tone})
	}
	return out
}
```

and the control literal gains `Display: c.Display, Primary: c.Primary`.

- [ ] **Step 9: Run the full gate**

Run: `mise run check`
Expected: PASS across all packages.

- [ ] **Step 10: Commit**

```bash
git add internal/backend/backend.go internal/plugin/proto/proto.go internal/plugin/sdk/sdk.go internal/plugin/host.go internal/plugin/proto/proto_test.go internal/plugin/host_test.go
git commit -m "feat: carry card status, artwork and display hints on items"
```

---

### Task 2: Compose the card's header, visual, text and footer

**Files:**
- Create: `internal/card/card.go`
- Create: `internal/card/card_test.go`

**Interfaces:**
- Consumes: `backend.Item`, `backend.PartKind`, `backend.Tone`.
- Produces: `card.Plan{Layout, Badge, Status, Visual, Title, Subtitle, Detail, Accessories []Part, Hold, Footer Part, Overflow int}`, `card.Part{Kind, Control int, Text, Tone, Style, Source string, Ratio float64, Ticks int, Label, Value string}`, `card.Compose(backend.Item) Plan`, `card.Layout` constants `LayoutHero`/`LayoutRow`/`LayoutBody`, `card.Style` constants `StyleHero`/`StyleValue`/`StyleLine`/`StyleWrap`. `Compose` sets `Accessories` to nil and `Overflow` to 0 until Task 3.

- [ ] **Step 1: Write the failing tests**

Create `internal/card/card_test.go`:

```go
package card

import (
	"testing"

	"github.com/darkliquid/keyshell/internal/backend"
)

// TestComposeHeroForALaunchItem checks the app card: a visual and a title and
// nothing else, with the footer suppressed by the hero layout.
func TestComposeHeroForALaunchItem(t *testing.T) {
	p := Compose(backend.Item{
		Title:   "Ghostty",
		Icon:    "utilities-terminal",
		Actions: []backend.Action{{Type: "exec", Value: "ghostty"}},
	})
	if p.Layout != LayoutHero {
		t.Fatalf("layout = %q, want %q", p.Layout, LayoutHero)
	}
	if p.Title.Style != StyleHero {
		t.Errorf("title style = %q, want %q", p.Title.Style, StyleHero)
	}
	if p.Footer.Kind != "" {
		t.Errorf("footer = %+v, want none in the hero layout", p.Footer)
	}
	if p.Visual.Source != "icon" {
		t.Errorf("visual source = %q, want icon", p.Visual.Source)
	}
}

// TestComposeRowForASpeaker checks the header chips, visual and text column.
func TestComposeRowForASpeaker(t *testing.T) {
	p := Compose(backend.Item{
		Title: "Speakers", Subtitle: "alsa_output.hdmi", Icon: "audio",
		Badge: "OUTPUT", Status: "DEFAULT", StatusTone: backend.TonePositive,
		Controls: []backend.Control{{ID: "volume", Kind: backend.ControlRange, Value: 0.35, Min: 0, Max: 1, Step: 0.01}},
	})
	if p.Layout != LayoutRow {
		t.Fatalf("layout = %q, want %q", p.Layout, LayoutRow)
	}
	if p.Badge.Kind != backend.PartBadge || p.Badge.Text != "OUTPUT" || p.Badge.Tone != backend.ToneAccent {
		t.Errorf("badge = %+v", p.Badge)
	}
	if p.Status.Kind != backend.PartStatus || p.Status.Tone != backend.TonePositive {
		t.Errorf("status = %+v", p.Status)
	}
	if p.Subtitle.Kind != backend.PartSubtitle || p.Subtitle.Text != "alsa_output.hdmi" {
		t.Errorf("subtitle = %+v", p.Subtitle)
	}
	if p.Title.Style != StyleLine {
		t.Errorf("title style = %q, want %q", p.Title.Style, StyleLine)
	}
	if p.Footer.Kind != "" {
		t.Errorf("footer = %+v, want none without actions", p.Footer)
	}
}

// TestComposeBodyForACalculator checks the iconless valuation card.
func TestComposeBodyForACalculator(t *testing.T) {
	p := Compose(backend.Item{
		Title: "14", Subtitle: "2+3*4",
		Actions: []backend.Action{{Type: "copy", Value: "14"}},
	})
	if p.Layout != LayoutBody {
		t.Fatalf("layout = %q, want %q", p.Layout, LayoutBody)
	}
	if p.Title.Style != StyleValue {
		t.Errorf("title style = %q, want %q", p.Title.Style, StyleValue)
	}
	if p.Footer.Text != "ENTER COPY" {
		t.Errorf("footer = %q, want %q", p.Footer.Text, "ENTER COPY")
	}
}

// TestComposeWrapsPlainResultText checks that prose-only results get the
// wrapped title treatment rather than a single elided line.
func TestComposeWrapsPlainResultText(t *testing.T) {
	p := Compose(backend.Item{Title: "some long copied paragraph", Icon: "edit-copy"})
	if p.Layout != LayoutRow {
		t.Fatalf("layout = %q, want %q", p.Layout, LayoutRow)
	}
	if p.Title.Style != StyleWrap {
		t.Errorf("title style = %q, want %q", p.Title.Style, StyleWrap)
	}
}

// TestComposeUsesDetailOnlyWithoutAVisual keeps the prose body from competing
// with a leading visual for the same space.
func TestComposeUsesDetailOnlyWithoutAVisual(t *testing.T) {
	withVisual := Compose(backend.Item{Title: "T", Icon: "audio", Detail: "body"})
	if withVisual.Detail.Kind != "" {
		t.Errorf("detail = %+v, want none beside a visual", withVisual.Detail)
	}
	withoutVisual := Compose(backend.Item{Title: "T", Subtitle: "S", Detail: "body"})
	if withoutVisual.Detail.Kind != backend.PartDetail || withoutVisual.Detail.Text != "body" {
		t.Errorf("detail = %+v, want the body text", withoutVisual.Detail)
	}
}

// TestComposePrefersImageThenPreviewThenIcon pins the visual precedence.
func TestComposePrefersImageThenPreviewThenIcon(t *testing.T) {
	for _, tc := range []struct {
		name string
		item backend.Item
		want string
	}{
		{"image", backend.Item{Title: "T", Image: "art", Preview: "p", Icon: "i", Glyph: "g"}, "image"},
		{"preview", backend.Item{Title: "T", Preview: "p", Icon: "i", Glyph: "g"}, "preview"},
		{"icon", backend.Item{Title: "T", Icon: "i", Glyph: "g"}, "icon"},
		{"glyph", backend.Item{Title: "T", Glyph: "g"}, "glyph"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Compose(tc.item).Visual.Source; got != tc.want {
				t.Errorf("source = %q, want %q", got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run the tests to confirm they fail**

Run: `mise exec -- go test ./internal/card/ -v`
Expected: FAIL, `no Go files` or undefined symbols.

- [ ] **Step 3: Write the composition**

Create `internal/card/card.go`:

```go
// Package card composes the generic result card from item data. It is pure:
// no theme, no measurement and no drawing, so the budget rules are testable
// without a graphics context.
package card

import "github.com/darkliquid/keyshell/internal/backend"

// Layout is the card arrangement derived from the item's parts.
type Layout string

const (
	LayoutHero Layout = "hero"
	LayoutRow  Layout = "row"
	LayoutBody Layout = "body"
)

// Style selects the title treatment.
type Style string

const (
	StyleHero  Style = "hero"
	StyleValue Style = "value"
	StyleLine  Style = "line"
	StyleWrap  Style = "wrap"
)

// Part is one resolved card widget. A zero Kind means the part is absent.
type Part struct {
	Kind    backend.PartKind
	Control int
	Text    string
	Tone    backend.Tone
	Style   Style
	Source  string
	Ratio   float64
	Ticks   int
	Label   string
	Value   string
}

// Plan is the composed card: what to draw, in which zones.
type Plan struct {
	Layout      Layout
	Badge       Part
	Status      Part
	Visual      Part
	Title       Part
	Subtitle    Part
	Detail      Part
	Accessories []Part
	Hold        Part
	Footer      Part
	Overflow    int
}

// Compose derives a card from item data.
func Compose(item backend.Item) Plan {
	p := Plan{
		Badge:    chip(backend.PartBadge, item.Badge, backend.ToneAccent),
		Status:   chip(backend.PartStatus, item.Status, item.StatusTone),
		Visual:   visual(item),
		Title:    Part{Kind: backend.PartTitle, Control: -1, Text: item.Title},
		Subtitle: chip(backend.PartSubtitle, item.Subtitle, ""),
		Footer:   footer(item),
	}
	if item.Detail != "" && p.Visual.Kind == "" {
		p.Detail = Part{Kind: backend.PartDetail, Control: -1, Text: item.Detail}
	}
	p.Layout = layoutFor(item, p)
	p.Title.Style = titleStyle(item, p)
	if p.Layout == LayoutHero {
		p.Footer = Part{}
	}
	return p
}

// chip builds a text chip part. An empty string means no part.
func chip(kind backend.PartKind, text string, tone backend.Tone) Part {
	if text == "" {
		return Part{}
	}
	if tone == "" {
		tone = backend.ToneNeutral
	}
	return Part{Kind: kind, Control: -1, Text: text, Tone: tone}
}

// visual picks the leading artwork, most specific source first.
func visual(item backend.Item) Part {
	switch {
	case item.Image != "":
		return Part{Kind: backend.PartVisual, Control: -1, Text: item.Image, Source: "image"}
	case item.Preview != "":
		return Part{Kind: backend.PartVisual, Control: -1, Text: item.Preview, Source: "preview"}
	case item.Icon != "":
		return Part{Kind: backend.PartVisual, Control: -1, Text: item.Icon, Source: "icon"}
	case item.Glyph != "":
		return Part{Kind: backend.PartVisual, Control: -1, Text: item.Glyph, Source: "glyph"}
	}
	return Part{}
}

// layoutFor chooses the arrangement from what the card actually has.
func layoutFor(item backend.Item, p Plan) Layout {
	if p.Visual.Kind != "" && p.Badge.Kind == "" && p.Status.Kind == "" &&
		p.Subtitle.Kind == "" && p.Detail.Kind == "" && len(p.Accessories) == 0 &&
		len(item.Controls) == 0 && !item.HasChildren {
		return LayoutHero
	}
	if p.Visual.Kind == "" && (p.Subtitle.Kind != "" || p.Detail.Kind != "") {
		return LayoutBody
	}
	return LayoutRow
}

// titleStyle picks the title treatment for the chosen layout.
func titleStyle(item backend.Item, p Plan) Style {
	switch p.Layout {
	case LayoutHero:
		return StyleHero
	case LayoutBody:
		return StyleValue
	}
	if p.Badge.Kind == "" && p.Status.Kind == "" && len(item.Controls) == 0 {
		return StyleWrap
	}
	return StyleLine
}

// footer is the hint line: the item's own hint, else one derived from the
// primary action.
func footer(item backend.Item) Part {
	if item.Hint != "" {
		return Part{Kind: backend.PartFooter, Control: -1, Text: item.Hint}
	}
	if len(item.Actions) == 0 {
		return Part{}
	}
	return Part{Kind: backend.PartFooter, Control: -1, Text: defaultHint(item.Actions[0], item.Destructive)}
}

func defaultHint(a backend.Action, destructive bool) string {
	if destructive {
		return "HOLD ENTER"
	}
	switch a.Type {
	case "copy":
		return "ENTER COPY"
	case "exec", "exec-terminal":
		return "ENTER RUN"
	case "open-url":
		return "ENTER OPEN"
	case "focus-window":
		return "ENTER FOCUS"
	case "plugin":
		return "ENTER ACTIVATE"
	case "set-query":
		return "ENTER SEARCH"
	}
	return "ENTER SELECT"
}
```

- [ ] **Step 4: Run the tests**

Run: `mise exec -- go test ./internal/card/ -v`
Expected: PASS for all six tests.

- [ ] **Step 5: Commit**

```bash
git add internal/card/card.go internal/card/card_test.go
git commit -m "feat: compose card zones from item data"
```

---

### Task 3: Accessories, ranking and the overflow count

**Files:**
- Create: `internal/card/accessory.go`
- Test: `internal/card/accessory_test.go`
- Modify: `internal/card/card.go` (wire `accessories` into `Compose`)

**Interfaces:**
- Consumes: `Compose`, `Part`, `Plan` from Task 2.
- Produces: `card.AccessoryRows` (const 2), `accessories(backend.Item) ([]Part, int)`, `ratio(backend.Control) float64`, `ratioLike(backend.Control) bool`, `anomalous(backend.Control) bool`. `Part.Control` is the control's index in `item.Controls`.

- [ ] **Step 1: Write the failing tests**

Create `internal/card/accessory_test.go`:

```go
package card

import (
	"testing"

	"github.com/darkliquid/keyshell/internal/backend"
)

// TestAccessoriesShowVolumeAndMuteForASpeaker checks that a primary meter and a
// true toggle both fit in the two-row budget, ranked primary first.
func TestAccessoriesShowVolumeAndMuteForASpeaker(t *testing.T) {
	p := Compose(backend.Item{Title: "Speakers", Icon: "audio", Controls: []backend.Control{
		{ID: "volume", Kind: backend.ControlRange, Label: "Volume", Value: 0.35, Min: 0, Max: 1, Step: 0.01, Primary: true},
		{ID: "mute", Kind: backend.ControlToggle, Label: "Mute", Bool: true},
	}})
	if len(p.Accessories) != 2 {
		t.Fatalf("accessories = %+v, want 2", p.Accessories)
	}
	if p.Accessories[0].Kind != backend.PartMeter || p.Accessories[0].Control != 0 {
		t.Errorf("first accessory = %+v, want the volume meter", p.Accessories[0])
	}
	if p.Accessories[0].Ticks != 10 {
		t.Errorf("ticks = %d, want 10 for a ratio control", p.Accessories[0].Ticks)
	}
	if p.Accessories[1].Kind != backend.PartNote || p.Accessories[1].Control != 1 {
		t.Errorf("second accessory = %+v, want the mute note", p.Accessories[1])
	}
	if p.Overflow != 0 {
		t.Errorf("overflow = %d, want 0", p.Overflow)
	}
}

// TestAccessoriesDropAFalseToggle keeps silence for healthy state.
func TestAccessoriesDropAFalseToggle(t *testing.T) {
	p := Compose(backend.Item{Title: "Speakers", Controls: []backend.Control{
		{ID: "mute", Kind: backend.ControlToggle, Bool: false},
	}})
	if len(p.Accessories) != 0 || p.Overflow != 0 {
		t.Errorf("accessories = %+v overflow = %d, want none", p.Accessories, p.Overflow)
	}
}

// TestAccessoriesCapAtTwoAndCountTheRest checks the budget and the marker count.
func TestAccessoriesCapAtTwoAndCountTheRest(t *testing.T) {
	p := Compose(backend.Item{Title: "Song", Controls: []backend.Control{
		{ID: "position", Kind: backend.ControlProgress, Label: "1:23", Text: "4:56"},
		{ID: "volume", Kind: backend.ControlRange, Value: 0.5, Min: 0, Max: 1, Primary: true},
		{ID: "mute", Kind: backend.ControlToggle, Bool: true},
	}})
	if len(p.Accessories) != AccessoryRows {
		t.Fatalf("accessories = %+v, want %d", p.Accessories, AccessoryRows)
	}
	if p.Overflow != 1 {
		t.Errorf("overflow = %d, want 1", p.Overflow)
	}
}

// TestAccessoriesRankAnomalyAbovePlain checks the middle rank.
func TestAccessoriesRankAnomalyAbovePlain(t *testing.T) {
	p := Compose(backend.Item{Title: "Battery", Controls: []backend.Control{
		{ID: "level", Kind: backend.ControlRange, Value: 0.1, Min: 0, Max: 1},
		{ID: "profile", Kind: backend.ControlRange, Value: 0.9, Min: 0, Max: 1},
	}})
	if len(p.Accessories) != 2 {
		t.Fatalf("accessories = %+v, want 2", p.Accessories)
	}
	if p.Accessories[0].Control != 0 {
		t.Errorf("first accessory control = %d, want the anomalous level (0)", p.Accessories[0].Control)
	}
}

// TestAccessoriesHonourPinsAndExpanded checks both explicit hints.
func TestAccessoriesHonourPinsAndExpanded(t *testing.T) {
	pinned := Compose(backend.Item{Title: "T", Controls: []backend.Control{
		{ID: "a", Kind: backend.ControlRange, Value: 0.9, Min: 0, Max: 1},
		{ID: "b", Kind: backend.ControlToggle, Bool: true, Display: backend.DisplayCard},
		{ID: "c", Kind: backend.ControlRange, Value: 0.9, Min: 0, Max: 1},
	}})
	if len(pinned.Accessories) == 0 || pinned.Accessories[0].Control != 1 {
		t.Errorf("accessories = %+v, want the pinned control first", pinned.Accessories)
	}

	hidden := Compose(backend.Item{Title: "T", Controls: []backend.Control{
		{ID: "a", Kind: backend.ControlToggle, Bool: true, Display: backend.DisplayExpanded, Primary: true},
	}})
	if len(hidden.Accessories) != 0 || hidden.Overflow != 0 {
		t.Errorf("accessories = %+v overflow = %d, want none", hidden.Accessories, hidden.Overflow)
	}
}

// TestAccessoriesNeverRenderTextOrUnpinnedChoice checks the eligibility filter.
func TestAccessoriesNeverRenderTextOrUnpinnedChoice(t *testing.T) {
	p := Compose(backend.Item{Title: "Wi-Fi", Controls: []backend.Control{
		{ID: "passphrase", Kind: backend.ControlText, Display: backend.DisplayCard, Primary: true},
		{ID: "wifi", Kind: backend.ControlChoice, Options: []string{"a", "b"}},
	}})
	if len(p.Accessories) != 0 {
		t.Errorf("accessories = %+v, want none", p.Accessories)
	}
}

// TestAccessoriesUseABarForWideRanges checks the tick rule boundary.
func TestAccessoriesUseABarForWideRanges(t *testing.T) {
	p := Compose(backend.Item{Title: "T", Controls: []backend.Control{
		{ID: "battery", Kind: backend.ControlRange, Value: 65, Min: 0, Max: 100, Step: 1, Primary: true},
	}})
	if len(p.Accessories) != 1 {
		t.Fatalf("accessories = %+v, want 1", p.Accessories)
	}
	if p.Accessories[0].Ticks != 0 {
		t.Errorf("ticks = %d, want 0 (bar) for a 0..100 range", p.Accessories[0].Ticks)
	}
	if got := p.Accessories[0].Ratio; got < 0.64 || got > 0.66 {
		t.Errorf("ratio = %v, want 0.65", got)
	}
}
```

- [ ] **Step 2: Run them to confirm they fail**

Run: `mise exec -- go test ./internal/card/ -run TestAccessories -v`
Expected: FAIL, `Accessories` empty and `AccessoryRows` undefined.

- [ ] **Step 3: Write the accessory rules**

Create `internal/card/accessory.go`:

```go
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
```

- [ ] **Step 4: Wire it into Compose**

In `internal/card/card.go`, inside `Compose`, after the `Detail` block and before `p.Layout = layoutFor(...)`:

```go
	p.Accessories, p.Overflow = accessories(item)
```

- [ ] **Step 5: Run the package tests**

Run: `mise exec -- go test ./internal/card/ -v`
Expected: PASS for all thirteen tests in the package.

- [ ] **Step 6: Commit**

```bash
git add internal/card/accessory.go internal/card/accessory_test.go internal/card/card.go
git commit -m "feat: budget card accessories and mark the overflow"
```

---

### Task 4: The explicit override path

**Files:**
- Create: `internal/card/override.go`
- Test: `internal/card/override_test.go`
- Modify: `internal/card/card.go` (dispatch to the override)

**Interfaces:**
- Consumes: `Plan`, `Part`, `chip`, `footer`, `layoutFor`, `titleStyle`, `AccessoryRows`.
- Produces: `composeOverride(backend.Item) Plan`. `Compose` returns it whenever `len(item.Card) > 0`.

- [ ] **Step 1: Write the failing tests**

Create `internal/card/override_test.go`:

```go
package card

import (
	"testing"

	"github.com/darkliquid/keyshell/internal/backend"
)

// TestOverrideReplacesTheBody checks an explicit list drives the card and that
// the header and footer still come from the item.
func TestOverrideReplacesTheBody(t *testing.T) {
	p := Compose(backend.Item{
		Title: "Speakers", Badge: "OUTPUT", Hint: "ENTER SELECT",
		Controls: []backend.Control{
			{ID: "volume", Kind: backend.ControlRange, Value: 0.4, Min: 0, Max: 1},
		},
		Card: []backend.CardPart{
			{Kind: backend.PartTitle, Text: "SPEAKERS"},
			{Kind: backend.PartMeter, Control: "volume", Text: "VOL"},
		},
	})
	if p.Badge.Kind != backend.PartBadge || p.Badge.Text != "OUTPUT" {
		t.Errorf("badge = %+v, want the derived badge", p.Badge)
	}
	if p.Footer.Text != "ENTER SELECT" {
		t.Errorf("footer = %q, want the item hint", p.Footer.Text)
	}
	if p.Title.Text != "SPEAKERS" {
		t.Errorf("title = %q, want the override text", p.Title.Text)
	}
	if len(p.Accessories) != 1 || p.Accessories[0].Kind != backend.PartMeter {
		t.Fatalf("accessories = %+v, want the override meter", p.Accessories)
	}
	if p.Accessories[0].Control != 0 || p.Accessories[0].Ratio < 0.39 || p.Accessories[0].Ratio > 0.41 {
		t.Errorf("meter = %+v, want control 0 at ratio 0.4", p.Accessories[0])
	}
}

// TestOverrideSkipsUnknownControls keeps a stale id from blanking the card.
func TestOverrideSkipsUnknownControls(t *testing.T) {
	p := Compose(backend.Item{
		Title: "T",
		Card:  []backend.CardPart{{Kind: backend.PartMeter, Control: "gone"}},
	})
	if len(p.Accessories) != 0 {
		t.Errorf("accessories = %+v, want none", p.Accessories)
	}
	if p.Title.Kind != backend.PartTitle || p.Title.Text != "T" {
		t.Errorf("title = %+v, want the item title as the default", p.Title)
	}
}

// TestOverrideStillCapsAccessories checks the budget survives the override.
func TestOverrideStillCapsAccessories(t *testing.T) {
	p := Compose(backend.Item{
		Title: "T",
		Controls: []backend.Control{
			{ID: "a", Kind: backend.ControlRange, Value: 0.5, Min: 0, Max: 1},
			{ID: "b", Kind: backend.ControlRange, Value: 0.5, Min: 0, Max: 1},
			{ID: "c", Kind: backend.ControlRange, Value: 0.5, Min: 0, Max: 1},
		},
		Card: []backend.CardPart{
			{Kind: backend.PartMeter, Control: "a"},
			{Kind: backend.PartMeter, Control: "b"},
			{Kind: backend.PartMeter, Control: "c"},
		},
	})
	if len(p.Accessories) != AccessoryRows {
		t.Errorf("accessories = %+v, want %d", p.Accessories, AccessoryRows)
	}
	if p.Overflow != 1 {
		t.Errorf("overflow = %d, want 1", p.Overflow)
	}
}
```

- [ ] **Step 2: Run them to confirm they fail**

Run: `mise exec -- go test ./internal/card/ -run TestOverride -v`
Expected: FAIL, the override list is ignored.

- [ ] **Step 3: Write the override**

Create `internal/card/override.go`:

```go
package card

import "github.com/darkliquid/keyshell/internal/backend"

// composeOverride renders an explicit part list. It replaces the derived body
// wholesale; the header and footer still follow the derive rules, because every
// card has them.
func composeOverride(item backend.Item) Plan {
	p := Plan{
		Badge:  chip(backend.PartBadge, item.Badge, backend.ToneAccent),
		Status: chip(backend.PartStatus, item.Status, item.StatusTone),
		Footer: footer(item),
	}

	index := make(map[string]int, len(item.Controls))
	for i, c := range item.Controls {
		index[c.ID] = i
	}

	for _, cp := range item.Card {
		part := Part{Kind: cp.Kind, Control: -1, Text: cp.Text, Tone: cp.Tone}
		if cp.Control != "" {
			i, ok := index[cp.Control]
			if !ok {
				continue
			}
			c := item.Controls[i]
			part.Control = i
			part.Ratio = ratio(c)
			if part.Text == "" {
				part.Text = c.Text
			}
			if part.Label == "" {
				part.Label = c.Label
			}
			if part.Tone == "" {
				part.Tone = c.Tone()
			}
			if cp.Kind == backend.PartMeter && ratioLike(c) {
				part.Ticks = 10
			}
		}
		switch part.Kind {
		case backend.PartBadge:
			p.Badge = part
		case backend.PartStatus:
			p.Status = part
		case backend.PartVisual:
			p.Visual = part
		case backend.PartTitle:
			p.Title = part
		case backend.PartSubtitle:
			p.Subtitle = part
		case backend.PartDetail:
			p.Detail = part
		case backend.PartMeter, backend.PartTimeline, backend.PartNote:
			if len(p.Accessories) < AccessoryRows {
				p.Accessories = append(p.Accessories, part)
			} else {
				p.Overflow++
			}
		case backend.PartHold:
			p.Hold = part
		case backend.PartFooter:
			p.Footer = part
		}
	}
	if p.Title.Kind == "" {
		p.Title = Part{Kind: backend.PartTitle, Control: -1, Text: item.Title}
	}
	p.Layout = layoutFor(item, p)
	p.Title.Style = titleStyle(item, p)
	if p.Layout == LayoutHero {
		p.Footer = Part{}
	}
	return p
}
```

`c.Tone()` does not exist. Replace that line with a plain neutral default:

```go
			if part.Tone == "" {
				part.Tone = backend.ToneNeutral
			}
```

- [ ] **Step 4: Dispatch from Compose**

At the top of `Compose` in `internal/card/card.go`:

```go
	if len(item.Card) > 0 {
		return composeOverride(item)
	}
```

- [ ] **Step 5: Run the package tests**

Run: `mise exec -- go test ./internal/card/ -v`
Expected: PASS, sixteen tests.

- [ ] **Step 6: Commit**

```bash
git add internal/card/override.go internal/card/override_test.go internal/card/card.go
git commit -m "feat: let an item override its card composition"
```

---

### Task 5: Draw the plan's header, text and footer

**Files:**
- Create: `internal/render/card.go`
- Modify: `internal/render/render.go` (`drawCards`, `drawCard`)
- Test: `internal/render/card_test.go`

**Interfaces:**
- Consumes: `card.Compose`, `card.Plan`, `card.Part`, `theme.Theme`, `gg.Context`.
- Produces: `drawCard(dc *gg.Context, th *theme.Theme, rev Reveal, cfg config.Reveal, x, y, w, h float64, p card.Plan, selected bool)`, `drawCardContent(dc *gg.Context, th *theme.Theme, p card.Plan, selected bool, x, y, w, h float64)`, `drawChip(dc *gg.Context, th *theme.Theme, x, y, maxW float64, text string, tone backend.Tone, selected bool) float64`, `drawVisual(dc *gg.Context, th *theme.Theme, p card.Part, selected bool, x, y, w, h float64)`, `chipColours(th *theme.Theme, tone backend.Tone, selected bool) (gg.RGBA, gg.RGBA)`, `elide(face text.Face, s string, width float64) string`. Task 6 adds `drawAccessories` and calls it from `drawCardContent`.

- [ ] **Step 1: Write the failing test**

Create `internal/render/card_test.go`:

```go
package render

import (
	"image"
	"strings"
	"testing"

	"github.com/darkliquid/keyshell/internal/backend"
	"github.com/darkliquid/keyshell/internal/card"
	"github.com/darkliquid/keyshell/internal/theme"
	"github.com/gogpu/gg"
)

// TestDrawCardPaintsChipsAndTitle checks a row card draws its header chips and
// that a selected card inverts to slab text on the accent fill.
func TestDrawCardPaintsChipsAndTitle(t *testing.T) {
	th, err := theme.Load()
	if err != nil {
		t.Fatalf("theme.Load = err %v", err)
	}
	defer th.Close()

	plan := card.Compose(backend.Item{
		Title: "Speakers", Subtitle: "hdmi", Icon: "audio",
		Badge: "OUTPUT", Status: "DEFAULT", StatusTone: backend.TonePositive,
	})
	dc := gg.NewContext(400, 260)
	drawCard(dc, th, Reveal{}, testRevealConfig(), 10, 10, 340, 220, plan, false)
	img := dc.Image().(*image.RGBA)

	if !containsColor(img, 0x08, 0xF7, 0xFE) {
		t.Error("cyan chip pixels missing")
	}
	if !containsColor(img, 0xF2, 0xF2, 0xF7) {
		t.Error("ink text pixels missing")
	}
}

// TestDrawCardSelectedUsesSlabText checks the slide-to-accent state does not
// paint ink on the accent fill.
func TestDrawCardSelectedUsesSlabText(t *testing.T) {
	th, err := theme.Load()
	if err != nil {
		t.Fatalf("theme.Load = err %v", err)
	}
	defer th.Close()

	plan := card.Compose(backend.Item{Title: "Ghostty", Icon: "terminal"})
	dc := gg.NewContext(400, 260)
	drawCard(dc, th, Reveal{}, testRevealConfig(), 10, 10, 340, 220, plan, true)
	img := dc.Image().(*image.RGBA)

	if containsColor(img, 0xF2, 0xF2, 0xF7) {
		t.Error("ink pixels on a selected card, want slab text only")
	}
	if !containsColorIn(img, 0x0B, 0x0B, 0x0F, 0x30) {
		t.Error("slab text pixels missing on the selected card")
	}
}

// TestElideFits checks the helper trims to the available width.
func TestElideFits(t *testing.T) {
	th, err := theme.Load()
	if err != nil {
		t.Fatalf("theme.Load = err %v", err)
	}
	defer th.Close()

	face := th.Face(12)
	got := elide(face, strings.Repeat("WIDE ", 40), 80)
	if face.Advance(got) > 80 {
		t.Errorf("elided %q advances %v, want <= 80", got, face.Advance(got))
	}
	if face.Advance(elide(face, "ok", 80)) != face.Advance("ok") {
		t.Error("short text was changed")
	}
}

// testRevealConfig returns the geometry the renderer expects.
func testRevealConfig() config.Reveal {
	return config.Default().Reveal
}

// containsColorIn allows a colour-range match, used where antialiasing means an
// exact pixel may not exist.
func containsColorIn(img *image.RGBA, r, g, b, tolerance uint8) bool {
	for y := 0; y < img.Bounds().Dy(); y += 2 {
		for x := 0; x < img.Bounds().Dx(); x += 2 {
			px := img.RGBAAt(x, y)
			if absDiff(px.R, r) <= tolerance && absDiff(px.G, g) <= tolerance && absDiff(px.B, b) <= tolerance {
				return true
			}
		}
	}
	return false
}

func absDiff(a, b uint8) int {
	if a > b {
		return int(a - b)
	}
	return int(b - a)
}
```

Add the missing `config` import to the imports block.

- [ ] **Step 2: Run the test to confirm it fails**

Run: `mise exec -- go test ./internal/render/ -run 'TestDrawCard' -v`
Expected: FAIL, `undefined: drawCard ... too many arguments` and `undefined: elide`.

- [ ] **Step 3: Write the card drawing**

Create `internal/render/card.go`:

```go
package render

import (
	"math"
	"strings"

	"github.com/darkliquid/keyshell/internal/backend"
	"github.com/darkliquid/keyshell/internal/card"
	"github.com/darkliquid/keyshell/internal/theme"
	"github.com/gogpu/gg"
	"github.com/gogpu/gg/text"
)

// drawCardContent paints a plan inside the axis-aligned content box, inset from
// the sheared quad the way the QML Loader clipped its delegate.
func drawCardContent(dc *gg.Context, th *theme.Theme, p card.Plan, selected bool, x, y, w, h float64) {
	ink, muted := th.Ink, th.Muted
	if selected {
		ink, muted = th.Slab, th.Slab
	}

	top := y
	if p.Badge.Kind != "" || p.Status.Kind != "" {
		if p.Badge.Kind != "" {
			drawChip(dc, th, x, top, w/2, p.Badge.Text, p.Badge.Tone, selected)
		}
		if p.Status.Kind != "" {
			drawChip(dc, th, x+w/2, top, w/2, p.Status.Text, p.Status.Tone, selected)
		}
		top += 18 + 6
	}

	bottom := y + h
	if text := footerText(p); text != "" {
		bottom -= 16
		face := th.Face(9)
		dc.SetFont(face)
		dc.SetColor(muted)
		dc.DrawStringAnchored(elide(face, text, w), x+w/2, bottom+8, 0.5, 0.5)
	}

	if bottom-top < 20 {
		return
	}
	switch p.Layout {
	case card.LayoutHero:
		drawHero(dc, th, p, selected, x, top, w, bottom-top)
	case card.LayoutBody:
		drawBody(dc, th, p, selected, x, top, w, bottom-top)
	default:
		drawRow(dc, th, p, selected, x, top, w, bottom-top)
	}
}

// footerText folds the overflow count into the hint line.
func footerText(p card.Plan) string {
	text := ""
	if p.Footer.Kind != "" {
		text = p.Footer.Text
	}
	if p.Overflow > 0 {
		marker := "+" + itoa(p.Overflow)
		if text == "" {
			return marker
		}
		return text + "  //  " + marker
	}
	return text
}

// drawHero paints the app card: a centred visual over a wrapped title.
func drawHero(dc *gg.Context, th *theme.Theme, p card.Plan, selected bool, x, y, w, h float64) {
	size := math.Max(36, math.Min(52, h*0.40))
	drawVisual(dc, th, p.Visual, selected, x+(w-size)/2, y+4, size, size)
	face := th.Face(math.Max(12, math.Min(16, h*0.14)))
	drawLines(dc, th, face, p.Title.Text, selected, x, y+size+10, w, 2, true)
}

// drawBody paints the iconless card: a large centred value over its detail.
func drawBody(dc *gg.Context, th *theme.Theme, p card.Plan, selected bool, x, y, w, h float64) {
	face := th.Face(24)
	lines := drawLines(dc, th, face, p.Title.Text, selected, x, y+4, w, 2, true)
	next := y + 4 + float64(len(lines))*28

	if p.Detail.Kind != "" {
		drawDetail(dc, th, p.Detail.Text, selected, x, next, w, h-(next-y)-4)
		return
	}
	if p.Subtitle.Kind != "" {
		sub := th.Face(11)
		dc.SetFont(sub)
		dc.SetColor(mutedOrInk(th, selected))
		dc.DrawStringAnchored(elide(sub, p.Subtitle.Text, w), x+w/2, next+6, 0.5, 0.5)
	}
}

// drawRow paints the standard card: a leading visual beside a text column.
func drawRow(dc *gg.Context, th *theme.Theme, p card.Plan, selected bool, x, y, w, h float64) {
	const box = 44.0
	drawVisual(dc, th, p.Visual, selected, x, y+(h-box)/2, box, box)

	colX := x + box + 10
	colW := w - box - 10
	if colW <= 0 {
		return
	}
	top := y + 2
	face := th.Face(titleSize(p.Title.Style))
	lines := drawLines(dc, th, face, titleText(p.Title), selected, colX, top, colW, maxLines(p.Title.Style), false)
	top += float64(len(lines)) * (titleSize(p.Title.Style) + 2)

	if p.Subtitle.Kind != "" {
		sub := th.Face(9)
		dc.SetFont(sub)
		dc.SetColor(mutedOrInk(th, selected))
		dc.DrawStringAnchored(elide(sub, strings.ToUpper(p.Subtitle.Text), colW), colX, top+7, 0, 0.5)
	}
}

func drawDetail(dc *gg.Context, th *theme.Theme, s string, selected bool, x, y, w, h float64) {
	face := th.Face(11)
	dc.SetFont(face)
	dc.SetColor(mutedOrInk(th, selected))
	for i, line := range dc.WordWrap(s, w) {
		if i == 3 || float64(i)*(14) > h {
			break
		}
		dc.DrawStringAnchored(elide(face, line, w), x+w/2, y+float64(i)*14+7, 0.5, 0.5)
	}
}

// drawLines draws up to max wrapped lines and returns them.
func drawLines(dc *gg.Context, th *theme.Theme, face text.Face, s string, selected bool, x, y, w float64, max int, centred bool) []string {
	dc.SetFont(face)
	dc.SetColor(inkOrSlab(th, selected))
	lines := dc.WordWrap(titleTextPart(face, s), w)
	if len(lines) > max {
		lines = lines[:max]
	}
	ax := 0.0
	if centred {
		ax = 0.5
	}
	lh := face.Metrics().LineHeight()
	for i, line := range lines {
		dc.DrawStringAnchored(line, x+w*ax, y+lh*(float64(i)+0.5), ax, 0.5)
	}
	return lines
}

func titleSize(s card.Style) float64 {
	switch s {
	case card.StyleHero:
		return 16
	case card.StyleValue:
		return 24
	case card.StyleWrap:
		return 11
	}
	return 13
}

func titleText(p card.Part) string {
	if p.Style == card.StyleLine || p.Style == card.StyleValue {
		return strings.ToUpper(p.Text)
	}
	return p.Text
}

func maxLines(s card.Style) int {
	switch s {
	case card.StyleWrap:
		return 3
	case card.StyleHero, card.StyleValue:
		return 2
	}
	return 1
}

func inkOrSlab(th *theme.Theme, selected bool) gg.RGBA {
	if selected {
		return th.Slab
	}
	return th.Ink
}

func mutedOrInk(th *theme.Theme, selected bool) gg.RGBA {
	if selected {
		return th.Slab
	}
	return th.Muted
}

// drawChip paints a small rounded chip and returns its width.
func drawChip(dc *gg.Context, th *theme.Theme, x, y, maxW float64, s string, tone backend.Tone, selected bool) float64 {
	face := th.Face(9)
	dc.SetFont(face)
	label := elide(face, strings.ToUpper(s), maxW-10)
	w := math.Min(face.Advance(label)+10, maxW)
	fill, ink := chipColours(th, tone, selected)
	dc.SetColor(fill)
	dc.DrawRoundedRectangle(x, y, w, 18, 2)
	_ = dc.Fill()
	dc.SetFont(face)
	dc.SetColor(ink)
	dc.DrawStringAnchored(label, x+w/2, y+9, 0.5, 0.5)
	return w
}

// chipColours maps a tone to its fill and text colour in both states.
func chipColours(th *theme.Theme, tone backend.Tone, selected bool) (gg.RGBA, gg.RGBA) {
	if selected {
		switch tone {
		case backend.ToneAccent, backend.ToneInfo, backend.TonePositive, backend.ToneAttention:
			return th.Slab, th.AccentAlt
		case backend.ToneWarn, backend.ToneAlert:
			return th.Slab, th.Accent
		}
		return th.Slab, th.Ink
	}
	switch tone {
	case backend.ToneAccent, backend.ToneInfo, backend.TonePositive, backend.ToneAttention:
		return th.AccentAlt, th.Slab
	case backend.ToneWarn, backend.ToneAlert:
		return th.Accent, th.Slab
	}
	return th.SurfaceAlt, th.Muted
}

// drawVisual paints the leading artwork: image and preview are placeholders
// until the icon and capture subsystems land, so they fall back to a monogram.
func drawVisual(dc *gg.Context, th *theme.Theme, p card.Part, selected bool, x, y, w, h float64) {
	if p.Kind == "" {
		return
	}
	if p.Source == "glyph" {
		face := th.Face(math.Min(28, h*0.6))
		dc.SetFont(face)
		dc.SetColor(inkOrSlab(th, selected))
		dc.DrawStringAnchored(p.Text, x+w/2, y+h/2, 0.5, 0.5)
		return
	}

	fill, border := th.SurfaceAlt, th.AccentAlt
	if selected {
		fill, border = th.Slab, th.Slab
	}
	dc.SetColor(fill)
	dc.DrawRoundedRectangle(x, y, w, h, 4)
	_ = dc.Fill()
	dc.NewSubPath()
	dc.SetColor(border)
	dc.SetLineWidth(2)
	dc.DrawRoundedRectangle(x, y, w, h, 4)
	_ = dc.Stroke()

	face := th.Face(math.Min(22, h*0.5))
	dc.SetFont(face)
	dc.SetColor(inkOrSlab(th, selected))
	dc.DrawStringAnchored(monogram(p.Text), x+w/2, y+h/2, 0.5, 0.5)
}

// monogram is the fallback mark for an icon or artwork source: its first
// letter, upper case.
func monogram(s string) string {
	for _, r := range s {
		if r != ' ' && r != '-' && r != '_' {
			return strings.ToUpper(string(r))
		}
	}
	return "?"
}

// elide trims s until it fits width, marking the cut with a full stop.
func elide(face text.Face, s string, width float64) string {
	if width <= 0 {
		return ""
	}
	if face.Advance(s) <= width {
		return s
	}
	r := []rune(s)
	for len(r) > 1 {
		r = r[:len(r)-1]
		if face.Advance(string(r)+".") <= width {
			return string(r) + "."
		}
	}
	return ""
}
```

`titleTextPart` does not exist: `drawLines` calls it. Replace that line with a plain `s` and let `titleText` be applied by the caller:

```go
	lines := dc.WordWrap(s, w)
```

and in `drawRow` pass `titleText(p.Title)` (already does).

- [ ] **Step 4: Rewire the reel**

In `internal/render/render.go`, change `drawCards` to compose each visible item and pass the plan:

```go
		drawCard(dc, th, rev, cfg, x, cardTop, cardW, cardH, card.Compose(results[i].Item), i == selected)
```

Replace the body of `drawCard` with the quad plus content:

```go
func drawCard(dc *gg.Context, th *theme.Theme, rev Reveal, cfg config.Reveal, x, y, w, h float64, p card.Plan, selected bool) {
	if !selected {
		shadow := th.AccentAlt
		shadow.A = 0.7
		cardQuad(dc, rev, x+4, y+4, w, h, shadow, th.AccentAlt, 0)
	}

	fill := th.Slab
	border := th.Muted
	if selected {
		fill = th.Accent
		border = th.Accent
	}
	cardQuad(dc, rev, x, y, w, h, fill, border, 3)

	inset := math.Ceil(math.Abs(rev.Shear)*h/2) + 4 + 10
	drawCardContent(dc, th, p, selected, x+inset, y+10, w-2*inset, h-20)
}
```

Remove the now-unused `strings` import from `render.go` if nothing else uses it (search before removing). Add the `card` import.

- [ ] **Step 5: Run the render and full test suites**

Run: `mise exec -- go test ./internal/render/ -v` then `mise run check`.
Expected: PASS. The existing `TestDrawFrameProducesImage` still finds ink text because row cards draw ink titles.

- [ ] **Step 6: Commit**

```bash
git add internal/render/card.go internal/render/card_test.go internal/render/render.go
git commit -m "feat: draw card header, text and footer from a plan"
```

---

### Task 6: Draw the accessory rows

**Files:**
- Modify: `internal/render/card.go`
- Test: `internal/render/card_test.go`

**Interfaces:**
- Consumes: `drawCardContent` and the helpers from Task 5.
- Produces: `drawAccessories(dc *gg.Context, th *theme.Theme, parts []card.Part, selected bool, x, y, w float64)`, `drawMeter`, `drawTimeline`, `drawNote`, `drawTicks`. `drawCardContent` calls `drawAccessories` in the body, below the text column.

- [ ] **Step 1: Write the failing tests**

Append to `internal/render/card_test.go`:

```go
// TestDrawCardMeterFillsTicks checks a ratio meter paints accent ticks in
// proportion to its value.
func TestDrawCardMeterFillsTicks(t *testing.T) {
	th, err := theme.Load()
	if err != nil {
		t.Fatalf("theme.Load = err %v", err)
	}
	defer th.Close()

	plan := card.Compose(backend.Item{Title: "Speakers", Icon: "audio", Controls: []backend.Control{
		{ID: "volume", Kind: backend.ControlRange, Value: 0.5, Min: 0, Max: 1, Primary: true},
	}})
	dc := gg.NewContext(400, 260)
	drawCard(dc, th, Reveal{}, testRevealConfig(), 10, 10, 340, 220, plan, false)
	img := dc.Image().(*image.RGBA)

	if !containsColor(img, 0xFF, 0x2E, 0x63) {
		t.Error("accent meter pixels missing")
	}
}

// TestDrawCardNotePaintsTheMutedPill checks a true toggle reaches the card.
func TestDrawCardNotePaintsTheMutedPill(t *testing.T) {
	th, err := theme.Load()
	if err != nil {
		t.Fatalf("theme.Load = err %v", err)
	}
	defer th.Close()

	plan := card.Compose(backend.Item{Title: "Speakers", Icon: "audio", Controls: []backend.Control{
		{ID: "mute", Kind: backend.ControlToggle, Label: "Mute", Bool: true, Display: backend.DisplayCard},
	}})
	dc := gg.NewContext(400, 260)
	drawCard(dc, th, Reveal{}, testRevealConfig(), 10, 10, 340, 220, plan, false)
	img := dc.Image().(*image.RGBA)

	if containsColor(img, 0xFF, 0x2E, 0x63) == false && containsColor(img, 0x08, 0xF7, 0xFE) == false {
		t.Error("no note pill ink pixels found")
	}
}

// TestDrawCardOverflowMarkerShowsOnTheFooter checks the hidden count is visible.
func TestDrawCardOverflowMarkerShowsOnTheFooter(t *testing.T) {
	if got := footerText(card.Plan{
		Footer:   card.Part{Kind: backend.PartFooter, Text: "ENTER SELECT"},
		Overflow: 2,
	}); got != "ENTER SELECT  //  +2" {
		t.Errorf("footer = %q, want %q", got, "ENTER SELECT  //  +2")
	}
}
```

- [ ] **Step 2: Run them to confirm they fail**

Run: `mise exec -- go test ./internal/render/ -run 'TestDrawCardMeter|TestDrawCardNote|TestDrawCardOverflow' -v`
Expected: the two draw tests FAIL on missing pixels; the footer test already passes (Task 5 wrote `footerText`).

- [ ] **Step 3: Write the accessory drawing**

Append to `internal/render/card.go`:

```go
// accessoryHeight is the row height of one accessory plus its gap.
const accessoryHeight = 18.0

// drawAccessories paints the meter, timeline and note rows under the text
// column, one row each.
func drawAccessories(dc *gg.Context, th *theme.Theme, parts []card.Part, selected bool, x, y, w float64) {
	for _, p := range parts {
		switch p.Kind {
		case backend.PartMeter:
			drawMeter(dc, th, p, selected, x, y, w)
		case backend.PartTimeline:
			drawTimeline(dc, th, p, selected, x, y, w)
		case backend.PartNote:
			drawNote(dc, th, p, selected, x, y, w)
		}
		y += accessoryHeight
	}
}

// drawMeter paints either ten sheared ticks or a progress bar.
func drawMeter(dc *gg.Context, th *theme.Theme, p card.Part, selected bool, x, y, w float64) {
	if p.Ticks > 0 {
		drawTicks(dc, th, p.Ratio, p.Ticks, selected, x, y, w)
		return
	}
	track, fill := th.SurfaceAlt, th.Accent
	if selected {
		track, fill = th.Slab, th.AccentAlt
	}
	barY := y + 4
	dc.SetColor(track)
	dc.DrawRoundedRectangle(x, barY, w, 5, 1)
	_ = dc.Fill()
	dc.SetColor(fill)
	dc.DrawRoundedRectangle(x, barY, math.Max(0, w*p.Ratio), 5, 1)
	_ = dc.Fill()
}

// drawTicks reproduces the QML meter: n bars sheared 0.25, filled by ratio.
func drawTicks(dc *gg.Context, th *theme.Theme, ratio float64, n int, selected bool, x, y, w float64) {
	if n <= 0 {
		return
	}
	gap := 2.0
	tickW := math.Max(2, (w-gap*float64(n-1))/float64(n))
	filled := math.Round(ratio * float64(n))
	on, off := th.Accent, th.SurfaceAlt
	if selected {
		on, off = th.Slab, th.SurfaceAlt
	}
	for i := 0; i < n; i++ {
		tx := x + float64(i)*(tickW+gap)
		dc.SetColor(off)
		if float64(i) < filled {
			dc.SetColor(on)
		}
		dc.Push()
		dc.Translate(tx+tickW/2, y+8)
		dc.Shear(-0.25, 0)
		dc.DrawRectangle(-tickW/2, -3.5, tickW, 7)
		_ = dc.Fill()
		dc.Pop()
	}
}

// drawTimeline paints the read-only position row with its elapsed slash total
// label on the right.
func drawTimeline(dc *gg.Context, th *theme.Theme, p card.Part, selected bool, x, y, w float64) {
	face := th.Face(8)
	dc.SetFont(face)
	label := p.Label
	if p.Value != "" {
		label = p.Label + "/" + p.Value
	}
	dc.SetColor(mutedOrInk(th, selected))
	labelW := face.Advance(label)
	dc.DrawStringAnchored(label, x+w, y+8, 1, 0.5)

	track, fill := th.SurfaceAlt, th.Accent
	if selected {
		track, fill = th.Slab, th.AccentAlt
	}
	barW := w - labelW - 6
	if barW <= 0 {
		return
	}
	dc.SetColor(track)
	dc.DrawRoundedRectangle(x, y+5.5, barW, 5, 1)
	_ = dc.Fill()
	dc.SetColor(fill)
	dc.DrawRoundedRectangle(x, y+5.5, math.Max(0, barW*p.Ratio), 5, 1)
	_ = dc.Fill()
}

// drawNote paints an inline state pill such as "MUTED".
func drawNote(dc *gg.Context, th *theme.Theme, p card.Part, selected bool, x, y, w float64) {
	text := p.Text
	if text == "" {
		text = p.Label
	}
	if text == "" {
		text = "ON"
	}
	drawChip(dc, th, x, y, w, text, p.Tone, selected)
}
```

- [ ] **Step 4: Call it from the content box**

In `drawRow`, after the subtitle block, add:

```go
	if len(p.Accessories) > 0 {
		accTop := top + 16
		if accTop+accessoryHeight <= y+h {
			drawAccessories(dc, th, p.Accessories, selected, colX, accTop, colW)
		}
	}
```

In `drawBody`, after the subtitle or detail block, add:

```go
	if len(p.Accessories) > 0 {
		drawAccessories(dc, th, p.Accessories, selected, x, y+h-accessoryHeight*float64(len(p.Accessories)), w)
	}
```

- [ ] **Step 5: Run the render and full test suites**

Run: `mise exec -- go test ./internal/render/ -v` then `mise run check`.
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/render/card.go internal/render/card_test.go
git commit -m "feat: draw card meters, timelines and state notes"
```

---

### Task 7: Migrate app, calculator and clipboard items to the new visual fields

**Files:**
- Modify: `internal/backend/apps/apps.go:59-67`
- Modify: `internal/backend/calc/calc.go:42-48`
- Modify: `internal/backend/clipplugin/clip.go`
- Test: `internal/backend/apps/apps_test.go`, `internal/backend/calc/calc_test.go`, `internal/backend/clipplugin/clip_test.go`

**Interfaces:**
- Consumes: `backend.Item.Icon`, `card.Compose` replayed in tests.
- Produces: app items that compose to `LayoutHero`, calculator items that compose to `LayoutBody`, clipboard items with an `Icon` that compose to a wrapped row card.

- [ ] **Step 1: Write the failing tests**

In `internal/backend/apps/apps_test.go`, replace the title-collecting loop in `TestNewListsApplications` with an item-collecting one and add the card assertions:

```go
	var items []backend.Item
	for {
		it, more, err := stream.Next()
		if err != nil {
			t.Fatalf("Next = err %v, want nil", err)
		}
		if !more {
			break
		}
		items = append(items, it)
	}
	if len(items) != 2 {
		t.Errorf("items = %v, want 2 (ghostty, firefox)", items)
	}
	for _, it := range items {
		if it.Icon == "" {
			t.Errorf("item %q has no icon name", it.Title)
		}
		if it.Subtitle != "" {
			t.Errorf("item %q carries subtitle %q, want none", it.Title, it.Subtitle)
		}
	}
```

Append to `internal/backend/calc/calc_test.go`:

```go
// TestResultHasNoIcon keeps the calculator a body card.
func TestResultHasNoIcon(t *testing.T) {
	b, err := New(config.Default())
	if err != nil {
		t.Fatalf("New = err %v", err)
	}
	stream, err := b.Query(context.Background(), backend.Query{Text: "1+1"})
	if err != nil {
		t.Fatalf("Query = err %v", err)
	}
	defer stream.Close()
	it, more, _ := stream.Next()
	if !more {
		t.Fatal("no item")
	}
	if it.Icon != "" || it.Glyph != "" {
		t.Errorf("icon = %q glyph = %q, want both empty", it.Icon, it.Glyph)
	}
}
```

In `internal/backend/clipplugin/clip_test.go`, inside `TestQueryHeader`, after the entry action assertion:

```go
	if entry.Icon == "" {
		t.Errorf("entry icon = %q, want a theme icon name", entry.Icon)
	}
```

- [ ] **Step 2: Run them to confirm they fail**

Run: `mise exec -- go test ./internal/backend/apps/ ./internal/backend/calc/ ./internal/backend/clipplugin/ -v`
Expected: FAIL. Apps carry a subtitle and no icon, the calculator still carries `Glyph: "calc"`, and the clipboard entry has no icon.

- [ ] **Step 3: Update the three backends**

In `internal/backend/apps/apps.go`, the item literal becomes:

```go
		items = append(items, backend.Item{
			ID:      backend.ResultID(e.ID),
			Title:   e.Name,
			Icon:    e.Icon,
			Actions: []backend.Action{{Type: "exec", Value: joinPlaceholder(argv)}},
		})
```

In `internal/backend/calc/calc.go`, drop the `Glyph` field:

```go
	return &sliceStream{items: []backend.Item{{
		ID:       backend.ResultID("result"),
		Title:    result,
		Subtitle: "Calculator",
		Actions:  []backend.Action{{Type: "copy", Value: result}},
	}}}, nil
```

In `internal/backend/clipplugin/clip.go`, replace every `Glyph: "clipboard"` with `Icon: "edit-copy"` (two occurrences).

- [ ] **Step 4: Run the three packages and the full gate**

Run: `mise exec -- go test ./internal/backend/apps/ ./internal/backend/calc/ ./internal/backend/clipplugin/ -v` then `mise run check`.
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/backend/apps internal/backend/calc internal/backend/clipplugin
git commit -m "feat: give app, calculator and clipboard items card-ready visuals"
```

---

### Task 8: Send status and read-only position from media, audio and network

**Files:**
- Modify: `internal/backend/mediaplugin/media.go:40-76`
- Modify: `internal/backend/audioplugin/audio.go:50-78`
- Modify: `internal/backend/netplugin/net.go:68-84`
- Test: `internal/backend/mediaplugin/media_test.go`, `internal/backend/audioplugin/audio_test.go`, `internal/backend/netplugin/net_test.go`

**Interfaces:**
- Consumes: `sdk.Item.Status`, `sdk.Item.StatusTone`, `sdk.Item.Badge`, `sdk.Control.Display`, `sdk.Control.Primary`, control kind `progress`.
- Produces: media cards with a `PLAYING`/`PAUSED` status and a timeline accessory, audio cards with an `OUTPUT` badge, a `DEFAULT` status and a pinned primary volume meter, network cards with `CONNECTED`/`OPEN` status.

- [ ] **Step 1: Write the failing tests**

In `internal/backend/mediaplugin/media_test.go`, inside `TestQueryListsPlayers` after the control-count assertions:

```go
	var song *sdk.Item
	for i := range items {
		if items[i].Title == "Cool Song" {
			song = &items[i]
		}
	}
	if song == nil {
		t.Fatal("no item for Cool Song")
	}
	if song.Status != "PLAYING" || song.StatusTone != "info" {
		t.Errorf("status = %q/%q, want PLAYING/info", song.Status, song.StatusTone)
	}
	var progress *sdk.Control
	for i := range song.Controls {
		if song.Controls[i].Kind == "progress" {
			progress = &song.Controls[i]
		}
	}
	if progress == nil {
		t.Fatalf("controls = %+v, want a progress control", song.Controls)
	}
	if progress.Label != "0:30" || progress.Text != "4:00" {
		t.Errorf("progress = %q/%q, want 0:30/4:00", progress.Label, progress.Text)
	}
```

In `internal/backend/audioplugin/audio_test.go`, add a default-sink aware stub and a test:

```go
// stubBackendDefault answers the default-sink query separately from the sink
// list, so the DEFAULT status can be asserted.
func stubBackendDefault(script, defaultSink string) *Backend {
	return &Backend{pactl: func(args ...string) *exec.Cmd {
		if len(args) > 0 && args[0] == "get-default-sink" {
			return exec.Command("sh", "-c", "printf '%s' '"+defaultSink+"'")
		}
		return exec.Command("sh", "-c", script)
	}}
}

// TestQueryMarksTheDefaultSink checks the badge, the status and the pinned,
// primary volume control the card budget relies on.
func TestQueryMarksTheDefaultSink(t *testing.T) {
	b := stubBackendDefault(`printf '%s' '`+sinksJSON+`'`, "alsa_output.pci-0000_01_00.1.hdmi-stereo")
	stream, err := b.Query(context.Background(), sdk.Query{})
	if err != nil {
		t.Fatalf("Query = err %v", err)
	}
	defer stream.Close()
	it, more, _ := stream.Next()
	if !more {
		t.Fatal("no item")
	}
	if it.Badge != "OUTPUT" {
		t.Errorf("badge = %q, want OUTPUT", it.Badge)
	}
	if it.Status != "DEFAULT" {
		t.Errorf("status = %q, want DEFAULT", it.Status)
	}
	if it.Controls[0].ID != "volume" || !it.Controls[0].Primary || it.Controls[0].Display != "card" {
		t.Errorf("volume control = %+v, want primary and pinned", it.Controls[0])
	}
}
```

In `internal/backend/netplugin/net_test.go`, inside `TestQueryListsConnectionsAndWifi` after the item count:

```go
	var wired, active *sdk.Item
	for i := range items {
		switch items[i].ID {
		case "Wired connection 1":
			wired = &items[i]
		case "Disable WiFi":
			active = &items[i]
		}
	}
	if active == nil || active.Status != "CONNECTED" || active.StatusTone != "positive" {
		t.Errorf("active connection = %+v, want CONNECTED/positive", active)
	}
	if wired == nil || wired.Status != "OPEN" || wired.StatusTone != "neutral" {
		t.Errorf("inactive connection = %+v, want OPEN/neutral", wired)
	}
```

- [ ] **Step 2: Run them to confirm they fail**

Run: `mise exec -- go test ./internal/backend/mediaplugin/ ./internal/backend/audioplugin/ ./internal/backend/netplugin/ -v`
Expected: FAIL on the missing status, badge, tone and control hints.

- [ ] **Step 3: Update the three backends**

In `internal/backend/mediaplugin/media.go`, the item block becomes:

```go
		controls := []sdk.Control{
			{ID: "playback", Kind: "choice", Label: "Playback", Text: status,
				Options: []string{"play", "pause", "next", "previous"}},
		}
		if length > 0 {
			controls = append(controls, sdk.Control{
				ID: "position", Kind: "progress", Label: formatClock(pos), Text: formatClock(length),
				Value: pos, Min: 0, Max: length, Step: 1,
			})
		}
		item := sdk.Item{
			ID: p, Title: display, Subtitle: artist,
			Glyph: "media", Controls: controls,
			Status: strings.ToUpper(status), StatusTone: "info",
		}
		if !strings.EqualFold(status, "Playing") {
			item.StatusTone = "neutral"
		}
		items = append(items, item)
```

Add beside `playerState`:

```go
// formatClock renders seconds as m:ss or h:mm:ss for the timeline label.
func formatClock(sec float64) string {
	if sec < 0 || math.IsInf(sec, 0) || math.IsNaN(sec) {
		return "0:00"
	}
	total := int(sec)
	h, m, s := total/3600, (total%3600)/60, total%60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}
```

Imports: add `math` if the compiler asks; `fmt` may already be imported.

In `internal/backend/audioplugin/audio.go`, read the default sink once before the loop:

```go
	defaultSink := b.defaultSink()
```

and the item becomes:

```go
		item := sdk.Item{
			ID: s.Name, Title: title, Subtitle: s.Name, Glyph: "audio",
			Badge: "OUTPUT", StatusTone: "info",
			Controls: []sdk.Control{
				{ID: "volume", Kind: "range", Label: "Volume", Value: volume, Min: 0, Max: 1, Step: 0.01,
					Display: "card", Primary: true},
				{ID: "mute", Kind: "toggle", Label: "Mute", Bool: s.Mute},
			},
		}
		if s.Name == defaultSink {
			item.Status = "DEFAULT"
		}
		items = append(items, item)
```

In `internal/backend/netplugin/net.go`, the item becomes:

```go
		item := sdk.Item{
			ID: c.name, Title: c.name, Subtitle: c.subtitle(), Glyph: "network",
			Status: "OPEN", StatusTone: "neutral",
			Controls: []sdk.Control{
				{ID: "connect", Kind: "toggle", Label: "Connected", Bool: c.active},
			},
		}
		if c.active {
			item.Status, item.StatusTone = "CONNECTED", "positive"
		}
		items = append(items, item)
```

- [ ] **Step 4: Run the three packages and the full gate**

Run: `mise exec -- go test ./internal/backend/mediaplugin/ ./internal/backend/audioplugin/ ./internal/backend/netplugin/ -v` then `mise run check`.
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/backend/mediaplugin internal/backend/audioplugin internal/backend/netplugin
git commit -m "feat: send card status and read-only position from backends"
```

---

### Task 9: Verify live on niri and record the outcome

**Files:**
- Modify: `docs/superpowers/specs/2026-10-20-card-composition-design.md` (only if live behaviour contradicts it)

**Interfaces:**
- Consumes: the built binary.
- Produces: a recorded verification, not new API.

- [ ] **Step 1: Build and run**

```bash
mise exec -- go build -o /tmp/keyshell ./cmd/keyshell
/tmp/keyshell &
```

Expected: the overlay opens with cards on screen.

- [ ] **Step 2: Capture and count pixels**

```bash
grim -t ppm /tmp/keyshell-card.ppm
python3 - <<'EOF'
def read_ppm(path):
    with open(path, 'rb') as f:
        assert f.readline().strip() == b'P6'
        line = f.readline()
        while line.startswith(b'#'):
            line = f.readline()
        width, height = map(int, line.split())
        f.readline()
        return width, height, f.read()

width, height, data = read_ppm('/tmp/keyshell-card.ppm')
print('size', width, height)

def count(rgb, tol=6, step=2):
    n = 0
    for i in range(0, len(data) - 2, 3 * step):
        r, g, b = data[i], data[i + 1], data[i + 2]
        if abs(r - rgb[0]) <= tol and abs(g - rgb[1]) <= tol and abs(b - rgb[2]) <= tol:
            n += 1
    return n

for name, rgb in [('accent', (0xFF, 0x2E, 0x63)),
                  ('cyan', (0x08, 0xF7, 0xFE)),
                  ('ink', (0xF2, 0xF2, 0xF7)),
                  ('muted', (0x5A, 0x5A, 0x66))]:
    print(name, count(rgb))
EOF
```

Expected: all four counts non-zero. Zero cyan means no chip was drawn; zero accent means no selection and no meter; zero ink means no card text.

- [ ] **Step 3: Exercise the new parts**

Type `hdmi` for the audio plugin, then the network trigger, then the media trigger. After each:

```bash
grim -t ppm /tmp/keyshell-mode.ppm
```

Expected: audio cards show an `OUTPUT` chip, a `DEFAULT` chip on the default sink and a 10-tick volume meter; network cards show `CONNECTED` on the active connection; media cards show `PLAYING` and a timeline reading the elapsed and total time.

- [ ] **Step 4: Confirm no spurious overflow marker**

No current backend emits more than two eligible accessories per item, so no card should show `+N`. Read the footer row of every visible card to confirm. The marker itself is covered by `TestDrawCardOverflowMarkerShowsOnTheFooter`, because no live item source can exceed the budget today.

- [ ] **Step 5: Stop the binary and record**

```bash
pkill -f /tmp/keyshell
```

If a card chose a different layout or style than the spec's rules predict, edit the spec's Layouts or Budget section to match what was verified, then commit:

```bash
git add docs/superpowers/specs/2026-10-20-card-composition-design.md
git commit -m "docs: record verified card composition behaviour"
```

If nothing diverged, skip the commit.

---

## Self-Review

**Spec coverage**

| spec section | task |
| --- | --- |
| part vocabulary | 2 (types), 5 (header, visual, text), 6 (accessories) |
| layouts | 2 |
| budget | 3 |
| eligibility | 3 |
| accessory ranking | 3 |
| overflow | 3 (count), 5 (`footerText`), 9 (no spurious marker live) |
| override path | 4 |
| type changes | 1 |
| module shape | 2 (`internal/card`), 5 (`internal/render`) |
| rendering notes | 5, 6 |
| testing | every task |
| rollout | 5 (rewire), 9 (verify) |

Two spec items are deliberately not implemented here and are recorded rather than silently dropped:

- `hold`: the part kind exists in the vocabulary, but the Go input layer has no long-press state to drive it. It lands with the input long-press work; `Compose` does not emit it and the renderer does not draw it.
- icon resolution and live preview capture: `Icon`, `Image` and `Preview` are carried across both seams, and the renderer draws a framed monogram placeholder. An icon-theme resolver (and the capture provider behind `Preview`) is its own subsystem.

**Placeholder scan:** no "TBD", no "similar to Task N", and every code step shows the code. Task 9 does not pretend to exercise overflow live: no item source in the repo can exceed the budget, so the unit test carries it and the live step only confirms the marker does not appear spuriously.

**Type consistency:** `Part.Control` is an `int` index everywhere (`-1` when absent); `card.Plan.Accessories` is a `[]card.Part`; `footerText` is the only place `Plan.Overflow` becomes text; `drawCard`'s signature is identical in Tasks 5 and 6; `AccessoryRows` is used by both the derivation and the override.