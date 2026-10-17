package eval

import (
	"github.com/grapestree/fgrapery/grapery-agent/internal/domain"
	"testing"
)

func TestOutcomeDoesNotConfuseSuccessfulToolsWithAcceptedWork(t *testing.T) {
	run := &domain.GenerationRun{Status: domain.RunStatusSucceeded, Output: map[string]any{"content": "story", "imageUrls": []string{"page"}, "comicDocument": map[string]any{}, "quality": map[string]any{"status": "unreviewed"}}}
	if GradeCreationOutcome(Seed{Kind: "fragment", ExpectedImages: 1}, run).Ready {
		t.Fatal("unreviewed artifact counted as ready")
	}
	run.Output["quality"] = map[string]any{"status": "passed"}
	if !GradeCreationOutcome(Seed{Kind: "fragment", ExpectedImages: 1}, run).Ready {
		t.Fatal("valid explicit outcome rejected")
	}
	if GradeCreationOutcome(Seed{Kind: "fragment", ExpectedImages: 2}, run).Ready {
		t.Fatal("wrong requested page count accepted")
	}
}
func TestCreationContractSeedsCoverThreeProducts(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range CreationContractSeeds("owned-fixture") {
		seen[s.Kind] = true
		if len(s.Rubric) == 0 {
			t.Fatalf("missing semantic rubric: %s", s.ID)
		}
	}
	for _, kind := range []string{"fragment", "storyboard", "character"} {
		if !seen[kind] {
			t.Fatal(kind)
		}
	}
}
func TestCharacterPartialAndMissingStoryboardEvidenceCannotPass(t *testing.T) {
	for _, kind := range []string{"character", "storyboard"} {
		if GradeCreationOutcome(Seed{Kind: kind}, &domain.GenerationRun{Status: domain.RunStatusSucceeded, Output: map[string]any{"partial": true}}).Ready {
			t.Fatal(kind)
		}
	}
}
