package generation

import (
	"context"
	"fmt"

	"github.com/grapestree/fgrapery/grapery-agent/internal/agentauth"
	"github.com/grapestree/fgrapery/grapery-agent/internal/grapery_client"
)

func userIDFromContext(ctx context.Context) string {
	if claims, ok := agentauth.ClaimsFromContext(ctx); ok && claims != nil {
		return claims.UserID
	}
	return ""
}

func (s *Service) startFragmentPanelTask(ctx context.Context, userID string, req grapery_client.GenerateFragmentPanelRequest) (map[string]any, error) {
	if userID != "" && s.execFragmentPanel {
		resp, err := s.client.GenerateFragmentPanelForUser(ctx, userID, req)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"taskId":          resp.TaskID,
			"status":          resp.Status,
			"draftFragmentId": resp.DraftFragmentID,
		}, nil
	}
	if userID != "" && !s.execFragmentPanel {
		return nil, fmt.Errorf("AGENT_EXEC_FRAGMENT_PANEL_ENABLED is false; enable it on agent and grapery for token-only fragment-panel generation")
	}
	resp, err := s.client.GenerateFragmentPanel(ctx, req)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"taskId":          resp.TaskID,
		"status":          resp.Status,
		"draftFragmentId": resp.DraftFragmentID,
	}, nil
}

func (s *Service) pollFragmentPanelTask(ctx context.Context, userID, taskID string) (*grapery_client.FragmentPanelTaskStatus, error) {
	if s.execFragmentPanel && userID != "" {
		return s.client.GetFragmentPanelTaskForUser(ctx, userID, taskID)
	}
	return s.pollFragmentPanel(ctx, taskID)
}
