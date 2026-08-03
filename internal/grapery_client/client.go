package grapery_client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/grapestree/fgrapery/grapery-agent/internal/config"
)

type contextKey struct{}
type idempotencyContextKey struct{}

// ContextWithAuthToken 将 auth token 存入 context，供并发安全的请求级传递
func ContextWithAuthToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, contextKey{}, token)
}

// AuthTokenFromContext returns the request-scoped user token, if one was forwarded.
func AuthTokenFromContext(ctx context.Context) (string, bool) {
	t, ok := ctx.Value(contextKey{}).(string)
	return t, ok && t != ""
}

func ContextWithIdempotencyKey(ctx context.Context, key string) context.Context {
	return context.WithValue(ctx, idempotencyContextKey{}, key)
}

// Client 是 grapery 后端的 HTTP 客户端
type Client struct {
	baseURL    string
	httpClient *http.Client
	authToken  string // 全局 fallback token（服务间认证）
}

func NewClient(cfg config.GraperyConfig) *Client {
	return &Client{
		baseURL: cfg.BaseURL,
		httpClient: &http.Client{
			// A workflow text stage may legitimately wait behind provider queues for
			// hours. The workflow context remains the primary cancellation boundary.
			Timeout: 12*time.Hour + 5*time.Minute,
		},
		authToken: cfg.APIKey,
	}
}

// tokenFromContext 优先从 context 取请求级 token，fallback 到全局 token
func (c *Client) tokenFromContext(ctx context.Context) string {
	if t, ok := ctx.Value(contextKey{}).(string); ok && t != "" {
		return t
	}
	return c.authToken
}

// Grapery 统一响应格式
type Response struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func (r *Response) IsSuccess() bool {
	return r.Code == 1
}

func (c *Client) get(ctx context.Context, path string, result interface{}) error {
	return c.doRequest(ctx, http.MethodGet, path, nil, result)
}

func (c *Client) post(ctx context.Context, path string, body interface{}, result interface{}) error {
	return c.doRequest(ctx, http.MethodPost, path, body, result)
}

func (c *Client) delete(ctx context.Context, path string, result interface{}) error {
	return c.doRequest(ctx, http.MethodDelete, path, nil, result)
}

// doRawRequest 发送请求并直接将响应体反序列化到 result，跳过 {code, message, data} 信封。
// 用于后端返回原始 JSON 的端点（如 fragment generation）。
func (c *Client) doRawRequest(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if key, _ := ctx.Value(idempotencyContextKey{}).(string); key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	if token := c.tokenFromContext(ctx); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respData))
	}

	if result != nil {
		if err := json.Unmarshal(respData, result); err != nil {
			return fmt.Errorf("unmarshal response: %w", err)
		}
	}

	return nil
}

func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if key, _ := ctx.Value(idempotencyContextKey{}).(string); key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	if token := c.tokenFromContext(ctx); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respData))
	}

	var gr Response
	if err := json.Unmarshal(respData, &gr); err != nil {
		return fmt.Errorf("unmarshal response: %w", err)
	}

	if !gr.IsSuccess() {
		return fmt.Errorf("grapery error (code=%d): %s", gr.Code, gr.Message)
	}

	if result != nil && gr.Data != nil {
		if err := json.Unmarshal(gr.Data, result); err != nil {
			return fmt.Errorf("unmarshal data: %w", err)
		}
	}

	return nil
}
