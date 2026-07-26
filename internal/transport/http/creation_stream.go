package http

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/grapestree/fgrapery/grapery-agent/internal/domain"
	"github.com/grapestree/fgrapery/grapery-agent/internal/grapery_client"
)

func (h *GenerationHandler) createCreationSession(c *gin.Context) {
	var req domain.CreationSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.fail(c, http.StatusBadRequest, err.Error())
		return
	}
	target := normalizeCreationTarget(req.TargetType, req.Context.TargetType)
	surface := firstNonEmpty(req.Surface, req.Context.Surface)
	if surface == "" {
		surface = target + "_create"
	}
	resp := domain.CreationSessionResponse{
		SessionID:  "cs_" + uuid.NewString(),
		Surface:    surface,
		TargetType: target,
		Context:    req.Context,
	}
	resp.Context.Surface = surface
	resp.Context.TargetType = target
	h.ok(c, resp)
}

func (h *GenerationHandler) streamCreationMessage(c *gin.Context) {
	sessionID := strings.TrimSpace(c.Param("id"))
	if sessionID == "" {
		h.fail(c, http.StatusBadRequest, "missing session id")
		return
	}
	var req domain.CreationMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.fail(c, http.StatusBadRequest, err.Error())
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	sequence := 0
	writeEvent := func(event string, payload gin.H) {
		sequence++
		payload["sessionId"] = sessionID
		enveloped := creationStreamEnvelope(event, sequence, payload)
		b, _ := json.Marshal(enveloped)
		c.SSEvent(event, string(b))
		c.Writer.Flush()
	}

	target := normalizeCreationTarget(req.Context.TargetType, "")
	if target == "" {
		target = "fragment"
	}
	writeEvent("accepted", gin.H{
		"event":      "accepted",
		"targetType": target,
	})

	switch target {
	case "fragment":
		if req.Options.PlanningOnly {
			h.streamCreationFragmentPlanning(c, req, writeEvent)
			return
		}
		h.streamCreationFragment(c, req, writeEvent)
	case "story":
		h.streamCreationStory(c, req, writeEvent)
	case "branch":
		h.streamCreationBranch(c, req, writeEvent)
	default:
		writeEvent("failed", gin.H{
			"event":      "failed",
			"targetType": target,
			"message":    "unsupported creation target",
			"status":     "unsupported",
		})
	}
}

func (h *GenerationHandler) streamCreationFragmentPlanning(c *gin.Context, req domain.CreationMessageRequest, writeEvent func(string, gin.H)) {
	if h.client == nil {
		writeEvent("failed", gin.H{"event": "failed", "message": "grapery client unavailable", "status": "failed"})
		return
	}
	resp, err := h.client.AnalyzeFragment(c.Request.Context(), grapery_client.AnalyzeFragmentRequest{
		UserInput:             req.Message,
		Language:              defaultCreationString(req.Options.Language, "zh-Hans"),
		AspectRatio:           req.Options.AspectRatio,
		ImageCount:            req.Options.ImageCount,
		Style:                 req.Options.Style,
		TargetDraftFragmentID: strings.TrimSpace(req.Context.DraftID),
	})
	if err != nil {
		writeEvent("failed", gin.H{"event": "failed", "message": err.Error(), "status": "failed"})
		return
	}
	output := gin.H{
		"assistantMessage":   resp.AssistantMessage,
		"intentType":         resp.IntentType,
		"generationIntent":   resp.GenerationIntent,
		"storyElements":      resp.StoryElements,
		"recommendedOptions": resp.RecommendedOptions,
		"targetType":         "fragment",
		"clientRequestId":    req.ClientRequestID,
	}
	writeEvent("planning", gin.H{
		"event":           "planning",
		"targetType":      "fragment",
		"message":         resp.AssistantMessage,
		"output":          output,
		"clientRequestId": req.ClientRequestID,
	})
	writeEvent("completed", gin.H{
		"event":           "completed",
		"targetType":      "fragment",
		"output":          output,
		"clientRequestId": req.ClientRequestID,
	})
}

