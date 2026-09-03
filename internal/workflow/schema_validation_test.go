package workflow

import (
	"strings"
	"testing"
)

func TestValidateJSONSchema(t *testing.T) {
	schema := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []any{"storyId", "sceneCount"},
		"properties": map[string]any{
			"storyId":   map[string]any{"type": "string", "minLength": 1},
			"sceneCount": map[string]any{"type": "integer", "minimum": 2, "maximum": 12},
		},
	}
	if err := ValidateJSONSchema("workflow input", schema, map[string]any{"storyId": "story-1", "sceneCount": 8}); err != nil {
		t.Fatalf("valid input rejected: %v", err)
	}
	err := ValidateJSONSchema("workflow input", schema, map[string]any{"storyId": "story-1", "sceneCount": 20})
	if err == nil || !strings.Contains(err.Error(), "published schema") {
		t.Fatalf("invalid input accepted: %v", err)
	}
}

func TestValidateJSONSchemaAllowsEmptySchema(t *testing.T) {
	if err := ValidateJSONSchema("workflow input", nil, map[string]any{"legacy": true}); err != nil {
		t.Fatal(err)
	}
}
