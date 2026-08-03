package generation

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/grapestree/fgrapery/grapery-agent/internal/domain"
	"github.com/grapestree/fgrapery/grapery-agent/internal/runstore"
	workflowruntime "github.com/grapestree/fgrapery/grapery-agent/internal/workflow"
)

func TestValidatePromptSnapshotsRequiresExactPinnedBundle(t *testing.T) {
	bundle := map[string]string{"generate": "ptv_storyboard_v1"}
	snapshots := map[string]domain.PromptTemplateVersion{
		"generate": {ID: "ptv_storyboard_v1", Checksum: "checksum"},
	}
	if err := validatePromptSnapshots(bundle, snapshots); err != nil {
		t.Fatalf("valid snapshot rejected: %v", err)
	}
	snapshots["generate"] = domain.PromptTemplateVersion{ID: "ptv_other", Checksum: "checksum"}
	if err := validatePromptSnapshots(bundle, snapshots); err == nil {
		t.Fatal("expected mismatched prompt version to be rejected")
	}
}

func TestExecuteWorkflowEnforcesOriginalMaxDuration(t *testing.T) {
	store := runstore.NewMemoryStore()
	service := NewService(nil, store, "", "", false)
	if err := service.workflowActivities.Register("test.never", func(context.Context, map[string]any, map[string]any) (map[string]any, error) {
		t.Fatal("expired workflow must not execute another activity")
		return nil, nil
	}); err != nil {
		t.Fatal(err)
	}
	release := &domain.WorkflowRelease{
		ID: "wfr_expired", Key: "expired", Version: 1, Status: "released",
		Policies:   domain.WorkflowPolicies{MaxDurationSeconds: 1},
		Definition: domain.WorkflowDefinition{Nodes: []domain.WorkflowNode{{ID: "never", Type: "activity", Activity: "test.never"}}},
	}
	compiled, err := workflowruntime.Compile(release, service.workflowActivities)
	if err != nil {
		t.Fatal(err)
	}
	run, err := store.CreateRun(context.Background(), domain.RunKindWorkflow, domain.AgentWorkflowRuntime, "expired", nil)
	if err != nil {
		t.Fatal(err)
	}
	state := &workflowCheckpoint{Version: workflowCheckpointVersion, ReleaseID: release.ID, StartedAt: time.Now().Add(-2 * time.Second), Input: map[string]any{}, NodeOutputs: map[string]map[string]any{}}
	service.executeWorkflow(context.Background(), run.ID, compiled, state)
	got, ok := store.GetRun(context.Background(), run.ID)
	if !ok || got.Status != domain.RunStatusFailed || !strings.Contains(got.Error, "max duration") {
		t.Fatalf("expired workflow was not failed: %+v", got)
	}
}

func TestAttachPromptSnapshotsOnlyTargetsBoundNode(t *testing.T) {
	release := &domain.WorkflowRelease{Definition: domain.WorkflowDefinition{Nodes: []domain.WorkflowNode{
		{ID: "generate", Config: map[string]any{"attempts": 2}},
		{ID: "persist"},
	}}}
	prompt := domain.PromptTemplateVersion{ID: "ptv_storyboard_v1", Checksum: "checksum"}
	scenePrompt := domain.PromptTemplateVersion{ID: "ptv_scene_v1", Checksum: "scene-checksum"}
	attachPromptSnapshots(release, map[string]domain.PromptTemplateVersion{"generate": prompt, "generate:scene_plan": scenePrompt})

	got, ok := release.Definition.Nodes[0].Config["promptTemplate"].(domain.PromptTemplateVersion)
	if !ok || got.ID != prompt.ID {
		t.Fatalf("prompt snapshot was not attached: %+v", release.Definition.Nodes[0].Config)
	}
	if release.Definition.Nodes[0].Config["attempts"] != 2 {
		t.Fatal("existing node configuration was overwritten")
	}
	slots, ok := release.Definition.Nodes[0].Config["promptTemplates"].(map[string]domain.PromptTemplateVersion)
	if !ok || slots["scene_plan"].ID != scenePrompt.ID {
		t.Fatalf("prompt slot was not attached: %+v", release.Definition.Nodes[0].Config)
	}
	if release.Definition.Nodes[1].Config != nil {
		t.Fatal("unbound node received prompt configuration")
	}
}
