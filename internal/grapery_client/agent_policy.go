package grapery_client

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/grapestree/fgrapery/grapery-agent/internal/domain"
)

// SaveGenerationExecution persists one complete run snapshot. Grapery assigns
// the monotonic sequence and publishes the corresponding Redis Stream event.
func (c *Client) SaveGenerationExecution(ctx context.Context, run *domain.GenerationRun, eventType, leaseValue string) (*domain.GenerationRun, error) {
	path := "/api/v1/agent-policy/generation-executions/" + url.PathEscape(run.ID)
	if eventType != "" {
		path += "?eventType=" + url.QueryEscape(eventType)
	}
	var out struct {
		Code    int                  `json:"code"`
		Data    domain.GenerationRun `json:"data"`
		Message string               `json:"message"`
	}
	if err := c.saveGenerationExecutionJSON(ctx, path, run, leaseValue, &out); err != nil {
		return nil, err
	}
	if out.Code != 1 {
		return nil, fmt.Errorf("save generation execution: %s", out.Message)
	}
	return &out.Data, nil
}

func (c *Client) saveGenerationExecutionJSON(ctx context.Context, path string, body interface{}, leaseValue string, dest interface{}) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.baseURL+path, strings.NewReader(string(b)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Api-Key", c.authToken)
	if leaseValue = strings.TrimSpace(leaseValue); leaseValue != "" {
		req.Header.Set("X-Generation-Lease", leaseValue)
	}
	return c.doJSON(req, dest)
}

func (c *Client) GetGenerationExecution(ctx context.Context, id string) (*domain.GenerationRun, error) {
	path := "/api/v1/agent-policy/generation-executions/" + url.PathEscape(id)
	var out struct {
		Code    int                  `json:"code"`
		Data    domain.GenerationRun `json:"data"`
		Message string               `json:"message"`
	}
	if err := c.getInternalJSON(ctx, path, &out); err != nil {
		return nil, err
	}
	if out.Code != 1 {
		return nil, fmt.Errorf("get generation execution: %s", out.Message)
	}
	return &out.Data, nil
}

func (c *Client) ListGenerationExecutions(ctx context.Context, kind domain.RunKind, limit int) ([]*domain.GenerationRun, error) {
	path := fmt.Sprintf("/api/v1/agent-policy/generation-executions?kind=%s&limit=%d", url.QueryEscape(string(kind)), limit)
	var out struct {
		Code int `json:"code"`
		Data struct {
			Runs []*domain.GenerationRun `json:"runs"`
		} `json:"data"`
		Message string `json:"message"`
	}
	if err := c.getInternalJSON(ctx, path, &out); err != nil {
		return nil, err
	}
	if out.Code != 1 {
		return nil, fmt.Errorf("list generation executions: %s", out.Message)
	}
	return out.Data.Runs, nil
}

func (c *Client) FindGenerationExecutionByRequest(ctx context.Context, kind domain.RunKind, userID, clientRequestID string) (*domain.GenerationRun, error) {
	path := fmt.Sprintf("/api/v1/agent-policy/generation-executions?kind=%s&userId=%s&clientRequestId=%s",
		url.QueryEscape(string(kind)), url.QueryEscape(strings.TrimSpace(userID)), url.QueryEscape(strings.TrimSpace(clientRequestID)))
	var out struct {
		Code int `json:"code"`
		Data struct {
			Runs []*domain.GenerationRun `json:"runs"`
		} `json:"data"`
		Message string `json:"message"`
	}
	if err := c.getInternalJSON(ctx, path, &out); err != nil {
		return nil, err
	}
	if out.Code != 1 || len(out.Data.Runs) == 0 {
		return nil, fmt.Errorf("find generation execution: %s", out.Message)
	}
	return out.Data.Runs[0], nil
}

func (c *Client) SaveGenerationCheckpoint(ctx context.Context, id string, state []byte) error {
	return c.SaveFencedGenerationCheckpoint(ctx, id, "", "", state)
}

func (c *Client) SaveFencedGenerationCheckpoint(ctx context.Context, id, runID, leaseValue string, state []byte) error {
	path := "/api/v1/agent-policy/generation-checkpoints/" + url.PathEscape(id)
	var out struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	body := map[string]string{"state": base64.StdEncoding.EncodeToString(state)}
	if runID = strings.TrimSpace(runID); runID != "" {
		body["runId"] = runID
	}
	if err := c.saveGenerationExecutionJSON(ctx, path, body, leaseValue, &out); err != nil {
		return err
	}
	if out.Code != 1 {
		return fmt.Errorf("save generation checkpoint: %s", out.Message)
	}
	return nil
}

