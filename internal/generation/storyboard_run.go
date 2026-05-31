package generation

import (
	"context"
	"time"

	"github.com/grapestree/fgrapery/grapery-agent/internal/domain"
	"github.com/grapestree/fgrapery/grapery-agent/internal/grapery_client"
	"github.com/grapestree/fgrapery/grapery-agent/internal/runstore"
)

func (s *Service) StartStoryboard(ctx context.Context, in domain.StoryboardGenerateInput) (*domain.GenerationRun, error) {
	input := map[string]any{
		"storyId":   in.StoryID,
		"rawInput":  in.RawInput,
		"sceneCount": in.SceneCount,
	}
	run, err := s.store.CreateRun(ctx, domain.RunKindStoryboard, domain.AgentStoryboardDirector, in.RawInput, input)
	if err != nil {
		return nil, err
	}
	go s.executeStoryboard(context.Background(), run.ID, in)
	return run, nil
}

func (s *Service) executeStoryboard(ctx context.Context, runID string, in domain.StoryboardGenerateInput) {
	ctx = runstore.ContextWithRunID(ctx, runID)
	s.markRunning(ctx, runID)

	sb, err := s.createStoryboard(ctx, in)
	if err != nil {
		_ = s.finishRun(ctx, runID, domain.RunStatusFailed, nil, domain.ContentRef{}, 0, err.Error())
		return
	}
	content := domain.ContentRef{StoryboardID: sb.ID, StoryID: sb.StoryID}

	if err := s.generateStoryboardContent(ctx, sb.ID, in); err != nil {
		_ = s.finishRun(ctx, runID, domain.RunStatusFailed, nil, content, 0, err.Error())
		return
	}

	if in.GenerateImages {
		if in.UseComicPagePipeline {
			_, err = tracedClientCall(ctx, s.store, "generate_all_comic_pages", map[string]any{"storyboardId": sb.ID}, func(c context.Context) (map[string]any, error) {
				resp, err := s.client.GenerateAllComicPages(c, sb.ID, grapery_client.GenerateAllComicPagesRequest{})
				if err != nil {
					return nil, err
				}
				return map[string]any{"total": resp.Total, "successCount": resp.SuccessCount, "failedCount": resp.FailedCount}, nil
			})
		} else {
			_, err = tracedClientCall(ctx, s.store, "generate_all_scene_images", map[string]any{"storyboardId": sb.ID}, func(c context.Context) (map[string]any, error) {
				if err := s.client.GenerateAllSceneImages(c, sb.ID, grapery_client.GenerateAllImagesRequest{}); err != nil {
					return nil, err
				}
				return map[string]any{"success": true}, nil
			})
		}
		if err != nil {
			_ = s.finishRun(ctx, runID, domain.RunStatusFailed, nil, content, 0, err.Error())
			return
		}
	}

	output := map[string]any{"storyboardId": sb.ID, "storyId": sb.StoryID}
	if in.PollProgress {
		timeout := time.Duration(defaultInt(in.PollTimeoutSec, 300)) * time.Second
		deadline := time.Now().Add(timeout)
		for time.Now().Before(deadline) {
			prog, err := s.client.GetGenerationProgress(ctx, sb.ID)
			if err == nil && !prog.IsGenerating && !prog.HasPendingTasks {
				output["workflowStatus"] = prog.WorkflowStatus
				output["generationMessage"] = prog.GenerationMessage
				break
			}
			time.Sleep(3 * time.Second)
		}
	}

	_ = s.finishRun(ctx, runID, domain.RunStatusSucceeded, output, content, 0, "")
	if run, ok := s.store.GetRun(ctx, runID); ok {
		s.traceArtifact(ctx, run)
	}
}

func (s *Service) createStoryboard(ctx context.Context, in domain.StoryboardGenerateInput) (*grapery_client.StoryboardResponse, error) {
	out, err := tracedClientCall(ctx, s.store, "create_storyboard", map[string]any{"storyId": in.StoryID}, func(c context.Context) (map[string]any, error) {
		resp, err := s.client.CreateStoryboard(c, grapery_client.CreateStoryboardRequest{
			StoryID:              in.StoryID,
			Title:                in.Title,
			RawInput:             in.RawInput,
			SceneCount:           defaultInt(in.SceneCount, 3),
			UseComicPagePipeline: in.UseComicPagePipeline,
		})
		if err != nil {
			return nil, err
		}
		return map[string]any{"id": resp.ID, "storyId": resp.StoryID}, nil
	})
	if err != nil {
		return nil, err
	}
	return &grapery_client.StoryboardResponse{ID: str(out["id"]), StoryID: str(out["storyId"])}, nil
}

func (s *Service) generateStoryboardContent(ctx context.Context, storyboardID string, in domain.StoryboardGenerateInput) error {
	_, err := tracedClientCall(ctx, s.store, "generate_storyboard_content", map[string]any{"storyboardId": storyboardID}, func(c context.Context) (map[string]any, error) {
		err := s.client.GenerateStoryboardContent(c, storyboardID, grapery_client.GenerateStoryboardContentRequest{
			RawInput:     in.RawInput,
			CharacterIDs: in.CharacterIDs,
			Style:        in.Style,
		})
		if err != nil {
			return nil, err
		}
		return map[string]any{"success": true}, nil
	})
	return err
}
