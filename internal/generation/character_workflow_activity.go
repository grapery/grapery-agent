package generation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/grapestree/fgrapery/grapery-agent/internal/domain"
	"github.com/grapestree/fgrapery/grapery-agent/internal/runstore"
)

func (s *Service) executeLegacyCharacterActivity(ctx context.Context, input, config map[string]any) (map[string]any, error) {
	input = applyWorkflowInputDefaults(input, config)
	payload, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	var in domain.CharacterGenerateInput
	if err = json.Unmarshal(payload, &in); err != nil {
		return nil, err
	}
	in.CreativeControls = controlsForActivity(ctx)
	if in.ClientRequestID == "" {
		in.ClientRequestID = stringFromAny(input["clientRequestId"])
	}
	child, err := s.StartCharacter(ctx, in)
	if err != nil {
		return nil, err
	}
	if parentID, ok := runstore.RunIDFromContext(ctx); ok {
		_ = s.store.UpdateRun(ctx, child.ID, func(run *domain.GenerationRun) { run.ParentRunID = parentID })
	}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		current, ok := s.store.GetRun(ctx, child.ID)
		if !ok {
			return nil, fmt.Errorf("character child run disappeared: %s", child.ID)
		}
		s.syncWorkflowChildRun(ctx, current)
		switch current.Status {
		case domain.RunStatusSucceeded:
			out := cloneWorkflowInput(current.Output)
			out["childRunId"] = current.ID
			out["tokensUsed"] = current.TokensUsed
			return out, nil
		case domain.RunStatusFailed:
			return nil, errors.New(current.Error)
		case domain.RunStatusCancelled:
			return nil, errors.New("character child run cancelled")
		}
		if parentID, ok := runstore.RunIDFromContext(ctx); ok {
			if parent, exists := s.store.GetRun(ctx, parentID); exists && parent.Status == domain.RunStatusCancelled {
				_ = s.CancelRun(ctx, child.ID)
				return nil, errors.New("workflow cancelled")
			}
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}
