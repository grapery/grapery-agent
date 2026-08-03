package runstore

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/grapestree/fgrapery/grapery-agent/internal/domain"
)

const maxRuns = 5000

// Store persists generation runs and RL artifacts in memory.
type Store interface {
	CreateRun(ctx context.Context, kind domain.RunKind, agent domain.AgentVersion, userIntent string, input map[string]any) (*domain.GenerationRun, error)
	GetRun(ctx context.Context, id string) (*domain.GenerationRun, bool)
	UpdateRun(ctx context.Context, id string, fn func(*domain.GenerationRun)) error
	ListRuns(ctx context.Context, kind domain.RunKind, limit int) []*domain.GenerationRun
	RecordToolCall(ctx context.Context, runID string, rec domain.ToolCallRecord) error
	RecordStepAudit(ctx context.Context, runID string, rec domain.GenerationStepAudit) error
	CancelRun(ctx context.Context, id string) error
	AppendArtifact(ctx context.Context, art *domain.RLArtifact) error
	ListArtifacts(ctx context.Context, typ domain.RLArtifactType, limit int) []*domain.RLArtifact
}

type MemoryStore struct {
	mu        sync.RWMutex
	runs      map[string]*domain.GenerationRun
	runOrder  []string
	artifacts []*domain.RLArtifact
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		runs: make(map[string]*domain.GenerationRun),
	}
}

func (s *MemoryStore) CreateRun(ctx context.Context, kind domain.RunKind, agent domain.AgentVersion, userIntent string, input map[string]any) (*domain.GenerationRun, error) {
	now := time.Now()
	run := &domain.GenerationRun{
		ID:           "run_" + uuid.New().String(),
		UserID:       userIDFromContext(ctx),
		Kind:         kind,
		Status:       domain.RunStatusPending,
		AgentVersion: agent,
		UserIntent:   userIntent,
		Input:        cloneMap(input),
		Output:       make(map[string]any),
		ToolCalls:    []domain.ToolCallRecord{},
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	run.ClientRequestID = stringInput(input, "clientRequestId", "clientMessageId")
	run.SourceTaskID = stringInput(input, "taskId", "sourceTaskId")
	s.mu.Lock()
	defer s.mu.Unlock()
	s.evictRunsIfNeeded(1)
	s.runs[run.ID] = run
	s.runOrder = append(s.runOrder, run.ID)
	return cloneRun(run), nil
}

func stringInput(input map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := input[key].(string); ok && value != "" {
			return value
		}
	}
	return ""
}

func (s *MemoryStore) GetRun(_ context.Context, id string) (*domain.GenerationRun, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.runs[id]
	if !ok {
		return nil, false
	}
	return cloneRun(r), true
}

func (s *MemoryStore) UpdateRun(_ context.Context, id string, fn func(*domain.GenerationRun)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.runs[id]
	if !ok {
		return fmt.Errorf("run not found: %s", id)
	}
	fn(r)
	r.UpdatedAt = time.Now()
	return nil
}

func (s *MemoryStore) ListRuns(_ context.Context, kind domain.RunKind, limit int) []*domain.GenerationRun {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 {
		limit = 50
	}
	out := make([]*domain.GenerationRun, 0, limit)
	for i := len(s.runOrder) - 1; i >= 0 && len(out) < limit; i-- {
		r := s.runs[s.runOrder[i]]
		if kind != "" && r.Kind != kind {
			continue
		}
		out = append(out, cloneRun(r))
	}
	return out
}

func (s *MemoryStore) RecordToolCall(_ context.Context, runID string, rec domain.ToolCallRecord) error {
	return s.UpdateRun(context.Background(), runID, func(r *domain.GenerationRun) {
		rec.Sequence = len(r.ToolCalls) + 1
		r.ToolCalls = append(r.ToolCalls, rec)
	})
}

// RecordStepAudit appends a generation step audit row. Each call is a distinct
// attempt; failed/retried attempts are never overwritten by later successes.
func (s *MemoryStore) RecordStepAudit(_ context.Context, runID string, rec domain.GenerationStepAudit) error {
	return s.UpdateRun(context.Background(), runID, func(r *domain.GenerationRun) {
		rec.Sequence = len(r.StepAudits) + 1
		rec.RunID = runID
		if rec.ID == "" {
			rec.ID = "audit_" + uuid.New().String()
		}
		if rec.BusinessType == "" {
			rec.BusinessType = r.Kind
		}
		if rec.AgentVersion == "" {
			rec.AgentVersion = r.AgentVersion
		}
		r.StepAudits = append(r.StepAudits, rec)
	})
}