func (h *GenerationHandler) streamCreationFragment(c *gin.Context, req domain.CreationMessageRequest, writeEvent func(string, gin.H)) {
	intent := creationFragmentIntent(req)
	writeEvent("intent", gin.H{
		"event":      "intent",
		"intent":     creationFragmentIntentName(intent),
		"targetType": "fragment",
		"imageCount": intent.ImageCount,
	})
	writeEvent("assistant_message", gin.H{
		"event":   "assistant_message",
		"message": creationFragmentAssistantMessage(intent),
	})

	run, err := h.gen.StartFragment(c.Request.Context(), intent)
	if err != nil {
		writeEvent("failed", gin.H{"event": "failed", "message": err.Error(), "status": "failed"})
		return
	}
	writeEvent("task_started", gin.H{
		"event":           "task_started",
		"runId":           run.ID,
		"taskId":          run.ContentIDs.TaskID,
		"draftFragmentId": run.ContentIDs.FragmentID,
		"fragmentId":      run.ContentIDs.FragmentID,
		"targetType":      "fragment",
		"clientRequestId": req.ClientRequestID,
	})
	h.streamRunSSEWithPrefix(c, run.ID, "creation", writeEvent)
}

func (h *GenerationHandler) streamCreationStory(c *gin.Context, req domain.CreationMessageRequest, writeEvent func(string, gin.H)) {
	in := domain.StoryGenerateInput{
		Prompt:  req.Message,
		Style:   req.Options.Style,
		Length:  req.Options.Length,
		Context: firstNonEmpty(req.Context.StoryID, req.Context.DraftID),
	}
	writeEvent("intent", gin.H{
		"event":      "intent",
		"intent":     "create",
		"targetType": "story",
	})
	writeEvent("assistant_message", gin.H{
		"event":   "assistant_message",
		"message": "好的，我会把你的想法整理成一个完整故事。",
	})
	run, err := h.gen.StartStory(c.Request.Context(), in)
	if err != nil {
		writeEvent("failed", gin.H{"event": "failed", "message": err.Error(), "status": "failed"})
		return
	}
	h.streamRunSSEWithPrefix(c, run.ID, "creation", writeEvent)
}

func (h *GenerationHandler) streamCreationBranch(c *gin.Context, req domain.CreationMessageRequest, writeEvent func(string, gin.H)) {
	parentStoryboardID := firstNonEmpty(req.Context.ParentStoryboardID, req.Context.BranchID)
	if parentStoryboardID == "" {
		writeEvent("failed", gin.H{
			"event":      "failed",
			"targetType": "branch",
			"message":    "branch creation requires parentStoryboardId in context",
			"status":     "needs_context",
		})
		return
	}
	in := domain.BranchExploreInput{
		ParentStoryboardID: parentStoryboardID,
		SeedPrompt:         req.Message,
		BranchCount:        defaultCreationInt(req.Options.BranchCount, 3),
		SceneCount:         defaultCreationInt(req.Options.SceneCount, 3),
		ComicStyle:         req.Options.Style,
	}
	writeEvent("intent", gin.H{
		"event":              "intent",
		"intent":             "branch_from",
		"targetType":         "branch",
		"parentStoryboardId": parentStoryboardID,
	})
	writeEvent("assistant_message", gin.H{
		"event":   "assistant_message",
		"message": "好的，我会从这个节点展开新的故事分支。",
	})
	run, err := h.gen.StartBranchBatch(c.Request.Context(), in)
	if err != nil {
		writeEvent("failed", gin.H{"event": "failed", "message": err.Error(), "status": "failed"})
		return
	}
	h.streamRunSSEWithPrefix(c, run.ID, "creation", writeEvent)
}

func creationFragmentIntent(req domain.CreationMessageRequest) domain.FragmentGenerateInput {
	opts := req.Options
	ctx := req.Context
	imageCount := opts.ImageCount
	if imageCount <= 0 {
		imageCount = opts.SceneCount
	}
	if imageCount <= 0 {
		if strings.TrimSpace(ctx.DraftID) != "" {
			imageCount = 1
		} else {
			imageCount = 4
		}
	}
	if imageCount > 8 {
		imageCount = 8
	}
	if ctx.SelectedImageIndex > 0 {
		imageCount = 1
	}
	return domain.FragmentGenerateInput{
		UserInput:              req.Message,
		ReferenceImages:        opts.ReferenceImages,
		ReferenceSlots:         opts.ReferenceSlots,
		ImageCount:             imageCount,
		Style:                  defaultCreationString(opts.Style, "fantasy"),
		Mood:                   defaultCreationString(opts.Mood, "mysterious"),
		Length:                 defaultCreationString(opts.Length, "medium"),
		Language:               defaultCreationString(opts.Language, "zh-Hans"),
		Visibility:             defaultCreationString(opts.Visibility, "private"),
		AspectRatio:            defaultCreationString(opts.AspectRatio, "9:16"),
		ConsistencyLevel:       opts.ConsistencyLevel,
		TargetDraftFragmentID:  strings.TrimSpace(ctx.DraftID),
		ReplaceImageIndex:      ctx.SelectedImageIndex,
		ClientMessageID:        strings.TrimSpace(req.ClientRequestID),
		EnableReferenceAssets:  opts.EnableReferenceAssets,
		IncludeGenerationTrace: opts.IncludeGenerationTrace,
		PollIntervalSec:        opts.PollIntervalSec,
		PollTimeoutSec:         opts.PollTimeoutSec,
	}
}

