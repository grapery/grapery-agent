package grapery_client

import "context"

// ============ Storyboard Generation ============

type CreateStoryboardRequest struct {
	StoryID               string                `json:"storyId"`
	ParentID              string                `json:"parentId,omitempty"`
	Title                 string                `json:"title,omitempty"`
	RawInput              string                `json:"rawInput"`
	Content               string                `json:"content,omitempty"`
	IsStandalone          bool                  `json:"isStandalone"`
	SceneCount            int                   `json:"sceneCount,omitempty"`
	CharacterRefs         []CharacterRef        `json:"characterRefs,omitempty"`
	SceneRefs             []SceneRef            `json:"sceneRefs,omitempty"`
	Tags                  []string              `json:"tags,omitempty"`
	UseComicPagePipeline  bool                  `json:"useComicPagePipeline"`
}

type CharacterRef struct {
	CharacterID string `json:"characterId"`
	Role        string `json:"role,omitempty"`
	Order       int    `json:"order,omitempty"`
	Notes       string `json:"notes,omitempty"`
}

type SceneRef struct {
	StorySceneID   string `json:"storySceneId"`
	Sequence       *int   `json:"sequence,omitempty"`
	IsPrimaryScene bool   `json:"isPrimaryScene,omitempty"`
}

type StoryboardResponse struct {
	ID      string `json:"id"`
	StoryID string `json:"storyId"`
	Title   string `json:"title"`
}

type GenerateStoryboardContentRequest struct {
	RawInput     string   `json:"rawInput"`
	CharacterIDs []string `json:"characterIds,omitempty"`
	SceneIDs     []string `json:"sceneIds,omitempty"`
	Style        string   `json:"style,omitempty"`
}

type GenerateSceneDetailsRequest struct {
	SceneID          string `json:"sceneId"`
	SceneTitle       string `json:"sceneTitle,omitempty"`
	SceneLocation    string `json:"sceneLocation,omitempty"`
	InputDescription string `json:"inputDescription"`
}

type GenerateSceneImageRequest struct {
	SceneID                  string   `json:"sceneId"`
	SceneTitle               string   `json:"sceneTitle,omitempty"`
	SceneDescription         string   `json:"sceneDescription"`
	ReferenceImages          []string `json:"referenceImages,omitempty"`
	SceneCharacters          []string `json:"sceneCharacters,omitempty"`
	CharacterReferenceImages []string `json:"characterReferenceImages,omitempty"`
	StoryStyleID             string   `json:"storyStyleId,omitempty"`
}

type GenerateAllImagesRequest struct {
	RegenerateAll bool   `json:"regenerateAll,omitempty"`
	StoryStyleID  string `json:"storyStyleId,omitempty"`
}

type StoryboardVideoRequest struct {
	SceneID           string `json:"sceneId"`
	SceneTitle        string `json:"sceneTitle,omitempty"`
	InputDescription  string `json:"inputDescription"`
	ReferenceImageURL string `json:"referenceImageUrl,omitempty"`
	EndFrameURL       string `json:"endFrameUrl,omitempty"`
}

type ContinueStoryboardRequest struct {
	RawInput      string   `json:"rawInput"`
	SceneCount    int      `json:"sceneCount,omitempty"`
	Characters    []string `json:"characters,omitempty"`
	GenerateVideo bool     `json:"generateVideo,omitempty"`
	ComicStyle    string   `json:"comicStyle,omitempty"`
}

type ContinueStoryboardResponse struct {
	NewStoryboard  *StoryboardResponse `json:"newStoryboard"`
	TokensUsed     int                 `json:"tokensUsed"`
}

type GenerateStructureResponse struct {
	AsyncAccepted      bool                        `json:"asyncAccepted"`
	Storyboard         *StoryboardResponse         `json:"storyboard,omitempty"`
	GenerationProgress *GenerationProgressResponse `json:"generationProgress,omitempty"`
}

type GenerationProgressResponse struct {
	StoryboardID          string `json:"storyboardId"`
	WorkflowStatus        string `json:"workflowStatus"`
	CurrentStep           int    `json:"currentStep"`
	TotalTokens           int    `json:"totalTokens"`
	IsGenerating          bool   `json:"isGenerating"`
	HasPendingTasks       bool   `json:"hasPendingTasks"`
	GenerationMessage     string `json:"generationMessage"`
	SuggestedResumeAction string `json:"suggestedResumeAction,omitempty"`
}

