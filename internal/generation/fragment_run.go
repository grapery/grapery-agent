package generation

import (
	"context"
	"fmt"
	"time"

	"github.com/grapestree/fgrapery/grapery-agent/internal/domain"
	"github.com/grapestree/fgrapery/grapery-agent/internal/grapery_client"
	"github.com/grapestree/fgrapery/grapery-agent/internal/runstore"
)

func (s *Service) StartFragment(ctx context.Context, in domain.FragmentGenerateInput) (*domain.GenerationRun, error) {
	input := map[string]any{
		"userInput":        in.UserInput,
		"imageCount":       in.ImageCount,
		"style":            in.Style,
		"mood":             in.Mood,
		"consistencyLevel": in.ConsistencyLevel,
	}
	run, err := s.store.CreateRun(ctx, domain.RunKindFragment, domain.AgentFragmentCreator, in.UserInput, input)
	if err != nil {
		return nil, err
	}
	ctx = runstore.ContextWithRunID(ctx, run.ID)
	s.markRunning(ctx, run.ID)

	go s.executeFragment(context.Background(), run.ID, in)
	return run, nil
}

func (s *Service) executeFragment(ctx context.Context, runID string, in domain.FragmentGenerateInput) {
	ctx = runstore.ContextWithRunID(ctx, runID)
	pollInterval := time.Duration(defaultInt(in.PollIntervalSec, 5)) * time.Second
	pollTimeout := time.Duration(defaultInt(in.PollTimeoutSec, 600)) * time.Second
	deadline := time.Now().Add(pollTimeout)

	req := grapery_client.GenerateFragmentRequest{
		UserInput:        in.UserInput,
		ImageUrls:        in.ReferenceImages,
		ImageCount:       defaultInt(in.ImageCount, 3),
		Style:            in.Style,
		Mood:             in.Mood,
		Length:           in.Length,
		Language:         defaultString(in.Language, "zh-Hans"),
		Visibility:       defaultString(in.Visibility, "private"),
		AspectRatio:      in.AspectRatio,
		ConsistencyLevel: in.ConsistencyLevel,
	}

	startResp, err := s.startFragmentTask(ctx, req)
	if err != nil {
		_ = s.finishRun(ctx, runID, domain.RunStatusFailed, nil, domain.ContentRef{}, 0, err.Error())
		return
	}

	content := domain.ContentRef{
		TaskID:     startResp.TaskID,
		FragmentID: startResp.DraftFragmentID,
	}
	_ = s.store.UpdateRun(ctx, runID, func(r *domain.GenerationRun) {
		r.Status = domain.RunStatusWaiting
		r.ContentIDs = content
	})

	var final *grapery_client.FragmentTaskStatus
	for time.Now().Before(deadline) {
		status, pollErr := s.pollFragment(ctx, startResp.TaskID)
		if pollErr != nil {
			_ = s.finishRun(ctx, runID, domain.RunStatusFailed, nil, content, 0, pollErr.Error())
			return
		}
		switch status.Status {
		case "completed", "succeeded", "success":
			final = status
			goto done
		case "failed", "cancelled", "canceled":
			_ = s.finishRun(ctx, runID, domain.RunStatusFailed, map[string]any{"status": status.Status, "error": status.Error}, content, 0, status.Error)
			return
		}
		time.Sleep(pollInterval)
	}
	_ = s.finishRun(ctx, runID, domain.RunStatusFailed, nil, content, 0, "poll timeout")
	return

done:
	output := map[string]any{
		"taskId": startResp.TaskID,
		"status": final.Status,
	}
	tokens := 0
	if final.Result != nil {
		output["content"] = final.Result.Content
		output["imageUrls"] = final.Result.ImageUrls
		output["aspectRatio"] = final.Result.AspectRatio
		tokens = final.Result.TokensUsed
	}
	_ = s.finishRun(ctx, runID, domain.RunStatusSucceeded, output, content, tokens, "")
	if run, ok := s.store.GetRun(ctx, runID); ok {
		s.traceArtifact(ctx, run)
	}
}

func (s *Service) startFragmentTask(ctx context.Context, req grapery_client.GenerateFragmentRequest) (*grapery_client.GenerateFragmentResponse, error) {
	out, err := tracedClientCall(ctx, s.store, "extract_elements", map[string]any{"userInput": req.UserInput}, func(c context.Context) (map[string]any, error) {
		resp, err := s.client.GenerateFragment(c, req)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"taskId":          resp.TaskID,
			"status":          resp.Status,
			"draftFragmentId": resp.DraftFragmentID,
		}, nil
	})
	if err != nil {
		return nil, err
	}
	return &grapery_client.GenerateFragmentResponse{
		TaskID:          str(out["taskId"]),
		Status:          str(out["status"]),
		DraftFragmentID: str(out["draftFragmentId"]),
	}, nil
}

func (s *Service) pollFragment(ctx context.Context, taskID string) (*grapery_client.FragmentTaskStatus, error) {
	out, err := tracedClientCall(ctx, s.store, "poll_task_status", map[string]any{"taskId": taskID}, func(c context.Context) (map[string]any, error) {
		st, err := s.client.GetFragmentTaskStatus(c, taskID)
		if err != nil {
			return nil, err
		}
		m := map[string]any{
			"taskId": st.TaskID, "status": st.Status, "progress": st.Progress,
			"currentStep": st.CurrentStep, "error": st.Error,
		}
		if st.Result != nil {
			m["result"] = st.Result
		}
		return m, nil
	})
	if err != nil {
		return nil, err
	}
	st := &grapery_client.FragmentTaskStatus{
		TaskID: str(out["taskId"]), Status: str(out["status"]),
		Progress: num(out["progress"]), CurrentStep: str(out["currentStep"]), Error: str(out["error"]),
	}
	if r, ok := out["result"].(*grapery_client.FragmentTaskResult); ok {
		st.Result = r
	}
	return st, nil
}

func defaultInt(v, d int) int {
	if v <= 0 {
		return d
	}
	return v
}

func defaultString(v, d string) string {
	if v == "" {
		return d
	}
	return v
}

func str(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}

func num(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	default:
		return 0
	}
}
