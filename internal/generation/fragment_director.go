package generation

import "fmt"

// fragmentDirectorReview is the Agent-side semantic gate. Grapery owns durable
// execution and rendering, while the Agent checks whether the returned v2
// document still represents an editable, narratively useful comic.
type fragmentDirectorReview struct {
	Ready  bool     `json:"ready"`
	Issues []string `json:"issues,omitempty"`
}

func reviewFragmentComicDocument(document map[string]any) fragmentDirectorReview {
	review := fragmentDirectorReview{Ready: true}
	if len(document) == 0 {
		return fragmentDirectorReview{Ready: false, Issues: []string{"missing comicDocument"}}
	}
	if intValue(document["schemaVersion"]) < 2 {
		review.Issues = append(review.Issues, "comicDocument schemaVersion must be v2 or newer")
	}
	pages, ok := document["pages"].([]any)
	if !ok || len(pages) == 0 {
		review.Issues = append(review.Issues, "comicDocument contains no pages")
		review.Ready = false
		return review
	}
	for pageIndex, rawPage := range pages {
		page, _ := rawPage.(map[string]any)
		plan, _ := page["plan"].(map[string]any)
		panels, _ := plan["panels"].([]any)
		panelCount := intValue(plan["panelCount"])
		if panelCount < 2 || panelCount > 10 || panelCount != len(panels) {
			review.Issues = append(review.Issues, fmt.Sprintf("page %d has invalid adaptive panel count", pageIndex+1))
		}
		layout, _ := plan["layout"].(map[string]any)
		layoutPanels, _ := layout["panels"].([]any)
		if len(layoutPanels) != len(panels) {
			review.Issues = append(review.Issues, fmt.Sprintf("page %d layout does not match its panels", pageIndex+1))
		}
		for panelIndex, rawPanel := range panels {
			panel, _ := rawPanel.(map[string]any)
			if stringValue(panel["newInformation"]) == "" || stringValue(panel["dramaticIntent"]) == "" {
				review.Issues = append(review.Issues, fmt.Sprintf("page %d panel %d does not advance the story", pageIndex+1, panelIndex+1))
			}
			texts, _ := panel["comicTexts"].([]any)
			if len(texts) == 0 && stringValue(panel["silentIntent"]) == "" {
				review.Issues = append(review.Issues, fmt.Sprintf("page %d panel %d has accidental silence", pageIndex+1, panelIndex+1))
			}
		}
	}
	review.Ready = len(review.Issues) == 0
	return review
}

func intValue(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case float64:
		return int(typed)
	default:
		return 0
	}
}

func stringValue(value any) string {
	if typed, ok := value.(string); ok {
		return typed
	}
	return ""
}
