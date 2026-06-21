package grapery_client

import "context"

// ============ Character Generation ============

type GenerateCharacterAttrsRequest struct {
	Prompt string `json:"prompt"`
	Name   string `json:"name,omitempty"`
}

type GeneratedCharacterAttrs struct {
	Description     string `json:"description"`
	Personality     string `json:"personality"`
	Background      string `json:"background"`
	ShortTermGoal   string `json:"shortTermGoal"`
	LongTermGoal    string `json:"longTermGoal"`
	HandlingStyle   string `json:"handlingStyle"`
	CognitionRange  string `json:"cognitionRange"`
	AbilityFeatures string `json:"abilityFeatures"`
	Appearance      string `json:"appearance"`
	DressPreference string `json:"dressPreference"`
}

type CreateCharacterRequest struct {
	Name            string   `json:"name"`
	Description     string   `json:"description,omitempty"`
	Avatar          string   `json:"avatar,omitempty"`
	Personality     string   `json:"personality,omitempty"`
	Background      string   `json:"background,omitempty"`
	ShortTermGoal   string   `json:"shortTermGoal,omitempty"`
	LongTermGoal    string   `json:"longTermGoal,omitempty"`
	HandlingStyle   string   `json:"handlingStyle,omitempty"`
	CognitionRange  string   `json:"cognitionRange,omitempty"`
	AbilityFeatures string   `json:"abilityFeatures,omitempty"`
	Appearance      string   `json:"appearance,omitempty"`
	DressPreference string   `json:"dressPreference,omitempty"`
	StoryID         string   `json:"storyId"`
	IsPublic        bool     `json:"isPublic"`
	SourceType                 string   `json:"sourceType"`
	ReferenceImage             string   `json:"referenceImage,omitempty"`
	SourceFragmentID           string   `json:"sourceFragmentId,omitempty"`
	SourceFragmentCharacterKey string   `json:"sourceFragmentCharacterKey,omitempty"`
	Tags                       []string `json:"tags,omitempty"`
	NeedsPortrait              bool     `json:"needsPortrait,omitempty"`
}

type CharacterResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Avatar      string `json:"avatar"`
	PortraitURL string `json:"portraitUrl,omitempty"`
}

type GenerateAvatarRequest struct {
	AspectRatio string `json:"aspectRatio,omitempty"`
}

type GenerateAvatarResult struct {
	AvatarURL string `json:"avatarUrl"`
	RecordID  string `json:"recordId"`
}

type GeneratePortraitRequest struct {
	CustomPrompt   string `json:"customPrompt,omitempty"`
	ReferenceImage string `json:"referenceImage,omitempty"`
	AspectRatio    string `json:"aspectRatio,omitempty"`
}

type GeneratePortraitResult struct {
	PortraitURL string `json:"portraitUrl"`
	RecordID    string `json:"recordId"`
}

type GenerateThreeViewsRequest struct {
	RegenerateAll  bool   `json:"regenerateAll,omitempty"`
	ReferenceImage string `json:"referenceImage,omitempty"`
}

type GenerateThreeViewsResult struct {
	Views ThreeViews `json:"views"`
}

type ThreeViews struct {
	Sheet string `json:"sheet"`
	Front string `json:"front,omitempty"`
	Side  string `json:"side,omitempty"`
	Back  string `json:"back,omitempty"`
}

type CharacterGenTaskRequest struct {
	StoryID                    string              `json:"storyId"`
	SourceType                 string              `json:"sourceType"`
	Name                       string              `json:"name,omitempty"`
	Prompt                     string              `json:"prompt,omitempty"`
	Description                string              `json:"description,omitempty"`
	Background                 string              `json:"background,omitempty"`
	Personality                string              `json:"personality,omitempty"`
	ShortTermGoal              string              `json:"shortTermGoal,omitempty"`
	LongTermGoal               string              `json:"longTermGoal,omitempty"`
	HandlingStyle              string              `json:"handlingStyle,omitempty"`
	CognitionRange             string              `json:"cognitionRange,omitempty"`
	AbilityFeatures            string              `json:"abilityFeatures,omitempty"`
	Appearance                 string              `json:"appearance,omitempty"`
	DressPreference            string              `json:"dressPreference,omitempty"`
	ReferenceImage             string              `json:"referenceImage,omitempty"`
	SourceFragmentID           string              `json:"sourceFragmentId,omitempty"`
	SourceFragmentCharacterKey string              `json:"sourceFragmentCharacterKey,omitempty"`
	Suggestion                 *FragmentSuggestion `json:"suggestion,omitempty"`
}

