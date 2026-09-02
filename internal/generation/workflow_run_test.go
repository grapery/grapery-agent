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

func TestWorkflowCompletionPreservesChildArtifactsAndTokens(t *testing.T) {
	current := &domain.GenerationRun{
		Output:     map[string]any{"taskId": "task_fragment", "provider": "test"},
		ContentIDs: domain.ContentRef{FragmentID: "fragment_1", TaskID: "task_fragment"},
		TokensUsed: 37,
	}
	result := &workflowruntime.ExecutionResult{NodeOutputs: map[string]map[string]any{
		"generate": {
			"draftFragmentId": "fragment_1",
			"taskId":          "task_fragment",
			"tokensUsed":      float64(37),
		},
	}}

	output, content, tokens := workflowCompletion(current, result, "wfr_1")
	if content.FragmentID != "fragment_1" || content.TaskID != "task_fragment" {
		t.Fatalf("content references were lost: %+v", content)
	}
	if tokens != 37 {
		t.Fatalf("tokens were lost: %d", tokens)
	}
	if output["provider"] != "test" || output["workflowReleaseId"] != "wfr_1" {
		t.Fatalf("parent output was overwritten: %+v", output)
	}
}

func TestWorkflowCompletionExtractsBranchArtifacts(t *testing.T) {
	result := &workflowruntime.ExecutionResult{NodeOutputs: map[string]map[string]any{
		"branch": {
			"storyId":             "story_1",
			"branchStoryboardIds": []any{"storyboard_a", "storyboard_b"},
			"tokensUsed":          81,
		},
	}}

	_, content, tokens := workflowCompletion(nil, result, "wfr_branch")
	if content.StoryID != "story_1" || len(content.BranchIDs) != 2 || content.BranchIDs[1] != "storyboard_b" {
		t.Fatalf("branch references were not extracted: %+v", content)
	}
	if tokens != 81 {
		t.Fatalf("branch token usage was not retained: %d", tokens)
	}
}

func TestWorkflowCompletionAggregatesTokensAcrossNodes(t *testing.T) {
	result := &workflowruntime.ExecutionResult{NodeOutputs: map[string]map[string]any{
		"text":  {"tokensUsed": float64(12)},
		"image": {"tokensUsed": float64(8)},
	}}
	_, _, tokens := workflowCompletion(&domain.GenerationRun{TokensUsed: 12}, result, "wfr_multi")
	if tokens != 20 {
		t.Fatalf("node tokens were not aggregated: %d", tokens)
	}
}

func TestApplyWorkflowInputDefaultsDoesNotOverrideUserChoice(t *testing.T) {
	input := map[string]any{"style": "ink", "sceneCount": float64(0)}
	config := map[string]any{"inputDefaults": map[string]any{
		"style": "watercolor", "sceneCount": float64(5), "language": "zh-Hans",
	}}

	got := applyWorkflowInputDefaults(input, config)
	if got["style"] != "ink" || got["sceneCount"] != float64(5) || got["language"] != "zh-Hans" {
		t.Fatalf("unexpected configured input: %+v", got)
	}
	if input["language"] != nil {
		t.Fatal("runtime input was mutated")
	}
}

func TestApplyLegacyWorkflowPromptUsesPinnedTemplate(t *testing.T) {
	prompt := domain.PromptTemplateVersion{
		ID: "ptv_fragment_1", SystemTemplate: "Keep {{.style}} continuity.", UserTemplate: "Create: {{.legacyUserPrompt}}",
	}
	input, err := applyLegacyWorkflowPrompt(
		map[string]any{"userInput": "a rainy reunion", "style": "ink"},
		map[string]any{"promptTemplate": prompt},
		"userInput",
	)
	if err != nil {
		t.Fatal(err)
	}
	if input["userInput"] != "a rainy reunion" || input["workflowSystemPrompt"] != "Keep ink continuity." || input["workflowUserPrompt"] != "Create: a rainy reunion" || input["workflowPromptVersionId"] != prompt.ID {
		t.Fatalf("pinned prompt was not applied: %+v", input)
	}
}

func TestFindWorkflowByClientRequestAvoidsReroutingRetry(t *testing.T) {
	store := runstore.NewMemoryStore()
	service := NewService(nil, store, "", "", false)
	run, err := store.CreateRun(context.Background(), domain.RunKindWorkflow, domain.AgentWorkflowRuntime, "storyboard", map[string]any{
		"clientRequestId": "request-1",
		"workflowSurface": "voyager.storyboard",
		"workflowAction":  "generate",
	})
	if err != nil {
		t.Fatal(err)
	}
	got := service.findWorkflowByClientRequest(context.Background(), domain.WorkflowStartInput{
		Surface: "voyager.storyboard", Action: "generate", ClientRequestID: "request-1",
	})
	if got == nil || got.ID != run.ID {
		t.Fatalf("retry did not reuse canonical workflow: %+v", got)
	}
}
