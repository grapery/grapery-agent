package runstore

import (
	"context"
	"log"
	"time"

	"github.com/grapestree/fgrapery/grapery-agent/internal/agentauth"
	"github.com/grapestree/fgrapery/grapery-agent/internal/domain"
	"github.com/grapestree/fgrapery/grapery-agent/internal/grapery_client"
)

// AuditSyncStore 在内存 run store 之上，将步骤审计异步同步到 grapery。
type AuditSyncStore struct {
	inner   Store
	client  *grapery_client.Client
	enabled bool
}

func NewAuditSyncStore(inner Store, client *grapery_client.Client, enabled bool) Store {
	if inner == nil {
		inner = NewMemoryStore()
	}
	return &AuditSyncStore{inner: inner, client: client, enabled: enabled && client != nil}
}

func (s *AuditSyncStore) CreateRun(ctx context.Context, kind domain.RunKind, agent domain.AgentVersion, userIntent string, input map[string]any) (*domain.GenerationRun, error) {
	return s.inner.CreateRun(ctx, kind, agent, userIntent, input)
}

func (s *AuditSyncStore) GetRun(ctx context.Context, id string) (*domain.GenerationRun, bool) {
	return s.inner.GetRun(ctx, id)
}

func (s *AuditSyncStore) FindRunByClientRequest(ctx context.Context, kind domain.RunKind, userID, clientRequestID string) (*domain.GenerationRun, bool) {
	return s.inner.FindRunByClientRequest(ctx, kind, userID, clientRequestID)
}

func (s *AuditSyncStore) UpdateRun(ctx context.Context, id string, fn func(*domain.GenerationRun)) error {
	return s.inner.UpdateRun(ctx, id, fn)
}

func (s *AuditSyncStore) ListRuns(ctx context.Context, kind domain.RunKind, limit int) []*domain.GenerationRun {
	return s.inner.ListRuns(ctx, kind, limit)
}

func (s *AuditSyncStore) RecordToolCall(ctx context.Context, runID string, rec domain.ToolCallRecord) error {
	return s.inner.RecordToolCall(ctx, runID, rec)
}

func (s *AuditSyncStore) RecordStepAudit(ctx context.Context, runID string, rec domain.GenerationStepAudit) error {
	if rec.UserID == "" {
		if uid := userIDFromContext(ctx); uid != "" {
			rec.UserID = uid
		}
	}
	if err := s.inner.RecordStepAudit(ctx, runID, rec); err != nil {
		return err
	}
	if !s.enabled {
		return nil
	}
	run, ok := s.inner.GetRun(ctx, runID)
	if !ok || len(run.StepAudits) == 0 {
		return nil
	}
	last := run.StepAudits[len(run.StepAudits)-1]
	recCopy := last
	go s.syncAudit(recCopy)
	return nil
}

func (s *AuditSyncStore) syncAudit(rec domain.GenerationStepAudit) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := s.client.RecordGenerationAudits(ctx, []domain.GenerationStepAudit{rec}); err != nil {
		log.Printf("audit sync to grapery failed runId=%s step=%s: %v", rec.RunID, rec.StepName, err)
	}
}

func (s *AuditSyncStore) CancelRun(ctx context.Context, id string) error {
	return s.inner.CancelRun(ctx, id)
}

func (s *AuditSyncStore) AppendArtifact(ctx context.Context, art *domain.RLArtifact) error {
	return s.inner.AppendArtifact(ctx, art)
}

func (s *AuditSyncStore) ListArtifacts(ctx context.Context, typ domain.RLArtifactType, limit int) []*domain.RLArtifact {
	return s.inner.ListArtifacts(ctx, typ, limit)
}

func userIDFromContext(ctx context.Context) string {
	if claims, ok := agentauth.ClaimsFromContext(ctx); ok && claims != nil {
		return claims.UserID
	}
	return ""
}
