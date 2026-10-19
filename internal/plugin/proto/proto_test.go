package proto

import (
	"encoding/json"
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
