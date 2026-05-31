package generation

import (
	"context"
	"fmt"
	"time"

	"github.com/grapestree/fgrapery/grapery-agent/internal/domain"
	"github.com/grapestree/fgrapery/grapery-agent/internal/grapery_client"
	"github.com/grapestree/fgrapery/grapery-agent/internal/runstore"
)

// Service orchestrates non-chat generation runs with tracing.
type Service struct {
	client   *grapery_client.Client
	store    runstore.Store
	provider string
	model    string
}

func NewService(client *grapery_client.Client, store runstore.Store, textProvider, textModel string) *Service {
	return &Service{
		client:   client,
		store:    store,
		provider: textProvider,
		model:    textModel,
	}
}

func (s *Service) GetRun(ctx context.Context, id string) (*domain.GenerationRun, error) {
	run, ok := s.store.GetRun(ctx, id)
	if !ok {
		return nil, fmt.Errorf("run not found: %s", id)
	}
	return run, nil
}

func (s *Service) ListRuns(ctx context.Context, kind domain.RunKind, limit int) []*domain.GenerationRun {
	return s.store.ListRuns(ctx, kind, limit)
}

func (s *Service) finishRun(ctx context.Context, runID string, status domain.RunStatus, output map[string]any, content domain.ContentRef, tokens int, errMsg string) error {
	now := time.Now()
	return s.store.UpdateRun(ctx, runID, func(r *domain.GenerationRun) {
		r.Status = status
		r.Output = output
		r.ContentIDs = content
		r.TokensUsed = tokens
		r.Error = errMsg
		r.CompletedAt = &now
		r.ModelProvider = s.provider
		r.ModelName = s.model
	})
}

func (s *Service) markRunning(ctx context.Context, runID string) {
	_ = s.store.UpdateRun(ctx, runID, func(r *domain.GenerationRun) {
		r.Status = domain.RunStatusRunning
	})
}

func (s *Service) traceArtifact(ctx context.Context, run *domain.GenerationRun) {
	_ = s.store.AppendArtifact(ctx, &domain.RLArtifact{
		Type:          domain.ArtifactTypeGenerationTrace,
		RunID:         run.ID,
		Prompt:        run.UserIntent,
		GenerationRun: run,
	})
}

func tracedClientCall(ctx context.Context, store runstore.Store, tool string, input map[string]any, fn func(context.Context) (map[string]any, error)) (map[string]any, error) {
	return runstore.TracedCall(ctx, store, tool, input, fn)
}
