package generation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"text/template"
	"time"

	"github.com/grapestree/fgrapery/grapery-agent/internal/domain"
	"github.com/grapestree/fgrapery/grapery-agent/internal/runstore"
	workflowruntime "github.com/grapestree/fgrapery/grapery-agent/internal/workflow"
)

const workflowCheckpointVersion = 3

type workflowCheckpoint struct {
	Version         int                                     `json:"version"`
	ReleaseID       string                                  `json:"releaseId"`
	PromptSnapshots map[string]domain.PromptTemplateVersion `json:"promptSnapshots,omitempty"`
	Input           map[string]any                          `json:"input"`
	NodeOutputs     map[string]map[string]any               `json:"nodeOutputs"`
	NodeAttempts    map[string]int                          `json:"nodeAttempts,omitempty"`
	StartedAt       time.Time                               `json:"startedAt"`
	UpdatedAt       time.Time                               `json:"updatedAt"`
}

type recoveredRunClaimer interface {
	ClaimRun(ctx context.Context, id string) (bool, error)
}

type generationLeaseProvider interface {
	GenerationLeaseValue(runID string) string
}

func (s *Service) newWorkflowActivityRegistry() *workflowruntime.ActivityRegistry {
	registry := workflowruntime.NewActivityRegistry()
	_ = registry.Register("ai.runtime.plan", s.executeAIPlannerActivity)
	_ = registry.Register("legacy.fragment.generate", s.executeLegacyFragmentActivity)
	_ = registry.Register("legacy.storyboard.branch", s.executeLegacyStoryboardBranchActivity)
	_ = registry.Register("legacy.storyboard.generate", s.executeLegacyStoryboardActivity)
	_ = registry.Register("storyboard.ensure_draft", s.executeEnsureStoryboardDraftActivity)
	_ = registry.Register("storyboard.generate_bible_plan", s.executeGenerateStoryboardBiblePlanActivity)
	_ = registry.Register("storyboard.generate_scene_plan", s.executeGenerateStoryboardScenePlanActivity)
	_ = registry.Register("storyboard.review_content", s.executeReviewStoryboardContentActivity)
	_ = registry.Register("storyboard.persist_content", s.executePersistStoryboardContentActivity)
	_ = registry.Register("storyboard.await_content", s.executeAwaitStoryboardContentActivity)
	_ = registry.Register("storyboard.ensure_images", s.executeEnsureStoryboardImagesActivity)
	return registry
}

