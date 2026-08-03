package generation

import (
	"context"
	"time"

	"github.com/grapestree/fgrapery/grapery-agent/internal/domain"
	"github.com/grapestree/fgrapery/grapery-agent/internal/grapery_client"
	"github.com/grapestree/fgrapery/grapery-agent/internal/runstore"
)

func (s *Service) StartStory(ctx context.Context, in domain.StoryGenerateInput) (*domain.GenerationRun, error) {
	input := map[string]any{"clientRequestId": in.ClientRequestID, "prompt": in.Prompt, "style": in.Style}
	run, err := s.store.CreateRun(ctx, domain.RunKindStory, domain.AgentStoryGenerator, in.Prompt, input)
	if err != nil {
		return nil, err
	}
	if run.Reused {
		return run, nil
	}
	go s.executeStory(context.Background(), run.ID, in)
	return run, nil
}

func (s *Service) executeStory(ctx context.Context, runID string, in domain.StoryGenerateInput) {
	ctx = runstore.ContextWithRunID(ctx, runID)
	s.markRunning(ctx, runID)

	startOut, err := tracedClientCall(ctx, s.store, "generate_story", map[string]any{"prompt": in.Prompt}, func(c context.Context) (map[string]any, error) {
		resp, err := s.client.GenerateStory(c, grapery_client.GenerateStoryRequest{
			Prompt:      in.Prompt,
			Context:     in.Context,
			Characters:  in.Characters,
			Style:       in.Style,
			Length:      in.Length,
			Temperature: in.Temperature,
		})
		if err != nil {
			return nil, err
		}
		return map[string]any{"taskId": resp.TaskID, "status": resp.Status}, nil
	})
	if err != nil {
		_ = s.finishRun(ctx, runID, domain.RunStatusFailed, nil, domain.ContentRef{}, 0, err.Error())
		return
	}

	taskID := str(startOut["taskId"])
	content := domain.ContentRef{TaskID: taskID}
	_ = s.store.UpdateRun(ctx, runID, func(r *domain.GenerationRun) {
		r.Status = domain.RunStatusWaiting
		r.ContentIDs = content
	})

	deadline := time.Now().Add(generationPollTimeout(0))
	for time.Now().Before(deadline) {
		st, pollErr := tracedClientCall(ctx, s.store, "poll_ai_task", map[string]any{"taskId": taskID}, func(c context.Context) (map[string]any, error) {
			resp, err := s.client.GetAITaskStatus(c, taskID)
			if err != nil {
				return nil, err
			}
			return map[string]any{"taskId": resp.TaskID, "status": resp.Status, "message": resp.Message}, nil
		})
		if pollErr != nil {
			_ = s.finishRun(ctx, runID, domain.RunStatusFailed, nil, content, 0, pollErr.Error())
			return
		}
		status := str(st["status"])
		switch status {
		case "completed", "succeeded", "success":
			_ = s.finishRun(ctx, runID, domain.RunStatusSucceeded, st, content, 0, "")
			if run, ok := s.store.GetRun(ctx, runID); ok {
				s.traceArtifact(ctx, run)
			}
			return
		case "failed", "cancelled", "canceled":
			_ = s.finishRun(ctx, runID, domain.RunStatusFailed, st, content, 0, str(st["message"]))
			return
		}
		time.Sleep(5 * time.Second)
	}
	_ = s.finishRun(ctx, runID, domain.RunStatusFailed, nil, content, 0, "poll timeout")
}
