package grapery_client

import "context"

// ============ Generic AI APIs ============

type GenerateStoryRequest struct {
	Prompt      string   `json:"prompt"`
	Context     string   `json:"context,omitempty"`
	Characters  []string `json:"characters,omitempty"`
	Style       string   `json:"style,omitempty"`
	Length      string   `json:"length,omitempty"`
	Temperature float64  `json:"temperature,omitempty"`
}

type AITaskResponse struct {
	TaskID  string `json:"taskId"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type EnhancePromptRequest struct {
	OriginalPrompt string `json:"originalPrompt"`
	TargetType     string `json:"targetType,omitempty"`
	Style          string `json:"style,omitempty"`
	DetailLevel    string `json:"detailLevel,omitempty"`
}

type GenerateImageRequest struct {
	Prompt  string `json:"prompt"`
	Size    string `json:"size,omitempty"`
	Quality string `json:"quality,omitempty"`
	Style   string `json:"style,omitempty"`
	N       int    `json:"n,omitempty"`
}

type GenerateVideoRequest struct {
	Prompt      string   `json:"prompt"`
	Duration    int      `json:"duration,omitempty"`
	Resolution  string   `json:"resolution,omitempty"`
	FrameRate   int      `json:"frameRate,omitempty"`
	Style       string   `json:"style,omitempty"`
	StartFrame  string   `json:"startFrame,omitempty"`
	EndFrame    string   `json:"endFrame,omitempty"`
	SceneImages []string `json:"sceneImages,omitempty"`
}

func (c *Client) GenerateStory(ctx context.Context, req GenerateStoryRequest) (*AITaskResponse, error) {
	var resp AITaskResponse
	if err := c.post(ctx, "/api/v1/ai/generate-story", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) EnhancePrompt(ctx context.Context, req EnhancePromptRequest) (*AITaskResponse, error) {
	var resp AITaskResponse
	if err := c.post(ctx, "/api/v1/ai/enhance-prompt", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) GenerateImage(ctx context.Context, req GenerateImageRequest) (*AITaskResponse, error) {
	var resp AITaskResponse
	if err := c.post(ctx, "/api/v1/ai/generate-image", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) GenerateVideo(ctx context.Context, req GenerateVideoRequest) (*AITaskResponse, error) {
	var resp AITaskResponse
	if err := c.post(ctx, "/api/v1/ai/generate-video", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) GetAITaskStatus(ctx context.Context, taskID string) (*AITaskResponse, error) {
	var resp AITaskResponse
	if err := c.get(ctx, "/api/v1/ai/tasks/"+taskID, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
