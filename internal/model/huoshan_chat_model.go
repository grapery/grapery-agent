package model

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/grapestree/fgrapery/grapery-agent/internal/config"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// HuoshanChatModel 将火山方舟 API 包装为 Eino ToolCallingChatModel
// 支持文本生成和 function calling
type HuoshanChatModel struct {
	apiKey     string
	baseURL    string
	model      string // endpoint ID，如 ep-xxxx
	httpClient *http.Client
	tools      []*schema.ToolInfo
}

func NewHuoshanChatModel(cfg config.EinoConfig) *HuoshanChatModel {
	return &HuoshanChatModel{
		apiKey:  cfg.HuoshanAPIKey,
		baseURL: cfg.HuoshanBaseURL,
		model:   cfg.TextModel,
		httpClient: &http.Client{
			Timeout: time.Duration(cfg.RequestTimeout) * time.Second,
		},
	}
}

// NewChatModel 根据 TextProvider 配置返回对应的 ChatModel。
// huoshan 和 gemini 均使用 OpenAI 兼容 API，复用同一个实现。
// 缺少必要配置时 panic 并给出明确错误信息。
func NewChatModel(cfg config.EinoConfig) *HuoshanChatModel {
	textModel := strings.TrimSpace(cfg.TextModel)
	if textModel == "" {
		if cfg.TextProvider == "gemini" {
			log.Fatalf("EINO_TEXT_MODEL is required when EINO_TEXT_PROVIDER=gemini")
		}
		textModel = config.DefaultHuoshanTextModel
		log.Printf("warning: EINO_TEXT_MODEL unset, using default %q", textModel)
	}
	cfg.TextModel = textModel
	switch cfg.TextProvider {
	case "gemini":
		if cfg.GeminiAPIKey == "" {
			log.Fatalf("GEMINI_API_KEY is required when EINO_TEXT_PROVIDER=gemini")
		}
		return &HuoshanChatModel{
			apiKey:  cfg.GeminiAPIKey,
			baseURL: "https://generativelanguage.googleapis.com/v1beta/openai",
			model:   cfg.TextModel,
			httpClient: &http.Client{
				Timeout: time.Duration(cfg.RequestTimeout) * time.Second,
			},
		}
	default:
		if cfg.TextProvider != "huoshan" && cfg.TextProvider != "" {
			log.Printf("warning: unknown EINO_TEXT_PROVIDER %q, falling back to huoshan", cfg.TextProvider)
		}
		if cfg.HuoshanAPIKey == "" {
			log.Fatalf("HUOSHAN_API_KEY is required when EINO_TEXT_PROVIDER=huoshan")
		}
		return NewHuoshanChatModel(cfg)
	}
}

// arkChatRequest 火山方舟 Chat API 请求格式（OpenAI 兼容）
type arkChatRequest struct {
	Model         string            `json:"model"`
	Messages      []arkMessage      `json:"messages"`
	Tools         []arkTool         `json:"tools,omitempty"`
	Temperature   float64           `json:"temperature,omitempty"`
	MaxTokens     int               `json:"max_tokens,omitempty"`
	Stream        bool              `json:"stream,omitempty"`
	StreamOptions *arkStreamOptions `json:"stream_options,omitempty"`
}

type arkStreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type arkMessage struct {
	Role       string        `json:"role"`
	Content    string        `json:"content,omitempty"`
	ToolCalls  []arkToolCall `json:"tool_calls,omitempty"`
	ToolCallID string        `json:"tool_call_id,omitempty"`
}

type arkTool struct {
	Type     string      `json:"type"`
	Function arkFunction `json:"function"`
}

type arkFunction struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  any    `json:"parameters,omitempty"`
}

