package generation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/grapestree/fgrapery/grapery-agent/internal/domain"
	"github.com/grapestree/fgrapery/grapery-agent/internal/grapery_client"
	"github.com/grapestree/fgrapery/grapery-agent/internal/runstore"
)

func (s *Service) executeEnsureStoryboardDraftActivity(ctx context.Context, input map[string]any, _ map[string]any) (map[string]any, error) {
	in, err := decodeStoryboardWorkflowInput(input)
	if err != nil {
		return nil, err
	}
	storyboard, err := s.resolveStoryboardTarget(ctx, in)
	if err != nil {
		return nil, err
	}
	workflowManagedStages, _ := input["workflowManagedStoryboardStages"].(bool)
	if in.RegenerateStructure && !workflowManagedStages {
		request := grapery_client.GenerateStructureRequest{
			UserDirective: strings.TrimSpace(in.RawInput),
			SceneCount:    in.SceneCount,
			ComicStyle:    firstNonEmptyStoryboardStyle(in.ComicStyle, in.Style),
		}
		if _, err := s.client.GenerateStructure(ctx, storyboard.ID, request); err != nil {
			return nil, err
		}
	}
	if runID, ok := runstore.RunIDFromContext(ctx); ok {
		content := domain.ContentRef{StoryboardID: storyboard.ID, StoryID: storyboard.StoryID}
		_ = s.store.UpdateRun(ctx, runID, func(run *domain.GenerationRun) {
			run.ContentIDs = content
			if run.Output == nil {
				run.Output = make(map[string]any)
			}
			run.Output["storyboardId"] = storyboard.ID
			run.Output["draftStoryboardId"] = storyboard.ID
			run.Output["storyId"] = storyboard.StoryID
		})
		s.appendStoryboardStepAudit(ctx, runID, "context", domain.StepStarted)
	}
	return map[string]any{
		"storyboardId": storyboard.ID, "draftStoryboardId": storyboard.ID,
		"storyId": storyboard.StoryID, "created": strings.TrimSpace(in.DraftStoryboardID) == "",
	}, nil
}

func (s *Service) executeAwaitStoryboardContentActivity(ctx context.Context, input map[string]any, _ map[string]any) (map[string]any, error) {
	in, err := decodeStoryboardWorkflowInput(input)
	if err != nil {
		return nil, err
	}
	storyboardID := storyboardIDFromWorkflowInput(input, in)
	if storyboardID == "" {
		return nil, errors.New("await storyboard content requires an upstream storyboardId")
	}
	return s.waitForStoryboardProgress(ctx, storyboardID, generationPollTimeout(in.PollTimeoutSec), storyboardContentReady)
}

func (s *Service) executeGenerateStoryboardBiblePlanActivity(ctx context.Context, input map[string]any, _ map[string]any) (map[string]any, error) {
	return s.executeStoryboardTextStage(ctx, input, "bible_plan", "bible_plan")
}

func (s *Service) executeGenerateStoryboardScenePlanActivity(ctx context.Context, input map[string]any, _ map[string]any) (map[string]any, error) {
	return s.executeStoryboardTextStage(ctx, input, "scene_plan", "scene_plan")
}

func (s *Service) executePersistStoryboardContentActivity(ctx context.Context, input map[string]any, _ map[string]any) (map[string]any, error) {
	return s.executeStoryboardTextStage(ctx, input, "persist_content", "consistency_audit")
}

func (s *Service) executeStoryboardTextStage(ctx context.Context, input map[string]any, stage, auditStep string) (map[string]any, error) {
	in, err := decodeStoryboardWorkflowInput(input)
	if err != nil {
		return nil, err
	}
	storyboardID := storyboardIDFromWorkflowInput(input, in)
	if storyboardID == "" {
		return nil, fmt.Errorf("storyboard %s requires an upstream storyboardId", stage)
	}
	if runID, ok := runstore.RunIDFromContext(ctx); ok {
		s.appendStoryboardStepAudit(ctx, runID, auditStep, domain.StepStarted)
	}
	request := grapery_client.StoryboardWorkflowStageRequest{GenerationRunID: generationRunIDFromWorkflowInput(input)}
	if stage == "bible_plan" {
		request = grapery_client.StoryboardWorkflowStageRequest{
			ClientRequestID: in.ClientRequestID, RegenerateStructure: in.RegenerateStructure,
			UserDirective: strings.TrimSpace(in.RawInput), SceneCount: in.SceneCount,
			ComicStyle: firstNonEmptyStoryboardStyle(in.ComicStyle, in.Style),
		}
	}
	result, err := s.client.ExecuteStoryboardWorkflowStage(ctx, storyboardID, stage, request)
	if err != nil {
		if runID, ok := runstore.RunIDFromContext(ctx); ok {
			s.appendStoryboardStepAudit(ctx, runID, auditStep, domain.StepFailed)
		}
		return nil, err
	}
	if runID, ok := runstore.RunIDFromContext(ctx); ok {
		s.appendStoryboardStepAudit(ctx, runID, auditStep, domain.StepSucceeded)
	}
	return map[string]any{
		"storyboardId": result.StoryboardID, "generationRunId": result.GenerationRunID,
		"stage": result.Stage, "progress": result.Progress, "alreadyComplete": result.AlreadyComplete,
	}, nil
}

func generationRunIDFromWorkflowInput(input map[string]any) string {
	upstream, _ := input["upstream"].(map[string]any)
	for _, value := range upstream {
		output, _ := value.(map[string]any)
		if id, _ := output["generationRunId"].(string); strings.TrimSpace(id) != "" {
			return strings.TrimSpace(id)
		}
	}
	return ""
}

