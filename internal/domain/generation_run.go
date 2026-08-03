package domain

import "time"

// RunKind identifies which generation pipeline a run belongs to.
type RunKind string

const (
	RunKindFragment      RunKind = "fragment"
	RunKindFragmentPanel RunKind = "fragment_panel"
	RunKindStory         RunKind = "story"
	RunKindStoryboard    RunKind = "storyboard"
	RunKindCharacter     RunKind = "character"
	RunKindBranchBatch   RunKind = "branch_batch"
	RunKindWorkflow      RunKind = "workflow"
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
	AgentFragmentCreator      AgentVersion = "fragment_creator:v1"
	AgentFragmentPanelCreator AgentVersion = "fragment_panel_creator:v1"
	AgentCharacterDesigner    AgentVersion = "character_designer:v1"
	AgentStoryboardDirector   AgentVersion = "storyboard_director:v1"
	AgentBranchExplorer       AgentVersion = "branch_explorer:v1"
	AgentStoryGenerator       AgentVersion = "story_generator:v1"
	AgentWorkflowRuntime      AgentVersion = "workflow_runtime:v1"
)

// GenerationRun is the unified traceable unit for all agent-orchestrated generation.
type GenerationRun struct {
	ID                string                           `json:"id"`
	UserID            string                           `json:"userId,omitempty"`
	Kind              RunKind                          `json:"kind"`
	Status            RunStatus                        `json:"status"`
	Phase             string                           `json:"phase,omitempty"`
	Progress          int                              `json:"progress,omitempty"`
	AgentVersion      AgentVersion                     `json:"agentVersion"`
	WorkflowReleaseID string                           `json:"workflowReleaseId,omitempty"`
	WorkflowKey       string                           `json:"workflowKey,omitempty"`
	WorkflowVersion   int                              `json:"workflowVersion,omitempty"`
	WorkflowChecksum  string                           `json:"workflowChecksum,omitempty"`
	PromptBundle      map[string]string                `json:"promptBundle,omitempty"`
	PromptSnapshots   map[string]PromptTemplateVersion `json:"promptSnapshots,omitempty"`
	UserIntent        string                           `json:"userIntent,omitempty"`
	Input             map[string]any                   `json:"input,omitempty"`
	Output            map[string]any                   `json:"output,omitempty"`
	ParentRunID       string                           `json:"parentRunId,omitempty"`
	BranchIndex       int                              `json:"branchIndex,omitempty"`
	Strategy          string                           `json:"strategy,omitempty"`
	ContentIDs        ContentRef                       `json:"contentIds,omitempty"`
	ToolCalls         []ToolCallRecord                 `json:"toolCalls,omitempty"`
	StepAudits        []GenerationStepAudit            `json:"stepAudits,omitempty"`
	Error             string                           `json:"error,omitempty"`
	TokensUsed        int                              `json:"tokensUsed,omitempty"`
	ModelProvider     string                           `json:"modelProvider,omitempty"`
	ModelName         string                           `json:"modelName,omitempty"`
	CheckpointID      string                           `json:"checkpointId,omitempty"`
	ClientRequestID   string                           `json:"clientRequestId,omitempty"`
	SourceTaskID      string                           `json:"sourceTaskId,omitempty"`
	Sequence          int64                            `json:"sequence,omitempty"`
	Reused            bool                             `json:"reused,omitempty"`
	CreatedAt         time.Time                        `json:"createdAt"`
	UpdatedAt         time.Time                        `json:"updatedAt"`
	CompletedAt       *time.Time                       `json:"completedAt,omitempty"`
}

// StepAuditStatus 是单步审计状态。
type StepAuditStatus string

const (
	StepStarted   StepAuditStatus = "started"
	StepSucceeded StepAuditStatus = "succeeded"
	StepFailed    StepAuditStatus = "failed"
	StepCancelled StepAuditStatus = "cancelled"
	StepRetrying  StepAuditStatus = "retrying"
)

// GenerationStepAudit 是跨 fragment/story/character/storyboard/fragment_panel 的统一生成步骤审计。
// 要求：不仅记录成功输出，还要记录提示词、中间结果、重试与失败，以及每步 token 用量。
// 失败/重试作为独立 attempt 记录，禁止用成功结果覆盖失败 attempt。
type GenerationStepAudit struct {
	ID           string          `json:"id"`
	Sequence     int             `json:"sequence"`
	RunID        string          `json:"runId"`
	TaskID       string          `json:"taskId,omitempty"`
	BusinessType RunKind         `json:"businessType,omitempty"`
	BusinessID   string          `json:"businessId,omitempty"`
	AgentVersion AgentVersion    `json:"agentVersion,omitempty"`
	StepName     string          `json:"stepName"`
	Attempt      int             `json:"attempt"`
	Status       StepAuditStatus `json:"status"`
	Provider     string          `json:"provider,omitempty"`
	Model        string          `json:"model,omitempty"`
	Prompt       string          `json:"prompt,omitempty"`
	InputRefs    []string        `json:"inputRefs,omitempty"`
	RawOutput    string          `json:"rawOutput,omitempty"`
	ParsedOutput map[string]any  `json:"parsedOutput,omitempty"`
	ErrorCode    string          `json:"errorCode,omitempty"`
	ErrorMessage string          `json:"errorMessage,omitempty"`
	InputTokens  int             `json:"inputTokens,omitempty"`
	OutputTokens int             `json:"outputTokens,omitempty"`
	TotalTokens  int             `json:"totalTokens,omitempty"`
	DurationMs   int64           `json:"durationMs,omitempty"`
	StartedAt    time.Time       `json:"startedAt"`
	EndedAt      time.Time       `json:"endedAt,omitempty"`
	Metadata     map[string]any  `json:"metadata,omitempty"`
	UserID       string          `json:"userId,omitempty"`
}

// ContentRef links a run to grapery content entities.
type ContentRef struct {
	FragmentID   string   `json:"fragmentId,omitempty"`
	StoryID      string   `json:"storyId,omitempty"`
	StoryboardID string   `json:"storyboardId,omitempty"`
	CharacterID  string   `json:"characterId,omitempty"`
	TaskID       string   `json:"taskId,omitempty"`
	BranchIDs    []string `json:"branchStoryboardIds,omitempty"`
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
	BranchIndex       int            `json:"branchIndex"`
	Strategy          string         `json:"strategy"`
	RawInput          string         `json:"rawInput"`
	StoryboardID      string         `json:"storyboardId,omitempty"`
	StoryID           string         `json:"storyId,omitempty"`
	RunID             string         `json:"runId,omitempty"`
	NarrativeHook     string         `json:"narrativeHook,omitempty"`
	DiffFromParent    string         `json:"diffFromParent,omitempty"`
	VisualFeasibility string         `json:"visualFeasibility,omitempty"`
	CommunityAppeal   string         `json:"communityAppeal,omitempty"`
	Metadata          map[string]any `json:"metadata,omitempty"`
}

// BranchBatchResult groups multi-branch generation output.
type BranchBatchResult struct {
	ParentStoryboardID string            `json:"parentStoryboardId"`
	SeedPrompt         string            `json:"seedPrompt"`
	Candidates         []BranchCandidate `json:"candidates"`
}
