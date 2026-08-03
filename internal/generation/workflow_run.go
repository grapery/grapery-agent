package generation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
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
	_ = registry.Register("legacy.storyboard.generate", s.executeLegacyStoryboardActivity)
	_ = registry.Register("storyboard.ensure_draft", s.executeEnsureStoryboardDraftActivity)
	_ = registry.Register("storyboard.generate_bible_plan", s.executeGenerateStoryboardBiblePlanActivity)
	_ = registry.Register("storyboard.generate_scene_plan", s.executeGenerateStoryboardScenePlanActivity)
	_ = registry.Register("storyboard.persist_content", s.executePersistStoryboardContentActivity)
	_ = registry.Register("storyboard.await_content", s.executeAwaitStoryboardContentActivity)
	_ = registry.Register("storyboard.ensure_images", s.executeEnsureStoryboardImagesActivity)
	return registry
}

func (s *Service) StartWorkflow(ctx context.Context, in domain.WorkflowStartInput) (*domain.GenerationRun, error) {
	if strings.TrimSpace(in.Surface) == "" || strings.TrimSpace(in.Action) == "" {
		return nil, errors.New("workflow surface and action are required")
	}
	entries, err := s.client.ListWorkflowCatalog(ctx, in.Surface, in.Action, in.TenantID)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("no active workflow binding for %s/%s", in.Surface, in.Action)
	}
	release := &entries[0].Release
	if requested := strings.TrimSpace(in.ReleaseID); requested != "" && requested != release.ID {
		return nil, fmt.Errorf("workflow release %s is not active for %s/%s", requested, in.Surface, in.Action)
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
		_ = s.finishRun(finishCtx, runID, domain.RunStatusFailed, nil, domain.ContentRef{}, 0, err.Error())
		return
	}
	output := map[string]any{"nodeOutputs": result.NodeOutputs, "workflowReleaseId": compiled.Release.ID}
	_ = s.finishRun(finishCtx, runID, domain.RunStatusSucceeded, output, workflowContentRef(result), 0, "")
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

func (s *Service) executeLegacyStoryboardActivity(ctx context.Context, input map[string]any, _ map[string]any) (map[string]any, error) {
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
		switch current.Status {
		case domain.RunStatusSucceeded:
			out := cloneWorkflowInput(current.Output)
			out["childRunId"] = current.ID
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

func workflowContentRef(result *workflowruntime.ExecutionResult) domain.ContentRef {
	content := domain.ContentRef{}
	for _, output := range result.NodeOutputs {
		if value, ok := output["storyboardId"].(string); ok && content.StoryboardID == "" {
			content.StoryboardID = value
		}
		if value, ok := output["storyId"].(string); ok && content.StoryID == "" {
			content.StoryID = value
		}
	}
	return content
}

func cloneWorkflowInput(input map[string]any) map[string]any {
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func cloneStringMap(input map[string]string) map[string]string {
	out := make(map[string]string, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}
