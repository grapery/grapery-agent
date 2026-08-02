package domain

// CreationSessionRequest creates a durable client-facing creative session.
// The initial implementation is stateless on the agent side, but the contract
// gives iOS one stable id to carry across fragment, story, and branch turns.
type CreationSessionRequest struct {
	Surface    string                 `json:"surface,omitempty"`
	TargetType string                 `json:"targetType,omitempty"`
	Context    CreationContext        `json:"context,omitempty"`
	Options    CreationOptions        `json:"options,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

type CreationSessionResponse struct {
	SessionID  string          `json:"sessionId"`
	Surface    string          `json:"surface,omitempty"`
	TargetType string          `json:"targetType,omitempty"`
	Context    CreationContext `json:"context,omitempty"`
}

type CreationMessageRequest struct {
	Message         string          `json:"message" binding:"required"`
	ClientRequestID string          `json:"clientRequestId,omitempty"`
	Context         CreationContext `json:"context,omitempty"`
	Options         CreationOptions `json:"options,omitempty"`
}

type CreationContext struct {
	Surface            string `json:"surface,omitempty"`
	TargetType         string `json:"targetType,omitempty"`
	DraftID            string `json:"draftId,omitempty"`
	StoryID            string `json:"storyId,omitempty"`
	BranchID           string `json:"branchId,omitempty"`
	ParentStoryID      string `json:"parentStoryId,omitempty"`
	ParentStoryboardID string `json:"parentStoryboardId,omitempty"`
	SelectedImageIndex int    `json:"selectedImageIndex,omitempty"` // 1-based for replace operations.
}

type CreationOptions struct {
	ImageCount             int                     `json:"imageCount,omitempty"`
	BranchCount            int                     `json:"branchCount,omitempty"`
	SceneCount             int                     `json:"sceneCount,omitempty"`
	PlanningOnly           bool                    `json:"planningOnly,omitempty"`
	Style                  string                  `json:"style,omitempty"`
	Mood                   string                  `json:"mood,omitempty"`
	Length                 string                  `json:"length,omitempty"`
	Language               string                  `json:"language,omitempty"`
	Visibility             string                  `json:"visibility,omitempty"`
	AspectRatio            string                  `json:"aspectRatio,omitempty"`
	ConsistencyLevel       string                  `json:"consistencyLevel,omitempty"`
	ReferenceImages        []string                `json:"referenceImages,omitempty"`
	ReferenceSlots         []FragmentReferenceSlot `json:"referenceSlots,omitempty"`
	EnableReferenceAssets  *bool                   `json:"enableReferenceAssets,omitempty"`
	IncludeGenerationTrace bool                    `json:"includeGenerationTrace,omitempty"`
	PollIntervalSec        int                     `json:"pollIntervalSec,omitempty"`
	PollTimeoutSec         int                     `json:"pollTimeoutSec,omitempty"`
	UseComicPagePipeline   bool                    `json:"useComicPagePipeline,omitempty"`
	CharacterIDs           []string                `json:"characterIds,omitempty"`
}
