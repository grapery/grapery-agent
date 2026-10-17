package eval

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/grapestree/fgrapery/grapery-agent/internal/domain"
	"github.com/grapestree/fgrapery/grapery-agent/internal/generation"
	"github.com/grapestree/fgrapery/grapery-agent/internal/runstore"
)

// Harness runs explicitly requested generation trials and grades their durable outcomes.
type Harness struct {
	gen   *generation.Service
	store runstore.Store
}

func NewHarness(gen *generation.Service, store runstore.Store) *Harness {
	return &Harness{gen: gen, store: store}
}

// RunEval executes eval for the given agent version and optional seed filter.
func (h *Harness) RunEval(ctx context.Context, agent domain.AgentVersion, seedIDs []string, waitSec int) (*domain.EvalRecord, error) {
	return h.RunEvalWithSeeds(ctx, agent, DefaultSeeds(), seedIDs, waitSec)
}

// RunEvalWithSeeds accepts explicit fixture IDs. Running it calls real generators; unit tests only exercise graders.
func (h *Harness) RunEvalWithSeeds(ctx context.Context, agent domain.AgentVersion, seeds []Seed, seedIDs []string, waitSec int) (*domain.EvalRecord, error) {
	if kind := seedKindForAgent(agent); kind != "" {
		filtered := make([]Seed, 0, len(seeds))
		for _, seed := range seeds {
			if seed.Kind == kind {
				filtered = append(filtered, seed)
			}
		}
		seeds = filtered
	}
	if len(seedIDs) > 0 {
		filter := map[string]bool{}
		for _, id := range seedIDs {
			filter[id] = true
		}
		filtered := make([]Seed, 0)
		for _, s := range seeds {
			if filter[s.ID] {
				filtered = append(filtered, s)
			}
		}
		seeds = filtered
	}
	if len(seeds) == 0 {
		return nil, fmt.Errorf("no seeds matched")
	}

	record := &domain.EvalRecord{
		AgentVersion: agent,
		Metrics:      map[string]float64{},
		RunIDs:       []string{},
	}
	var totalTools, failedTools int64
	var totalDur int64
	var successful, ready, graded int

	for _, seed := range seeds {
		run, err := h.startSeed(ctx, seed)
		if err != nil {
			record.Notes += fmt.Sprintf("seed %s start error: %v; ", seed.ID, err)
			continue
		}
		record.RunIDs = append(record.RunIDs, run.ID)
		final, err := h.waitRun(ctx, run.ID, time.Duration(waitSec)*time.Second)
		if err != nil {
			record.Notes += fmt.Sprintf("seed %s wait error: %v; ", seed.ID, err)
			continue
		}
		for _, tc := range final.ToolCalls {
			totalTools++
			totalDur += tc.DurationMs
			if !tc.Success {
				failedTools++
			}
		}
		grade := GradeCreationOutcome(seed, final)
		graded++
		if final.Status == domain.RunStatusSucceeded {
			successful++
		}
		if grade.Ready {
			ready++
		}
		for _, issue := range grade.Issues {
			record.Notes += fmt.Sprintf("seed %s outcome: %s; ", seed.ID, issue)
		}
		record.TotalTokens += final.TokensUsed
	}

	if totalTools > 0 {
		record.ToolFailureRate = float64(failedTools) / float64(totalTools)
		record.AvgToolDurationMs = float64(totalDur) / float64(totalTools)
	}
	record.Metrics["seeds_requested"] = float64(len(seeds))
	record.Metrics["outcomes_graded"] = float64(graded)
	record.Metrics["run_success_rate"] = float64(successful) / float64(len(seeds))
	record.Metrics["artifact_ready_rate"] = float64(ready) / float64(len(seeds))
	record.Metrics["failed_or_incomplete_seeds"] = float64(len(seeds) - successful)
	record.Metrics["seeds_executed"] = float64(len(record.RunIDs))
	record.Metrics["tool_calls"] = float64(totalTools)
	record.Metrics["tool_failures"] = float64(failedTools)

	art := &domain.RLArtifact{
		Type: domain.ArtifactTypeEvalResult,
		Eval: record,
	}
	if err := h.store.AppendArtifact(ctx, art); err != nil {
		return record, fmt.Errorf("persist evaluation: %w", err)
	}
	return record, nil
}

