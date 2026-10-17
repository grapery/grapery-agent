package domain

import (
	"encoding/json"
	"os"
	"testing"
)

func TestSharedCreativeControlsWireContract(t *testing.T) {
	raw, err := os.ReadFile("testdata/creative_controls_contract.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name  string          `json:"name"`
		Value json.RawMessage `json:"value"`
		Valid bool            `json:"valid"`
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			var input any
			if err := json.Unmarshal(tc.Value, &input); err != nil {
				t.Fatal(err)
			}
			_, err := ParseCreativeControls(input)
			if (err == nil) != tc.Valid {
				t.Fatalf("valid=%v want=%v err=%v", err == nil, tc.Valid, err)
			}
		})
	}
}