func (c *Client) GetGenerationCheckpoint(ctx context.Context, id string) ([]byte, bool, error) {
	path := "/api/v1/agent-policy/generation-checkpoints/" + url.PathEscape(id)
	var out struct {
		Code int `json:"code"`
		Data struct {
			State string `json:"state"`
		} `json:"data"`
		Message string `json:"message"`
	}
	if err := c.getInternalJSON(ctx, path, &out); err != nil {
		if strings.Contains(err.Error(), "status 404") {
			return nil, false, nil
		}
		return nil, false, err
	}
	if out.Code != 1 {
		return nil, false, nil
	}
	state, err := base64.StdEncoding.DecodeString(out.Data.State)
	if err != nil {
		return nil, false, fmt.Errorf("decode generation checkpoint: %w", err)
	}
	return state, true, nil
}

type GenerationLease struct {
	RunID string `json:"runId"`
	Owner string `json:"owner"`
	Token int64  `json:"token"`
	Value string `json:"value"`
}

func (c *Client) AcquireGenerationLease(ctx context.Context, id, owner string, ttlSeconds int) (*GenerationLease, bool, error) {
	path := "/api/v1/agent-policy/generation-executions/" + url.PathEscape(id) + "/lease"
	var out struct {
		Code int `json:"code"`
		Data struct {
			Acquired bool             `json:"acquired"`
			Lease    *GenerationLease `json:"lease"`
		} `json:"data"`
		Message string `json:"message"`
	}
	if err := c.postInternalJSON(ctx, path, map[string]any{"owner": owner, "ttlSeconds": ttlSeconds}, &out); err != nil {
		return nil, false, err
	}
	if out.Code != 1 {
		return nil, false, fmt.Errorf("acquire generation lease: %s", out.Message)
	}
	return out.Data.Lease, out.Data.Acquired, nil
}

func (c *Client) RenewGenerationLease(ctx context.Context, id, value string, ttlSeconds int) (bool, error) {
	path := "/api/v1/agent-policy/generation-executions/" + url.PathEscape(id) + "/lease"
	var out struct {
		Code int `json:"code"`
		Data struct {
			Renewed bool `json:"renewed"`
		} `json:"data"`
		Message string `json:"message"`
	}
	if err := c.putInternalJSON(ctx, path, map[string]any{"value": value, "ttlSeconds": ttlSeconds}, &out); err != nil {
		return false, err
	}
	if out.Code != 1 {
		return false, fmt.Errorf("renew generation lease: %s", out.Message)
	}
	return out.Data.Renewed, nil
}

func (c *Client) ReleaseGenerationLease(ctx context.Context, id, value string) error {
	path := "/api/v1/agent-policy/generation-executions/" + url.PathEscape(id) + "/lease"
	var out struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := c.deleteInternalJSON(ctx, path, map[string]string{"value": value}, &out); err != nil {
		return err
	}
	if out.Code != 1 {
		return fmt.Errorf("release generation lease: %s", out.Message)
	}
	return nil
}

// AgentTokenJTIStatus 查询 grapery agent-policy API 的 jti 状态。
func (c *Client) AgentTokenJTIStatus(ctx context.Context, jti string) (string, error) {
	var out struct {
		Code int `json:"code"`
		Data struct {
			Status string `json:"status"`
		} `json:"data"`
		Message string `json:"message"`
	}
	if err := c.getJSON(ctx, "/api/v1/agent-policy/tokens/"+jti+"/status", &out); err != nil {
		return "", err
	}
	if out.Code != 1 {
		return "", fmt.Errorf("policy api: %s", out.Message)
	}
	return out.Data.Status, nil
}

// ConsumeAgentTokenJTI 一次性消费 jti（replay 保护）。
func (c *Client) ConsumeAgentTokenJTI(ctx context.Context, jti string) error {
	var out struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := c.postJSON(ctx, "/api/v1/agent-policy/tokens/"+jti+"/consume", nil, &out); err != nil {
		return err
	}
	if out.Code != 1 {
		return fmt.Errorf("consume jti: %s", out.Message)
	}
	return nil
}

// UserQuotaSnapshot 读取用户 token 余额快照。
func (c *Client) UserQuotaSnapshot(ctx context.Context, userID string) (int, error) {
	var out struct {
		Code int `json:"code"`
		Data struct {
			TokenBalance int `json:"tokenBalance"`
		} `json:"data"`
		Message string `json:"message"`
	}
	path := "/api/v1/agent-policy/users/" + strings.TrimSpace(userID) + "/quota"
	if err := c.getJSON(ctx, path, &out); err != nil {
		return 0, err
	}
	if out.Code != 1 {
		return 0, fmt.Errorf("quota snapshot: %s", out.Message)
	}
	return out.Data.TokenBalance, nil
}

