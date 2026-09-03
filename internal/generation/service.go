package generation

import (
	"context"
	"fmt"
	"time"

	modelcomponent "github.com/cloudwego/eino/components/model"
	"github.com/grapestree/fgrapery/grapery-agent/internal/domain"
	"github.com/grapestree/fgrapery/grapery-agent/internal/grapery_client"
	"github.com/grapestree/fgrapery/grapery-agent/internal/runstore"
	workflowruntime "github.com/grapestree/fgrapery/grapery-agent/internal/workflow"
)

const maxGenerationPollDuration = 12 * time.Hour

func generationPollTimeout(seconds int) time.Duration {
	if seconds <= 0 {
		return maxGenerationPollDuration
	}
	timeout := time.Duration(seconds) * time.Second
	if timeout > maxGenerationPollDuration {
		return maxGenerationPollDuration
	}
	return timeout
}

// Service orchestrates non-chat generation runs with tracing.
type Service struct {
	client             *grapery_client.Client
	store              runstore.Store
	provider           string
	model              string
	execFragmentPanel  bool
	workflowPlanner    modelcomponent.BaseChatModel
	workflowActivities *workflowruntime.ActivityRegistry
}

// SetWorkflowPlannerModel injects the Eino model used by AI planning nodes.
// The outer durable workflow remains the owner of retries and checkpoints.
func (s *Service) SetWorkflowPlannerModel(chatModel modelcomponent.BaseChatModel) {
	s.workflowPlanner = chatModel
}

func NewService(client *grapery_client.Client, store runstore.Store, textProvider, textModel string, execFragmentPanel bool) *Service {
	service := &Service{
		client:            client,
		store:             store,
		provider:          textProvider,
		model:             textModel,
		execFragmentPanel: execFragmentPanel,
	}
	service.workflowActivities = service.newWorkflowActivityRegistry()
	return service
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

func (s *Service) CancelRun(ctx context.Context, runID string) error {
	if s.store == nil {
		return fmt.Errorf("run store unavailable")
	}
	run, ok := s.store.GetRun(ctx, runID)
	if !ok {
		return fmt.Errorf("run not found: %s", runID)
	}
	if err := s.store.CancelRun(ctx, runID); err != nil {
		return err
	}
	// Keep the local cancellation authoritative, then stop the business task on
	// a best-effort basis so provider work does not continue unnecessarily.
	if run.ContentIDs.StoryboardID != "" {
		_ = s.client.CancelStoryboardGeneration(ctx, run.ContentIDs.StoryboardID)
	} else if run.ContentIDs.TaskID != "" && run.Kind == domain.RunKindFragment {
		_ = s.client.CancelFragmentTask(ctx, run.ContentIDs.TaskID)
	}
	s.settleQuota(ctx, domain.RunStatusCancelled, 0)
	return nil
}

func (s *Service) ExecFragmentPanelEnabled() bool {
	return s.execFragmentPanel
}

func (s *Service) finishRun(ctx context.Context, runID string, status domain.RunStatus, output map[string]any, content domain.ContentRef, tokens int, errMsg string) error {
	now := time.Now()
	err := s.store.UpdateRun(ctx, runID, func(r *domain.GenerationRun) {
		r.Status = status
		r.Output = output
		r.ContentIDs = content
		r.TokensUsed = tokens
		r.Error = errMsg
		r.CompletedAt = &now
		r.ModelProvider = s.provider
		r.ModelName = s.model
	})
	s.settleQuota(ctx, status, tokens)
	return err
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
