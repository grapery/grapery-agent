package domain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

// CreativeControls are bounded editorial choices. They never replace the
// engineering prompt, output contract, validation, model, tools or billing.
type CreativeControls struct {
	Version         int               `json:"version"`
	Pacing          string            `json:"pacing"`
	DialogueDensity string            `json:"dialogueDensity"`
	VisualEmphasis  string            `json:"visualEmphasis"`
	Tone            string            `json:"tone"`
	Examples        []CreativeExample `json:"examples"`
}
type CreativeExample struct {
	Input  string `json:"input"`
	Output string `json:"output"`
}

func DefaultCreativeControls() CreativeControls {
	return CreativeControls{Version: 1, Pacing: "balanced", DialogueDensity: "balanced", VisualEmphasis: "balanced", Tone: "neutral", Examples: []CreativeExample{}}
}
func (c *CreativeControls) UnmarshalJSON(raw []byte) error {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return fmt.Errorf("creativeControls must be an object")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return err
	}
	for key, value := range fields {
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return fmt.Errorf("creativeControls.%s cannot be null", key)
		}
	}

	type wire CreativeControls
	defaults := DefaultCreativeControls()
	value := wire(defaults)
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return fmt.Errorf("creativeControls: %w", err)
	}
	candidate := CreativeControls(value)
	if err := candidate.Validate(); err != nil {
		return err
	}
	*c = candidate
	return nil
}
func (c CreativeControls) Validate() error {
	if c.Version != 1 {
		return fmt.Errorf("creativeControls.version must be 1")
	}
	allowed := func(value string, values ...string) bool {
		for _, v := range values {
			if v == value {
				return true
			}
		}
		return false
	}
	if !allowed(c.Pacing, "balanced", "slow", "fast") {
		return fmt.Errorf("invalid creativeControls.pacing")
	}
	if !allowed(c.DialogueDensity, "balanced", "sparse", "rich") {
		return fmt.Errorf("invalid creativeControls.dialogueDensity")
	}
	if !allowed(c.VisualEmphasis, "balanced", "atmosphere", "action", "character") {
		return fmt.Errorf("invalid creativeControls.visualEmphasis")
	}
	if !allowed(c.Tone, "neutral", "warm", "tense", "playful") {
		return fmt.Errorf("invalid creativeControls.tone")
	}
	if len(c.Examples) > 3 {
		return fmt.Errorf("creativeControls.examples supports at most 3 examples")
	}
	for i, e := range c.Examples {
		if utf8.RuneCountInString(e.Input) > 500 || utf8.RuneCountInString(e.Output) > 1500 || strings.TrimSpace(e.Input) == "" || strings.TrimSpace(e.Output) == "" {
			return fmt.Errorf("creativeControls.examples[%d] requires input 1-500 and output 1-1500 Unicode characters", i)
		}
	}
	return nil
}
func ParseCreativeControls(value any) (CreativeControls, error) {
	if value == nil {
		return CreativeControls{}, fmt.Errorf("creativeControls must be an object")
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return CreativeControls{}, err
	}
	if string(raw) == "null" {
		return CreativeControls{}, fmt.Errorf("creativeControls must be an object")
	}
	var result CreativeControls
	if err = json.Unmarshal(raw, &result); err != nil {
		return CreativeControls{}, err
	}
	return result, nil
}