func (h *Harness) startSeed(ctx context.Context, seed Seed) (*domain.GenerationRun, error) {
	switch strings.ToLower(seed.Kind) {
	case "fragment":
		return h.gen.StartFragment(ctx, domain.FragmentGenerateInput{
			UserInput:       seed.Prompt,
			ImageCount:      1,
			Style:           "thriller",
			PollIntervalSec: 3,
			PollTimeoutSec:  120,
		})
	case "fragment_panel":
		if seed.ReferenceImageURL == "" {
			return nil, fmt.Errorf("fragment_panel seed requires referenceImageUrl")
		}
		return h.gen.StartFragmentPanel(ctx, domain.FragmentPanelGenerateInput{
			UserInput:         seed.Prompt,
			ReferenceImageURL: seed.ReferenceImageURL,
			PanelCount:        3,
			PollIntervalSec:   3,
			PollTimeoutSec:    120,
		})
	case "story":
		return h.gen.StartStory(ctx, domain.StoryGenerateInput{Prompt: seed.Prompt, Style: "literary"})
	case "storyboard":
		if seed.StoryID == "" {
			return nil, fmt.Errorf("storyboard seed requires storyId")
		}
		return h.gen.StartStoryboard(ctx, domain.StoryboardGenerateInput{
			StoryID:    seed.StoryID,
			RawInput:   seed.Prompt,
			SceneCount: 3,
		})
	case "character":
		if seed.StoryID == "" {
			return nil, fmt.Errorf("character seed requires storyId")
		}
		return h.gen.StartCharacter(ctx, domain.CharacterGenerateInput{StoryID: seed.StoryID, Prompt: seed.Prompt, UseAsyncTask: true, CreateRecord: true, GeneratePortrait: true})
	case "branch":
		if seed.ParentID == "" {
			return nil, fmt.Errorf("branch seed requires parentStoryboardId")
		}
		return h.gen.StartBranchBatch(ctx, domain.BranchExploreInput{
			ParentStoryboardID: seed.ParentID,
			SeedPrompt:         seed.Prompt,
			BranchCount:        2,
		})
	default:
		return nil, fmt.Errorf("unknown seed kind: %s", seed.Kind)
	}
}

func (h *Harness) waitRun(ctx context.Context, runID string, timeout time.Duration) (*domain.GenerationRun, error) {
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		run, err := h.gen.GetRun(ctx, runID)
		if err != nil {
			return nil, err
		}
		switch run.Status {
		case domain.RunStatusSucceeded, domain.RunStatusFailed, domain.RunStatusCancelled:
			return run, nil
		}
		time.Sleep(2 * time.Second)
	}
	return nil, fmt.Errorf("timeout waiting for run %s", runID)
}

// BranchDiversityScore computes normalized unique strategy count from a batch run.
func BranchDiversityScore(batch *domain.BranchBatchResult) float64 {
	if batch == nil || len(batch.Candidates) == 0 {
		return 0
	}
	seen := map[string]bool{}
	for _, c := range batch.Candidates {
		seen[c.Strategy] = true
	}
	return float64(len(seen)) / float64(len(batch.Candidates))
}

func seedKindForAgent(agent domain.AgentVersion) string {
	switch agent {
	case domain.AgentFragmentCreator, domain.AgentFragmentComicDirector:
		return "fragment"
	case domain.AgentFragmentPanelCreator:
		return "fragment_panel"
	case domain.AgentStoryboardDirector:
		return "storyboard"
	case domain.AgentCharacterDesigner:
		return "character"
	case domain.AgentStoryGenerator:
		return "story"
	case domain.AgentBranchExplorer:
		return "branch"
	default:
		return ""
	}
}