type GenerateComicPageRequest struct {
	SceneID                  string   `json:"sceneId"`
	SceneTitle               string   `json:"sceneTitle,omitempty"`
	SceneDescription         string   `json:"sceneDescription"`
	ReferenceImages          []string `json:"referenceImages,omitempty"`
	SceneCharacters          []string `json:"sceneCharacters,omitempty"`
	CharacterReferenceImages []string `json:"characterReferenceImages,omitempty"`
	LayoutPreset             string   `json:"layoutPreset,omitempty"`
	PanelCount               int      `json:"panelCount,omitempty"`
	PageAspectRatio          string   `json:"pageAspectRatio,omitempty"`
	DialogueMode             string   `json:"dialogueMode,omitempty"`
}

type GenerateAllComicPagesRequest struct {
	RegenerateAll   bool   `json:"regenerateAll,omitempty"`
	LayoutPreset    string `json:"layoutPreset,omitempty"`
	PanelCount      int    `json:"panelCount,omitempty"`
	PageAspectRatio string `json:"pageAspectRatio,omitempty"`
	DialogueMode    string `json:"dialogueMode,omitempty"`
}

type BatchComicPagesResponse struct {
	Results      []ComicPageResult `json:"results"`
	Total        int               `json:"total"`
	SuccessCount int               `json:"successCount"`
	FailedCount  int               `json:"failedCount"`
}

type ComicPageResult struct {
	SceneID          string `json:"sceneId"`
	SceneTitle       string `json:"sceneTitle"`
	Status           string `json:"status"`
	ErrorMessage     string `json:"errorMessage,omitempty"`
	ExistingImageURL string `json:"existingImageUrl,omitempty"`
}

func (c *Client) CreateStoryboard(ctx context.Context, req CreateStoryboardRequest) (*StoryboardResponse, error) {
	var resp StoryboardResponse
	if err := c.post(ctx, "/api/v1/storyboards", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) GenerateStoryboardContent(ctx context.Context, storyboardID string, req GenerateStoryboardContentRequest) error {
	// Async: backend starts content generation, no immediate data to return.
	return c.post(ctx, "/api/v1/storyboards/"+storyboardID+"/generate/content", req, nil)
}

func (c *Client) GenerateSceneDetails(ctx context.Context, storyboardID string, req GenerateSceneDetailsRequest) error {
	return c.post(ctx, "/api/v1/storyboards/"+storyboardID+"/generate/scene-details", req, nil)
}

func (c *Client) GenerateSceneImage(ctx context.Context, storyboardID string, req GenerateSceneImageRequest) error {
	// Async: backend starts image generation, no immediate data to return.
	return c.post(ctx, "/api/v1/storyboards/"+storyboardID+"/generate/image", req, nil)
}

func (c *Client) GenerateAllSceneImages(ctx context.Context, storyboardID string, req GenerateAllImagesRequest) error {
	// Async: backend starts batch image generation, no immediate data to return.
	return c.post(ctx, "/api/v1/storyboards/"+storyboardID+"/generate/images", req, nil)
}

func (c *Client) GenerateSceneVideo(ctx context.Context, storyboardID string, req StoryboardVideoRequest) error {
	// Async: backend starts video generation, no immediate data to return.
	return c.post(ctx, "/api/v1/storyboards/"+storyboardID+"/generate/video", req, nil)
}

func (c *Client) ContinueStoryboard(ctx context.Context, storyboardID string, req ContinueStoryboardRequest) (*ContinueStoryboardResponse, error) {
	var resp ContinueStoryboardResponse
	if err := c.post(ctx, "/api/v1/storyboards/"+storyboardID+"/continue", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) GenerateStructure(ctx context.Context, storyboardID string) (*GenerateStructureResponse, error) {
	var resp GenerateStructureResponse
	if err := c.post(ctx, "/api/v1/storyboards/"+storyboardID+"/generate/structure", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) GenerateComicPage(ctx context.Context, storyboardID string, req GenerateComicPageRequest) error {
	// Backend returns StoryboardImageGeneration but agent only needs success/fail.
	return c.post(ctx, "/api/v1/storyboards/"+storyboardID+"/generate/comic-page", req, nil)
}

func (c *Client) GenerateAllComicPages(ctx context.Context, storyboardID string, req GenerateAllComicPagesRequest) (*BatchComicPagesResponse, error) {
	var resp BatchComicPagesResponse
	if err := c.post(ctx, "/api/v1/storyboards/"+storyboardID+"/generate/comic-pages", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) GetGenerationProgress(ctx context.Context, storyboardID string) (*GenerationProgressResponse, error) {
	var resp GenerationProgressResponse
	if err := c.get(ctx, "/api/v1/storyboards/"+storyboardID+"/generation-progress", &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
