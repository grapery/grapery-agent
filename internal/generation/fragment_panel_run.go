package generation

import (
	"context"
	"time"

	"github.com/grapestree/fgrapery/grapery-agent/internal/agentauth"
	"github.com/grapestree/fgrapery/grapery-agent/internal/domain"
	"github.com/grapestree/fgrapery/grapery-agent/internal/grapery_client"
	"github.com/grapestree/fgrapery/grapery-agent/internal/runstore"
)

func (s *Service) StartFragmentPanel(ctx context.Context, in domain.FragmentPanelGenerateInput) (*domain.GenerationRun, error) {
	input := map[string]any{
		"clientRequestId":   in.ClientRequestID,
		"userInput":         in.UserInput,
		"referenceImageUrl": in.ReferenceImageURL,
		"panelCount":        in.PanelCount,
	}
	run, err := s.store.CreateRun(ctx, domain.RunKindFragmentPanel, domain.AgentFragmentPanelCreator, in.UserInput, input)
	if err != nil {
		return nil, err
	}
	if run.Reused {
		return run, nil
	}
	execCtx := context.Background()
	if claims, ok := agentauth.ClaimsFromContext(ctx); ok {
		execCtx = agentauth.ContextWithClaims(execCtx, claims)
	}
	execCtx = runstore.ContextWithRunID(execCtx, run.ID)
	go s.executeFragmentPanel(execCtx, run.ID, in)
	return run, nil
}

