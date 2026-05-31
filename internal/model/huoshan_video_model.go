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

const DefaultSeedanceModel = "doubao-seedance-1-5-pro-251215"

// HuoshanVideoModel Seedance 视频生成（异步任务模式）
type HuoshanVideoModel struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
}

func NewVideoModel(apiKey, baseURL, model string, timeout int) *HuoshanVideoModel {
	if model == "" {
		model = DefaultSeedanceModel
	}
	return &HuoshanVideoModel{
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
		httpClient: &http.Client{
			Timeout: time.Duration(timeout) * time.Second,
		},
	}
}

type VideoRequest struct {
	Prompt         string   // 视频描述提示词
	Duration       int      // 时长（秒）
	Ratio          string   // 分辨率比例，如 "16:9", "9:16"
	FirstFrame     string   // 起始帧图片 URL
	LastFrame      string   // 结束帧图片 URL（keyframe 模式）
	RefImages      []string // 参考图（image-to-video 多图模式）
	ReturnLastFrame bool    // 是否返回最后一帧
}

type VideoTask struct {
	TaskID       string
	Status       string // queued, running, succeeded, failed, cancelled
	VideoURL     string
	LastFrameURL string
	Seed         int64
	Duration     float64
	Resolution   string
	Error        string
}

// CreateTask 创建视频生成任务
func (m *HuoshanVideoModel) CreateTask(ctx context.Context, req VideoRequest) (*VideoTask, error) {
	content := m.buildContent(req)

	body := map[string]any{
		"model":              m.model,
		"content":            content,
		"return_last_frame":  req.ReturnLastFrame,
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, m.baseURL+"/contents/generations/tasks", strings.NewReader(string(data)))
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
		return nil, fmt.Errorf("video create API error %d: %s", resp.StatusCode, string(respData))
	}

	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(respData, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &VideoTask{TaskID: result.ID, Status: "queued"}, nil
}

// GetTask 查询视频任务状态
func (m *HuoshanVideoModel) GetTask(ctx context.Context, taskID string) (*VideoTask, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, m.baseURL+"/contents/generations/tasks/"+taskID, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
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
		return nil, fmt.Errorf("video get API error %d: %s", resp.StatusCode, string(respData))
	}

	var result struct {
		ID     string `json:"id"`
		Status string `json:"status"`
		Content struct {
			VideoURL     string `json:"video_url"`
			LastFrameURL string `json:"last_frame_url"`
		} `json:"content"`
		Seed       int64   `json:"seed"`
		Duration   float64 `json:"duration"`
		Resolution string  `json:"resolution"`
		Error      *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(respData, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	task := &VideoTask{
		TaskID:       result.ID,
		Status:       result.Status,
		VideoURL:     result.Content.VideoURL,
		LastFrameURL: result.Content.LastFrameURL,
		Seed:         result.Seed,
		Duration:     result.Duration,
		Resolution:   result.Resolution,
	}
	if result.Error != nil {
		task.Error = result.Error.Message
	}
	return task, nil
}

// WaitTask 轮询等待视频任务完成，interval 为轮询间隔
func (m *HuoshanVideoModel) WaitTask(ctx context.Context, taskID string, interval time.Duration) (*VideoTask, error) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			task, err := m.GetTask(ctx, taskID)
			if err != nil {
				return nil, err
			}
			switch task.Status {
			case "succeeded":
				return task, nil
			case "failed", "cancelled":
				return task, fmt.Errorf("video task %s: %s", task.Status, task.Error)
			}
			// queued / running: continue polling
		}
	}
}

// buildContent 构建视频请求的 content 数组
func (m *HuoshanVideoModel) buildContent(req VideoRequest) []map[string]any {
	content := make([]map[string]any, 0, 4)

	// 文本 prompt（含 --dur 和 --ratio 参数）
	prompt := req.Prompt
	if req.Duration > 0 {
		prompt += fmt.Sprintf(" --dur %d", req.Duration)
	}
	if req.Ratio != "" {
		prompt += fmt.Sprintf(" --ratio %s", req.Ratio)
	}
	content = append(content, map[string]any{
		"type": "text",
		"text": prompt,
	})

	// Keyframe 模式：first_frame + last_frame
	if req.FirstFrame != "" && req.LastFrame != "" {
		content = append(content, map[string]any{
			"type":      "image_url",
			"image_url": map[string]string{"url": req.FirstFrame},
			"role":      "first_frame",
		})
		content = append(content, map[string]any{
			"type":      "image_url",
			"image_url": map[string]string{"url": req.LastFrame},
			"role":      "last_frame",
		})
		return content
	}

	// 单图 first_frame 模式
	if req.FirstFrame != "" {
		content = append(content, map[string]any{
			"type":      "image_url",
			"image_url": map[string]string{"url": req.FirstFrame},
			"role":      "first_frame",
		})
		return content
	}

	// 多图 reference_image 模式
	for _, url := range req.RefImages {
		content = append(content, map[string]any{
			"type":      "image_url",
			"image_url": map[string]string{"url": url},
			"role":      "reference_image",
		})
	}

	return content
}
