package generation

import (
	"context"
	"errors"
	"testing"

	modelcomponent "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/grapestree/fgrapery/grapery-agent/internal/runstore"
)

type plannerTestModel struct {
	responses []string
	errors    []error
	calls     int
}

func (m *plannerTestModel) Generate(_ context.Context, _ []*schema.Message, _ ...modelcomponent.Option) (*schema.Message, error) {
	index := m.calls
	m.calls++
	if index < len(m.errors) && m.errors[index] != nil {
		return nil, m.errors[index]
	}
	if index >= len(m.responses) {
		return nil, errors.New("unexpected planner call")
	}
	return schema.AssistantMessage(m.responses[index], nil), nil
}

func (m *plannerTestModel) Stream(context.Context, []*schema.Message, ...modelcomponent.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, errors.New("stream is not used by planner")
}

func TestAIPlannerActivityReturnsValidatedPlanAndInputPatch(t *testing.T) {
	model := &plannerTestModel{responses: []string{`{
		"schemaVersion":1,"intent":"fork","narrativeMode":"dialogue","sceneCount":5,
		"continuityLevel":"strong","visualBibleStrategy":"inherit_and_patch","characterStrategy":"reuse_existing",
		"imageStrategy":{"generate":true,"generateReferences":false,"parallelism":2},
		"steps":[
			{"activity":"storyboard.ensure_draft","required":true},
			{"activity":"storyboard.generate_bible_plan","required":true},
			{"activity":"storyboard.generate_scene_plan","required":true},
			{"activity":"storyboard.review_content","required":true},
			{"activity":"storyboard.persist_content","required":true},
			{"activity":"storyboard.ensure_images","required":true}
		],
		"acceptanceChecks":["parent_ending_continuity","character_identity"],
		"reasonSummary":"Preserve the parent ending and character voice."
	}`}}
	service := NewService(nil, runstore.NewMemoryStore(), "test", "planner", false)
	service.SetWorkflowPlannerModel(model)
	output, err := service.executeAIPlannerActivity(context.Background(), map[string]any{"parentStoryboardId": "sb-parent"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if output["fallback"] != false {
		t.Fatalf("unexpected fallback: %#v", output)
	}
	patch, ok := output["workflowInputPatch"].(map[string]any)
	if !ok || intFromAny(patch["sceneCount"]) != 5 || patch["continuityLevel"] != "strong" {
		t.Fatalf("unexpected workflow input patch: %#v", output)
	}
}

func TestAIPlannerActivityRepairsInvalidResponse(t *testing.T) {
	model := &plannerTestModel{responses: []string{
		`{"schemaVersion":1,"sceneCount":99}`,
		`{"schemaVersion":1,"intent":"create","narrativeMode":"general","sceneCount":3,"continuityLevel":"standard","visualBibleStrategy":"regenerate","characterStrategy":"reuse_existing","imageStrategy":{"generate":false,"generateReferences":false,"parallelism":1},"steps":[{"activity":"storyboard.ensure_draft","required":true},{"activity":"storyboard.generate_bible_plan","required":true},{"activity":"storyboard.generate_scene_plan","required":true},{"activity":"storyboard.review_content","required":true},{"activity":"storyboard.persist_content","required":true}],"acceptanceChecks":["narrative_coherence"],"reasonSummary":"Standard plan."}`,
	}}
	service := NewService(nil, runstore.NewMemoryStore(), "test", "planner", false)
	service.SetWorkflowPlannerModel(model)
	output, err := service.executeAIPlannerActivity(context.Background(), map[string]any{"sceneCount": 3}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if model.calls != 2 || output["fallback"] != false {
		t.Fatalf("planner repair was not used: calls=%d output=%#v", model.calls, output)
	}
}

func TestAIPlannerActivityFallsBackWhenModelUnavailable(t *testing.T) {
	service := NewService(nil, runstore.NewMemoryStore(), "test", "planner", false)
	output, err := service.executeAIPlannerActivity(context.Background(), map[string]any{"parentStoryboardId": "sb-parent", "sceneCount": 4}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if output["fallback"] != true {
		t.Fatalf("expected deterministic fallback: %#v", output)
	}
	plan := output["generationPlan"].(map[string]any)
	if plan["intent"] != "fork" || intFromAny(plan["sceneCount"]) != 4 || plan["continuityLevel"] != "strong" {
		t.Fatalf("unexpected fallback plan: %#v", plan)
	}
}
