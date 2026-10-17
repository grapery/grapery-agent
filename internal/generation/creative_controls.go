package generation

import (
	"context"
	"fmt"

	"github.com/grapestree/fgrapery/grapery-agent/internal/domain"
)

type creativeControlsContextKey struct{}

func withCreativeControls(ctx context.Context, controls domain.CreativeControls) context.Context {
	controls.Examples = append([]domain.CreativeExample{}, controls.Examples...)
	return context.WithValue(ctx, creativeControlsContextKey{}, controls)
}
func controlsForGeneration(ctx context.Context, provided *domain.CreativeControls) (*domain.CreativeControls, error) {
	controls, trusted := ctx.Value(creativeControlsContextKey{}).(domain.CreativeControls)
	if !trusted {
		if provided != nil {
			return nil, fmt.Errorf("creativeControls are owned by the selected workflow release")
		}
		controls = domain.DefaultCreativeControls()
	}
	if err := controls.Validate(); err != nil {
		return nil, err
	}
	controls.Examples = append([]domain.CreativeExample{}, controls.Examples...)
	return &controls, nil
}
func controlsForActivity(ctx context.Context) *domain.CreativeControls {
	controls, _ := controlsForGeneration(ctx, nil)
	return controls
}

// These fields are authority-bearing engine inputs, never client tuning knobs.
func rejectWorkflowOverrides(input map[string]any) error {
	for _, key := range []string{"creativeControls", "workflowSystemPrompt", "workflowUserPrompt", "workflowModelConfig", "workflowOutputSchema", "workflowPromptVersionId", "promptTemplate", "promptTemplates", "promptSnapshots", "workflowManagedStoryboardStages", "workflowTestRun"} {
		if _, exists := input[key]; exists {
			return fmt.Errorf("workflow input %s is reserved for engineering/release configuration", key)
		}
	}
	return nil
}

func creativeControlsFromRelease(release *domain.WorkflowRelease) (domain.CreativeControls, error) {
	if release == nil {
		return domain.CreativeControls{}, fmt.Errorf("workflow release is required")
	}
	value, present := release.Manifest["creativeControls"]
	if !present {
		return domain.DefaultCreativeControls(), nil
	}
	return domain.ParseCreativeControls(value)
}
