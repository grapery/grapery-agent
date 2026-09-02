package generation

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/grapestree/fgrapery/grapery-agent/internal/agentauth"
	"github.com/grapestree/fgrapery/grapery-agent/internal/domain"
	"github.com/grapestree/fgrapery/grapery-agent/internal/grapery_client"
	"github.com/grapestree/fgrapery/grapery-agent/internal/runstore"
)

func (s *Service) StartStoryboard(ctx context.Context, in domain.StoryboardGenerateInput) (*domain.GenerationRun, error) {
	input := map[string]any{
		"clientRequestId": in.ClientRequestID,
		"storyId":         in.StoryID,
		"rawInput":        in.RawInput,
		"sceneCount":      in.SceneCount,
	}
	run, err := s.store.CreateRun(ctx, domain.RunKindStoryboard, domain.AgentStoryboardDirector, in.RawInput, input)
	if err != nil {
		return nil, err
	}
	if run.Reused {
		return run, nil
	}
	if in.WorkflowReleaseID != "" {
		if err := s.store.UpdateRun(ctx, run.ID, func(current *domain.GenerationRun) {
			current.WorkflowReleaseID = in.WorkflowReleaseID
		}); err != nil {
			return nil, err
		}
	}
	execCtx := context.Background()
	if claims, ok := agentauth.ClaimsFromContext(ctx); ok {
		execCtx = agentauth.ContextWithClaims(execCtx, claims)
	}
	if token, ok := grapery_client.AuthTokenFromContext(ctx); ok {
		execCtx = grapery_client.ContextWithAuthToken(execCtx, token)
	}
	execCtx = runstore.ContextWithRunID(execCtx, run.ID)
	go s.executeStoryboard(execCtx, run.ID, in)
	return run, nil
}

func storyboardShouldPoll(in domain.StoryboardGenerateInput) bool {
	if in.PollProgress == nil {
		return true
	}
	return *in.PollProgress
}