func (s *Service) executeFragmentPanel(ctx context.Context, runID string, in domain.FragmentPanelGenerateInput) {
	ctx = runstore.ContextWithRunID(ctx, runID)
	s.markRunning(ctx, runID)

	pollInterval := time.Duration(defaultInt(in.PollIntervalSec, 5)) * time.Second
	pollTimeout := generationPollTimeout(in.PollTimeoutSec)
	deadline := time.Now().Add(pollTimeout)
	req := grapery_client.GenerateFragmentPanelRequest{
		UserInput:              in.UserInput,
		ReferenceImageURL:      in.ReferenceImageURL,
		Style:                  in.Style,
		PanelCount:             in.PanelCount,
		Visibility:             defaultString(in.Visibility, "private"),
		Topic:                  in.Topic,
		AspectRatio:            in.AspectRatio,
		DialogueMode:           in.DialogueMode,
		ConsistencyLevel:       in.ConsistencyLevel,
		EnableReferenceAssets:  in.EnableReferenceAssets,
		IncludeGenerationTrace: in.IncludeGenerationTrace,
	}

	var startResp map[string]any
	err := runstore.TracedStep(ctx, s.store, domain.GenerationStepAudit{
		StepName:     "panel_generate_start",
		Attempt:      1,
		BusinessType: domain.RunKindFragmentPanel,
		AgentVersion: domain.AgentFragmentPanelCreator,
		Provider:     panelProviderLabel(s.execFragmentPanel, userIDFromContext(ctx)),
		Prompt:       in.UserInput,
		InputRefs:    nonEmptyRefs(in.ReferenceImageURL),
	}, func(c context.Context) error {
		var callErr error
		startResp, callErr = tracedClientCall(c, s.store, "generate_panel_fragment", map[string]any{"userInput": in.UserInput}, func(cc context.Context) (map[string]any, error) {
			return s.startFragmentPanelTask(cc, userIDFromContext(cc), req)
		})
		return callErr
	})
	if err != nil {
		_ = s.finishRun(ctx, runID, domain.RunStatusFailed, nil, domain.ContentRef{}, 0, err.Error())
		return
	}

	taskID := str(startResp["taskId"])
	userID := userIDFromContext(ctx)
	content := domain.ContentRef{
		TaskID:     taskID,
		FragmentID: str(startResp["draftFragmentId"]),
	}
	_ = s.store.UpdateRun(ctx, runID, func(r *domain.GenerationRun) {
		r.Status = domain.RunStatusWaiting
		r.ContentIDs = content
	})

	var final *grapery_client.FragmentPanelTaskStatus
	for time.Now().Before(deadline) {
		if run, ok := s.store.GetRun(ctx, runID); ok && run.Status == domain.RunStatusCancelled {
			return
		}
		status, pollErr := s.pollFragmentPanelTask(ctx, userID, taskID)
		if pollErr != nil {
			_ = s.finishRun(ctx, runID, domain.RunStatusFailed, nil, content, 0, pollErr.Error())
			return
		}
		switch status.Status {
		case "completed", "succeeded", "success":
			final = status
			goto done
		case "failed", "cancelled", "canceled":
			_ = s.store.RecordStepAudit(ctx, runID, domain.GenerationStepAudit{
				StepName:     "panel_render",
				Attempt:      1,
				TaskID:       taskID,
				BusinessType: domain.RunKindFragmentPanel,
				AgentVersion: domain.AgentFragmentPanelCreator,
				Status:       domain.StepFailed,
				ErrorMessage: status.Error,
				StartedAt:    time.Now(),
			})
			_ = s.finishRun(ctx, runID, domain.RunStatusFailed, map[string]any{"status": status.Status, "error": status.Error}, content, 0, status.Error)
			return
		}
		time.Sleep(pollInterval)
	}
	_ = s.store.RecordStepAudit(ctx, runID, domain.GenerationStepAudit{
		StepName:     "panel_render",
		Attempt:      1,
		TaskID:       taskID,
		BusinessType: domain.RunKindFragmentPanel,
		AgentVersion: domain.AgentFragmentPanelCreator,
		Status:       domain.StepFailed,
		ErrorMessage: "poll timeout",
		StartedAt:    time.Now(),
	})
	_ = s.finishRun(ctx, runID, domain.RunStatusFailed, nil, content, 0, "poll timeout")
	return

done:
	output := map[string]any{
		"taskId":          taskID,
		"status":          final.Status,
		"draftFragmentId": final.DraftFragmentID,
		"combinedContent": final.CombinedContent,
	}
	if len(final.Panels) > 0 {
		panels := make([]map[string]any, 0, len(final.Panels))
		for _, p := range final.Panels {
			panels = append(panels, map[string]any{"index": p.Index, "imageUrl": p.ImageURL, "caption": p.Caption})
		}
		output["panels"] = panels
	}
	content.FragmentID = final.DraftFragmentID
	_ = s.store.RecordStepAudit(ctx, runID, domain.GenerationStepAudit{
		StepName:     "panel_render",
		Attempt:      1,
		TaskID:       taskID,
		BusinessType: domain.RunKindFragmentPanel,
		BusinessID:   final.DraftFragmentID,
		AgentVersion: domain.AgentFragmentPanelCreator,
		Status:       domain.StepSucceeded,
		TotalTokens:  final.TokensUsed,
		InputRefs:    nonEmptyRefs(in.ReferenceImageURL),
		StartedAt:    time.Now(),
	})
	_ = s.finishRun(ctx, runID, domain.RunStatusSucceeded, output, content, final.TokensUsed, "")
	if run, ok := s.store.GetRun(ctx, runID); ok {
		s.traceArtifact(ctx, run)
	}
}

func panelProviderLabel(execPolicy bool, userID string) string {
	if execPolicy && userID != "" {
		return "grapery-agent-policy"
	}
	return "grapery"
}

func nonEmptyRefs(refs ...string) []string {
	out := make([]string, 0, len(refs))
	for _, r := range refs {
		if r != "" {
			out = append(out, r)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (s *Service) pollFragmentPanel(ctx context.Context, taskID string) (*grapery_client.FragmentPanelTaskStatus, error) {
	var status *grapery_client.FragmentPanelTaskStatus
	_, err := tracedClientCall(ctx, s.store, "poll_panel_task_status", map[string]any{"taskId": taskID}, func(c context.Context) (map[string]any, error) {
		st, err := s.client.GetFragmentPanelTaskStatus(c, taskID)
		if err != nil {
			return nil, err
		}
		status = st
		return map[string]any{
			"taskId": st.TaskID, "status": st.Status, "progress": st.Progress,
			"currentStep": st.CurrentStep,
		}, nil
	})
	if err != nil {
		return nil, err
	}
	return status, nil
}