func (s *MemoryStore) CancelRun(_ context.Context, id string) error {
	return s.UpdateRun(context.Background(), id, func(r *domain.GenerationRun) {
		switch r.Status {
		case domain.RunStatusSucceeded, domain.RunStatusFailed, domain.RunStatusCancelled:
			return
		default:
			r.Status = domain.RunStatusCancelled
			r.Error = "cancelled by client"
		}
	})
}

func (s *MemoryStore) AppendArtifact(_ context.Context, art *domain.RLArtifact) error {
	if art.ID == "" {
		art.ID = "art_" + uuid.New().String()
	}
	if art.CreatedAt.IsZero() {
		art.CreatedAt = time.Now()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.artifacts = append(s.artifacts, art)
	return nil
}

func (s *MemoryStore) ListArtifacts(_ context.Context, typ domain.RLArtifactType, limit int) []*domain.RLArtifact {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 {
		limit = 100
	}
	out := make([]*domain.RLArtifact, 0, limit)
	for i := len(s.artifacts) - 1; i >= 0 && len(out) < limit; i-- {
		a := s.artifacts[i]
		if typ != "" && a.Type != typ {
			continue
		}
		out = append(out, a)
	}
	return out
}

func (s *MemoryStore) evictRunsIfNeeded(add int) {
	for len(s.runs)+add > maxRuns && len(s.runOrder) > 0 {
		evict := s.runOrder[0]
		s.runOrder = s.runOrder[1:]
		delete(s.runs, evict)
	}
}

func cloneRun(r *domain.GenerationRun) *domain.GenerationRun {
	cp := *r
	cp.Input = cloneMap(r.Input)
	cp.Output = cloneMap(r.Output)
	if r.PromptBundle != nil {
		cp.PromptBundle = make(map[string]string, len(r.PromptBundle))
		for key, value := range r.PromptBundle {
			cp.PromptBundle[key] = value
		}
	}
	if r.PromptSnapshots != nil {
		cp.PromptSnapshots = make(map[string]domain.PromptTemplateVersion, len(r.PromptSnapshots))
		for key, value := range r.PromptSnapshots {
			cp.PromptSnapshots[key] = value
		}
	}
	if len(r.ToolCalls) > 0 {
		cp.ToolCalls = append([]domain.ToolCallRecord(nil), r.ToolCalls...)
	}
	if len(r.StepAudits) > 0 {
		cp.StepAudits = append([]domain.GenerationStepAudit(nil), r.StepAudits...)
	}
	if r.ContentIDs.FragmentID != "" || r.ContentIDs.StoryboardID != "" {
		cp.ContentIDs = r.ContentIDs
	}
	if r.CompletedAt != nil {
		t := *r.CompletedAt
		cp.CompletedAt = &t
	}
	return &cp
}

func cloneMap(m map[string]any) map[string]any {
	if m == nil {
		return make(map[string]any)
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// TracedCall records a tool invocation on the active run in context.
func TracedCall(ctx context.Context, store Store, toolName string, input map[string]any, fn func(context.Context) (map[string]any, error)) (map[string]any, error) {
	runID, ok := RunIDFromContext(ctx)
	if !ok || store == nil {
		return fn(ctx)
	}
	start := time.Now()
	out, err := fn(ctx)
	end := time.Now()
	rec := domain.ToolCallRecord{
		ToolName:   toolName,
		Input:      cloneMap(input),
		Output:     out,
		StartedAt:  start,
		EndedAt:    end,
		DurationMs: end.Sub(start).Milliseconds(),
		Success:    err == nil,
	}
	if err != nil {
		rec.Error = err.Error()
	}
	_ = store.RecordToolCall(ctx, runID, rec)
	return out, err
}

// TracedStep records a GenerationStepAudit for the active run while executing fn.
// It captures a started/succeeded/failed attempt with duration; callers may enrich
// the base record (prompt, provider, model, tokens, refs) via the base argument.
func TracedStep(ctx context.Context, store Store, base domain.GenerationStepAudit, fn func(context.Context) error) error {
	runID, ok := RunIDFromContext(ctx)
	if !ok || store == nil {
		if fn == nil {
			return nil
		}
		return fn(ctx)
	}
	start := time.Now()
	var err error
	if fn != nil {
		err = fn(ctx)
	}
	end := time.Now()
	base.StartedAt = start
	base.EndedAt = end
	base.DurationMs = end.Sub(start).Milliseconds()
	if err != nil {
		base.Status = domain.StepFailed
		base.ErrorMessage = err.Error()
	} else if base.Status == "" {
		base.Status = domain.StepSucceeded
	}
	_ = store.RecordStepAudit(ctx, runID, base)
	return err
}
