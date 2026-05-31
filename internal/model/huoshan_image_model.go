package model

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const DefaultSeedreamModel = "doubao-seedream-5-0-260128"

// HuoshanImageModel Seedream 图片生成
type HuoshanImageModel struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
}

func NewImageModel(apiKey, baseURL, model string, timeout int) *HuoshanImageModel {
	if model == "" {
		model = DefaultSeedreamModel
	}
	return &HuoshanImageModel{
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
		httpClient: &http.Client{
			Timeout: time.Duration(timeout) * time.Second,
		},
	}
}

type ImageRequest struct {
	Prompt    string   // 生成提示词
	RefImages []string // 参考图 URL（最多 14 张）
	Size      string   // 如 "1024x1024", "adaptive"
	MaxImages int      // 图片集数量（>1 时使用 set 模式）
	Seed      int64    // 随机种子（0 为不设）
	Watermark bool     // 是否添加水印
}

type ImageResult struct {
	URLs  []string
	Usage ImageUsage
}

type ImageUsage struct {
	GeneratedImages int `json:"generated_images"`
	OutputTokens    int `json:"output_tokens"`
	TotalTokens     int `json:"total_tokens"`
}

func (m *HuoshanImageModel) Generate(ctx context.Context, req ImageRequest) (*ImageResult, error) {
	mode := inferSeedreamMode(req.RefImages, req.MaxImages)
	isSet := strings.Contains(mode, "_set")

	body := m.buildRequestBody(req, mode)

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, m.baseURL+"/images/generations", strings.NewReader(string(data)))
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

	if resp.StatusCode != http.StatusOK {
		bodyData, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("image API error %d: %s", resp.StatusCode, string(bodyData))
	}

	if isSet {
		return parseImageStreamResponse(resp.Body)
	}
	return parseImageSyncResponse(resp.Body)
}

// inferSeedreamMode 根据参考图数量和输出数量推断 Seedream 5.0 模式
func inferSeedreamMode(refImages []string, maxImages int) string {
	refCount := len(refImages)
	multi := maxImages > 1

	switch {
	case refCount == 0 && !multi:
		return "seedream50_text_single"
	case refCount == 0 && multi:
		return "seedream50_text_set"
	case refCount == 1 && !multi:
		return "seedream50_i2i_1_single"
	case refCount == 1 && multi:
		return "seedream50_i2i_1_set"
	case refCount >= 2 && !multi:
		return "seedream50_i2i_n_single"
	default: // refCount >= 2 && multi
		return "seedream50_i2i_n_set"
	}
}

func (m *HuoshanImageModel) buildRequestBody(req ImageRequest, mode string) map[string]any {
	isSet := strings.Contains(mode, "_set")

	body := map[string]any{
		"model":           m.model,
		"prompt":          req.Prompt,
		"response_format": "url",
	}

	if req.Size != "" {
		body["size"] = req.Size
	}

	if req.Seed != 0 {
		body["seed"] = req.Seed
	}

	body["watermark"] = req.Watermark

	// 参考图
	if len(req.RefImages) == 1 {
		body["image"] = req.RefImages[0]
	} else if len(req.RefImages) > 1 {
		body["image"] = req.RefImages
	}

	// 图片集模式
	if isSet {
		body["sequential_image_generation"] = "auto"
		if req.MaxImages > 0 {
			body["sequential_image_generation_options"] = map[string]any{
				"max_images": req.MaxImages,
			}
		}
		body["stream"] = true
	} else {
		body["sequential_image_generation"] = "disabled"
	}

	return body
}

// parseImageSyncResponse 解析非流式图片响应
func parseImageSyncResponse(body io.Reader) (*ImageResult, error) {
	var resp struct {
		Data []struct {
			URL string `json:"url"`
		} `json:"data"`
		Usage ImageUsage `json:"usage"`
	}
	if err := json.NewDecoder(body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("decode image response: %w", err)
	}

	urls := make([]string, 0, len(resp.Data))
	for _, d := range resp.Data {
		if d.URL != "" {
			urls = append(urls, d.URL)
		}
	}

	return &ImageResult{URLs: urls, Usage: resp.Usage}, nil
}

// parseImageStreamResponse 解析流式 SSE 图片响应
func parseImageStreamResponse(body io.Reader) (*ImageResult, error) {
	var urls []string
	var usage ImageUsage

	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var event struct {
			Type string `json:"type"`
			URL  string `json:"url"`
			Usage *ImageUsage `json:"usage,omitempty"`
		}
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		switch event.Type {
		case "image_generation.partial_succeeded":
			if event.URL != "" {
				urls = append(urls, event.URL)
			}
		case "image_generation.completed":
			if event.Usage != nil {
				usage = *event.Usage
			}
		}
	}

	return &ImageResult{URLs: urls, Usage: usage}, nil
}