func (h *GenerationHandler) streamRunSSEWithPrefix(c *gin.Context, runID, phase string, writeEvent func(string, gin.H)) {
	writeRun := func(run *domain.GenerationRun, terminal bool) bool {
		progress := streamRunProgress(run)
		currentStep := streamRunCurrentStep(run)
		writeEvent("progress", gin.H{
			"event":           "progress",
			"runId":           run.ID,
			"taskId":          run.ContentIDs.TaskID,
			"fragmentId":      run.ContentIDs.FragmentID,
			"draftFragmentId": run.ContentIDs.FragmentID,
			"status":          run.Status,
			"progress":        progress,
			"currentStep":     currentStep,
			"messageKey":      streamRunMessageKey(run, currentStep),
			"message":         run.Error,
			"output":          run.Output,
			"phase":           phase,
		})
		switch run.Status {
		case domain.RunStatusSucceeded:
			writeEvent("completed", gin.H{
				"event":           "completed",
				"runId":           run.ID,
				"taskId":          run.ContentIDs.TaskID,
				"fragmentId":      run.ContentIDs.FragmentID,
				"draftFragmentId": run.ContentIDs.FragmentID,
				"output":          run.Output,
				"tokensUsed":      run.TokensUsed,
				"phase":           phase,
			})
			return true
		case domain.RunStatusFailed, domain.RunStatusCancelled:
			writeEvent("failed", gin.H{"event": "failed", "runId": run.ID, "message": run.Error, "status": run.Status, "phase": phase})
			return true
		default:
			return terminal
		}
	}

	if run, err := h.gen.GetRun(c.Request.Context(), runID); err == nil && writeRun(run, false) {
		return
	}

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	deadline := time.After(30 * time.Minute)

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-deadline:
			writeEvent("failed", gin.H{"event": "failed", "runId": runID, "message": "stream timeout", "status": "timeout"})
			return
		case <-ticker.C:
			run, err := h.gen.GetRun(c.Request.Context(), runID)
			if err != nil {
				writeEvent("failed", gin.H{"event": "failed", "runId": runID, "message": err.Error(), "status": "failed"})
				return
			}
			if writeRun(run, false) {
				return
			}
		}
	}
}

func creationStreamEnvelope(event string, sequence int, payload gin.H) gin.H {
	eventType := "creation." + event
	out := gin.H{
		"protocolVersion": "2026-07-02",
		"eventType":       eventType,
		"phase":           "creation",
		"sequence":        sequence,
		"createdAt":       time.Now().UnixMilli(),
		"payload":         payload,
	}
	for k, v := range payload {
		out[k] = v
	}
	if step, _ := payload["currentStep"].(string); step != "" {
		out["messageKey"] = generationStepMessageKey(step)
		if _, ok := payload["messageKey"]; !ok {
			payload["messageKey"] = out["messageKey"]
		}
	}
	return out
}

func creationFragmentIntentName(in domain.FragmentGenerateInput) string {
	if in.ReplaceImageIndex > 0 {
		return "replace_image"
	}
	if strings.TrimSpace(in.TargetDraftFragmentID) != "" {
		return "append_image"
	}
	return "create"
}

func creationFragmentAssistantMessage(in domain.FragmentGenerateInput) string {
	switch creationFragmentIntentName(in) {
	case "replace_image":
		return "好，我会重绘这一张并替换到当前草稿里。"
	case "append_image":
		return "好嘞，这就继续补充新的故事画面。"
	default:
		return "好的，我正在看你的故事并准备生成画面。"
	}
}

func normalizeCreationTarget(values ...string) string {
	for _, value := range values {
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "fragment", "story_fragment", "story-fragment":
			return "fragment"
		case "story":
			return "story"
		case "branch", "story_branch", "story-branch":
			return "branch"
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func defaultCreationString(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}

func defaultCreationInt(v, fallback int) int {
	if v <= 0 {
		return fallback
	}
	return v
}
