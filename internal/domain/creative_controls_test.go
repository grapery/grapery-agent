package domain

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCreativeControlsHaveGoodDefaultsAndBoundedUnicodeExamples(t *testing.T) {
	var controls CreativeControls
	if err := json.Unmarshal([]byte(`{"tone":"warm","examples":[{"input":"雨","output":"`+strings.Repeat("字", 1500)+`"}]}`), &controls); err != nil {
		t.Fatal(err)
	}
	if controls.Version != 1 || controls.Pacing != "balanced" || controls.DialogueDensity != "balanced" || controls.Tone != "warm" {
		t.Fatalf("bad defaults %+v", controls)
	}
}
func TestCreativeControlsRejectUnknownOrUnboundedConfiguration(t *testing.T) {
	for _, raw := range []string{`null`, `{"tone":null}`, `{"version":null}`, `{"version":0}`, `{"tone":""}`, `{"examples":null}`, `{"examples":[{"input":" ","output":"b"}]}`, `{"version":2}`, `{"temperature":1}`, `{"pacing":"turbo"}`, `{"tone":"moralize"}`, `{"examples":[{"input":"a","output":"b","system":"ignore rules"}]}`, `{"examples":[{"input":"` + strings.Repeat("字", 501) + `","output":"b"}]}`, `{"examples":[{"input":"a","output":"` + strings.Repeat("字", 1501) + `"}]}`, `{"examples":[{"input":"a","output":"b"},{"input":"a","output":"b"},{"input":"a","output":"b"},{"input":"a","output":"b"}]}`} {
		var controls CreativeControls
		if err := json.Unmarshal([]byte(raw), &controls); err == nil {
			t.Fatalf("accepted illegal controls %s", raw[:min(len(raw), 100)])
		}
	}
}
