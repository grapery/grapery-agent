package runstore

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/grapestree/fgrapery/grapery-agent/internal/domain"
	"github.com/grapestree/fgrapery/grapery-agent/internal/grapery_client"
)

// GraperyStore keeps a bounded hot copy in memory while synchronously writing
// every lifecycle mutation to Grapery. Reads recover from Grapery after an
// agent restart, so memory is no longer the source of truth.
type GraperyStore struct {
	inner  *MemoryStore
	client *grapery_client.Client
	owner  string
	mu     sync.Mutex
	leases map[string]leaseHandle
}

type leaseHandle struct {
	value  string
	cancel context.CancelFunc
}

const (
	generationLeaseTTL     = 5 * time.Minute
	generationLeaseRenewal = time.Minute
	generationLeaseTimeout = 15 * time.Second
)

func NewGraperyStore(client *grapery_client.Client) Store {
	return &GraperyStore{
		inner:  NewMemoryStore(),
		client: client,
		owner:  "agent_" + uuid.NewString(),
		leases: make(map[string]leaseHandle),
	}
}

func (s *GraperyStore) CreateRun(ctx context.Context, kind domain.RunKind, agent domain.AgentVersion, userIntent string, input map[string]any) (*domain.GenerationRun, error) {
	run, err := s.inner.CreateRun(ctx, kind, agent, userIntent, input)
	if err != nil {
		return nil, err
	}
	enrichRuntimeSnapshot(run)
	saved, err := s.client.SaveGenerationExecution(ctx, run, "run.created", "")
	if err != nil {
		return nil, err
	}
	if saved.ID != run.ID {
		saved.Reused = true
		s.cache(saved)
		return saved, nil
	}
	lease, acquired, leaseErr := s.client.AcquireGenerationLease(ctx, saved.ID, s.owner, int(generationLeaseTTL/time.Second))
	if leaseErr != nil {
		log.Printf("generation lease unavailable; continuing in degraded mode runId=%s: %v", saved.ID, leaseErr)
	} else if !acquired || lease == nil {
		return nil, fmt.Errorf("generation already executing: %s", saved.ID)
	} else {
		s.startLeaseRenewal(saved.ID, lease.Value)
	}
	s.cache(saved)
	return saved, nil
}

func (s *GraperyStore) GetRun(ctx context.Context, id string) (*domain.GenerationRun, bool) {
	run, err := s.client.GetGenerationExecution(ctx, id)
	if err != nil {
		return s.inner.GetRun(ctx, id)
	}
	s.cache(run)
	if isTerminalStatus(run.Status) {
		s.stopAndReleaseLease(run.ID)
	}
	return cloneRun(run), true
}

func (s *GraperyStore) UpdateRun(ctx context.Context, id string, fn func(*domain.GenerationRun)) error {
	if latest, err := s.client.GetGenerationExecution(ctx, id); err == nil {
		s.cache(latest)
		if isTerminalStatus(latest.Status) {
			s.stopAndReleaseLease(latest.ID)
			return nil
		}
	} else if _, ok := s.inner.GetRun(ctx, id); !ok {
		return err
	}
	if err := s.inner.UpdateRun(ctx, id, fn); err != nil {
		return err
	}
	run, _ := s.inner.GetRun(ctx, id)
	enrichRuntimeSnapshot(run)
	eventType := "run.updated"
	if run != nil {
		switch run.Status {
		case domain.RunStatusSucceeded:
			eventType = "run.succeeded"
		case domain.RunStatusFailed:
			eventType = "run.failed"
		case domain.RunStatusCancelled:
			eventType = "run.cancelled"
		}
	}
	saved, err := s.client.SaveGenerationExecution(ctx, run, eventType, s.leaseValue(id))
	if err != nil {
		return err
	}
	s.cache(saved)
	if isTerminalStatus(saved.Status) {
		s.stopAndReleaseLease(saved.ID)
	}
	return nil
}

func (s *GraperyStore) ListRuns(ctx context.Context, kind domain.RunKind, limit int) []*domain.GenerationRun {
	runs, err := s.client.ListGenerationExecutions(ctx, kind, limit)
	if err != nil {
		return s.inner.ListRuns(ctx, kind, limit)
	}
	for _, run := range runs {
		s.cache(run)
	}
	return runs
}

func (s *GraperyStore) RecordToolCall(ctx context.Context, runID string, rec domain.ToolCallRecord) error {
	return s.UpdateRun(ctx, runID, func(r *domain.GenerationRun) {
		rec.Sequence = len(r.ToolCalls) + 1
		r.ToolCalls = append(r.ToolCalls, rec)
	})
}

