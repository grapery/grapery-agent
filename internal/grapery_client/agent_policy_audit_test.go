package grapery_client

import (
	"testing"
	"time"

	"github.com/grapestree/fgrapery/grapery-agent/internal/domain"
)

func TestGenerationStepAuditToRecord(t *testing.T) {
	started := time.Now()
	rec := generationStepAuditToRecord(domain.GenerationStepAudit{
		ID:           "audit_1",
		RunID:        "run_1",
		StepName:     "panel_plan",
		Status:       domain.StepSucceeded,
		TotalTokens:  100,
		StartedAt:    started,
		InputRefs:    []string{"https://img.example/a.png"},
		BusinessType: domain.RunKindFragmentPanel,
	})
	if rec["runId"] != "run_1" || rec["stepName"] != "panel_plan" {
		t.Fatalf("unexpected record: %#v", rec)
	}
	if rec["startedAt"] == nil {
		t.Fatal("expected startedAt")
	}
}
