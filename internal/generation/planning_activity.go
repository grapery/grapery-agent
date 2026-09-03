package generation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/cloudwego/eino/schema"
	"github.com/grapestree/fgrapery/grapery-agent/internal/domain"
	"github.com/grapestree/fgrapery/grapery-agent/internal/runstore"
	workflowruntime "github.com/grapestree/fgrapery/grapery-agent/internal/workflow"
)

const generationPlannerSystemPrompt = `You are Grapery's bounded storyboard planning agent.
Return exactly one JSON object and no prose. Never choose a workflow release, model provider, or unregistered tool.
Use this schema:
{
  "schemaVersion": 1,
  "intent": "create|continue|fork|revise",
  "narrativeMode": "general|action|dialogue",
  "sceneCount": 1,
  "continuityLevel": "standard|strong",
  "visualBibleStrategy": "inherit|inherit_and_patch|regenerate",
  "characterStrategy": "reuse_existing|review_existing|create_missing",
  "imageStrategy": {"generate": true, "generateReferences": false, "parallelism": 1},
  "steps": [{"activity": "storyboard.ensure_draft", "required": true}],
  "acceptanceChecks": ["parent_ending_continuity"],
  "reasonSummary": "short auditable explanation"
}
sceneCount must be 1-8 and image parallelism 1-4. The only allowed steps are storyboard.ensure_draft, storyboard.generate_bible_plan, storyboard.generate_scene_plan, storyboard.review_content, storyboard.persist_content, and storyboard.ensure_images. The first five are mandatory and ordered; storyboard.ensure_images is included only when imageStrategy.generate is true. Use only these acceptance checks: parent_ending_continuity, character_identity, world_rules, narrative_coherence, visual_consistency, image_completeness. Respect explicit user choices and use strong continuity for forks.`

var generationPlanStepOrder = []string{
	"storyboard.ensure_draft",
	"storyboard.generate_bible_plan",
	"storyboard.generate_scene_plan",
	"storyboard.review_content",
	"storyboard.persist_content",
	"storyboard.ensure_images",
}

var generationPlanChecks = map[string]bool{
	"parent_ending_continuity": true,
	"character_identity":       true,
	"world_rules":              true,
	"narrative_coherence":      true,
	"visual_consistency":       true,
	"image_completeness":       true,
}

func (s *Service) executeAIPlannerActivity(ctx context.Context, input map[string]any, config map[string]any) (map[string]any, error) {
	started := time.Now()
	systemPrompt, userPrompt, outputSchema, err := generationPlannerPrompts(input, config)
	if err != nil {
		return nil, err
	}

	plan, tokens, planErr := s.generateGenerationPlan(ctx, systemPrompt, userPrompt, outputSchema)
	if planErr != nil {
		plan = defaultGenerationPlan(input, planErr.Error())
	}
	output := generationPlanOutput(plan, tokens)
	s.recordGenerationPlanAudit(ctx, output, planErr, started)
	return output, nil
}