type policyPanelGenerateRequest struct {
	UserID                 string `json:"userId"`
	UserInput              string `json:"userInput"`
	ReferenceImageURL      string `json:"referenceImageUrl"`
	Style                  string `json:"style,omitempty"`
	Language               string `json:"language,omitempty"`
	PanelCount             int    `json:"panelCount,omitempty"`
	Visibility             string `json:"visibility,omitempty"`
	Topic                  string `json:"topic,omitempty"`
	AspectRatio            string `json:"aspectRatio,omitempty"`
	DialogueMode           string `json:"dialogueMode,omitempty"`
	ConsistencyLevel       string `json:"consistencyLevel,omitempty"`
	EnableReferenceAssets  *bool  `json:"enableReferenceAssets,omitempty"`
	IncludeGenerationTrace bool   `json:"includeGenerationTrace,omitempty"`
}

// GenerateFragmentPanelForUser 经 agent-policy 内部 API 启动 fragment-panel 生成（无需用户 JWT）。
func (c *Client) GenerateFragmentPanelForUser(ctx context.Context, userID string, req GenerateFragmentPanelRequest) (*GenerateFragmentPanelResponse, error) {
	body := policyPanelGenerateRequest{
		UserID:                 userID,
		UserInput:              req.UserInput,
		ReferenceImageURL:      req.ReferenceImageURL,
		Style:                  req.Style,
		Language:               req.Language,
		PanelCount:             req.PanelCount,
		Visibility:             req.Visibility,
		Topic:                  req.Topic,
		AspectRatio:            req.AspectRatio,
		DialogueMode:           req.DialogueMode,
		ConsistencyLevel:       req.ConsistencyLevel,
		EnableReferenceAssets:  req.EnableReferenceAssets,
		IncludeGenerationTrace: req.IncludeGenerationTrace,
	}
	var out struct {
		Code    int                           `json:"code"`
		Data    GenerateFragmentPanelResponse `json:"data"`
		Message string                        `json:"message"`
	}
	if err := c.postJSON(ctx, "/api/v1/agent-policy/fragment-panels/generate", body, &out); err != nil {
		return nil, err
	}
	if out.Code != 1 {
		return nil, fmt.Errorf("policy panel generate: %s", out.Message)
	}
	return &out.Data, nil
}

// GetFragmentPanelTaskForUser 经 agent-policy 查询 fragment-panel 任务状态。
func (c *Client) GetFragmentPanelTaskForUser(ctx context.Context, userID, taskID string) (*FragmentPanelTaskStatus, error) {
	path := "/api/v1/agent-policy/fragment-panels/generate/" + url.PathEscape(taskID) + "?userId=" + url.QueryEscape(strings.TrimSpace(userID))
	var out struct {
		Code int `json:"code"`
		Data struct {
			TaskID          string                `json:"taskId"`
			Status          string                `json:"status"`
			Progress        float64               `json:"progress"`
			CurrentStep     string                `json:"currentStep"`
			DraftFragmentID string                `json:"draftFragmentId"`
			Error           string                `json:"error"`
			CombinedContent string                `json:"combinedContent"`
			Panels          []FragmentPanelResult `json:"panels"`
			Metrics         struct {
				TotalTokens int `json:"totalTokens"`
			} `json:"metrics"`
		} `json:"data"`
		Message string `json:"message"`
	}
	if err := c.getJSON(ctx, path, &out); err != nil {
		return nil, err
	}
	if out.Code != 1 {
		return nil, fmt.Errorf("policy panel status: %s", out.Message)
	}
	st := &FragmentPanelTaskStatus{
		TaskID:          out.Data.TaskID,
		Status:          out.Data.Status,
		Progress:        out.Data.Progress,
		CurrentStep:     out.Data.CurrentStep,
		DraftFragmentID: out.Data.DraftFragmentID,
		Error:           out.Data.Error,
		CombinedContent: out.Data.CombinedContent,
		Panels:          out.Data.Panels,
		TokensUsed:      out.Data.Metrics.TotalTokens,
	}
	return st, nil
}

// RecordGenerationAudits 将 agent 步骤审计批量写入 grapery。
func (c *Client) RecordGenerationAudits(ctx context.Context, audits []domain.GenerationStepAudit) error {
	if len(audits) == 0 {
		return nil
	}
	records := make([]map[string]any, 0, len(audits))
	for _, a := range audits {
		records = append(records, generationStepAuditToRecord(a))
	}
	var out struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	body := map[string]any{"records": records}
	if err := c.postJSON(ctx, "/api/v1/agent-policy/generation-audits", body, &out); err != nil {
		return err
	}
	if out.Code != 1 {
		return fmt.Errorf("record audits: %s", out.Message)
	}
	return nil
}

