package generation

import (
	"context"
	"fmt"

	"github.com/grapestree/fgrapery/grapery-agent/internal/agentauth"
	"github.com/grapestree/fgrapery/grapery-agent/internal/domain"
	"github.com/grapestree/fgrapery/grapery-agent/internal/grapery_client"
	"github.com/grapestree/fgrapery/grapery-agent/internal/prompt"
	"github.com/grapestree/fgrapery/grapery-agent/internal/runstore"
)

func (s *Service) StartBranchBatch(ctx context.Context, in domain.BranchExploreInput) (*domain.GenerationRun, error) {
	strategies := in.Strategies
	if len(strategies) == 0 {
		count := defaultInt(in.BranchCount, 3)
		if count > len(prompt.DefaultBranchStrategies) {
			count = len(prompt.DefaultBranchStrategies)
		}
		strategies = prompt.DefaultBranchStrategies[:count]
	}
	input := map[string]any{
		"clientRequestId":    in.ClientRequestID,
		"parentStoryboardId": in.ParentStoryboardID,
		"strategies":         strategies,
		"seedPrompt":         in.SeedPrompt,
		"workflowReleaseId":  in.WorkflowReleaseID,
	}
	intent := in.SeedPrompt
	if intent == "" {
		intent = fmt.Sprintf("branch from %s", in.ParentStoryboardID)
	}
	run, err := s.store.CreateRun(ctx, domain.RunKindBranchBatch, domain.AgentBranchExplorer, intent, input)
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
	go s.executeBranchBatch(execCtx, run.ID, in, strategies)
	return run, nil
}

func (s *Service) executeBranchBatch(ctx context.Context, runID string, in domain.BranchExploreInput, strategies []string) {
	ctx = runstore.ContextWithRunID(ctx, runID)
	s.markRunning(ctx, runID)

	batch := domain.BranchBatchResult{
		ParentStoryboardID: in.ParentStoryboardID,
		SeedPrompt:         in.SeedPrompt,
		Candidates:         make([]domain.BranchCandidate, 0, len(strategies)),
	}
	totalTokens := 0

	for i, strategy := range strategies {
		rawInput := prompt.BuildBranchRawInput(in.SeedPrompt, strategy)
		childRun, childErr := s.store.CreateRun(ctx, domain.RunKindStoryboard, domain.AgentBranchExplorer, rawInput, map[string]any{
			"parentStoryboardId": in.ParentStoryboardID,
			"strategy":           strategy,
			"parentRunId":        runID,
		})
		if childErr != nil || childRun == nil {
			batch.Candidates = append(batch.Candidates, domain.BranchCandidate{
				BranchIndex: i + 1,
				Strategy:    strategy,
				RawInput:    rawInput,
				Metadata:    map[string]any{"error": "failed to create child run"},
			})
			continue
		}
		_ = s.store.UpdateRun(ctx, childRun.ID, func(r *domain.GenerationRun) {
			r.ParentRunID = runID
			r.BranchIndex = i + 1
			r.Strategy = strategy
		})

		cand := domain.BranchCandidate{
			BranchIndex:       i + 1,
			Strategy:          strategy,
			RawInput:          rawInput,
			NarrativeHook:     prompt.StrategyNarrativeHook(strategy),
			DiffFromParent:    prompt.StrategyDiff(strategy),
			VisualFeasibility: "high",
			CommunityAppeal:   prompt.StrategyAppeal(strategy),
			RunID:             childRun.ID,
		}

		childCtx := runstore.ContextWithRunID(ctx, childRun.ID)
		out, err := tracedClientCall(childCtx, s.store, "continue_storyboard", map[string]any{
			"parentStoryboardId": in.ParentStoryboardID,
			"strategy":           strategy,
		}, func(c context.Context) (map[string]any, error) {
			resp, err := s.client.ContinueStoryboard(c, in.ParentStoryboardID, grapery_client.ContinueStoryboardRequest{
				RawInput:          rawInput,
				SceneCount:        defaultInt(in.SceneCount, 3),
				Characters:        in.Characters,
				ComicStyle:        in.ComicStyle,
				WorkflowReleaseID: in.WorkflowReleaseID,
				WorkflowRunID:     runID,
			})
			if err != nil {
				return nil, err
			}
			if resp.NewStoryboard == nil {
				return nil, fmt.Errorf("continue_storyboard: empty response")
			}
			return map[string]any{
				"id":      resp.NewStoryboard.ID,
				"storyId": resp.NewStoryboard.StoryID,
				"tokens":  resp.TokensUsed,
			}, nil
		})
		if err != nil {
			cand.Metadata = map[string]any{"error": err.Error()}
			if childRun != nil {
				_ = s.finishRun(ctx, childRun.ID, domain.RunStatusFailed, nil, domain.ContentRef{}, 0, err.Error())
			}
		} else {
			cand.StoryboardID = str(out["id"])
			cand.StoryID = str(out["storyId"])
			if childRun != nil {
				tokens := int(num(out["tokens"]))
				totalTokens += tokens
				_ = s.finishRun(ctx, childRun.ID, domain.RunStatusSucceeded, out, domain.ContentRef{
					StoryboardID: cand.StoryboardID,
					StoryID:      cand.StoryID,
				}, tokens, "")
			}
		}
		batch.Candidates = append(batch.Candidates, cand)
	}

	// branchBatch 与内容引用必须在 finishRun 的同一次写入中持久化：
	// 终态运行在 GraperyStore.UpdateRun 中不再回写，事后追加只会留在内存。
	output := map[string]any{"candidateCount": len(batch.Candidates), "tokensUsed": totalTokens, "branchBatch": batch}
	branchIDs := make([]string, 0, len(batch.Candidates))
	for _, c := range batch.Candidates {
		if c.StoryboardID != "" {
			branchIDs = append(branchIDs, c.StoryboardID)
		}
	}
	content := domain.ContentRef{BranchIDs: branchIDs}
	if len(branchIDs) > 0 {
		content.StoryboardID = branchIDs[0]
	}
	_ = s.finishRun(ctx, runID, domain.RunStatusSucceeded, output, content, totalTokens, "")

	_ = s.store.AppendArtifact(ctx, &domain.RLArtifact{
		Type:        domain.ArtifactTypeGenerationTrace,
		RunID:       runID,
		Prompt:      in.SeedPrompt,
		BranchBatch: &batch,
	})
}