func (s *Service) executeLegacyFragmentActivity(ctx context.Context, input map[string]any, config map[string]any) (map[string]any, error) {
	input = applyWorkflowInputDefaults(input, config)
	input, err := applyLegacyWorkflowPrompt(input, config, "userInput")
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	var fragmentInput domain.FragmentGenerateInput
	if err := json.Unmarshal(payload, &fragmentInput); err != nil {
		return nil, err
	}
	if fragmentInput.ClientMessageID == "" {
		fragmentInput.ClientMessageID, _ = input["clientRequestId"].(string)
	}
	child, err := s.StartFragment(ctx, fragmentInput)
	if err != nil {
		return nil, err
	}
	if parentID, ok := runstore.RunIDFromContext(ctx); ok {
		_ = s.store.UpdateRun(ctx, child.ID, func(run *domain.GenerationRun) { run.ParentRunID = parentID })
	}
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		current, ok := s.store.GetRun(ctx, child.ID)
		if !ok {
			return nil, fmt.Errorf("fragment child run disappeared: %s", child.ID)
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
			return nil, errors.New("fragment child run cancelled")
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

func (s *Service) StartWorkflow(ctx context.Context, in domain.WorkflowStartInput) (*domain.GenerationRun, error) {
	if strings.TrimSpace(in.Surface) == "" || strings.TrimSpace(in.Action) == "" {
		return nil, errors.New("workflow surface and action are required")
	}
	if existing, err := s.findWorkflowByClientRequest(ctx, in); err != nil {
		return nil, err
	} else if existing != nil {
		existing.Reused = true
		return existing, nil
	}
	enrichedInput, err := s.enrichWorkflowRoutingInput(ctx, in.Input)
	if err != nil {
		return nil, err
	}
	in.Input = enrichedInput

	// 固定版本试运行（已通过内部密钥鉴权）：与恢复路径一致，直接按
	// ReleaseID 取不可变版本执行，不要求该版本是路由当前生效版本。
	// 正常客户端仍走 resolve，保证只执行路由指向的版本。
	var release *domain.WorkflowRelease
	var selection map[string]any
	if in.TestRun {
		requested := strings.TrimSpace(in.ReleaseID)
		if requested == "" {
			return nil, errors.New("test run requires an explicit releaseId")
		}
		release, err = s.client.GetWorkflowRelease(ctx, requested)
		if err != nil {
			return nil, fmt.Errorf("load pinned release for test run: %w", err)
		}
		if release.Status != "released" && release.Status != "active" {
			return nil, fmt.Errorf("test run release %s is not executable", requested)
		}
		selection = map[string]any{"testRun": true, "pinnedReleaseId": requested}
	} else {
		resolution, err := s.client.ResolveWorkflow(ctx, in.Surface, in.Action, in.TenantID, in.Input)
		if err != nil {
			return nil, err
		}
		if resolution == nil || strings.TrimSpace(resolution.Entry.Release.ID) == "" {
			return nil, fmt.Errorf("no active workflow binding for %s/%s", in.Surface, in.Action)
		}
		release = &resolution.Entry.Release
		if requested := strings.TrimSpace(in.ReleaseID); requested != "" && requested != release.ID {
			return nil, fmt.Errorf("workflow release %s is not active for %s/%s", requested, in.Surface, in.Action)
		}
		selection = map[string]any{
			"bindingId":           resolution.Entry.Binding.ID,
			"routerVersion":       resolution.RouterVersion,
			"profile":             resolution.Profile,
			"routeReason":         resolution.RouteReason,
			"confidence":          resolution.Confidence,
			"fallback":            resolution.Fallback,
			"candidateReleaseIds": append([]string(nil), resolution.CandidateIDs...),
		}
	}
	if err := workflowruntime.ValidateJSONSchema("workflow input", release.Definition.InputSchema, in.Input); err != nil {
		return nil, err
	}
	promptSnapshots, err := s.resolvePromptSnapshots(ctx, release.PromptBundle)
	if err != nil {
		return nil, err
	}
	attachPromptSnapshots(release, promptSnapshots)
	compiled, err := workflowruntime.Compile(release, s.workflowActivities)
	if err != nil {
		return nil, err
	}
	input := cloneWorkflowInput(in.Input)
	input["clientRequestId"] = in.ClientRequestID
	input["workflowReleaseId"] = release.ID
	input["workflowSurface"] = in.Surface
	input["workflowAction"] = in.Action
	input["workflowManagedStoryboardStages"] = workflowManagesStoryboardStages(release.Definition)
	if in.TestRun {
		input["workflowTestRun"] = true
	}
	input["workflowSelection"] = selection
	run, err := s.store.CreateRun(ctx, domain.RunKindWorkflow, domain.AgentWorkflowRuntime, release.Name, input)
	if err != nil {
		return nil, err
	}
	if run.Reused {
		return run, nil
	}
	checkpointID := workflowCheckpointID(run.ID)
	err = s.store.UpdateRun(ctx, run.ID, func(current *domain.GenerationRun) {
		current.WorkflowReleaseID = release.ID
		current.WorkflowKey = release.Key
		current.WorkflowVersion = release.Version
		current.WorkflowChecksum = release.Checksum
		current.PromptBundle = cloneStringMap(release.PromptBundle)
		current.PromptSnapshots = clonePromptSnapshots(promptSnapshots)
		current.CheckpointID = checkpointID
		current.Phase = "workflow.compiled"
	})
	if err != nil {
		return nil, err
	}
	state := &workflowCheckpoint{Version: workflowCheckpointVersion, ReleaseID: release.ID, PromptSnapshots: clonePromptSnapshots(promptSnapshots), Input: cloneWorkflowInput(input), NodeOutputs: map[string]map[string]any{}, NodeAttempts: map[string]int{}, StartedAt: time.Now().UTC()}
	if err := s.saveWorkflowCheckpoint(ctx, checkpointID, state); err != nil {
		_ = s.finishRun(ctx, run.ID, domain.RunStatusFailed, nil, domain.ContentRef{}, 0, "initialize workflow checkpoint: "+err.Error())
		return nil, err
	}
	execCtx := context.WithoutCancel(ctx)
	go s.executeWorkflow(execCtx, run.ID, compiled, state)
	updated, _ := s.store.GetRun(ctx, run.ID)
	return updated, nil
}

func (s *Service) enrichWorkflowRoutingInput(ctx context.Context, input map[string]any) (map[string]any, error) {
	out := cloneWorkflowInput(input)
	parentID := stringFromAny(out["parentStoryboardId"])
	if parentID == "" {
		return out, nil
	}
	parent, err := s.client.GetStoryboard(ctx, parentID)
	if err != nil {
		return nil, fmt.Errorf("load parent storyboard routing context: %w", err)
	}
	out["storyId"] = firstNonEmptyString(stringFromAny(out["storyId"]), parent.StoryID)
	out["chapterContent"] = firstNonEmptyString(parent.Content, parent.ContinuationSummary, parent.RawInput)
	out["parentEnding"] = firstNonEmptyString(parent.ContinuationSummary, parent.Content)
	return out, nil
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func (s *Service) findWorkflowByClientRequest(ctx context.Context, in domain.WorkflowStartInput) (*domain.GenerationRun, error) {
	requestID := strings.TrimSpace(in.ClientRequestID)
	if requestID == "" {
		return nil, nil
	}
	run, ok := s.store.FindRunByClientRequest(ctx, domain.RunKindWorkflow, userIDFromContext(ctx), requestID)
	if !ok {
		return nil, nil
	}
	if stringFromAny(run.Input["workflowSurface"]) != strings.TrimSpace(in.Surface) ||
		stringFromAny(run.Input["workflowAction"]) != strings.TrimSpace(in.Action) ||
		workflowTargetIdentity(run.Input) != workflowTargetIdentity(in.Input) {
		return nil, fmt.Errorf("clientRequestId %s is already bound to a different workflow target", requestID)
	}
	return run, nil
}

func workflowTargetIdentity(input map[string]any) string {
	for _, key := range []string{"draftStoryboardId", "storyboardId", "targetDraftFragmentId", "fragmentId", "parentStoryboardId", "parentFragmentId", "parentId", "storyId"} {
		if value := stringFromAny(input[key]); value != "" {
			return key + ":" + value
		}
	}
	return ""
}

func workflowManagesStoryboardStages(definition domain.WorkflowDefinition) bool {
	required := map[string]bool{
		"storyboard.generate_bible_plan": false,
		"storyboard.generate_scene_plan": false,
		"storyboard.persist_content":     false,
	}
	for _, node := range definition.Nodes {
		if _, ok := required[strings.TrimSpace(node.Activity)]; ok {
			required[strings.TrimSpace(node.Activity)] = true
		}
	}
	for _, found := range required {
		if !found {
			return false
		}
	}
	return true
}

func (s *Service) executeWorkflow(ctx context.Context, runID string, compiled *workflowruntime.CompiledWorkflow, state *workflowCheckpoint) {
	ctx = runstore.ContextWithRunID(ctx, runID)
	finishCtx := context.WithoutCancel(ctx)
	if state.StartedAt.IsZero() {
		state.StartedAt = time.Now().UTC()
	}
	if seconds := compiled.Release.Policies.MaxDurationSeconds; seconds > 0 {
		deadline := state.StartedAt.Add(time.Duration(seconds) * time.Second)
		if !deadline.After(time.Now()) {
			_ = s.finishRun(ctx, runID, domain.RunStatusFailed, nil, domain.ContentRef{}, 0, "workflow exceeded max duration")
			return
		}
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, deadline)
		defer cancel()
	}
	s.markRunning(ctx, runID)
	_ = s.store.UpdateRun(ctx, runID, func(run *domain.GenerationRun) { run.Phase = "workflow.executing" })
	result, err := workflowruntime.ExecuteWithOptions(ctx, compiled, s.workflowActivities, state.Input, workflowruntime.ExecutionOptions{
		NodeOutputs:   state.NodeOutputs,
		AttemptCounts: state.NodeAttempts,
		BeforeNodeAttempt: func(node domain.WorkflowNode, attempt int) error {
			if state.NodeAttempts == nil {
				state.NodeAttempts = make(map[string]int)
			}
			state.NodeAttempts[node.ID] = attempt
			return s.saveWorkflowCheckpoint(ctx, workflowCheckpointID(runID), state)
		},
		AfterNode: func(node domain.WorkflowNode, outputs map[string]map[string]any) error {
			state.NodeOutputs = outputs
			if err := s.saveWorkflowCheckpoint(ctx, workflowCheckpointID(runID), state); err != nil {
				return err
			}
			progress := len(outputs) * 100 / len(compiled.Release.Definition.Nodes)
			return s.store.UpdateRun(ctx, runID, func(run *domain.GenerationRun) {
				run.Phase = "workflow.node." + node.ID + ".completed"
				run.Progress = progress
			})
		},
		OnNodeRetry: func(node domain.WorkflowNode, nextAttempt int, cause error) {
			_ = s.store.UpdateRun(ctx, runID, func(run *domain.GenerationRun) {
				run.Phase = fmt.Sprintf("workflow.node.%s.retry.%d", node.ID, nextAttempt)
				run.Error = cause.Error()
			})
		},
	})
	if err != nil {
		if run, ok := s.store.GetRun(finishCtx, runID); ok && run.Status == domain.RunStatusCancelled {
			return
		}
		current, _ := s.store.GetRun(finishCtx, runID)
		output, content, tokens := workflowCompletion(current, &workflowruntime.ExecutionResult{NodeOutputs: state.NodeOutputs}, compiled.Release.ID)
		_ = s.finishRun(finishCtx, runID, domain.RunStatusFailed, output, content, tokens, err.Error())
		return
	}
	current, _ := s.store.GetRun(finishCtx, runID)
	output, content, tokens := workflowCompletion(current, result, compiled.Release.ID)
	_ = s.finishRun(finishCtx, runID, domain.RunStatusSucceeded, output, content, tokens, "")
}

// ResumeWorkflows recovers non-terminal workflow runs after a process restart.
// Lease acquisition makes this safe when multiple agent replicas start together.
func (s *Service) ResumeWorkflows(ctx context.Context) error {
	claimer, ok := s.store.(recoveredRunClaimer)
	if !ok {
		return nil
	}
	for _, run := range s.store.ListRuns(ctx, domain.RunKindWorkflow, 500) {
		if run.Status != domain.RunStatusPending && run.Status != domain.RunStatusRunning && run.Status != domain.RunStatusWaiting {
			continue
		}
		claimed, err := claimer.ClaimRun(ctx, run.ID)
		if err != nil {
			log.Printf("workflow recovery claim failed runId=%s: %v", run.ID, err)
			continue
		}
		if !claimed {
			continue
		}
		checkpointID := run.CheckpointID
		if checkpointID == "" {
			checkpointID = workflowCheckpointID(run.ID)
		}
		state, err := s.loadWorkflowCheckpoint(ctx, checkpointID)
		if err != nil {
			_ = s.finishRun(ctx, run.ID, domain.RunStatusFailed, nil, domain.ContentRef{}, 0, "workflow recovery: "+err.Error())
			continue
		}
		if state.StartedAt.IsZero() {
			state.StartedAt = run.CreatedAt
			if state.StartedAt.IsZero() {
				state.StartedAt = time.Now().UTC()
			}
			state.Version = workflowCheckpointVersion
			if err := s.saveWorkflowCheckpoint(ctx, checkpointID, state); err != nil {
				_ = s.finishRun(ctx, run.ID, domain.RunStatusFailed, nil, domain.ContentRef{}, 0, "workflow recovery: persist start time: "+err.Error())
				continue
			}
		}
		releaseID := run.WorkflowReleaseID
		if state.ReleaseID != "" {
			releaseID = state.ReleaseID
		}
		release, err := s.client.GetWorkflowRelease(ctx, releaseID)
		if err != nil {
			_ = s.finishRun(ctx, run.ID, domain.RunStatusFailed, nil, domain.ContentRef{}, 0, "workflow recovery: "+err.Error())
			continue
		}
		if run.WorkflowChecksum != "" && release.Checksum != run.WorkflowChecksum {
			_ = s.finishRun(ctx, run.ID, domain.RunStatusFailed, nil, domain.ContentRef{}, 0, "workflow recovery: pinned release checksum mismatch")
			continue
		}
		promptSnapshots := state.PromptSnapshots
		if len(promptSnapshots) == 0 {
			promptSnapshots = run.PromptSnapshots
		}
		if len(promptSnapshots) == 0 && len(release.PromptBundle) > 0 {
			promptSnapshots, err = s.resolvePromptSnapshots(ctx, release.PromptBundle)
			if err != nil {
				_ = s.finishRun(ctx, run.ID, domain.RunStatusFailed, nil, domain.ContentRef{}, 0, "workflow recovery: "+err.Error())
				continue
			}
			state.Version = workflowCheckpointVersion
			state.PromptSnapshots = clonePromptSnapshots(promptSnapshots)
			if err := s.saveWorkflowCheckpoint(ctx, checkpointID, state); err != nil {
				_ = s.finishRun(ctx, run.ID, domain.RunStatusFailed, nil, domain.ContentRef{}, 0, "workflow recovery: persist prompt snapshots: "+err.Error())
				continue
			}
			_ = s.store.UpdateRun(ctx, run.ID, func(current *domain.GenerationRun) {
				current.PromptSnapshots = clonePromptSnapshots(promptSnapshots)
			})
		}
		if err := validatePromptSnapshots(release.PromptBundle, promptSnapshots); err != nil {
			_ = s.finishRun(ctx, run.ID, domain.RunStatusFailed, nil, domain.ContentRef{}, 0, "workflow recovery: "+err.Error())
			continue
		}
		attachPromptSnapshots(release, promptSnapshots)
		compiled, err := workflowruntime.Compile(release, s.workflowActivities)
		if err != nil {
			_ = s.finishRun(ctx, run.ID, domain.RunStatusFailed, nil, domain.ContentRef{}, 0, "workflow recovery: "+err.Error())
			continue
		}
		log.Printf("resuming workflow runId=%s releaseId=%s completedNodes=%d", run.ID, release.ID, len(state.NodeOutputs))
		go s.executeWorkflow(context.Background(), run.ID, compiled, state)
	}
	return nil
}

func workflowCheckpointID(runID string) string { return "workflow_" + runID }

func (s *Service) saveWorkflowCheckpoint(ctx context.Context, id string, state *workflowCheckpoint) error {
	state.UpdatedAt = time.Now().UTC()
	payload, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshal workflow checkpoint: %w", err)
	}
	lease := ""
	if provider, ok := s.store.(generationLeaseProvider); ok {
		lease = provider.GenerationLeaseValue(strings.TrimPrefix(id, "workflow_"))
	}
	return s.client.SaveFencedGenerationCheckpoint(ctx, id, strings.TrimPrefix(id, "workflow_"), lease, payload)
}

func (s *Service) loadWorkflowCheckpoint(ctx context.Context, id string) (*workflowCheckpoint, error) {
	payload, found, err := s.client.GetGenerationCheckpoint(ctx, id)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, errors.New("durable checkpoint not found")
	}
	var state workflowCheckpoint
	if err := json.Unmarshal(payload, &state); err != nil {
		return nil, fmt.Errorf("decode durable checkpoint: %w", err)
	}
	if (state.Version < 1 || state.Version > workflowCheckpointVersion) || strings.TrimSpace(state.ReleaseID) == "" {
		return nil, errors.New("unsupported workflow checkpoint")
	}
	if state.NodeOutputs == nil {
		state.NodeOutputs = make(map[string]map[string]any)
	}
	if state.NodeAttempts == nil {
		state.NodeAttempts = make(map[string]int)
	}
	return &state, nil
}

