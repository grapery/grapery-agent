package eval

import (
	"testing"

	"github.com/grapestree/fgrapery/grapery-agent/internal/domain"
)

func TestBranchDiversityScore(t *testing.T) {
	batch := &domain.BranchBatchResult{
		Candidates: []domain.BranchCandidate{
			{Strategy: "hopeful_turn"},
			{Strategy: "darker_twist"},
			{Strategy: "hopeful_turn"},
		},
	}
	got := BranchDiversityScore(batch)
	want := 2.0 / 3.0
	if got < want-0.001 || got > want+0.001 {
		t.Fatalf("diversity: got %v want %v", got, want)
	}
}
