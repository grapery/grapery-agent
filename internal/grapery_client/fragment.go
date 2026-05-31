package grapery_client

import (
	"context"
	"net/http"
)

// ============ Fragment Generation ============

type GenerateFragmentRequest struct {
	UserInput              string   `json:"userInput"`
	ImageUrls              []string `json:"imageUrls,omitempty"`
	ImageCount             int      `json:"imageCount"`
	Style                  string   `json:"style"`
	Mood                   string   `json:"mood,omitempty"`
	Length                 string   `json:"length,omitempty"`
	Language               string   `json:"language"`
	Visibility             string   `json:"visibility"`
	AspectRatio            string   `json:"aspectRatio,omitempty"`
	ConsistencyLevel       string   `json:"consistencyLevel,omitempty"`
	EnableReferenceAssets  *bool    `json:"enableReferenceAssets,omitempty"`
	IncludeGenerationTrace bool     `json:"includeGenerationTrace,omitempty"`
}

type GenerateFragmentResponse struct {
	TaskID          string  `json:"taskId"`
	Status          string  `json:"status"`
	Progress        float64 `json:"progress"`
	DraftFragmentID string  `json:"draftFragmentId,omitempty"`
}

type FragmentTaskStatus struct {
	TaskID      string              `json:"taskId"`
	Status      string              `json:"status"`
	Progress    float64             `json:"progress"`
	CurrentStep string              `json:"currentStep"`
	Error       string              `json:"error,omitempty"`
	CreatedAt   int64               `json:"createdAt"`
	Result      *FragmentTaskResult `json:"result,omitempty"`
}

type FragmentTaskResult struct {
	Content     string   `json:"content,omitempty"`
	ImageUrls   []string `json:"imageUrls,omitempty"`
	TokensUsed  int      `json:"tokensUsed,omitempty"`
	AspectRatio string   `json:"aspectRatio,omitempty"`
}

func (c *Client) GenerateFragment(ctx context.Context, req GenerateFragmentRequest) (*GenerateFragmentResponse, error) {
	var resp GenerateFragmentResponse
	if err := c.doRawRequest(ctx, http.MethodPost, "/api/v1/fragments/generate", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) GetFragmentTaskStatus(ctx context.Context, taskID string) (*FragmentTaskStatus, error) {
	var resp FragmentTaskStatus
	if err := c.doRawRequest(ctx, http.MethodGet, "/api/v1/fragments/generate/"+taskID, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) CancelFragmentTask(ctx context.Context, taskID string) error {
	return c.doRawRequest(ctx, http.MethodDelete, "/api/v1/fragments/generate/"+taskID, nil, nil)
}

type ConvertFragmentToStoryRequest struct {
	Title             string `json:"title"`
	Description       string `json:"description,omitempty"`
	Genre             string `json:"genre,omitempty"`
	CoverImage        string `json:"coverImage,omitempty"`
	SceneCount        int    `json:"sceneCount,omitempty"`
	UseAI             bool   `json:"useAI,omitempty"`
	CollaborationType string `json:"collaborationType,omitempty"`
}

type ConvertFragmentToStoryResponse struct {
	Story      *StoryBrief `json:"story"`
	FragmentID string      `json:"fragmentId"`
}

type StoryBrief struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type StoryPrefillAIRequest struct {
	SceneCount int `json:"sceneCount,omitempty"`
}

type StoryPrefillAIResponse struct {
	Title               string                `json:"title"`
	Description         string                `json:"description"`
	Summary             string                `json:"summary,omitempty"`
	Style               string                `json:"style"`
	Genre               string                `json:"genre,omitempty"`
	Tags                []string              `json:"tags,omitempty"`
	SuggestedCharacters []SuggestedCharacter  `json:"suggestedCharacters,omitempty"`
}

type SuggestedCharacter struct {
	Name       string `json:"name"`
	Role       string `json:"role,omitempty"`
	Background string `json:"background,omitempty"`
}

func (c *Client) ConvertFragmentToStory(ctx context.Context, fragmentID string, req ConvertFragmentToStoryRequest) (*ConvertFragmentToStoryResponse, error) {
	var resp ConvertFragmentToStoryResponse
	if err := c.post(ctx, "/api/v1/fragments/"+fragmentID+"/convert-to-story", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) GetFragmentStoryPrefill(ctx context.Context, fragmentID string, req StoryPrefillAIRequest) (*StoryPrefillAIResponse, error) {
	var resp StoryPrefillAIResponse
	if err := c.post(ctx, "/api/v1/fragments/"+fragmentID+"/story-prefill-ai", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