type FragmentSuggestion struct {
	Key                 string `json:"key,omitempty"`
	Name                string `json:"name,omitempty"`
	Role                string `json:"role,omitempty"`
	Description         string `json:"description,omitempty"`
	Appearance          string `json:"appearance,omitempty"`
	Background          string `json:"background,omitempty"`
	ReferenceImage      string `json:"referenceImage,omitempty"`
	ReferenceImageURL   string `json:"referenceImageUrl,omitempty"`
	AlreadyCreated      bool   `json:"alreadyCreated,omitempty"`
	ExistingCharacterID string `json:"existingCharacterId,omitempty"`
}

type CharacterGenTask struct {
	ID                         string             `json:"id"`
	StoryID                    string             `json:"storyId"`
	CharacterID                string             `json:"characterId,omitempty"`
	SourceType                 string             `json:"sourceType"`
	SourceFragmentID           string             `json:"sourceFragmentId,omitempty"`
	SourceFragmentCharacterKey string             `json:"sourceFragmentCharacterKey,omitempty"`
	Status                     string             `json:"status"`
	Progress                   int                `json:"progress"`
	CurrentStep                string             `json:"currentStep,omitempty"`
	ErrorMessage               string             `json:"errorMessage,omitempty"`
	Character                  *CharacterResponse `json:"character,omitempty"`
}

type FragmentCharacterSuggestionsResponse struct {
	StoryID      string              `json:"storyId"`
	FragmentID   string              `json:"fragmentId"`
	Suggestions  []FragmentSuggestion `json:"suggestions"`
}

func (c *Client) GenerateCharacterAttrs(ctx context.Context, req GenerateCharacterAttrsRequest) (*GeneratedCharacterAttrs, error) {
	var resp GeneratedCharacterAttrs
	if err := c.post(ctx, "/api/v1/characters/generate", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) CreateCharacter(ctx context.Context, req CreateCharacterRequest) (*CharacterResponse, error) {
	var resp CharacterResponse
	if err := c.post(ctx, "/api/v1/characters", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) GenerateCharacterAvatar(ctx context.Context, characterID string, req GenerateAvatarRequest) (*GenerateAvatarResult, error) {
	var resp GenerateAvatarResult
	if err := c.post(ctx, "/api/v1/characters/"+characterID+"/generate-avatar", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) GenerateCharacterPortrait(ctx context.Context, characterID string, req GeneratePortraitRequest) (*GeneratePortraitResult, error) {
	var resp GeneratePortraitResult
	if err := c.post(ctx, "/api/v1/characters/"+characterID+"/generate-portrait", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) GenerateCharacterThreeViews(ctx context.Context, characterID string, req GenerateThreeViewsRequest) (*GenerateThreeViewsResult, error) {
	var resp GenerateThreeViewsResult
	if err := c.post(ctx, "/api/v1/characters/"+characterID+"/generate-three-views", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) StartCharacterGenTask(ctx context.Context, req CharacterGenTaskRequest) (*CharacterGenTask, error) {
	var resp CharacterGenTask
	if err := c.post(ctx, "/api/v1/character-generation-tasks", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) GetCharacterGenTask(ctx context.Context, taskID string) (*CharacterGenTask, error) {
	var resp CharacterGenTask
	if err := c.get(ctx, "/api/v1/character-generation-tasks/"+taskID, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) GetFragmentCharacterSuggestions(ctx context.Context, storyID string) (*FragmentCharacterSuggestionsResponse, error) {
	var resp FragmentCharacterSuggestionsResponse
	if err := c.get(ctx, "/api/v1/stories/"+storyID+"/fragment-character-suggestions", &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