func (s *Service) generateGenerationPlan(ctx context.Context, systemPrompt, userPrompt string, outputSchema map[string]any) (domain.GenerationPlan, int, error) {
	if s.workflowPlanner == nil {
		return domain.GenerationPlan{}, 0, errors.New("workflow planner model is unavailable")
	}
	var lastErr error
	var invalidOutput string
	tokens := 0
	for attempt := 1; attempt <= 2; attempt++ {
		prompt := userPrompt
		if attempt == 2 {
			prompt = fmt.Sprintf("Repair the previous response. Return only a valid JSON object. Validation error: %s\nPrevious response: %s", lastErr, truncatePlannerText(invalidOutput, 6000))
		}
		message, err := s.workflowPlanner.Generate(ctx, []*schema.Message{schema.SystemMessage(systemPrompt), schema.UserMessage(prompt)})
		if err != nil {
			lastErr = err
			continue
		}
		if message != nil && message.ResponseMeta != nil && message.ResponseMeta.Usage != nil {
			tokens += message.ResponseMeta.Usage.TotalTokens
		}
		if message == nil {
			lastErr = errors.New("planner returned an empty response")
			continue
		}
		invalidOutput = message.Content
		plan, err := decodeGenerationPlan(message.Content)
		if err == nil {
			err = validateGenerationPlan(plan)
		}
		if err == nil && len(outputSchema) > 0 {
			err = workflowruntime.ValidateJSONSchema("generation plan", outputSchema, plan)
		}
		if err == nil {
			return plan, tokens, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = errors.New("planner failed without an error")
	}
	return domain.GenerationPlan{}, tokens, lastErr
}

func generationPlannerPrompts(input, config map[string]any) (string, string, map[string]any, error) {
	payload, err := json.Marshal(input)
	if err != nil {
		return "", "", nil, fmt.Errorf("marshal planning input: %w", err)
	}
	systemPrompt := generationPlannerSystemPrompt
	userPrompt := "Plan this storyboard generation from the following runtime input:\n" + truncatePlannerText(string(payload), 24000)
	raw, ok := config["promptTemplate"]
	if !ok {
		return systemPrompt, userPrompt, nil, nil
	}
	promptPayload, err := json.Marshal(raw)
	if err != nil {
		return "", "", nil, fmt.Errorf("marshal planner prompt template: %w", err)
	}
	var prompt domain.PromptTemplateVersion
	if err := json.Unmarshal(promptPayload, &prompt); err != nil {
		return "", "", nil, fmt.Errorf("decode planner prompt template: %w", err)
	}
	variables := cloneWorkflowInput(input)
	variables["inputJSON"] = string(payload)
	if rendered, err := renderWorkflowPromptPart(prompt.SystemTemplate, variables); err != nil {
		return "", "", nil, fmt.Errorf("render planner system prompt %s: %w", prompt.ID, err)
	} else if strings.TrimSpace(rendered) != "" {
		systemPrompt = rendered
	}
	if rendered, err := renderWorkflowPromptPart(prompt.UserTemplate, variables); err != nil {
		return "", "", nil, fmt.Errorf("render planner user prompt %s: %w", prompt.ID, err)
	} else if strings.TrimSpace(rendered) != "" {
		userPrompt = rendered
	}
	return systemPrompt, userPrompt, prompt.OutputSchema, nil
}

func decodeGenerationPlan(raw string) (domain.GenerationPlan, error) {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "```") {
		raw = strings.TrimPrefix(raw, "```json")
		raw = strings.TrimPrefix(raw, "```JSON")
		raw = strings.TrimPrefix(raw, "```")
		raw = strings.TrimSuffix(strings.TrimSpace(raw), "```")
	}
	start, end := strings.Index(raw, "{"), strings.LastIndex(raw, "}")
	if start < 0 || end < start {
		return domain.GenerationPlan{}, errors.New("planner response does not contain a JSON object")
	}
	var plan domain.GenerationPlan
	if err := json.Unmarshal([]byte(raw[start:end+1]), &plan); err != nil {
		return plan, fmt.Errorf("decode planner response: %w", err)
	}
	return plan, nil
}

func validateGenerationPlan(plan domain.GenerationPlan) error {
	if plan.SchemaVersion != 1 {
		return fmt.Errorf("unsupported generation plan schema version %d", plan.SchemaVersion)
	}
	if !oneOf(plan.Intent, "create", "continue", "fork", "revise") {
		return fmt.Errorf("invalid generation intent %q", plan.Intent)
	}
	if !oneOf(plan.NarrativeMode, "general", "action", "dialogue") {
		return fmt.Errorf("invalid narrative mode %q", plan.NarrativeMode)
	}
	if plan.SceneCount < 1 || plan.SceneCount > 8 {
		return fmt.Errorf("scene count must be between 1 and 8")
	}
	if !oneOf(plan.ContinuityLevel, "standard", "strong") {
		return fmt.Errorf("invalid continuity level %q", plan.ContinuityLevel)
	}
	if !oneOf(plan.VisualBibleStrategy, "inherit", "inherit_and_patch", "regenerate") {
		return fmt.Errorf("invalid visual bible strategy %q", plan.VisualBibleStrategy)
	}
	if !oneOf(plan.CharacterStrategy, "reuse_existing", "review_existing", "create_missing") {
		return fmt.Errorf("invalid character strategy %q", plan.CharacterStrategy)
	}
	if plan.ImageStrategy.Parallelism < 1 || plan.ImageStrategy.Parallelism > 4 {
		return fmt.Errorf("image parallelism must be between 1 and 4")
	}
	expected := generationPlanStepOrder[:5]
	if plan.ImageStrategy.Generate {
		expected = generationPlanStepOrder
	}
	if len(plan.Steps) != len(expected) {
		return errors.New("generation plan steps do not match the bounded workflow")
	}
	for index, step := range plan.Steps {
		if step.Activity != expected[index] || (index < 5 && !step.Required) {
			return fmt.Errorf("invalid generation plan step %d", index+1)
		}
	}
	for _, check := range plan.AcceptanceChecks {
		if !generationPlanChecks[check] {
			return fmt.Errorf("unsupported acceptance check %q", check)
		}
	}
	if utf8.RuneCountInString(plan.ReasonSummary) > 500 {
		return errors.New("generation plan reason summary is too long")
	}
	return nil
}