func generationStepAuditToRecord(a domain.GenerationStepAudit) map[string]any {
	rec := map[string]any{
		"id":           a.ID,
		"sequence":     a.Sequence,
		"runId":        a.RunID,
		"taskId":       a.TaskID,
		"businessType": string(a.BusinessType),
		"businessId":   a.BusinessID,
		"agentVersion": string(a.AgentVersion),
		"stepName":     a.StepName,
		"attempt":      a.Attempt,
		"status":       string(a.Status),
		"provider":     a.Provider,
		"model":        a.Model,
		"prompt":       a.Prompt,
		"rawOutput":    a.RawOutput,
		"errorCode":    a.ErrorCode,
		"errorMessage": a.ErrorMessage,
		"inputTokens":  a.InputTokens,
		"outputTokens": a.OutputTokens,
		"totalTokens":  a.TotalTokens,
		"durationMs":   a.DurationMs,
	}
	if len(a.InputRefs) > 0 {
		rec["inputRefs"] = a.InputRefs
	}
	if len(a.ParsedOutput) > 0 {
		rec["parsedOutput"] = a.ParsedOutput
	}
	if len(a.Metadata) > 0 {
		rec["metadata"] = a.Metadata
	}
	if a.UserID != "" {
		rec["userId"] = a.UserID
	}
	if !a.StartedAt.IsZero() {
		rec["startedAt"] = a.StartedAt.UnixMilli()
	}
	if !a.EndedAt.IsZero() {
		rec["endedAt"] = a.EndedAt.UnixMilli()
	}
	return rec
}

// ConfirmQuotaReservation 确认 agent 生成成功后的配额扣减。
func (c *Client) ConfirmQuotaReservation(ctx context.Context, reservationID string, actualTokens int) error {
	path := "/api/v1/agent-policy/quota/reservations/" + url.PathEscape(strings.TrimSpace(reservationID)) + "/confirm"
	var out struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	body := map[string]any{"actualTokens": actualTokens}
	if err := c.postJSON(ctx, path, body, &out); err != nil {
		return err
	}
	if out.Code != 1 {
		return fmt.Errorf("confirm quota: %s", out.Message)
	}
	return nil
}

// ReleaseQuotaReservation 释放 agent 配额预留。
func (c *Client) ReleaseQuotaReservation(ctx context.Context, reservationID string) error {
	path := "/api/v1/agent-policy/quota/reservations/" + url.PathEscape(strings.TrimSpace(reservationID)) + "/release"
	var out struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := c.postJSON(ctx, path, nil, &out); err != nil {
		return err
	}
	if out.Code != 1 {
		return fmt.Errorf("release quota: %s", out.Message)
	}
	return nil
}

func (c *Client) getJSON(ctx context.Context, path string, dest interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	return c.doJSON(req, dest)
}

func (c *Client) postJSON(ctx context.Context, path string, body interface{}, dest interface{}) error {
	var bodyReader *strings.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		bodyReader = strings.NewReader(string(b))
	} else {
		bodyReader = strings.NewReader("{}")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bodyReader)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return c.doJSON(req, dest)
}

func (c *Client) getInternalJSON(ctx context.Context, path string, dest interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Internal-Api-Key", c.authToken)
	return c.doJSON(req, dest)
}

func (c *Client) putInternalJSON(ctx context.Context, path string, body interface{}, dest interface{}) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.baseURL+path, strings.NewReader(string(b)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Api-Key", c.authToken)
	return c.doJSON(req, dest)
}

func (c *Client) postInternalJSON(ctx context.Context, path string, body interface{}, dest interface{}) error {
	return c.internalJSON(ctx, http.MethodPost, path, body, dest)
}

func (c *Client) deleteInternalJSON(ctx context.Context, path string, body interface{}, dest interface{}) error {
	return c.internalJSON(ctx, http.MethodDelete, path, body, dest)
}

func (c *Client) internalJSON(ctx context.Context, method, path string, body interface{}, dest interface{}) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, strings.NewReader(string(b)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Api-Key", c.authToken)
	return c.doJSON(req, dest)
}

func (c *Client) doJSON(req *http.Request, dest interface{}) error {
	if key := c.tokenFromContext(req.Context()); key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	} else if c.authToken != "" {
		req.Header.Set("X-Internal-Api-Key", c.authToken)
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("grapery policy api %s: status %d", req.URL.Path, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(dest)
}