func (s *Service) executeStoryboard(ctx context.Context, runID string, in domain.StoryboardGenerateInput) {
	ctx = runstore.ContextWithRunID(ctx, runID)
	s.markRunning(ctx, runID)

	sb, err := s.resolveStoryboardTarget(ctx, in)
	if err != nil {
		_ = s.finishRun(ctx, runID, domain.RunStatusFailed, nil, domain.ContentRef{}, 0, err.Error())
		return
	}
	content := domain.ContentRef{StoryboardID: sb.ID, StoryID: sb.StoryID}
	_ = s.store.UpdateRun(ctx, runID, func(r *domain.GenerationRun) {
		r.ContentIDs = content
		if r.Output == nil {
			r.Output = map[string]any{}
		}
		r.Output["storyboardId"] = sb.ID
		r.Output["draftStoryboardId"] = sb.ID
		r.Output["storyId"] = sb.StoryID
	})
	s.appendStoryboardStepAudit(ctx, runID, "context", domain.StepStarted)

	// create_storyboard with rawInput already triggers redesign pipeline in grapery (async).
	if in.RegenerateStructure {
		s.appendStoryboardStepAudit(ctx, runID, "bible_plan", domain.StepStarted)
		// This turn's message is the revision instruction; the storyboard's RawInput
		// still holds the original description and must not be overwritten by it.
		structureReq := grapery_client.GenerateStructureRequest{
			UserDirective: strings.TrimSpace(in.RawInput),
			SceneCount:    in.SceneCount,
			ComicStyle:    firstNonEmptyStoryboardStyle(in.ComicStyle, in.Style),
		}
		_, err = tracedClientCall(ctx, s.store, "regenerate_structure", map[string]any{
			"storyboardId":  sb.ID,
			"userDirective": structureReq.UserDirective,
		}, func(c context.Context) (map[string]any, error) {
			resp, err := s.client.GenerateStructure(c, sb.ID, structureReq)
			if err != nil {
				return nil, err
			}
			return map[string]any{"asyncAccepted": resp.AsyncAccepted}, nil
		})
		if err != nil {
			s.appendStoryboardStepAudit(ctx, runID, "bible_plan", domain.StepFailed)
			_ = s.finishRun(ctx, runID, domain.RunStatusFailed, nil, content, 0, err.Error())
			return
		}
	}

	output := map[string]any{"storyboardId": sb.ID, "storyId": sb.StoryID, "draftStoryboardId": sb.ID}
	lastStepKey := ""

	if storyboardShouldPoll(in) {
		timeout := generationPollTimeout(in.PollTimeoutSec)
		deadline := time.Now().Add(timeout)
		completed := false
		for time.Now().Before(deadline) {
			prog, err := s.client.GetGenerationProgress(ctx, sb.ID)
			if err == nil && prog != nil {
				s.updateStoryboardRunProgress(ctx, runID, prog)
				stepKey := strings.TrimSpace(prog.StepKey)
				if stepKey == "" && prog.MessageKey != "" {
					stepKey = prog.MessageKey
				}
				if stepKey != "" && stepKey != lastStepKey {
					s.appendStoryboardStepAudit(ctx, runID, stepKey, domain.StepStarted)
					lastStepKey = stepKey
				}
				output["workflowStatus"] = prog.WorkflowStatus
				output["generationMessage"] = prog.GenerationMessage
				output["stepKey"] = prog.StepKey
				output["messageKey"] = prog.MessageKey
				output["stage"] = prog.Stage
				output["progress"] = prog.ProgressPercent
				output["currentStep"] = prog.StepKey
				if !prog.IsGenerating && !prog.HasPendingTasks {
					completed = true
					break
				}
			}
			time.Sleep(3 * time.Second)
		}
		if !completed {
			_ = s.finishRun(ctx, runID, domain.RunStatusFailed, output, content, 0, "storyboard generation poll timeout")
			return
		}
	}

	if in.GenerateImages {
		s.appendStoryboardStepAudit(ctx, runID, "image", domain.StepStarted)
		if in.UseComicPagePipeline {
			_, err = tracedClientCall(ctx, s.store, "generate_all_comic_pages", map[string]any{"storyboardId": sb.ID}, func(c context.Context) (map[string]any, error) {
				resp, err := s.client.GenerateAllComicPages(c, sb.ID, grapery_client.GenerateAllComicPagesRequest{PageAspectRatio: in.AspectRatio})
				if err != nil {
					return nil, err
				}
				return map[string]any{"total": resp.Total, "successCount": resp.SuccessCount, "failedCount": resp.FailedCount}, nil
			})
		} else {
			_, err = tracedClientCall(ctx, s.store, "generate_all_scene_images", map[string]any{"storyboardId": sb.ID}, func(c context.Context) (map[string]any, error) {
				if err := s.client.GenerateAllSceneImages(c, sb.ID, grapery_client.GenerateAllImagesRequest{AspectRatio: in.AspectRatio}); err != nil {
					return nil, err
				}
				return map[string]any{"success": true}, nil
			})
		}
		if err != nil {
			s.appendStoryboardStepAudit(ctx, runID, "image", domain.StepFailed)
			_ = s.finishRun(ctx, runID, domain.RunStatusFailed, output, content, 0, err.Error())
			return
		}
		// Final poll after images kickoff.
		if storyboardShouldPoll(in) {
			timeout := generationPollTimeout(in.PollTimeoutSec)
			deadline := time.Now().Add(timeout)
			completed := false
			for time.Now().Before(deadline) {
				prog, err := s.client.GetGenerationProgress(ctx, sb.ID)
				if err == nil && prog != nil {
					s.updateStoryboardRunProgress(ctx, runID, prog)
					output["workflowStatus"] = prog.WorkflowStatus
					output["generationMessage"] = prog.GenerationMessage
					output["stepKey"] = prog.StepKey
					output["messageKey"] = prog.MessageKey
					output["stage"] = prog.Stage
					output["progress"] = prog.ProgressPercent
					output["currentStep"] = prog.StepKey
					if !prog.IsGenerating && !prog.HasPendingTasks {
						completed = true
						break
					}
				}
				time.Sleep(3 * time.Second)
			}
			if !completed {
				_ = s.finishRun(ctx, runID, domain.RunStatusFailed, output, content, 0, "storyboard image generation poll timeout")
				return
			}
		}
		s.appendStoryboardStepAudit(ctx, runID, "consistency_audit", domain.StepSucceeded)
	}

	output["progress"] = 100
	_ = s.finishRun(ctx, runID, domain.RunStatusSucceeded, output, content, 0, "")
	if run, ok := s.store.GetRun(ctx, runID); ok {
		s.traceArtifact(ctx, run)
	}
}

func (s *Service) updateStoryboardRunProgress(ctx context.Context, runID string, prog *grapery_client.GenerationProgressResponse) {
	if prog == nil {
		return
	}
	_ = s.store.UpdateRun(ctx, runID, func(r *domain.GenerationRun) {
		if r.Output == nil {
			r.Output = map[string]any{}
		}
		r.Output["workflowStatus"] = prog.WorkflowStatus
		r.Output["generationMessage"] = prog.GenerationMessage
		r.Output["stepKey"] = prog.StepKey
		r.Output["messageKey"] = prog.MessageKey
		r.Output["stage"] = prog.Stage
		r.Output["progress"] = prog.ProgressPercent
		r.Output["currentStep"] = prog.StepKey
		r.Output["isGenerating"] = prog.IsGenerating
		r.Output["hasPendingTasks"] = prog.HasPendingTasks
		if r.ContentIDs.StoryboardID == "" {
			r.ContentIDs.StoryboardID = prog.StoryboardID
		}
	})
}

