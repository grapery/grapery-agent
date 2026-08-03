package domain

import "time"

type WorkflowNode struct {
	ID        string         `json:"id"`
	Type      string         `json:"type"`
	Activity  string         `json:"activity,omitempty"`
	DependsOn []string       `json:"dependsOn,omitempty"`
	Config    map[string]any `json:"config,omitempty"`
}

type WorkflowDefinition struct {
	InputSchema  map[string]any `json:"inputSchema,omitempty"`
	OutputSchema map[string]any `json:"outputSchema,omitempty"`
	Nodes        []WorkflowNode `json:"nodes"`
}

type WorkflowPolicies struct {
	MaxDurationSeconds int `json:"maxDurationSeconds,omitempty"`
	MaxParallelism     int `json:"maxParallelism,omitempty"`
	MaxAttempts        int `json:"maxAttempts,omitempty"`
}

type WorkflowRelease struct {
	ID           string             `json:"id"`
	Key          string             `json:"key"`
	Version      int                `json:"version"`
	Name         string             `json:"name"`
	Description  string             `json:"description,omitempty"`
	Status       string             `json:"status"`
	Manifest     map[string]any     `json:"manifest,omitempty"`
	Definition   WorkflowDefinition `json:"definition"`
	PromptBundle map[string]string  `json:"promptBundle,omitempty"`
	Policies     WorkflowPolicies   `json:"policies,omitempty"`
	Checksum     string             `json:"checksum"`
	CreatedBy    string             `json:"createdBy,omitempty"`
	ApprovedBy   []string           `json:"approvedBy,omitempty"`
	PublishedAt  time.Time          `json:"publishedAt"`
	CreatedAt    time.Time          `json:"createdAt"`
}

type PromptTemplateVersion struct {
	ID              string         `json:"id"`
	Key             string         `json:"key"`
	Version         int            `json:"version"`
	Type            string         `json:"type"`
	SystemTemplate  string         `json:"systemTemplate,omitempty"`
	UserTemplate    string         `json:"userTemplate,omitempty"`
	VariablesSchema map[string]any `json:"variablesSchema,omitempty"`
	OutputSchema    map[string]any `json:"outputSchema,omitempty"`
	ModelConfig     map[string]any `json:"modelConfig,omitempty"`
	Checksum        string         `json:"checksum"`
	CreatedBy       string         `json:"createdBy,omitempty"`
	CreatedAt       time.Time      `json:"createdAt"`
}

type WorkflowBinding struct {
	ID        string `json:"id"`
	Surface   string `json:"surface"`
	Action    string `json:"action"`
	TenantID  string `json:"tenantId,omitempty"`
	ReleaseID string `json:"releaseId"`
	Enabled   bool   `json:"enabled"`
}

type WorkflowCatalogEntry struct {
	Binding WorkflowBinding `json:"binding"`
	Release WorkflowRelease `json:"release"`
}
