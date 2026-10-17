package generation

import (
	"github.com/grapestree/fgrapery/grapery-agent/internal/domain"
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

func TestFragmentProgressExposesQualityWithoutRequiringFullTrace(t *testing.T) {
	quality := &grapery_client.FragmentGenerationQuality{Status: "unreviewed", Reason: "audit unavailable"}
	output := map[string]any{}
	copyFragmentStatusOutput(output, &grapery_client.FragmentTaskStatus{Result: &grapery_client.FragmentTaskResult{Quality: quality}})
	if output["quality"] != quality {
		t.Fatal("publication quality was lost in agent progress")
	}
	copyFragmentStatusOutput(output, &grapery_client.FragmentTaskStatus{})
	if output["quality"] != quality {
		t.Fatal("empty poll erased quality outcome")
	}
}

func TestFragmentFailedBillingRetainsEditableAssetsAndRecoveryActions(t *testing.T) {
	amount := 0.0012
	status := &grapery_client.FragmentTaskStatus{TaskID: "task", Status: "failed", Error: "billing_unsettled: unavailable", Cost: &grapery_client.FragmentGenerationCost{Amount: &amount, State: "unsettled"}, Result: &grapery_client.FragmentTaskResult{DraftFragmentID: "saved-draft", Content: "saved story", ImageUrls: []string{"saved.jpg"}, Quality: &grapery_client.FragmentGenerationQuality{Status: "passed"}, ComicDocument: map[string]any{"revision": 2}, TokensUsed: 12}}
	output, content, tokens := fragmentFailureRecoveryOutput(status, domain.ContentRef{TaskID: "task"})
	if content.FragmentID != "saved-draft" || tokens != 12 || output["content"] != "saved story" || output["quality"] == nil || output["comicDocument"] == nil || output["cost"] != status.Cost || output["result"] != status.Result || output["draftFragmentId"] != "saved-draft" {
		t.Fatalf("failed generation discarded recovery snapshot %#v", output)
	}
}