type arkToolCall struct {
	Index    int    `json:"index,omitempty"`
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type arkChatResponse struct {
	Choices []struct {
		Message struct {
			Role      string        `json:"role"`
			Content   string        `json:"content"`
			ToolCalls []arkToolCall `json:"tool_calls,omitempty"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// arkStreamChunk SSE 流式响应块
type arkStreamChunk struct {
	Choices []struct {
		Delta struct {
			Role      string        `json:"role,omitempty"`
			Content   string        `json:"content,omitempty"`
			ToolCalls []arkToolCall `json:"tool_calls,omitempty"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

func (m *HuoshanChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	options := model.GetCommonOptions(&model.Options{}, opts...)
	tools := m.tools
	if len(options.Tools) > 0 {
		tools = options.Tools
	}

	messages := toArkMessages(input)
	arkTools := toArkTools(tools)

	reqBody := arkChatRequest{
		Model:       m.model,
		Messages:    messages,
		Tools:       arkTools,
		Stream:      false,
		Temperature: applyFloatPtr(options.Temperature),
		MaxTokens:   applyIntPtr(options.MaxTokens),
	}

	resp, err := m.doRequest(ctx, reqBody)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("huoshan API error %d: %s", resp.StatusCode, string(body))
	}

	var result arkChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	message := arkToSchemaMessage(result.Choices[0].Message)
	message.ResponseMeta = &schema.ResponseMeta{
		FinishReason: result.Choices[0].FinishReason,
		Usage: &schema.TokenUsage{
			PromptTokens: result.Usage.PromptTokens, CompletionTokens: result.Usage.CompletionTokens, TotalTokens: result.Usage.TotalTokens,
		},
	}
	return message, nil
}

func (m *HuoshanChatModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	options := model.GetCommonOptions(&model.Options{}, opts...)
	tools := m.tools
	if len(options.Tools) > 0 {
		tools = options.Tools
	}

	messages := toArkMessages(input)
	arkTools := toArkTools(tools)

	reqBody := arkChatRequest{
		Model:         m.model,
		Messages:      messages,
		Tools:         arkTools,
		Stream:        true,
		StreamOptions: &arkStreamOptions{IncludeUsage: true},
		Temperature:   applyFloatPtr(options.Temperature),
		MaxTokens:     applyIntPtr(options.MaxTokens),
	}

	resp, err := m.doRequest(ctx, reqBody)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("huoshan API error %d: %s", resp.StatusCode, string(body))
	}

	_sr, sw := schema.Pipe[*schema.Message](1)

	go func() {
		defer sw.Close()
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				return
			}

			var chunk arkStreamChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}

			if len(chunk.Choices) > 0 {
				delta := chunk.Choices[0].Delta
				msg := &schema.Message{
					Role:    schema.RoleType(delta.Role),
					Content: delta.Content,
				}
				for _, tc := range delta.ToolCalls {
					idx := tc.Index
					msg.ToolCalls = append(msg.ToolCalls, schema.ToolCall{
						Index: &idx,
						ID:    tc.ID,
						Function: schema.FunctionCall{
							Name:      tc.Function.Name,
							Arguments: tc.Function.Arguments,
						},
					})
				}
				if chunk.Choices[0].FinishReason != nil {
					msg.Role = schema.Assistant
				}
				sw.Send(msg, nil)
			}
			if chunk.Usage.TotalTokens > 0 {
				sw.Send(&schema.Message{
					Role: schema.Assistant,
					ResponseMeta: &schema.ResponseMeta{Usage: &schema.TokenUsage{
						PromptTokens: chunk.Usage.PromptTokens, CompletionTokens: chunk.Usage.CompletionTokens, TotalTokens: chunk.Usage.TotalTokens,
					}},
				}, nil)
			}
		}
		if err := scanner.Err(); err != nil {
			sw.Send(nil, fmt.Errorf("scan stream: %w", err))
		}
	}()

	return _sr, nil
}

func (m *HuoshanChatModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	newModel := *m
	newModel.tools = tools
	return &newModel, nil
}

func (m *HuoshanChatModel) BindTools(tools []*schema.ToolInfo) error {
	m.tools = tools
	return nil
}

func (m *HuoshanChatModel) doRequest(ctx context.Context, body any) (*http.Response, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.baseURL+"/chat/completions", strings.NewReader(string(data)))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+m.apiKey)

	return m.httpClient.Do(req)
}

func toArkMessages(msgs []*schema.Message) []arkMessage {
	result := make([]arkMessage, 0, len(msgs))
	for _, msg := range msgs {
		am := arkMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
		}
		for _, tc := range msg.ToolCalls {
			am.ToolCalls = append(am.ToolCalls, arkToolCall{
				ID:   tc.ID,
				Type: "function",
				Function: struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				}{Name: tc.Function.Name, Arguments: tc.Function.Arguments},
			})
		}
		if msg.ToolCallID != "" {
			am.ToolCallID = msg.ToolCallID
		}
		result = append(result, am)
	}
	return result
}

func toArkTools(tools []*schema.ToolInfo) []arkTool {
	if len(tools) == 0 {
		return nil
	}
	result := make([]arkTool, 0, len(tools))
	for _, t := range tools {
		var params any
		if t.ParamsOneOf != nil {
			js, err := t.ParamsOneOf.ToJSONSchema()
			if err == nil && js != nil {
				params = js
			}
		}
		result = append(result, arkTool{
			Type: "function",
			Function: arkFunction{
				Name:        t.Name,
				Description: t.Desc,
				Parameters:  params,
			},
		})
	}
	return result
}

func applyFloatPtr(p *float32) float64 {
	if p == nil {
		return 0
	}
	return float64(*p)
}

func applyIntPtr(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

func arkToSchemaMessage(am struct {
	Role      string        `json:"role"`
	Content   string        `json:"content"`
	ToolCalls []arkToolCall `json:"tool_calls,omitempty"`
}) *schema.Message {
	msg := &schema.Message{
		Role:    schema.RoleType(am.Role),
		Content: am.Content,
	}
	for _, tc := range am.ToolCalls {
		msg.ToolCalls = append(msg.ToolCalls, schema.ToolCall{
			ID:   tc.ID,
			Type: tc.Type,
			Function: schema.FunctionCall{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		})
	}
	return msg
}
