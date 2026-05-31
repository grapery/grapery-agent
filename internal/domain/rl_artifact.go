package domain

import "time"

// RLArtifactType classifies exportable training/eval records.
type RLArtifactType string

const (
	ArtifactTypeGenerationTrace RLArtifactType = "generation_trace"
	ArtifactTypeBranchPair      RLArtifactType = "branch_pair_preference"
	ArtifactTypeBranchSelection RLArtifactType = "branch_selection"
	ArtifactTypeEvalResult      RLArtifactType = "eval_result"
)

// RLArtifact is one JSONL-exportable record for offline RL / preference learning.
type RLArtifact struct {
	ID        string         `json:"id"`
	Type      RLArtifactType `json:"type"`
	RunID     string         `json:"runId,omitempty"`
	CreatedAt time.Time      `json:"createdAt"`

	// Common fields for preference samples
	Prompt           string   `json:"prompt,omitempty"`
	BranchA          string   `json:"branchA,omitempty"`
	BranchB          string   `json:"branchB,omitempty"`
	Preferred        string   `json:"preferred,omitempty"` // "a" | "b"
	SelectedBranchID string   `json:"selectedBranchId,omitempty"`
	RejectedIDs      []string `json:"rejectedBranchIds,omitempty"`
	SimulatedReward  float64  `json:"simulatedReward,omitempty"`

	// Trace payload
	GenerationRun *GenerationRun `json:"generationRun,omitempty"`
	BranchBatch   *BranchBatchResult `json:"branchBatch,omitempty"`

	// Eval payload
	Eval *EvalRecord `json:"eval,omitempty"`

	// Future grapery community feedback (not wired in phase 1)
	FeedbackWindow *FeedbackWindow `json:"feedbackWindow,omitempty"`
}

// FeedbackWindow reserves slots for future community signals from grapery.
type FeedbackWindow struct {
	Impressions   int     `json:"impressions,omitempty"`
	Clicks        int     `json:"clicks,omitempty"`
	DwellMs       int64   `json:"dwellMs,omitempty"`
	Likes         int     `json:"likes,omitempty"`
	Saves         int     `json:"saves,omitempty"`
	Comments      int     `json:"comments,omitempty"`
	Shares        int     `json:"shares,omitempty"`
	Forks         int     `json:"forks,omitempty"`
	NegativeSignals int   `json:"negativeSignals,omitempty"`
	RewardScore   float64 `json:"rewardScore,omitempty"`
}

// EvalRecord captures one eval harness execution result.
type EvalRecord struct {
	SeedID            string             `json:"seedId"`
	AgentVersion      AgentVersion       `json:"agentVersion"`
	ToolFailureRate   float64            `json:"toolFailureRate"`
	BranchDiversity   float64            `json:"branchDiversity,omitempty"`
	AvgToolDurationMs float64            `json:"avgToolDurationMs"`
	TotalTokens       int                `json:"totalTokens"`
	RunIDs            []string           `json:"runIds,omitempty"`
	Metrics           map[string]float64 `json:"metrics,omitempty"`
	Notes             string             `json:"notes,omitempty"`
}

// PreferencePairRequest records a human or simulated preference between two branches.
type PreferencePairRequest struct {
	RunID     string  `json:"runId,omitempty"`
	Prompt    string  `json:"prompt" binding:"required"`
	BranchA   string  `json:"branchA" binding:"required"`
	BranchB   string  `json:"branchB" binding:"required"`
	Preferred string  `json:"preferred" binding:"required"` // a | b
	Reward    float64 `json:"reward,omitempty"`
}

// BranchSelectionRequest records winner/losers from a batch.
type BranchSelectionRequest struct {
	RunID            string   `json:"runId,omitempty"`
	Prompt           string   `json:"prompt" binding:"required"`
	SelectedBranchID string   `json:"selectedBranchId" binding:"required"`
	RejectedIDs      []string `json:"rejectedBranchIds" binding:"required"`
	Reward           float64  `json:"reward,omitempty"`
}
