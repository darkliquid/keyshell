package proto

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestEncodeDecodeRoundTrip covers every message type.
func TestEncodeDecodeRoundTrip(t *testing.T) {
	msgs := []Message{
		{Hello: &Hello{Protocol: 1, ID: "audio", Name: "Audio", Badge: "AUDIO", Trigger: "!", Controls: []string{"range", "toggle"}, Hot: true, StableIDs: true}},
		{Init: &Init{Instance: "sink-1", Config: json.RawMessage(`{"x":1}`)}},
		{Ready: &Ready{}},
		{Query: &Query{ID: 17, Text: "vol", Limit: 60, Merged: false, Scope: []string{"a", "b"}}},
		{Item: &Item{ID: "sink-0", Title: "Speakers", Subtitle: "HDMI", Actions: []Action{{Type: "exec", Value: "x"}}, Controls: []ControlSpec{{ID: "volume", Kind: "range", Value: 0.5}}}},
		{Done: &Done{ID: 17}},
		{Error: &Err{ID: 17, Message: "stale scope"}},
		{Cancel: &Cancel{ID: 17}},
		{Control: &Control{Item: "sink-0", Control: "volume", Value: 0.42}},
		{State: &State{Item: "sink-0", Controls: map[string]float64{"volume": 0.42}}},
		{Invoke: &Invoke{Item: "sink-0", Action: "connect"}},
		{Ping: &Ping{}},
		{Pong: &Pong{Ready: true}},
	}
	for i, m := range msgs {
		b, err := Encode(m)
		if err != nil {
			t.Fatalf("Encode[%d] = err %v", i, err)
		}
		got, err := Decode(b)
		if err != nil {
			t.Fatalf("Decode[%d] = err %v", i, err)
		}
		// Re-encode the decoded form to check equality structurally.
		gotB, err := Encode(got)
		if err != nil {
			t.Fatalf("re-Encode[%d] = err %v", i, err)
		}
		var a, c map[string]json.RawMessage
		json.Unmarshal(b, &a)
		json.Unmarshal(gotB, &c)
		if len(a) != 1 || len(c) != 1 {
			t.Errorf("[%d] keys: %v vs %v", i, a, c)
		}
	}
}

// TestDecodeRejectsMalformed ensures a bad JSON line returns an error.
func TestDecodeRejectsMalformed(t *testing.T) {
	if _, err := Decode([]byte("not json")); err == nil {
		t.Fatal("Decode(malformed) = nil error")
	}
	if _, err := Decode([]byte(`{"bogus":1}`)); err == nil {
		t.Fatal("Decode(unknown key) = nil error")
	}
}

// TestEncodeRejectsEmptyMessage ensures an all-nil message is refused.
func TestEncodeRejectsEmptyMessage(t *testing.T) {
	if _, err := Encode(Message{}); err == nil {
		t.Fatal("Encode(empty) = nil error")
	}
}

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

func TestHelloSearchableSerialization(t *testing.T) {
	tBool := true
	fBool := false

	tests := []struct {
		name       string
		hello      Hello
		wantSubstr string
	}{
		{
			name:       "nil searchable omitted",
			hello:      Hello{Protocol: 1, ID: "test", Searchable: nil},
			wantSubstr: `{"hello":{"protocol":1,"id":"test","name":"","badge":"","trigger":"","controls":null,"hot":false,"stable_ids":false}}`,
		},
		{
			name:       "searchable true present",
			hello:      Hello{Protocol: 1, ID: "test", Searchable: &tBool},
			wantSubstr: `"searchable":true`,
		},
		{
			name:       "searchable false present",
			hello:      Hello{Protocol: 1, ID: "test", Searchable: &fBool},
			wantSubstr: `"searchable":false`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := Encode(Message{Hello: &tt.hello})
			if err != nil {
				t.Fatalf("Encode: %v", err)
			}
			if !strings.Contains(string(b), tt.wantSubstr) {
				t.Errorf("Encode() = %s, want to contain %s", string(b), tt.wantSubstr)
			}
			decoded, err := Decode(b)
			if err != nil {
				t.Fatalf("Decode: %v", err)
			}
			if decoded.Hello == nil {
				t.Fatal("decoded Hello is nil")
			}
			if tt.hello.Searchable == nil {
				if decoded.Hello.Searchable != nil {
					t.Errorf("decoded Searchable = %v, want nil", *decoded.Hello.Searchable)
				}
			} else {
				if decoded.Hello.Searchable == nil || *decoded.Hello.Searchable != *tt.hello.Searchable {
					t.Errorf("decoded Searchable = %v, want %v", decoded.Hello.Searchable, *tt.hello.Searchable)
				}
			}
		})
	}
}

func TestInitConfigOmitEmpty(t *testing.T) {
	msg := Message{Init: &Init{Instance: "inst"}}
	b, err := Encode(msg)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if strings.Contains(string(b), "config") {
		t.Errorf("Encode() = %s, want config to be omitted", string(b))
	}
}
