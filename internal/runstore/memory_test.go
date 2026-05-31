package runstore

import (
	"context"
	"testing"

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