func (s *Service) executeLegacyStoryboardActivity(ctx context.Context, input map[string]any, config map[string]any) (map[string]any, error) {
	input = applyWorkflowInputDefaults(input, config)
	payload, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	var storyboardInput domain.StoryboardGenerateInput
	if err := json.Unmarshal(payload, &storyboardInput); err != nil {
		return nil, err
	}
	if storyboardInput.ClientRequestID == "" {
		storyboardInput.ClientRequestID, _ = input["clientRequestId"].(string)
	}
	child, err := s.StartStoryboard(ctx, storyboardInput)
	if err != nil {
		return nil, err
	}
	if child.Reused && child.Status != domain.RunStatusSucceeded && child.Status != domain.RunStatusFailed && child.Status != domain.RunStatusCancelled {
		if claimer, ok := s.store.(recoveredRunClaimer); ok {
			claimed, claimErr := claimer.ClaimRun(ctx, child.ID)
			if claimErr != nil {
				return nil, fmt.Errorf("recover storyboard child run: %w", claimErr)
			}
			if claimed {
				go s.executeStoryboard(context.WithoutCancel(ctx), child.ID, storyboardInput)
			}
		}
	}
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		current, ok := s.store.GetRun(ctx, child.ID)
		if !ok {
			return nil, fmt.Errorf("storyboard child run disappeared: %s", child.ID)
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
			return nil, errors.New("storyboard child run cancelled")
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

func (s *Service) executeLegacyStoryboardBranchActivity(ctx context.Context, input map[string]any, config map[string]any) (map[string]any, error) {
	input = applyWorkflowInputDefaults(input, config)
	payload, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	var branchInput domain.BranchExploreInput
	if err := json.Unmarshal(payload, &branchInput); err != nil {
		return nil, err
	}
	if branchInput.ClientRequestID == "" {
		branchInput.ClientRequestID, _ = input["clientRequestId"].(string)
	}
	child, err := s.StartBranchBatch(ctx, branchInput)
	if err != nil {
		return nil, err
	}
	if parentID, ok := runstore.RunIDFromContext(ctx); ok {
		_ = s.store.UpdateRun(ctx, child.ID, func(run *domain.GenerationRun) {
			run.ParentRunID = parentID
			run.WorkflowReleaseID = branchInput.WorkflowReleaseID
		})
	}
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		current, ok := s.store.GetRun(ctx, child.ID)
		if !ok {
			return nil, fmt.Errorf("storyboard branch child run disappeared: %s", child.ID)
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
			return nil, errors.New("storyboard branch child run cancelled")
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

func (s *Service) syncWorkflowChildRun(ctx context.Context, child *domain.GenerationRun) {
	parentID, ok := runstore.RunIDFromContext(ctx)
	if !ok || child == nil {
		return
	}
	_ = s.store.UpdateRun(ctx, parentID, func(parent *domain.GenerationRun) {
		mergeContentRef(&parent.ContentIDs, child.ContentIDs)
		if child.TokensUsed > parent.TokensUsed {
			parent.TokensUsed = child.TokensUsed
		}
		if len(child.Output) > 0 {
			if parent.Output == nil {
				parent.Output = map[string]any{}
			}
			for key, value := range child.Output {
				parent.Output[key] = value
			}
		}
	})
}

func (s *Service) resolvePromptSnapshots(ctx context.Context, bundle map[string]string) (map[string]domain.PromptTemplateVersion, error) {
	snapshots := make(map[string]domain.PromptTemplateVersion, len(bundle))
	for nodeID, promptID := range bundle {
		prompt, err := s.client.GetPromptVersion(ctx, promptID)
		if err != nil {
			return nil, fmt.Errorf("resolve prompt for node %s: %w", nodeID, err)
		}
		if prompt == nil || prompt.ID != promptID || strings.TrimSpace(prompt.Checksum) == "" {
			return nil, fmt.Errorf("resolve prompt for node %s: invalid immutable prompt version", nodeID)
		}
		snapshots[nodeID] = *prompt
	}
	return snapshots, nil
}

func validatePromptSnapshots(bundle map[string]string, snapshots map[string]domain.PromptTemplateVersion) error {
	if len(bundle) != len(snapshots) {
		return errors.New("pinned prompt snapshot set does not match workflow bundle")
	}
	for nodeID, promptID := range bundle {
		snapshot, ok := snapshots[nodeID]
		if !ok || snapshot.ID != promptID || strings.TrimSpace(snapshot.Checksum) == "" {
			return fmt.Errorf("pinned prompt snapshot mismatch for node %s", nodeID)
		}
	}
	return nil
}

func attachPromptSnapshots(release *domain.WorkflowRelease, snapshots map[string]domain.PromptTemplateVersion) {
	for index := range release.Definition.Nodes {
		node := &release.Definition.Nodes[index]
		var defaultPrompt *domain.PromptTemplateVersion
		slotPrompts := make(map[string]domain.PromptTemplateVersion)
		for bindingKey, snapshot := range snapshots {
			boundNodeID, slot := splitPromptSnapshotBinding(bindingKey)
			if boundNodeID != node.ID {
				continue
			}
			if slot == "" {
				copy := snapshot
				defaultPrompt = &copy
			} else {
				slotPrompts[slot] = snapshot
			}
		}
		if defaultPrompt == nil && len(slotPrompts) == 0 {
			continue
		}
		if node.Config == nil {
			node.Config = make(map[string]any)
		}
		if defaultPrompt != nil {
			node.Config["promptTemplate"] = *defaultPrompt
		}
		if len(slotPrompts) > 0 {
			node.Config["promptTemplates"] = slotPrompts
		}
	}
}

func splitPromptSnapshotBinding(key string) (string, string) {
	nodeID, slot, found := strings.Cut(strings.TrimSpace(key), ":")
	if !found {
		return nodeID, ""
	}
	return strings.TrimSpace(nodeID), strings.TrimSpace(slot)
}

func clonePromptSnapshots(source map[string]domain.PromptTemplateVersion) map[string]domain.PromptTemplateVersion {
	if source == nil {
		return nil
	}
	out := make(map[string]domain.PromptTemplateVersion, len(source))
	for key, value := range source {
		out[key] = value
	}
	return out
}

func workflowCompletion(current *domain.GenerationRun, result *workflowruntime.ExecutionResult, releaseID string) (map[string]any, domain.ContentRef, int) {
	output := map[string]any{}
	content := domain.ContentRef{}
	tokens := 0
	if current != nil {
		output = cloneWorkflowInput(current.Output)
		content = current.ContentIDs
		tokens = current.TokensUsed
	}
	output["nodeOutputs"] = result.NodeOutputs
	output["workflowReleaseId"] = releaseID
	nodeTokens := 0
	for _, nodeOutput := range result.NodeOutputs {
		mergeContentRef(&content, contentRefFromOutput(nodeOutput))
		nodeTokens += intFromAny(nodeOutput["tokensUsed"])
	}
	if nodeTokens > 0 {
		tokens = nodeTokens
	}
	return output, content, tokens
}

func contentRefFromOutput(output map[string]any) domain.ContentRef {
	return domain.ContentRef{
		FragmentID:   stringFromAny(output["fragmentId"], output["draftFragmentId"]),
		StoryID:      stringFromAny(output["storyId"]),
		StoryboardID: stringFromAny(output["storyboardId"], output["draftStoryboardId"]),
		CharacterID:  stringFromAny(output["characterId"]),
		TaskID:       stringFromAny(output["taskId"]),
		BranchIDs:    stringsFromAny(output["branchStoryboardIds"]),
	}
}

func mergeContentRef(target *domain.ContentRef, source domain.ContentRef) {
	if target.FragmentID == "" {
		target.FragmentID = source.FragmentID
	}
	if target.StoryID == "" {
		target.StoryID = source.StoryID
	}
	if target.StoryboardID == "" {
		target.StoryboardID = source.StoryboardID
	}
	if target.CharacterID == "" {
		target.CharacterID = source.CharacterID
	}
	if target.TaskID == "" {
		target.TaskID = source.TaskID
	}
	if len(target.BranchIDs) == 0 && len(source.BranchIDs) > 0 {
		target.BranchIDs = append([]string(nil), source.BranchIDs...)
	}
}

func stringFromAny(values ...any) string {
	for _, value := range values {
		if text, ok := value.(string); ok && strings.TrimSpace(text) != "" {
			return strings.TrimSpace(text)
		}
	}
	return ""
}

func stringsFromAny(value any) []string {
	switch values := value.(type) {
	case []string:
		return append([]string(nil), values...)
	case []any:
		out := make([]string, 0, len(values))
		for _, value := range values {
			if text := stringFromAny(value); text != "" {
				out = append(out, text)
			}
		}
		return out
	default:
		return nil
	}
}

func intFromAny(value any) int {
	switch number := value.(type) {
	case int:
		return number
	case int64:
		return int(number)
	case float64:
		return int(number)
	default:
		return 0
	}
}

func cloneWorkflowInput(input map[string]any) map[string]any {
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

// applyWorkflowInputDefaults is the executable contract for operator node
// configuration. A release may provide {"inputDefaults": {...}}; runtime input
// always wins so a published workflow cannot silently override an explicit
// user choice.
func applyWorkflowInputDefaults(input, config map[string]any) map[string]any {
	out := cloneWorkflowInput(input)
	defaults, ok := config["inputDefaults"].(map[string]any)
	if !ok {
		return out
	}
	for key, value := range defaults {
		if strings.TrimSpace(key) == "" || value == nil {
			continue
		}
		if existing, found := out[key]; !found || workflowInputValueEmpty(existing) {
			out[key] = value
		}
	}
	return out
}

func workflowInputValueEmpty(value any) bool {
	switch typed := value.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(typed) == ""
	case float64:
		return typed == 0
	case int:
		return typed == 0
	case []any:
		return len(typed) == 0
	case []string:
		return len(typed) == 0
	default:
		return false
	}
}

func applyLegacyWorkflowPrompt(input, config map[string]any, targetKey string) (map[string]any, error) {
	raw, found := config["promptTemplate"]
	if !found {
		return input, nil
	}
	payload, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("marshal workflow prompt template: %w", err)
	}
	var prompt domain.PromptTemplateVersion
	if err := json.Unmarshal(payload, &prompt); err != nil {
		return nil, fmt.Errorf("decode workflow prompt template: %w", err)
	}
	variables := cloneWorkflowInput(input)
	legacyInput := stringFromAny(input[targetKey], input["userInput"], input["rawInput"], input["seedPrompt"])
	variables["legacyUserPrompt"] = legacyInput
	variables["userInput"] = legacyInput
	variables["rawInput"] = legacyInput
	variables["seedPrompt"] = legacyInput
	systemText, err := renderWorkflowPromptPart(prompt.SystemTemplate, variables)
	if err != nil {
		return nil, fmt.Errorf("render workflow system prompt %s: %w", prompt.ID, err)
	}
	userText, err := renderWorkflowPromptPart(prompt.UserTemplate, variables)
	if err != nil {
		return nil, fmt.Errorf("render workflow user prompt %s: %w", prompt.ID, err)
	}
	if strings.TrimSpace(userText) == "" {
		userText = legacyInput
	}
	out := cloneWorkflowInput(input)
	out["workflowSystemPrompt"] = strings.TrimSpace(systemText)
	out["workflowUserPrompt"] = strings.TrimSpace(userText)
	out["workflowModelConfig"] = prompt.ModelConfig
	out["workflowOutputSchema"] = prompt.OutputSchema
	out["workflowPromptVersionId"] = prompt.ID
	return out, nil
}

func renderWorkflowPromptPart(source string, variables map[string]any) (string, error) {
	if strings.TrimSpace(source) == "" {
		return "", nil
	}
	tmpl, err := template.New("workflow-prompt").Option("missingkey=error").Parse(source)
	if err != nil {
		return "", err
	}
	var rendered bytes.Buffer
	if err := tmpl.Execute(&rendered, variables); err != nil {
		return "", err
	}
	return rendered.String(), nil
}

func cloneStringMap(input map[string]string) map[string]string {
	out := make(map[string]string, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}
