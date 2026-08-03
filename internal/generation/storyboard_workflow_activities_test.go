package generation

import (
	"testing"

	"github.com/grapestree/fgrapery/grapery-agent/internal/domain"
	"github.com/grapestree/fgrapery/grapery-agent/internal/grapery_client"
	"github.com/grapestree/fgrapery/grapery-agent/internal/runstore"
	workflowruntime "github.com/grapestree/fgrapery/grapery-agent/internal/workflow"
)

func TestSplitStoryboardWorkflowActivitiesCompile(t *testing.T) {
	service := NewService(nil, runstore.NewMemoryStore(), "", "", false)
	release := &domain.WorkflowRelease{
		ID: "wfr_split", Key: "storyboard", Version: 1, Status: "released",
		Definition: domain.WorkflowDefinition{Nodes: []domain.WorkflowNode{
			{ID: "generate_storyboard", Type: "activity", Activity: "storyboard.ensure_draft"},
			{ID: "bible", Type: "activity", Activity: "storyboard.generate_bible_plan", DependsOn: []string{"generate_storyboard"}},
			{ID: "scene_plan", Type: "activity", Activity: "storyboard.generate_scene_plan", DependsOn: []string{"bible"}},
			{ID: "persist", Type: "persist", Activity: "storyboard.persist_content", DependsOn: []string{"scene_plan"}},
			{ID: "ensure_images", Type: "activity", Activity: "storyboard.ensure_images", DependsOn: []string{"persist"}},
		}},
	}
	if _, err := workflowruntime.Compile(release, service.workflowActivities); err != nil {
		t.Fatalf("split storyboard workflow did not compile: %v", err)
	}
}

func TestWorkflowManagesStoryboardStagesRequiresAllDurableTextStages(t *testing.T) {
	definition := domain.WorkflowDefinition{Nodes: []domain.WorkflowNode{
		{Activity: "storyboard.generate_bible_plan"},
		{Activity: "storyboard.generate_scene_plan"},
		{Activity: "storyboard.persist_content"},
	}}
	if !workflowManagesStoryboardStages(definition) {
		t.Fatal("complete durable storyboard workflow was not recognized")
	}
	definition.Nodes = definition.Nodes[:2]
	if workflowManagesStoryboardStages(definition) {
		t.Fatal("partial durable storyboard workflow must retain legacy behavior")
	}
}

func TestStoryboardIDFromWorkflowInputUsesUpstreamOutput(t *testing.T) {
	input := map[string]any{"upstream": map[string]any{
		"generate_storyboard": map[string]any{"storyboardId": "sb_123"},
	}}
	if got := storyboardIDFromWorkflowInput(input, domain.StoryboardGenerateInput{}); got != "sb_123" {
		t.Fatalf("unexpected storyboard id: %q", got)
	}
	if got := storyboardIDFromWorkflowInput(input, domain.StoryboardGenerateInput{DraftStoryboardID: "sb_pinned"}); got != "sb_pinned" {
		t.Fatalf("draft storyboard id must win: %q", got)
	}
}

func TestGenerationRunIDFromWorkflowInputPinsNextStage(t *testing.T) {
	input := map[string]any{"upstream": map[string]any{
		"bible": map[string]any{"storyboardId": "sb_123", "generationRunId": "sgr_456"},
	}}
	if got := generationRunIDFromWorkflowInput(input); got != "sgr_456" {
		t.Fatalf("unexpected generation run id: %q", got)
	}
}

func TestStoryboardWorkflowProgressBoundaries(t *testing.T) {
	if storyboardContentReady(&grapery_client.GenerationProgressResponse{WorkflowStatus: "draft", IsGenerating: false}) {
		t.Fatal("idle draft must not be treated as content ready")
	}
	if !storyboardContentReady(&grapery_client.GenerationProgressResponse{WorkflowStatus: "content_ready"}) {
		t.Fatal("content_ready status was not recognized")
	}
	if storyboardImagesReady(&grapery_client.GenerationProgressResponse{WorkflowStatus: "content_ready"}) {
		t.Fatal("content_ready must not be treated as images ready")
	}
	if !storyboardImagesReady(&grapery_client.GenerationProgressResponse{WorkflowStatus: "images_ready"}) {
		t.Fatal("images_ready status was not recognized")
	}
	if storyboardShouldStartImages(&grapery_client.GenerationProgressResponse{WorkflowStatus: "content_ready", HasPendingTasks: true}) {
		t.Fatal("active automatic image work must not be triggered again")
	}
	if !storyboardShouldStartImages(&grapery_client.GenerationProgressResponse{WorkflowStatus: "content_ready"}) {
		t.Fatal("idle content_ready storyboard should start images")
	}
}