func (s *Service) executeEnsureStoryboardImagesActivity(ctx context.Context, input map[string]any, _ map[string]any) (map[string]any, error) {
	in, err := decodeStoryboardWorkflowInput(input)
	if err != nil {
		return nil, err
	}
	storyboardID := storyboardIDFromWorkflowInput(input, in)
	if storyboardID == "" {
		return nil, errors.New("ensure storyboard images requires an upstream storyboardId")
	}
	if !in.GenerateImages {
		return map[string]any{"storyboardId": storyboardID, "imagesRequested": false}, nil
	}
	if runID, ok := runstore.RunIDFromContext(ctx); ok {
		s.appendStoryboardStepAudit(ctx, runID, "image", domain.StepStarted)
	}
	progress, err := s.client.GetGenerationProgress(ctx, storyboardID)
	if err != nil {
		return nil, fmt.Errorf("inspect storyboard image progress: %w", err)
	}
	if progress == nil {
		return nil, errors.New("inspect storyboard image progress: empty response")
	}
	if storyboardShouldStartImages(progress) {
		if in.UseComicPagePipeline {
			if _, err := s.client.GenerateAllComicPages(ctx, storyboardID, grapery_client.GenerateAllComicPagesRequest{}); err != nil {
				return nil, err
			}
		} else if err := s.client.GenerateAllSceneImages(ctx, storyboardID, grapery_client.GenerateAllImagesRequest{}); err != nil {
			return nil, err
		}
	}
	seenImageActivity := progress != nil && (progress.IsGenerating || progress.HasPendingTasks || progress.StepKey == "image")
	output, err := s.waitForStoryboardProgress(ctx, storyboardID, generationPollTimeout(in.PollTimeoutSec), func(current *grapery_client.GenerationProgressResponse) bool {
		if storyboardImagesReady(current) {
			return true
		}
		if current != nil && (current.IsGenerating || current.HasPendingTasks || current.StepKey == "image") {
			seenImageActivity = true
		}
		return seenImageActivity && current != nil && !current.IsGenerating && !current.HasPendingTasks
	})
	if err != nil {
		return nil, err
	}
	output["imagesRequested"] = true
	if runID, ok := runstore.RunIDFromContext(ctx); ok {
		s.appendStoryboardStepAudit(ctx, runID, "consistency_audit", domain.StepSucceeded)
	}
	return output, nil
}

func decodeStoryboardWorkflowInput(input map[string]any) (domain.StoryboardGenerateInput, error) {
	payload, err := json.Marshal(input)
	if err != nil {
		return domain.StoryboardGenerateInput{}, err
	}
	var in domain.StoryboardGenerateInput
	if err := json.Unmarshal(payload, &in); err != nil {
		return in, err
	}
	if in.ClientRequestID == "" {
		in.ClientRequestID, _ = input["clientRequestId"].(string)
	}
	return in, nil
}

func storyboardIDFromWorkflowInput(input map[string]any, in domain.StoryboardGenerateInput) string {
	if id := strings.TrimSpace(in.DraftStoryboardID); id != "" {
		return id
	}
	upstream, _ := input["upstream"].(map[string]any)
	for _, value := range upstream {
		output, _ := value.(map[string]any)
		if id, _ := output["storyboardId"].(string); strings.TrimSpace(id) != "" {
			return strings.TrimSpace(id)
		}
	}
	return ""
}

type storyboardProgressDone func(*grapery_client.GenerationProgressResponse) bool

func (s *Service) waitForStoryboardProgress(ctx context.Context, storyboardID string, timeout time.Duration, done storyboardProgressDone) (map[string]any, error) {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		if runID, ok := runstore.RunIDFromContext(ctx); ok {
			if run, exists := s.store.GetRun(ctx, runID); exists && run.Status == domain.RunStatusCancelled {
				return nil, errors.New("workflow cancelled")
			}
		}
		progress, err := s.client.GetGenerationProgress(ctx, storyboardID)
		if err == nil && progress != nil {
			if runID, ok := runstore.RunIDFromContext(ctx); ok {
				s.updateStoryboardRunProgress(ctx, runID, progress)
			}
			if done(progress) {
				return storyboardProgressOutput(storyboardID, progress), nil
			}
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("storyboard %s generation poll timeout", storyboardID)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}

func storyboardContentReady(progress *grapery_client.GenerationProgressResponse) bool {
	if progress == nil {
		return false
	}
	switch strings.TrimSpace(progress.WorkflowStatus) {
	case "content_ready", "images_ready", "video_ready", "published":
		return true
	}
	return progress.StepKey == "image" || progress.StepKey == "consistency_audit"
}

func storyboardImagesReady(progress *grapery_client.GenerationProgressResponse) bool {
	if progress == nil {
		return false
	}
	switch strings.TrimSpace(progress.WorkflowStatus) {
	case "images_ready", "video_ready", "published":
		return true
	default:
		return false
	}
}

func storyboardShouldStartImages(progress *grapery_client.GenerationProgressResponse) bool {
	return progress != nil && !storyboardImagesReady(progress) && !progress.IsGenerating && !progress.HasPendingTasks
}

func storyboardProgressOutput(storyboardID string, progress *grapery_client.GenerationProgressResponse) map[string]any {
	return map[string]any{
		"storyboardId": storyboardID, "workflowStatus": progress.WorkflowStatus,
		"generationMessage": progress.GenerationMessage, "stepKey": progress.StepKey,
		"messageKey": progress.MessageKey, "stage": progress.Stage,
		"progress": progress.ProgressPercent, "currentStep": progress.StepKey,
	}
}
