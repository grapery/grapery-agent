package http

import (
	"encoding/json"
	"testing"
)

func TestExtractStructuredPayload(t *testing.T) {
	raw := "好的，我来帮你规划。\n[[voyager_structured]]{\"type\":\"planning\",\"generationIntent\":{\"agent\":\"fragment\"}}[[/voyager_structured]]\n还需要确认风格。"
	clean, structured := extractStructuredPayload(raw)
	if clean == raw {
		t.Fatal("expected structured block stripped from message")
	}
	if structured == nil {
		t.Fatal("expected structured payload")
	}
	m, ok := structured.(map[string]any)
	if !ok || m["type"] != "planning" {
		t.Fatalf("unexpected structured: %#v", structured)
	}
}

func TestExtractStructuredPayload_invalidJSON(t *testing.T) {
	msg := "hello [[voyager_structured]]not-json[[/voyager_structured]]"
	clean, structured := extractStructuredPayload(msg)
	if clean != msg || structured != nil {
		t.Fatalf("invalid json should leave message unchanged, got clean=%q structured=%v", clean, structured)
	}
}

func TestExtractStructuredPayload_roundTrip(t *testing.T) {
	payload := map[string]any{
		"type": "revision",
		"generationIntent": map[string]any{
			"userInput": "续写第二幕",
			"imageCount": float64(4),
		},
	}
	b, _ := json.Marshal(payload)
	msg := "prefix [[voyager_structured]]" + string(b) + "[[/voyager_structured]] suffix"
	clean, structured := extractStructuredPayload(msg)
	if clean != "prefix  suffix" {
		t.Fatalf("clean=%q", clean)
	}
	m := structured.(map[string]any)
	if m["type"] != "revision" {
		t.Fatalf("type=%v", m["type"])
	}
}
