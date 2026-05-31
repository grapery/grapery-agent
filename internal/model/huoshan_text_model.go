package model

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// HuoshanTextModel 结构化文本生成（区别于对话用的 ChatModel）。
// 支持 JSON mode 和多模态（图片输入）。
type HuoshanTextModel struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
}

func NewTextModel(apiKey, baseURL, model string, timeout int) *HuoshanTextModel {
	return &HuoshanTextModel{
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
		httpClient: &http.Client{
			Timeout: time.Duration(timeout) * time.Second,
		},
	}
}

type TextRequest struct {
	SystemPrompt string
	UserPrompt   string
	MaxTokens    int
	Temperature  float32
	JSONMode     bool
	ImageURLs    []string
}

type TextResponse struct {
	Content          string
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

func (m *HuoshanTextModel) Generate(ctx context.Context, req TextRequest) (*TextResponse, error) {
	messages := m.buildMessages(req)

	body := map[string]any{
		"model":       m.model,
		"messages":    messages,
		"max_tokens":  req.MaxTokens,
		"temperature": float64(req.Temperature),
	}
	if req.JSONMode {
		body["response_format"] = map[string]string{"type": "json_object"}
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, m.baseURL+"/chat/completions", strings.NewReader(string(data)))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+m.apiKey)

	resp, err := m.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("text API error %d: %s", resp.StatusCode, string(respData))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(respData, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	return &TextResponse{
		Content:          result.Choices[0].Message.Content,
		PromptTokens:     result.Usage.PromptTokens,
		CompletionTokens: result.Usage.CompletionTokens,
		TotalTokens:      result.Usage.TotalTokens,
	}, nil
}

// buildMessages 构建消息数组，支持多模态（图片输入）
func (m *HuoshanTextModel) buildMessages(req TextRequest) []map[string]any {
	msgs := make([]map[string]any, 0, 2)

	if req.SystemPrompt != "" {
		msgs = append(msgs, map[string]any{
			"role":    "system",
			"content": req.SystemPrompt,
		})
	}

	if len(req.ImageURLs) > 0 {
		// 多模态：构建 content parts
		parts := make([]map[string]any, 0, len(req.ImageURLs)*2+1)
		for _, url := range req.ImageURLs {
			parts = append(parts, map[string]any{
				"type":      "image_url",
				"image_url": map[string]string{"url": url},
			})
		}
		parts = append(parts, map[string]any{
			"type": "text",
			"text": req.UserPrompt,
		})
		msgs = append(msgs, map[string]any{
			"role":    "user",
			"content": parts,
		})
	} else {
		msgs = append(msgs, map[string]any{
			"role":    "user",
			"content": req.UserPrompt,
		})
	}

	return msgs
}
