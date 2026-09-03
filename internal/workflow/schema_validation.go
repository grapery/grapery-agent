package workflow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	jsonschemavalidator "github.com/santhosh-tekuri/jsonschema/v6"
)

// ValidateJSONSchema validates a runtime value against a Forge-published JSON
// Schema. Both documents are round-tripped through JSON so validation observes
// the same representation that is persisted in workflow checkpoints.
func ValidateJSONSchema(label string, schemaDocument map[string]any, value any) error {
	if len(schemaDocument) == 0 {
		return nil
	}
	label = strings.TrimSpace(label)
	if label == "" {
		label = "value"
	}

	rawSchema, err := json.Marshal(schemaDocument)
	if err != nil {
		return fmt.Errorf("marshal %s schema: %w", label, err)
	}
	parsedSchema, err := jsonschemavalidator.UnmarshalJSON(bytes.NewReader(rawSchema))
	if err != nil {
		return fmt.Errorf("decode %s schema: %w", label, err)
	}
	compiler := jsonschemavalidator.NewCompiler()
	compiler.DefaultDraft(jsonschemavalidator.Draft2020)
	const resource = "workflow-schema.json"
	if err := compiler.AddResource(resource, parsedSchema); err != nil {
		return fmt.Errorf("load %s schema: %w", label, err)
	}
	compiled, err := compiler.Compile(resource)
	if err != nil {
		return fmt.Errorf("compile %s schema: %w", label, err)
	}

	rawValue, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal %s: %w", label, err)
	}
	parsedValue, err := jsonschemavalidator.UnmarshalJSON(bytes.NewReader(rawValue))
	if err != nil {
		return fmt.Errorf("decode %s: %w", label, err)
	}
	if err := compiled.Validate(parsedValue); err != nil {
		return fmt.Errorf("%s does not match published schema: %w", label, err)
	}
	return nil
}
