package http

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/grapestree/fgrapery/grapery-agent/internal/domain"
)

func (h *GenerationHandler) streamFragment(c *gin.Context) {
	var in domain.FragmentGenerateInput
	if err := c.ShouldBindJSON(&in); err != nil {
		h.fail(c, http.StatusBadRequest, err.Error())
		return
	}
	run, err := h.gen.StartFragment(c.Request.Context(), in)
	if err != nil {
		h.fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	h.streamRunSSE(c, run.ID)
}

func (h *GenerationHandler) streamFragmentPanel(c *gin.Context) {
	if err := h.requireFragmentPanelExec(c); err != nil {
		return
	}
	var in domain.FragmentPanelGenerateInput
	if err := c.ShouldBindJSON(&in); err != nil {
		h.fail(c, http.StatusBadRequest, err.Error())
		return
	}
	run, err := h.gen.StartFragmentPanel(c.Request.Context(), in)
	if err != nil {
		h.fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	h.streamRunSSE(c, run.ID)
}

func (h *GenerationHandler) cancelRun(c *gin.Context) {
	runID := c.Param("id")
	if err := h.gen.CancelRun(c.Request.Context(), runID); err != nil {
		h.fail(c, http.StatusBadRequest, err.Error())
		return
	}
	h.ok(c, gin.H{"runId": runID, "cancelled": true})
}

// streamRunSSE 轮询 run 状态并以 NDJSON/SSE 推送 progress/completed/failed 事件。
func (h *GenerationHandler) streamRunSSE(c *gin.Context, runID string) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	sequence := 0
	writeEvent := func(event string, payload gin.H) {
		sequence++
		enveloped := generationStreamEnvelope(event, sequence, payload)
		b, _ := json.Marshal(enveloped)
		c.SSEvent(event, string(b))
		c.Writer.Flush()
	}

	writeEvent("accepted", gin.H{"event": "accepted", "runId": runID})

	pushRunProgress := func() {
		run, err := h.gen.GetRun(c.Request.Context(), runID)
		if err != nil {
			writeEvent("failed", gin.H{"event": "failed", "runId": runID, "message": err.Error()})
			return
		}
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
			})
		case domain.RunStatusFailed, domain.RunStatusCancelled:
			writeEvent("failed", gin.H{"event": "failed", "runId": run.ID, "message": run.Error, "status": run.Status})
		}
	}

	pushRunProgress()
	if run, err := h.gen.GetRun(c.Request.Context(), runID); err == nil {
		switch run.Status {
		case domain.RunStatusSucceeded, domain.RunStatusFailed, domain.RunStatusCancelled:
			return
		}
	}

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	deadline := time.After(30 * time.Minute)
	lastStep := 0

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-deadline:
			writeEvent("failed", gin.H{"event": "failed", "runId": runID, "message": "stream timeout"})
			return
		case <-ticker.C:
			run, err := h.gen.GetRun(c.Request.Context(), runID)
			if err != nil {
				writeEvent("failed", gin.H{"event": "failed", "runId": runID, "message": err.Error()})
				return
			}
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
			})
			if len(run.StepAudits) > lastStep {
				for _, sa := range run.StepAudits[lastStep:] {
					writeEvent("step_started", gin.H{
						"event":         "step_started",
						"runId":         run.ID,
						"auditId":       sa.ID,
						"step":          sa.StepName,
						"currentStep":   sa.StepName,
						"messageKey":    generationStepMessageKey(sa.StepName),
						"attempt":       sa.Attempt,
						"status":        sa.Status,
						"auditRecordId": sa.ID,
					})
				}
				lastStep = len(run.StepAudits)
			}
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
				})
				return
			case domain.RunStatusFailed, domain.RunStatusCancelled:
				writeEvent("failed", gin.H{"event": "failed", "runId": run.ID, "message": run.Error, "status": run.Status})
				return
			}
		}
	}
}

func generationStreamEnvelope(event string, sequence int, payload gin.H) gin.H {
	eventType := "generation." + event
	if event == "step_started" {
		eventType = "generation.step"
	}
	out := gin.H{
		"protocolVersion": "2026-06-20",
		"eventType":       eventType,
		"phase":           "generation",
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

func generationStepMessageKey(step string) string {
	switch step {
	case "extract_elements", "extracting_elements":
		return "fragment_generation_analyzing_story"
	case "expand_scenes", "expanding_scenes":
		return "fragment_generation_writing_story"
	case "generating_reference_assets", "reference_assets":
		return "fragment_generation_designing_style"
	case "generating_images", "scene_images", "panel_render":
		return "fragment_generation_generating_images"
	case "checking_consistency":
		return "fragment_generation_checking_consistency"
	case "poll_task_status":
		return "fragment_generation_waiting_images"
	default:
		return ""
	}
}

func runStatusProgress(st domain.RunStatus) float64 {
	switch st {
	case domain.RunStatusPending:
		return 0.05
	case domain.RunStatusRunning:
		return 0.35
	case domain.RunStatusWaiting:
		return 0.55
	case domain.RunStatusSucceeded:
		return 1
	default:
		return 0
	}
}

func streamRunProgress(run *domain.GenerationRun) float64 {
	if run != nil && run.Output != nil {
		switch v := run.Output["progress"].(type) {
		case float64:
			if v > 1 {
				return v / 100
			}
			return v
		case int:
			if v > 1 {
				return float64(v) / 100
			}
			return float64(v)
		}
	}
	if run == nil {
		return 0
	}
	return runStatusProgress(run.Status)
}

func streamRunCurrentStep(run *domain.GenerationRun) string {
	if run != nil && run.Output != nil {
		if step, _ := run.Output["currentStep"].(string); step != "" {
			return step
		}
	}
	if run == nil {
		return ""
	}
	return latestStepName(run.StepAudits)
}

func streamRunMessageKey(run *domain.GenerationRun, step string) string {
	if run != nil && run.Output != nil {
		if key, _ := run.Output["messageKey"].(string); key != "" {
			return key
		}
	}
	return generationStepMessageKey(step)
}

func latestStepName(steps []domain.GenerationStepAudit) string {
	if len(steps) == 0 {
		return ""
	}
	return steps[len(steps)-1].StepName
}