func defaultGenerationPlan(input map[string]any, reason string) domain.GenerationPlan {
	intent := "create"
	if stringFromAny(input["parentStoryboardId"], input["parentFragmentId"], input["parentId"]) != "" {
		intent = "fork"
	} else if regenerate, _ := input["regenerateStructure"].(bool); regenerate {
		intent = "revise"
	} else if stringFromAny(input["draftStoryboardId"], input["storyboardId"], input["targetDraftFragmentId"]) != "" {
		intent = "continue"
	}
	sceneCount := intFromAny(input["sceneCount"])
	if sceneCount < 1 || sceneCount > 8 {
		sceneCount = 3
	}
	generateImages := true
	if value, ok := input["generateImages"].(bool); ok {
		generateImages = value
	}
	continuity := "standard"
	visualBible := "regenerate"
	checks := []string{"narrative_coherence", "character_identity"}
	if intent == "fork" || intent == "continue" || intent == "revise" {
		continuity = "strong"
		visualBible = "inherit_and_patch"
		checks = append([]string{"parent_ending_continuity"}, checks...)
	}
	steps := make([]domain.GenerationPlanStep, 0, len(generationPlanStepOrder))
	for index, activity := range generationPlanStepOrder {
		if index == len(generationPlanStepOrder)-1 && !generateImages {
			break
		}
		steps = append(steps, domain.GenerationPlanStep{Activity: activity, Required: index < 5 || generateImages})
	}
	return domain.GenerationPlan{
		SchemaVersion: 1, Intent: intent, NarrativeMode: "general", SceneCount: sceneCount,
		ContinuityLevel: continuity, VisualBibleStrategy: visualBible, CharacterStrategy: "reuse_existing",
		ImageStrategy: domain.GenerationPlanImageStrategy{Generate: generateImages, GenerateReferences: false, Parallelism: 1},
		Steps:         steps, AcceptanceChecks: checks, ReasonSummary: "Standard deterministic plan used because dynamic planning was unavailable.",
		Fallback: true, FallbackReason: truncatePlannerText(reason, 500),
	}
}

func generationPlanOutput(plan domain.GenerationPlan, tokens int) map[string]any {
	payload, _ := json.Marshal(plan)
	var value map[string]any
	_ = json.Unmarshal(payload, &value)
	return map[string]any{
		"generationPlan": value,
		"workflowInputPatch": map[string]any{
			"sceneCount":          plan.SceneCount,
			"generateImages":      plan.ImageStrategy.Generate,
			"continuityLevel":     plan.ContinuityLevel,
			"visualBibleStrategy": plan.VisualBibleStrategy,
			"characterStrategy":   plan.CharacterStrategy,
			"acceptanceChecks":    append([]string(nil), plan.AcceptanceChecks...),
		},
		"tokensUsed": tokens,
		"fallback":   plan.Fallback,
	}
}

func (s *Service) recordGenerationPlanAudit(ctx context.Context, output map[string]any, planErr error, started time.Time) {
	runID, ok := runstore.RunIDFromContext(ctx)
	if !ok || s.store == nil {
		return
	}
	execution, _ := workflowruntime.ActivityExecutionFromContext(ctx)
	metadata := map[string]any{"fallback": output["fallback"]}
	if planErr != nil {
		metadata["fallbackReason"] = truncatePlannerText(planErr.Error(), 500)
	}
	_ = s.store.RecordStepAudit(ctx, runID, domain.GenerationStepAudit{
		StepName: "generation_plan", Attempt: execution.Attempt, Status: domain.StepSucceeded,
		Provider: s.provider, Model: s.model, ParsedOutput: output, TotalTokens: intFromAny(output["tokensUsed"]),
		DurationMs: time.Since(started).Milliseconds(), StartedAt: started, EndedAt: time.Now(), Metadata: metadata,
	})
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func truncatePlannerText(value string, maxRunes int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= maxRunes {
		return string(runes)
	}
	return string(runes[:maxRunes])
}
