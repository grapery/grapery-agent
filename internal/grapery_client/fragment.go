package grapery_client

import (
	"context"
	"net/http"
)

// ============ Fragment Generation ============

type GenerateFragmentRequest struct {
	UserInput               string                  `json:"userInput"`
	ImageUrls               []string                `json:"imageUrls,omitempty"`
	ReferenceSlots          []FragmentReferenceSlot `json:"referenceSlots,omitempty"`
	ImageCount              int                     `json:"imageCount"`
	Style                   string                  `json:"style"`
	Mood                    string                  `json:"mood,omitempty"`
	Length                  string                  `json:"length,omitempty"`
	Language                string                  `json:"language"`
	Visibility              string                  `json:"visibility"`
	AspectRatio             string                  `json:"aspectRatio,omitempty"`
	ConsistencyLevel        string                  `json:"consistencyLevel,omitempty"`
	TargetDraftFragmentID   string                  `json:"targetDraftFragmentId,omitempty"`
	ReplaceImageIndex       int                     `json:"replaceImageIndex,omitempty"`
	ClientMessageID         string                  `json:"clientMessageId,omitempty"`
	EnableReferenceAssets   *bool                   `json:"enableReferenceAssets,omitempty"`
	IncludeGenerationTrace  bool                    `json:"includeGenerationTrace,omitempty"`
	WorkflowReleaseID       string                  `json:"workflowReleaseId,omitempty"`
	WorkflowRunID           string                  `json:"workflowRunId,omitempty"`
	WorkflowSystemPrompt    string                  `json:"workflowSystemPrompt,omitempty"`
	WorkflowUserPrompt      string                  `json:"workflowUserPrompt,omitempty"`
	WorkflowModelConfig     map[string]any          `json:"workflowModelConfig,omitempty"`
	WorkflowOutputSchema    map[string]any          `json:"workflowOutputSchema,omitempty"`
	WorkflowPromptVersionID string                  `json:"workflowPromptVersionId,omitempty"`
}

type AnalyzeFragmentRequest struct {
	UserInput             string `json:"userInput"`
	Language              string `json:"language,omitempty"`
	AspectRatio           string `json:"aspectRatio,omitempty"`
	ImageCount            int    `json:"imageCount,omitempty"`
	Style                 string `json:"style,omitempty"`
	TargetDraftFragmentID string `json:"targetDraftFragmentId,omitempty"`
	EditOperation         string `json:"editOperation,omitempty"`
	SelectedImageIndex    int    `json:"selectedImageIndex,omitempty"`
}

type AnalyzeFragmentResponse struct {
	AssistantMessage   string                     `json:"assistantMessage"`
	IntentType         string                     `json:"intentType,omitempty"`
	EditPlan           CreativeEditPlan           `json:"editPlan"`
	GenerationIntent   FragmentGenerationIntent   `json:"generationIntent"`
	StoryElements      []FragmentReferenceSlot    `json:"storyElements"`
	RecommendedOptions FragmentRecommendedOptions `json:"recommendedOptions"`
}

type CreativeEditPlan struct {
	Operation                  string   `json:"operation"`
	TargetIndexes              []int    `json:"targetIndexes,omitempty"`
	RequestedChanges           []string `json:"requestedChanges,omitempty"`
	Preserve                   []string `json:"preserve,omitempty"`
	NeedsClarification         bool     `json:"needsClarification"`
	ClarificationQuestion      string   `json:"clarificationQuestion,omitempty"`
	EstimatedRegenerationCount int      `json:"estimatedRegenerationCount,omitempty"`
}

type FragmentGenerationIntent struct {
	UserInput   string `json:"userInput"`
	ImageCount  int    `json:"imageCount"`
	Style       string `json:"style"`
	Mood        string `json:"mood,omitempty"`
	Length      string `json:"length,omitempty"`
	Language    string `json:"language"`
	Visibility  string `json:"visibility"`
	AspectRatio string `json:"aspectRatio"`
	Topic       string `json:"topic,omitempty"`
}

type FragmentRecommendedOptions struct {
	StyleCandidates []string `json:"styleCandidates,omitempty"`
	CanStart        bool     `json:"canStart"`
}

