package domain

// GenerationPlan is the bounded, auditable decision produced by the planning
// model. The durable workflow remains responsible for which activities exist
// and in which order they execute.
type GenerationPlan struct {
	SchemaVersion       int                         `json:"schemaVersion"`
	Intent              string                      `json:"intent"`
	NarrativeMode       string                      `json:"narrativeMode"`
	SceneCount          int                         `json:"sceneCount"`
	ContinuityLevel     string                      `json:"continuityLevel"`
	VisualBibleStrategy string                      `json:"visualBibleStrategy"`
	CharacterStrategy   string                      `json:"characterStrategy"`
	ImageStrategy       GenerationPlanImageStrategy `json:"imageStrategy"`
	Steps               []GenerationPlanStep        `json:"steps"`
	AcceptanceChecks    []string                    `json:"acceptanceChecks"`
	ReasonSummary       string                      `json:"reasonSummary"`
	Fallback            bool                        `json:"fallback,omitempty"`
	FallbackReason      string                      `json:"fallbackReason,omitempty"`
}

type GenerationPlanImageStrategy struct {
	Generate           bool `json:"generate"`
	GenerateReferences bool `json:"generateReferences"`
	Parallelism        int  `json:"parallelism"`
}

type GenerationPlanStep struct {
	Activity string `json:"activity"`
	Required bool   `json:"required"`
}
