package domain

import "time"

// RunKind identifies which generation pipeline a run belongs to.
type RunKind string

const (
	RunKindFragment    RunKind = "fragment"
	RunKindStory       RunKind = "story"
	RunKindStoryboard  RunKind = "storyboard"
	RunKindCharacter   RunKind = "character"
	RunKindBranchBatch RunKind = "branch_batch"
)

// RunStatus is the lifecycle state of a generation run.
type RunStatus string

const (
	RunStatusPending   RunStatus = "pending"
	RunStatusRunning   RunStatus = "running"
	RunStatusWaiting   RunStatus = "waiting" // async task / human interrupt
	RunStatusSucceeded RunStatus = "succeeded"
	RunStatusFailed    RunStatus = "failed"
	RunStatusCancelled RunStatus = "cancelled"
)

// AgentVersion identifies prompt/strategy bundle used for the run.
type AgentVersion string

const (
	AgentFragmentCreator    AgentVersion = "fragment_creator:v1"
	AgentCharacterDesigner  AgentVersion = "character_designer:v1"
	AgentStoryboardDirector AgentVersion = "storyboard_director:v1"
	AgentBranchExplorer     AgentVersion = "branch_explorer:v1"
	AgentStoryGenerator     AgentVersion = "story_generator:v1"
)

// GenerationRun is the unified traceable unit for all agent-orchestrated generation.
type GenerationRun struct {
	ID              string            `json:"id"`
	Kind            RunKind           `json:"kind"`
	Status          RunStatus         `json:"status"`
	AgentVersion    AgentVersion      `json:"agentVersion"`
	UserIntent      string            `json:"userIntent,omitempty"`
	Input           map[string]any    `json:"input,omitempty"`
	Output          map[string]any    `json:"output,omitempty"`
	ParentRunID     string            `json:"parentRunId,omitempty"`
	BranchIndex     int               `json:"branchIndex,omitempty"`
	Strategy        string            `json:"strategy,omitempty"`
	ContentIDs      ContentRef        `json:"contentIds,omitempty"`
	ToolCalls       []ToolCallRecord  `json:"toolCalls,omitempty"`
	Error           string            `json:"error,omitempty"`
	TokensUsed      int               `json:"tokensUsed,omitempty"`
	ModelProvider   string            `json:"modelProvider,omitempty"`
	ModelName       string            `json:"modelName,omitempty"`
	CheckpointID    string            `json:"checkpointId,omitempty"`
	CreatedAt       time.Time         `json:"createdAt"`
	UpdatedAt       time.Time         `json:"updatedAt"`
	CompletedAt     *time.Time        `json:"completedAt,omitempty"`
}

// ContentRef links a run to grapery content entities.
type ContentRef struct {
	FragmentID    string   `json:"fragmentId,omitempty"`
	StoryID       string   `json:"storyId,omitempty"`
	StoryboardID  string   `json:"storyboardId,omitempty"`
	CharacterID   string   `json:"characterId,omitempty"`
	TaskID        string   `json:"taskId,omitempty"`
	BranchIDs     []string `json:"branchStoryboardIds,omitempty"`
}

// ToolCallRecord captures one tool invocation within a run.
type ToolCallRecord struct {
	Sequence   int            `json:"sequence"`
	ToolName   string         `json:"toolName"`
	Input      map[string]any `json:"input,omitempty"`
	Output     map[string]any `json:"output,omitempty"`
	StartedAt  time.Time      `json:"startedAt"`
	EndedAt    time.Time      `json:"endedAt"`
	DurationMs int64          `json:"durationMs"`
	Success    bool           `json:"success"`
	Error      string         `json:"error,omitempty"`
}

// BranchCandidate is one parallel-universe branch produced from a seed storyboard.
type BranchCandidate struct {
	BranchIndex      int            `json:"branchIndex"`
	Strategy         string         `json:"strategy"`
	RawInput         string         `json:"rawInput"`
	StoryboardID     string         `json:"storyboardId,omitempty"`
	StoryID          string         `json:"storyId,omitempty"`
	RunID            string         `json:"runId,omitempty"`
	NarrativeHook    string         `json:"narrativeHook,omitempty"`
	DiffFromParent   string         `json:"diffFromParent,omitempty"`
	VisualFeasibility string        `json:"visualFeasibility,omitempty"`
	CommunityAppeal  string         `json:"communityAppeal,omitempty"`
	Metadata         map[string]any `json:"metadata,omitempty"`
}

// BranchBatchResult groups multi-branch generation output.
type BranchBatchResult struct {
	ParentStoryboardID string            `json:"parentStoryboardId"`
	SeedPrompt         string            `json:"seedPrompt"`
	Candidates         []BranchCandidate `json:"candidates"`
}
