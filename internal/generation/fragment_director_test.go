package generation

import (
	"github.com/grapestree/fgrapery/grapery-agent/internal/grapery_client"
	"testing"
)

func TestFragmentProgressExposesComicDocumentForBackgroundRecovery(t *testing.T) {
	document := map[string]any{"schemaVersion": 2, "revision": 3}
	output := map[string]any{}
	copyFragmentStatusOutput(output, &grapery_client.FragmentTaskStatus{
		Result: &grapery_client.FragmentTaskResult{ComicDocument: document},
	})
	got, ok := output["comicDocument"].(map[string]any)
	if !ok || got["revision"] != 3 {
		t.Fatalf("progress lost editable document: %#v", output)
	}
	copyFragmentStatusOutput(output, &grapery_client.FragmentTaskStatus{})
	if output["comicDocument"] == nil {
		t.Fatal("empty status erased existing document")
	}
}

func TestReviewFragmentComicDocumentAcceptsEditableNarrativePage(t *testing.T) {
	document := map[string]any{
		"schemaVersion": float64(2),
		"pages": []any{map[string]any{
			"plan": map[string]any{
				"panelCount": float64(2),
				"layout":     map[string]any{"panels": []any{map[string]any{}, map[string]any{}}},
				"panels": []any{
					map[string]any{"newInformation": "arrives", "dramaticIntent": "establish", "silentIntent": "quiet reveal"},
					map[string]any{"newInformation": "door opens", "dramaticIntent": "hook", "comicTexts": []any{map[string]any{"type": "dialogue"}}},
				},
			},
		}},
	}
	if review := reviewFragmentComicDocument(document); !review.Ready {
		t.Fatalf("expected document to pass Agent review: %#v", review)
	}
}

func TestReviewFragmentComicDocumentRejectsTemplatePadding(t *testing.T) {
	document := map[string]any{
		"schemaVersion": float64(2),
		"pages": []any{map[string]any{
			"plan": map[string]any{
				"panelCount": float64(2),
				"layout":     map[string]any{"panels": []any{map[string]any{}, map[string]any{}}},
				"panels":     []any{map[string]any{}, map[string]any{}},
			},
		}},
	}
	if review := reviewFragmentComicDocument(document); review.Ready || len(review.Issues) == 0 {
		t.Fatalf("expected padded page to fail Agent review: %#v", review)
	}
}
