package eval

import (
	"encoding/json"
	"fmt"
	"github.com/grapestree/fgrapery/grapery-agent/internal/domain"
	"strings"
)

// OutcomeGrade is an objective artifact gate, not a claim of artistic quality.
// Seed.Rubric remains a separate human review of the actual images and narrative.
type OutcomeGrade struct {
	Ready  bool     `json:"ready"`
	Issues []string `json:"issues,omitempty"`
}

func GradeCreationOutcome(seed Seed, run *domain.GenerationRun) OutcomeGrade {
	grade := OutcomeGrade{}
	fail := func(reason string) { grade.Issues = append(grade.Issues, reason) }
	if run == nil || run.Status != domain.RunStatusSucceeded {
		fail("run did not succeed")
		return grade
	}
	raw, _ := json.Marshal(run.Output)
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	switch seed.Kind {
	case "fragment":
		if strings.TrimSpace(stringValue(out["content"])) == "" {
			fail("missing story text")
		}
		images, _ := out["imageUrls"].([]any)
		expected := seed.ExpectedImages
		if expected == 0 {
			expected = 1
		}
		if len(images) != expected {
			fail(fmt.Sprintf("expected %d pages, received %d", expected, len(images)))
		}
		quality, _ := out["quality"].(map[string]any)
		if quality["status"] != "passed" {
			fail("rendered artwork lacks explicit passed quality")
		}
		doc, _ := out["comicDocument"].(map[string]any)
		if doc == nil {
			fail("missing editable comic document")
		}
	case "character":
		if out["partial"] == true {
			fail("character has incomplete stages")
		}
		stages, _ := out["stages"].(map[string]any)
		for _, key := range []string{"attributes", "character", "portrait"} {
			stage, _ := stages[key].(map[string]any)
			if stage["status"] != "completed" && stage["status"] != "succeeded" {
				fail("incomplete character stage: " + key)
			}
		}
		if run.ContentIDs.CharacterID == "" {
			fail("missing persisted character")
		}
	case "storyboard":
		artifact, _ := out["artifact"].(map[string]any)
		if artifact == nil {
			fail("missing accepted storyboard artifact")
			break
		}
		content := stringValue(artifact["acceptedContent"])
		if content == "" {
			content = stringValue(artifact["content"])
		}
		if strings.TrimSpace(content) == "" {
			fail("missing accepted chapter text")
		}
		scenes, _ := artifact["scenes"].([]any)
		if seed.ExpectedScenes > 0 && len(scenes) != seed.ExpectedScenes {
			fail("accepted scene count differs from request")
		}
	default:
		if len(out) == 0 {
			fail("missing output artifact")
		}
	}
	grade.Ready = len(grade.Issues) == 0
	return grade
}
func stringValue(value any) string { s, _ := value.(string); return s }