func (s *GraperyStore) RecordStepAudit(ctx context.Context, runID string, rec domain.GenerationStepAudit) error {
	if rec.UserID == "" {
		rec.UserID = userIDFromContext(ctx)
	}
	if err := s.inner.RecordStepAudit(ctx, runID, rec); err != nil {
		return err
	}
	run, ok := s.inner.GetRun(ctx, runID)
	if !ok {
		return nil
	}
	if _, err := s.client.SaveGenerationExecution(ctx, run, "step.recorded", s.leaseValue(runID)); err != nil {
		return err
	}
	last := run.StepAudits[len(run.StepAudits)-1]
	go func() {
		syncCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := s.client.RecordGenerationAudits(syncCtx, []domain.GenerationStepAudit{last}); err != nil {
			log.Printf("audit sync to grapery failed runId=%s step=%s: %v", last.RunID, last.StepName, err)
		}
	}()
	return nil
}

func (s *GraperyStore) CancelRun(ctx context.Context, id string) error {
	return s.UpdateRun(ctx, id, func(r *domain.GenerationRun) {
		switch r.Status {
		case domain.RunStatusSucceeded, domain.RunStatusFailed, domain.RunStatusCancelled:
		default:
			r.Status, r.Error = domain.RunStatusCancelled, "cancelled by client"
		}
	})
}

func (s *GraperyStore) AppendArtifact(ctx context.Context, art *domain.RLArtifact) error {
	return s.inner.AppendArtifact(ctx, art)
}

func (s *GraperyStore) ListArtifacts(ctx context.Context, typ domain.RLArtifactType, limit int) []*domain.RLArtifact {
	return s.inner.ListArtifacts(ctx, typ, limit)
}

func (s *GraperyStore) cache(run *domain.GenerationRun) {
	if run == nil {
		return
	}
	s.inner.mu.Lock()
	defer s.inner.mu.Unlock()
	if _, exists := s.inner.runs[run.ID]; !exists {
		s.inner.evictRunsIfNeeded(1)
		s.inner.runOrder = append(s.inner.runOrder, run.ID)
	}
	s.inner.runs[run.ID] = cloneRun(run)
}

func (s *GraperyStore) startLeaseRenewal(runID, value string) {
	leaseCtx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	if previous, ok := s.leases[runID]; ok {
		previous.cancel()
	}
	s.leases[runID] = leaseHandle{value: value, cancel: cancel}
	s.mu.Unlock()

	go func() {
		ticker := time.NewTicker(generationLeaseRenewal)
		defer ticker.Stop()
		for {
			select {
			case <-leaseCtx.Done():
				return
			case <-ticker.C:
				renewCtx, renewCancel := context.WithTimeout(context.Background(), generationLeaseTimeout)
				renewed, err := s.client.RenewGenerationLease(renewCtx, runID, value, int(generationLeaseTTL/time.Second))
				renewCancel()
				if err != nil {
					log.Printf("generation lease renewal failed runId=%s: %v", runID, err)
					continue
				}
				if !renewed {
					log.Printf("generation lease lost runId=%s; fencing token no longer valid", runID)
					return
				}
			}
		}
	}()
}

func (s *GraperyStore) leaseValue(runID string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.leases[runID].value
}

func (s *GraperyStore) stopAndReleaseLease(runID string) {
	s.mu.Lock()
	handle, ok := s.leases[runID]
	if ok {
		delete(s.leases, runID)
	}
	s.mu.Unlock()
	if !ok {
		return
	}
	handle.cancel()
	go func() {
		releaseCtx, cancel := context.WithTimeout(context.Background(), generationLeaseTimeout)
		defer cancel()
		if err := s.client.ReleaseGenerationLease(releaseCtx, runID, handle.value); err != nil {
			log.Printf("generation lease release failed runId=%s: %v", runID, err)
		}
	}()
}

func isTerminalStatus(status domain.RunStatus) bool {
	switch status {
	case domain.RunStatusSucceeded, domain.RunStatusFailed, domain.RunStatusCancelled:
		return true
	default:
		return false
	}
}

func enrichRuntimeSnapshot(run *domain.GenerationRun) {
	if run == nil {
		return
	}
	if run.SourceTaskID == "" {
		run.SourceTaskID = run.ContentIDs.TaskID
	}
	if run.Phase == "" {
		for _, key := range []string{"phase", "currentStep", "step", "stage"} {
			if value, ok := run.Output[key].(string); ok && value != "" {
				run.Phase = value
				break
			}
		}
	}
	if value, ok := numericValue(run.Output["progress"]); ok {
		if value <= 1 {
			value *= 100
		}
		if value < 0 {
			value = 0
		}
		if value > 100 {
			value = 100
		}
		run.Progress = int(value)
	}
	if run.Status == domain.RunStatusSucceeded {
		run.Progress = 100
	}
}

func numericValue(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	default:
		return 0, false
	}
}

var _ Store = (*GraperyStore)(nil)
