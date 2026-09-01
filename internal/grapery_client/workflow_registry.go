package grapery_client

import (
	"context"
	"fmt"
	"net/url"

	"github.com/grapestree/fgrapery/grapery-agent/internal/domain"
)

func (c *Client) GetWorkflowRelease(ctx context.Context, id string) (*domain.WorkflowRelease, error) {
	path := "/api/v1/agent-policy/workflow-releases/" + url.PathEscape(id)
	var out struct {
		Code    int                    `json:"code"`
		Data    domain.WorkflowRelease `json:"data"`
		Message string                 `json:"message"`
	}
	if err := c.getInternalJSON(ctx, path, &out); err != nil {
		return nil, err
	}
	if out.Code != 1 {
		return nil, fmt.Errorf("get workflow release: %s", out.Message)
	}
	return &out.Data, nil
}

func (c *Client) GetPromptVersion(ctx context.Context, id string) (*domain.PromptTemplateVersion, error) {
	path := "/api/v1/agent-policy/prompt-versions/" + url.PathEscape(id)
	var out struct {
		Code    int                          `json:"code"`
		Data    domain.PromptTemplateVersion `json:"data"`
		Message string                       `json:"message"`
	}
	if err := c.getInternalJSON(ctx, path, &out); err != nil {
		return nil, err
	}
	if out.Code != 1 {
		return nil, fmt.Errorf("get prompt version: %s", out.Message)
	}
	return &out.Data, nil
}

func (c *Client) ListWorkflowCatalog(ctx context.Context, surface, action, tenantID string) ([]domain.WorkflowCatalogEntry, error) {
	query := url.Values{}
	query.Set("surface", surface)
	query.Set("action", action)
	if tenantID != "" {
		query.Set("tenantId", tenantID)
	}
	var out struct {
		Code int `json:"code"`
		Data struct {
			Items []domain.WorkflowCatalogEntry `json:"items"`
		} `json:"data"`
		Message string `json:"message"`
	}
	if err := c.getInternalJSON(ctx, "/api/v1/agent-policy/workflow-catalog?"+query.Encode(), &out); err != nil {
		return nil, err
	}
	if out.Code != 1 {
		return nil, fmt.Errorf("list workflow catalog: %s", out.Message)
	}
	return out.Data.Items, nil
}

func (c *Client) ResolveWorkflow(ctx context.Context, surface, action, tenantID string, input map[string]any) (*domain.WorkflowResolution, error) {
	var out struct {
		Code    int                       `json:"code"`
		Data    domain.WorkflowResolution `json:"data"`
		Message string                    `json:"message"`
	}
	body := map[string]any{"surface": surface, "action": action, "input": input}
	if tenantID != "" {
		body["tenantId"] = tenantID
	}
	if err := c.postInternalJSON(ctx, "/api/v1/agent-policy/workflow-resolve", body, &out); err != nil {
		return nil, err
	}
	if out.Code != 1 {
		return nil, fmt.Errorf("resolve workflow: %s", out.Message)
	}
	return &out.Data, nil
}
