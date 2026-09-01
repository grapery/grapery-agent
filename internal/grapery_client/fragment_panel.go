package grapery_client

import (
	"context"
	"net/http"
)

// ============ Fragment Panel Generation ============

type GenerateFragmentPanelRequest struct {
	UserInput              string `json:"userInput"`
	ReferenceImageURL      string `json:"referenceImageUrl"`
	Style                  string `json:"style,omitempty"`
	Language               string `json:"language,omitempty"`
	PanelCount             int    `json:"panelCount,omitempty"`
	Visibility             string `json:"visibility,omitempty"`
	Topic                  string `json:"topic,omitempty"`
	AspectRatio            string `json:"aspectRatio,omitempty"`
	DialogueMode           string `json:"dialogueMode,omitempty"`
	ConsistencyLevel       string `json:"consistencyLevel,omitempty"`
	EnableReferenceAssets  *bool  `json:"enableReferenceAssets,omitempty"`
	IncludeGenerationTrace bool   `json:"includeGenerationTrace,omitempty"`
}

type GenerateFragmentPanelResponse struct {
	TaskID          string  `json:"taskId"`
	Status          string  `json:"status"`
	Progress        float64 `json:"progress"`
	CurrentStep     string  `json:"currentStep,omitempty"`
	DraftFragmentID string  `json:"draftFragmentId,omitempty"`
}

type FragmentPanelTaskStatus struct {
	TaskID          string                `json:"taskId"`
	Status          string                `json:"status"`
	Progress        float64               `json:"progress"`
	CurrentStep     string                `json:"currentStep"`
	DraftFragmentID string                `json:"draftFragmentId,omitempty"`
	Error           string                `json:"error,omitempty"`
	CombinedContent string                `json:"combinedContent,omitempty"`
	Panels          []FragmentPanelResult `json:"panels,omitempty"`
	VisualBible     any                   `json:"visualBible,omitempty"`
	TokensUsed      int                   `json:"tokensUsed,omitempty"`
}

type FragmentPanelResult struct {
	Index    int    `json:"index"`
	ImageURL string `json:"imageUrl"`
	Caption  string `json:"caption"`
}

func (c *Client) GenerateFragmentPanel(ctx context.Context, req GenerateFragmentPanelRequest) (*GenerateFragmentPanelResponse, error) {
	var resp GenerateFragmentPanelResponse
	if err := c.doRawRequest(ctx, http.MethodPost, "/api/v1/fragment-panels/generate", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) RetryFragmentPanelTask(ctx context.Context, taskID string) (*GenerateFragmentPanelResponse, error) {
	var resp GenerateFragmentPanelResponse
	if err := c.doRawRequest(ctx, http.MethodPost, "/api/v1/fragment-panels/generate/"+taskID+"/retry", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) ResumeFragmentPanelTask(ctx context.Context, taskID string) (*GenerateFragmentPanelResponse, error) {
	var resp GenerateFragmentPanelResponse
	if err := c.doRawRequest(ctx, http.MethodPost, "/api/v1/fragment-panels/generate/"+taskID+"/resume", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) GetFragmentPanelTaskStatus(ctx context.Context, taskID string) (*FragmentPanelTaskStatus, error) {
	var raw map[string]any
	if err := c.doRawRequest(ctx, http.MethodGet, "/api/v1/fragment-panels/generate/"+taskID, nil, &raw); err != nil {
		return nil, err
	}
	st := &FragmentPanelTaskStatus{
		TaskID:          strMap(raw, "taskId"),
		Status:          strMap(raw, "status"),
		Progress:        numMap(raw, "progress"),
		CurrentStep:     strMap(raw, "currentStep"),
		DraftFragmentID: strMap(raw, "draftFragmentId"),
		Error:           strMap(raw, "error"),
		CombinedContent: strMap(raw, "combinedContent"),
		VisualBible:     raw["visualBible"],
	}
	if panels, ok := raw["panels"].([]any); ok {
		for _, p := range panels {
			pm, _ := p.(map[string]any)
			if pm == nil {
				continue
			}
			st.Panels = append(st.Panels, FragmentPanelResult{
				Index:    int(numMap(pm, "index")),
				ImageURL: strMap(pm, "imageUrl"),
				Caption:  strMap(pm, "caption"),
			})
		}
	}
	if metrics, ok := raw["metrics"].(map[string]any); ok {
		st.TokensUsed = int(numMap(metrics, "totalTokens"))
	}
	return st, nil
}

func strMap(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func numMap(m map[string]any, key string) float64 {
	if m == nil {
		return 0
	}
	v, ok := m[key]
	if !ok || v == nil {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	default:
		return 0
	}
}