func (s *Service) appendStoryboardStepAudit(ctx context.Context, runID, stepName string, status domain.StepAuditStatus) {
	stepName = strings.TrimSpace(stepName)
	if stepName == "" {
		return
	}
	_ = s.store.UpdateRun(ctx, runID, func(r *domain.GenerationRun) {
		for _, existing := range r.StepAudits {
			if existing.StepName == stepName && existing.Status == status {
				return
			}
		}
		seq := len(r.StepAudits) + 1
		r.StepAudits = append(r.StepAudits, domain.GenerationStepAudit{
			ID:       uuid.New().String(),
			Sequence: seq,
			StepName: stepName,
			Attempt:  1,
			Status:   status,
		})
		if r.Output == nil {
			r.Output = map[string]any{}
		}
		if status == domain.StepStarted {
			r.Output["currentStep"] = stepName
			r.Output["stepKey"] = stepName
			r.Output["messageKey"] = storyboardStepMessageKey(stepName)
		}
	})
}

func firstNonEmptyStoryboardStyle(values ...string) string {
	for _, v := range values {
		if trimmed := strings.TrimSpace(v); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func storyboardStepMessageKey(step string) string {
	switch strings.TrimSpace(step) {
	case "context":
		return "storyboard_generation_reading_context"
	case "bible_plan":
		return "storyboard_generation_planning_bible"
	case "scene_plan":
		return "storyboard_generation_writing_scenes"
	case "image":
		return "storyboard_generation_generating_images"
	case "consistency_audit":
		return "storyboard_generation_checking_consistency"
	default:
		return "storyboard_generation_in_progress"
	}
}

// resolveStoryboardTarget reuses the draft the caller already created for this turn.
// The client creates the draft first so it can poll progress even if the SSE drops,
// and grapery already kicks off the content pipeline on create — creating another
// storyboard here would orphan one and stream progress for the wrong ID.
func (s *Service) resolveStoryboardTarget(ctx context.Context, in domain.StoryboardGenerateInput) (*grapery_client.StoryboardResponse, error) {
	if id := strings.TrimSpace(in.DraftStoryboardID); id != "" {
		return &grapery_client.StoryboardResponse{ID: id, StoryID: strings.TrimSpace(in.StoryID)}, nil
	}
	return s.createStoryboard(ctx, in)
}

func (s *Service) createStoryboard(ctx context.Context, in domain.StoryboardGenerateInput) (*grapery_client.StoryboardResponse, error) {
	out, err := tracedClientCall(ctx, s.store, "create_storyboard", map[string]any{"storyId": in.StoryID}, func(c context.Context) (map[string]any, error) {
		var charRefs []grapery_client.CharacterRef
		for i, id := range in.CharacterIDs {
			if strings.TrimSpace(id) == "" {
				continue
			}
			charRefs = append(charRefs, grapery_client.CharacterRef{
				CharacterID: id,
				Order:       i + 1,
			})
		}
		parentID := strings.TrimSpace(in.ParentStoryboardID)
		comicStyle := strings.TrimSpace(in.ComicStyle)
		if comicStyle == "" {
			comicStyle = strings.TrimSpace(in.Style)
		}
		idempotencyKey := strings.TrimSpace(in.ClientRequestID)
		if idempotencyKey == "" {
			if runID, ok := runstore.RunIDFromContext(c); ok {
				idempotencyKey = "storyboard:" + runID
			}
		}
		requestContext := grapery_client.ContextWithIdempotencyKey(c, idempotencyKey)
		workflowRunID, _ := runstore.RunIDFromContext(c)
		resp, err := s.client.CreateStoryboard(requestContext, grapery_client.CreateStoryboardRequest{
			StoryID:              in.StoryID,
			ParentID:             parentID,
			Title:                in.Title,
			RawInput:             in.RawInput,
			SceneCount:           defaultInt(in.SceneCount, 3),
			CharacterRefs:        charRefs,
			UseComicPagePipeline: in.UseComicPagePipeline,
			WorkflowReleaseID:    in.WorkflowReleaseID,
			WorkflowRunID:        workflowRunID,
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
