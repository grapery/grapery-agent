package domain

// Contract notes (read-only mapping from grapery HTTP APIs).
//
// Fragment:       POST /api/v1/fragments/generate, GET /api/v1/fragments/generate/:taskId
// FragmentPanel:  POST /api/v1/fragment-panels/generate, GET /api/v1/fragment-panels/generate/:taskId
// Story:          POST /api/v1/ai/generate-story, GET /api/v1/ai/tasks/:id
// Storyboard:     POST /api/v1/storyboards, .../generate/structure|image|images|continue
// Character:      POST /api/v1/characters/generate, POST /api/v1/characters, .../character-generation-tasks

// FragmentGenerateInput mirrors grapery text fragment generation (FragmentCreator).
type FragmentGenerateInput struct {
	UserInput              string                  `json:"userInput"`
	ReferenceImages        []string                `json:"referenceImages,omitempty"`
	ReferenceSlots         []FragmentReferenceSlot `json:"referenceSlots,omitempty"`
	ImageCount             int                     `json:"imageCount,omitempty"`
	Style                  string                  `json:"style,omitempty"`
	Mood                   string                  `json:"mood,omitempty"`
	Length                 string                  `json:"length,omitempty"`
	Language               string                  `json:"language,omitempty"`
	Visibility             string                  `json:"visibility,omitempty"`
	AspectRatio            string                  `json:"aspectRatio,omitempty"`
	ConsistencyLevel       string                  `json:"consistencyLevel,omitempty"`
	TargetDraftFragmentID  string                  `json:"targetDraftFragmentId,omitempty"`
	ReplaceImageIndex      int                     `json:"replaceImageIndex,omitempty"`
	ClientMessageID        string                  `json:"clientMessageId,omitempty"`
	EnableReferenceAssets  *bool                   `json:"enableReferenceAssets,omitempty"`
	IncludeGenerationTrace bool                    `json:"includeGenerationTrace,omitempty"`
	PollIntervalSec        int                     `json:"pollIntervalSec,omitempty"`
	PollTimeoutSec         int                     `json:"pollTimeoutSec,omitempty"`
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

// FragmentPanelGenerateInput mirrors grapery reference-image panel fragment (FragmentPanelCreator).
type FragmentPanelGenerateInput struct {
	ClientRequestID        string `json:"clientRequestId,omitempty"`
	UserInput              string `json:"userInput"`
	ReferenceImageURL      string `json:"referenceImageUrl"`
	Style                  string `json:"style,omitempty"`
	PanelCount             int    `json:"panelCount,omitempty"`
	Visibility             string `json:"visibility,omitempty"`
	Topic                  string `json:"topic,omitempty"`
	AspectRatio            string `json:"aspectRatio,omitempty"`
	DialogueMode           string `json:"dialogueMode,omitempty"`
	ConsistencyLevel       string `json:"consistencyLevel,omitempty"`
	EnableReferenceAssets  *bool  `json:"enableReferenceAssets,omitempty"`
	IncludeGenerationTrace bool   `json:"includeGenerationTrace,omitempty"`
	PollIntervalSec        int    `json:"pollIntervalSec,omitempty"`
	PollTimeoutSec         int    `json:"pollTimeoutSec,omitempty"`
}

// StoryGenerateInput mirrors grapery AI story generation.
type StoryGenerateInput struct {
	ClientRequestID string   `json:"clientRequestId,omitempty"`
	Prompt          string   `json:"prompt"`
	Context         string   `json:"context,omitempty"`
	Characters      []string `json:"characters,omitempty"`
	Style           string   `json:"style,omitempty"`
	Length          string   `json:"length,omitempty"`
	Temperature     float64  `json:"temperature,omitempty"`
}

// StoryboardGenerateInput drives storyboard creation pipeline (StoryboardDirector).
type StoryboardGenerateInput struct {
	ClientRequestID      string   `json:"clientRequestId,omitempty"`
	StoryID              string   `json:"storyId"`
	Title                string   `json:"title,omitempty"`
	RawInput             string   `json:"rawInput"`
	SceneCount           int      `json:"sceneCount,omitempty"`
	CharacterIDs         []string `json:"characterIds,omitempty"`
	Style                string   `json:"style,omitempty"`
	ComicStyle           string   `json:"comicStyle,omitempty"`
	AspectRatio          string   `json:"aspectRatio,omitempty"`
	ParentStoryboardID   string   `json:"parentStoryboardId,omitempty"`
	DraftStoryboardID    string   `json:"draftStoryboardId,omitempty"`
	UseComicPagePipeline bool     `json:"useComicPagePipeline,omitempty"`
	GenerateImages       bool     `json:"generateImages,omitempty"`
	PollProgress         *bool    `json:"pollProgress,omitempty"` // nil/true: wait for create-time redesign; false: return after create
	RegenerateStructure  bool     `json:"regenerateStructure,omitempty"`
	PollTimeoutSec       int      `json:"pollTimeoutSec,omitempty"`
	WorkflowReleaseID    string   `json:"workflowReleaseId,omitempty"`
}

// CharacterGenerateInput drives attribute gen + optional create + portrait (CharacterDesigner).
type CharacterGenerateInput struct {
	ClientRequestID            string `json:"clientRequestId,omitempty"`
	StoryID                    string `json:"storyId"`
	Prompt                     string `json:"prompt"`
	Name                       string `json:"name,omitempty"`
	CreateRecord               bool   `json:"createRecord,omitempty"`
	UseAsyncTask               bool   `json:"useAsyncTask,omitempty"`
	GeneratePortrait           bool   `json:"generatePortrait,omitempty"`
	GenerateAvatar             bool   `json:"generateAvatar,omitempty"`
	GenerateThreeViews         bool   `json:"generateThreeViews,omitempty"`
	ReferenceImage             string `json:"referenceImage,omitempty"`
	SourceFragmentID           string `json:"sourceFragmentId,omitempty"`
	SourceFragmentCharacterKey string `json:"sourceFragmentCharacterKey,omitempty"`
	SourceType                 string `json:"sourceType,omitempty"`
}

// BranchExploreInput requests N parallel continuations from a parent storyboard.
type BranchExploreInput struct {
	ClientRequestID    string   `json:"clientRequestId,omitempty"`
	ParentStoryboardID string   `json:"parentStoryboardId"`
	SeedPrompt         string   `json:"seedPrompt,omitempty"`
	BranchCount        int      `json:"branchCount,omitempty"`
	SceneCount         int      `json:"sceneCount,omitempty"`
	Characters         []string `json:"characters,omitempty"`
	Strategies         []string `json:"strategies,omitempty"`
	ComicStyle         string   `json:"comicStyle,omitempty"`
}

type WorkflowStartInput struct {
	Surface         string         `json:"surface"`
	Action          string         `json:"action"`
	TenantID        string         `json:"tenantId,omitempty"`
	ReleaseID       string         `json:"releaseId,omitempty"`
	ClientRequestID string         `json:"clientRequestId,omitempty"`
	Input           map[string]any `json:"input"`
}
