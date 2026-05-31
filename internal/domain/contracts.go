package domain

// Contract notes (read-only mapping from grapery HTTP APIs).
//
// Fragment:  POST /api/v1/fragments/generate, GET /api/v1/fragments/generate/:taskId
// Story:     POST /api/v1/ai/generate-story, GET /api/v1/ai/tasks/:id
// Storyboard: POST /api/v1/storyboards, .../generate/content|structure|image|images|continue
// Character: POST /api/v1/characters/generate, POST /api/v1/characters, .../generate-portrait|avatar|three-views

// FragmentGenerateInput mirrors grapery fragment generation request.
type FragmentGenerateInput struct {
	UserInput        string   `json:"userInput"`
	ReferenceImages  []string `json:"referenceImages,omitempty"`
	ImageCount       int      `json:"imageCount,omitempty"`
	Style            string   `json:"style,omitempty"`
	Mood             string   `json:"mood,omitempty"`
	Length           string   `json:"length,omitempty"`
	Language         string   `json:"language,omitempty"`
	Visibility       string   `json:"visibility,omitempty"`
	AspectRatio      string   `json:"aspectRatio,omitempty"`
	ConsistencyLevel string   `json:"consistencyLevel,omitempty"`
	PollIntervalSec  int      `json:"pollIntervalSec,omitempty"`
	PollTimeoutSec   int      `json:"pollTimeoutSec,omitempty"`
}

// StoryGenerateInput mirrors grapery AI story generation.
type StoryGenerateInput struct {
	Prompt      string   `json:"prompt"`
	Context     string   `json:"context,omitempty"`
	Characters  []string `json:"characters,omitempty"`
	Style       string   `json:"style,omitempty"`
	Length      string   `json:"length,omitempty"`
	Temperature float64  `json:"temperature,omitempty"`
}

// StoryboardGenerateInput drives storyboard creation + content pipeline.
type StoryboardGenerateInput struct {
	StoryID              string   `json:"storyId"`
	Title                string   `json:"title,omitempty"`
	RawInput             string   `json:"rawInput"`
	SceneCount           int      `json:"sceneCount,omitempty"`
	CharacterIDs         []string `json:"characterIds,omitempty"`
	Style                string   `json:"style,omitempty"`
	UseComicPagePipeline bool     `json:"useComicPagePipeline,omitempty"`
	GenerateImages       bool     `json:"generateImages,omitempty"`
	PollProgress         bool     `json:"pollProgress,omitempty"`
	PollTimeoutSec       int      `json:"pollTimeoutSec,omitempty"`
}

// CharacterGenerateInput drives attribute gen + optional create + portrait.
type CharacterGenerateInput struct {
	StoryID         string `json:"storyId"`
	Prompt          string `json:"prompt"`
	Name            string `json:"name,omitempty"`
	CreateRecord    bool   `json:"createRecord,omitempty"`
	GeneratePortrait bool  `json:"generatePortrait,omitempty"`
	GenerateAvatar  bool   `json:"generateAvatar,omitempty"`
	ReferenceImage  string `json:"referenceImage,omitempty"`
}

// BranchExploreInput requests N parallel continuations from a parent storyboard.
type BranchExploreInput struct {
	ParentStoryboardID string   `json:"parentStoryboardId"`
	SeedPrompt         string   `json:"seedPrompt,omitempty"`
	BranchCount        int      `json:"branchCount,omitempty"`
	SceneCount         int      `json:"sceneCount,omitempty"`
	Characters         []string `json:"characters,omitempty"`
	Strategies         []string `json:"strategies,omitempty"`
	ComicStyle         string   `json:"comicStyle,omitempty"`
}
