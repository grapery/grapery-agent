package runstore

import (
	"context"
	"testing"

	"github.com/grapestree/fgrapery/grapery-agent/internal/agentauth"
	"github.com/grapestree/fgrapery/grapery-agent/internal/domain"
)

func TestMemoryStoreCreateAndToolTrace(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()
	run, err := s.CreateRun(ctx, domain.RunKindFragment, domain.AgentFragmentCreator, "test", map[string]any{"k": "v"})
	if err != nil {
		t.Fatal(err)
	}
	ctx = ContextWithRunID(ctx, run.ID)
	_, err = TracedCall(ctx, s, "poll_task_status", map[string]any{"taskId": "t1"}, func(c context.Context) (map[string]any, error) {
		return map[string]any{"status": "running"}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	got, ok := s.GetRun(ctx, run.ID)
	if !ok || len(got.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %+v", got)
	}
}

func TestMemoryStoreFindRunByClientRequestIsNotWindowed(t *testing.T) {
	ctx := agentauth.ContextWithClaims(context.Background(), &agentauth.Claims{UserID: "user-1"})
	store := NewMemoryStore()
	wanted, err := store.CreateRun(ctx, domain.RunKindWorkflow, domain.AgentWorkflowRuntime, "wanted", map[string]any{"clientRequestId": "request-1"})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 600; i++ {
		if _, err := store.CreateRun(ctx, domain.RunKindWorkflow, domain.AgentWorkflowRuntime, "noise", map[string]any{"clientRequestId": "noise"}); err != nil {
			t.Fatal(err)
		}
	}
	got, ok := store.FindRunByClientRequest(ctx, domain.RunKindWorkflow, "user-1", "request-1")
	if !ok || got.ID != wanted.ID {
		t.Fatalf("exact request lookup failed: got=%+v ok=%v", got, ok)
	}
}
