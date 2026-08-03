package workflow

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/grapestree/fgrapery/grapery-agent/internal/domain"
)

func TestCompileAndExecuteWorkflow(t *testing.T) {
	registry := NewActivityRegistry()
	if err := registry.Register("story.load", func(_ context.Context, input, _ map[string]any) (map[string]any, error) {
		return map[string]any{"title": input["title"]}, nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register("story.persist", func(_ context.Context, input, _ map[string]any) (map[string]any, error) {
		upstream := input["upstream"].(map[string]any)
		return map[string]any{"saved": upstream["load"] != nil}, nil
	}); err != nil {
		t.Fatal(err)
	}

	release := &domain.WorkflowRelease{
		ID: "wfr_story_1", Key: "story", Version: 1, Status: "released",
		Definition: domain.WorkflowDefinition{Nodes: []domain.WorkflowNode{
			{ID: "persist", Type: "persist", Activity: "story.persist", DependsOn: []string{"load"}},
			{ID: "load", Type: "activity", Activity: "story.load"},
		}},
	}
	compiled, err := Compile(release, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(compiled.Batches) != 2 || compiled.Batches[0][0].ID != "load" {
		t.Fatalf("unexpected topological batches: %+v", compiled.Batches)
	}
	result, err := Execute(context.Background(), compiled, registry, map[string]any{"title": "Voyager"})
	if err != nil {
		t.Fatal(err)
	}
	if saved, _ := result.NodeOutputs["persist"]["saved"].(bool); !saved {
		t.Fatalf("persist did not receive upstream output: %+v", result.NodeOutputs)
	}
}

func TestExecuteRetriesNodeAccordingToReleasePolicy(t *testing.T) {
	registry := NewActivityRegistry()
	attempts := 0
	if err := registry.Register("flaky", func(_ context.Context, _, _ map[string]any) (map[string]any, error) {
		attempts++
		if attempts < 3 {
			return nil, errors.New("temporary provider failure")
		}
		return map[string]any{"ok": true}, nil
	}); err != nil {
		t.Fatal(err)
	}
	release := &domain.WorkflowRelease{
		ID: "wfr_retry", Key: "retry", Version: 1, Status: "released",
		Policies:   domain.WorkflowPolicies{MaxAttempts: 3},
		Definition: domain.WorkflowDefinition{Nodes: []domain.WorkflowNode{{ID: "generate", Type: "activity", Activity: "flaky"}}},
	}
	compiled, err := Compile(release, registry)
	if err != nil {
		t.Fatal(err)
	}
	retries := 0
	result, err := ExecuteWithOptions(context.Background(), compiled, registry, nil, ExecutionOptions{
		RetryDelay:  func(int) time.Duration { return 0 },
		OnNodeRetry: func(_ domain.WorkflowNode, _ int, _ error) { retries++ },
	})
	if err != nil {
		t.Fatal(err)
	}
	if attempts != 3 || retries != 2 || result.NodeOutputs["generate"]["ok"] != true {
		t.Fatalf("unexpected retry result: attempts=%d retries=%d outputs=%+v", attempts, retries, result.NodeOutputs)
	}
}

func TestExecuteRetryBackoffHonorsContextCancellation(t *testing.T) {
	registry := NewActivityRegistry()
	if err := registry.Register("failing", func(_ context.Context, _, _ map[string]any) (map[string]any, error) {
		return nil, errors.New("still failing")
	}); err != nil {
		t.Fatal(err)
	}
	release := &domain.WorkflowRelease{
		ID: "wfr_cancel_retry", Key: "retry", Version: 1, Status: "released",
		Policies:   domain.WorkflowPolicies{MaxAttempts: 3},
		Definition: domain.WorkflowDefinition{Nodes: []domain.WorkflowNode{{ID: "generate", Type: "activity", Activity: "failing"}}},
	}
	compiled, err := Compile(release, registry)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = ExecuteWithOptions(ctx, compiled, registry, nil, ExecutionOptions{RetryDelay: func(int) time.Duration { return time.Hour }})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation during retry backoff, got %v", err)
	}
}

func TestExecuteResumesFromDurableAttemptCount(t *testing.T) {
	registry := NewActivityRegistry()
	executions := 0
	if err := registry.Register("resume.attempt", func(_ context.Context, _, _ map[string]any) (map[string]any, error) {
		executions++
		return map[string]any{"ok": true}, nil
	}); err != nil {
		t.Fatal(err)
	}
	release := &domain.WorkflowRelease{
		ID: "wfr_attempt_resume", Key: "retry", Version: 1, Status: "released",
		Policies:   domain.WorkflowPolicies{MaxAttempts: 3},
		Definition: domain.WorkflowDefinition{Nodes: []domain.WorkflowNode{{ID: "generate", Type: "activity", Activity: "resume.attempt"}}},
	}
	compiled, err := Compile(release, registry)
	if err != nil {
		t.Fatal(err)
	}
	persistedAttempt := 0
	_, err = ExecuteWithOptions(context.Background(), compiled, registry, nil, ExecutionOptions{
		AttemptCounts: map[string]int{"generate": 2},
		BeforeNodeAttempt: func(_ domain.WorkflowNode, attempt int) error {
			persistedAttempt = attempt
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if executions != 1 || persistedAttempt != 3 {
		t.Fatalf("durable attempt count was reset: executions=%d persisted=%d", executions, persistedAttempt)
	}
}

func TestCompileRejectsUnregisteredAndCyclicNodes(t *testing.T) {
	registry := NewActivityRegistry()
	release := &domain.WorkflowRelease{
		ID: "wfr_bad_1", Key: "bad", Version: 1, Status: "released",
		Definition: domain.WorkflowDefinition{Nodes: []domain.WorkflowNode{
			{ID: "a", Type: "activity", Activity: "missing", DependsOn: []string{"b"}},
			{ID: "b", Type: "activity", Activity: "missing", DependsOn: []string{"a"}},
		}},
	}
	if _, err := Compile(release, registry); err == nil {
		t.Fatal("expected unregistered activity error")
	}
	if err := registry.Register("missing", func(context.Context, map[string]any, map[string]any) (map[string]any, error) {
		return nil, nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := Compile(release, registry); err == nil {
		t.Fatal("expected cycle error")
	}
}

func TestExecuteWithOptionsResumesAfterCompletedNode(t *testing.T) {
	registry := NewActivityRegistry()
	executions := map[string]int{}
	for _, name := range []string{"first", "second"} {
		activityName := name
		if err := registry.Register(activityName, func(_ context.Context, _ map[string]any, _ map[string]any) (map[string]any, error) {
			executions[activityName]++
			return map[string]any{"activity": activityName}, nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	release := &domain.WorkflowRelease{
		ID: "wfr_resume_1", Key: "resume", Version: 1, Status: "released",
		Definition: domain.WorkflowDefinition{Nodes: []domain.WorkflowNode{
			{ID: "a", Type: "activity", Activity: "first"},
			{ID: "b", Type: "activity", Activity: "second", DependsOn: []string{"a"}},
		}},
	}
	compiled, err := Compile(release, registry)
	if err != nil {
		t.Fatal(err)
	}
	checkpointCount := 0
	result, err := ExecuteWithOptions(context.Background(), compiled, registry, nil, ExecutionOptions{
		NodeOutputs: map[string]map[string]any{"a": {"activity": "first"}},
		AfterNode: func(node domain.WorkflowNode, outputs map[string]map[string]any) error {
			checkpointCount++
			if node.ID != "b" || len(outputs) != 2 {
				t.Fatalf("unexpected checkpoint: node=%s outputs=%+v", node.ID, outputs)
			}
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if executions["first"] != 0 || executions["second"] != 1 {
		t.Fatalf("completed node was executed again: %+v", executions)
	}
	if checkpointCount != 1 || len(result.NodeOutputs) != 2 {
		t.Fatalf("unexpected resumed result: checkpoints=%d result=%+v", checkpointCount, result)
	}
}