type FragmentReferenceSlot struct {
	Key        string `json:"key"`
	Label      string `json:"label"`
	Kind       string `json:"kind"`
	Required   bool   `json:"required,omitempty"`
	InputType  string `json:"inputType,omitempty"`
	ImageURL   string `json:"imageUrl,omitempty"`
	HelperText string `json:"helperText,omitempty"`
}

type GenerateFragmentResponse struct {
	TaskID          string  `json:"taskId"`
	Status          string  `json:"status"`
	Progress        float64 `json:"progress"`
	DraftFragmentID string  `json:"draftFragmentId,omitempty"`
}

type FragmentTaskStatus struct {
	TaskID          string                           `json:"taskId"`
	Status          string                           `json:"status"`
	Progress        float64                          `json:"progress"`
	CurrentStep     string                           `json:"currentStep"`
	MessageKey      string                           `json:"messageKey,omitempty"`
	Stage           string                           `json:"stage,omitempty"`
	StoryText       string                           `json:"storyText,omitempty"`
	ImageSlots      []FragmentGenerationImageSlot    `json:"imageSlots,omitempty"`
	SlotMode        string                           `json:"slotMode,omitempty"`
	ImageProgress   *FragmentGenerationImageProgress `json:"imageProgress,omitempty"`
	GeneratedImages []string                         `json:"generatedImages,omitempty"`
	ChatMessages    []FragmentGenerationChatMessage  `json:"chatMessages,omitempty"`
	Cost            *FragmentGenerationCost          `json:"cost,omitempty"`
	Error           string                           `json:"error,omitempty"`
	CreatedAt       int64                            `json:"createdAt"`
	Result          *FragmentTaskResult              `json:"result,omitempty"`
}

type FragmentTaskResult struct {
	Content            string                           `json:"content,omitempty"`
	ImageUrls          []string                         `json:"imageUrls,omitempty"`
	TokensUsed         int                              `json:"tokensUsed,omitempty"`
	AspectRatio        string                           `json:"aspectRatio,omitempty"`
	ExpectedImageCount int                              `json:"expectedImageCount,omitempty"`
	ImageSlots         []FragmentGenerationImageSlot    `json:"imageSlots,omitempty"`
	ImageProgress      *FragmentGenerationImageProgress `json:"imageProgress,omitempty"`
	StoryElements      []FragmentReferenceSlot          `json:"storyElements,omitempty"`
	ComicDocument      map[string]any                   `json:"comicDocument,omitempty"`
}

type FragmentGenerationImageSlot struct {
	Index    int    `json:"index"`
	Title    string `json:"title,omitempty"`
	Caption  string `json:"caption,omitempty"`
	Status   string `json:"status,omitempty"`
	ImageURL string `json:"imageUrl,omitempty"`
}

type FragmentGenerationImageProgress struct {
	CompletedCount int `json:"completedCount"`
	TotalCount     int `json:"totalCount"`
}

type FragmentGenerationChatMessage struct {
	ID   string `json:"id,omitempty"`
	Type string `json:"type,omitempty"`
	Text string `json:"text,omitempty"`
}

type FragmentGenerationCost struct {
	Amount int    `json:"amount,omitempty"`
	Unit   string `json:"unit,omitempty"`
	Text   string `json:"text,omitempty"`
}

func (c *Client) GenerateFragment(ctx context.Context, req GenerateFragmentRequest) (*GenerateFragmentResponse, error) {
	var resp GenerateFragmentResponse
	if err := c.doRawRequest(ctx, http.MethodPost, "/api/v1/fragments/generate", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) AnalyzeFragment(ctx context.Context, req AnalyzeFragmentRequest) (*AnalyzeFragmentResponse, error) {
	var resp AnalyzeFragmentResponse
	if err := c.doRawRequest(ctx, http.MethodPost, "/api/v1/fragments/analyze", req, &resp); err != nil {
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
	Title               string               `json:"title"`
	Description         string               `json:"description"`
	Summary             string               `json:"summary,omitempty"`
	Style               string               `json:"style"`
	Genre               string               `json:"genre,omitempty"`
	Tags                []string             `json:"tags,omitempty"`
	SuggestedCharacters []SuggestedCharacter `json:"suggestedCharacters,omitempty"`
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
