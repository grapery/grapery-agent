package observability

import (
	"context"
	"errors"
	"testing"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	modelcomponent "github.com/cloudwego/eino/components/model"
)

func TestCollectorRecordsTokensAndErrors(t *testing.T) {
	collector := NewCollector(10)
	handler := collector.Handler("fragment", "cp-1")
	info := &callbacks.RunInfo{Name: "model", Type: "test", Component: components.ComponentOfChatModel}
	ctx := handler.OnStart(context.Background(), info, nil)
	handler.OnEnd(ctx, info, &modelcomponent.CallbackOutput{TokenUsage: &modelcomponent.TokenUsage{TotalTokens: 42}})
	ctx = handler.OnStart(context.Background(), info, nil)
	handler.OnError(ctx, info, errors.New("provider unavailable"))

	snapshot := collector.Snapshot()
	stats := snapshot.Components[string(components.ComponentOfChatModel)]
	if stats.Started != 2 || stats.Succeeded != 1 || stats.Failed != 1 || stats.TokensUsed != 42 {
		t.Fatalf("unexpected component stats: %+v", stats)
	}
	if len(snapshot.Recent) != 2 || snapshot.Recent[1].Status != "failed" {
		t.Fatalf("unexpected spans: %+v", snapshot.Recent)
	}
}
