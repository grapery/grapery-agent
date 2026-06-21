package fragment_panel

import (
	"context"
	"fmt"

	"github.com/grapestree/fgrapery/grapery-agent/internal/grapery_client"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

func NewGeneratePanelFragmentTool(client *grapery_client.Client) (tool.InvokableTool, error) {
	return utils.InferTool(
		"generate_panel_fragment",
		"从一张参考图启动多面板故事碎片生成（异步）",
		func(ctx context.Context, input *GeneratePanelInput) (*GeneratePanelOutput, error) {
			resp, err := client.GenerateFragmentPanel(ctx, grapery_client.GenerateFragmentPanelRequest{
				UserInput:              input.UserInput,
				ReferenceImageURL:      input.ReferenceImageURL,
				Style:                  input.Style,
				PanelCount:             input.PanelCount,
				Visibility:             input.Visibility,
				Topic:                  input.Topic,
				AspectRatio:            input.AspectRatio,
				DialogueMode:           input.DialogueMode,
				ConsistencyLevel:       input.ConsistencyLevel,
				EnableReferenceAssets:  input.EnableReferenceAssets,
				IncludeGenerationTrace: input.IncludeGenerationTrace,
			})
			if err != nil {
				return nil, fmt.Errorf("generate panel fragment: %w", err)
			}
			return &GeneratePanelOutput{
				TaskID:          resp.TaskID,
				Status:          resp.Status,
				DraftFragmentID: resp.DraftFragmentID,
			}, nil
		},
	)
}

func NewPollPanelTaskStatusTool(client *grapery_client.Client) (tool.InvokableTool, error) {
	return utils.InferTool(
		"poll_panel_task_status",
		"查询参考图多面板碎片任务进度",
		func(ctx context.Context, input *PollPanelStatusInput) (*PollPanelStatusOutput, error) {
			st, err := client.GetFragmentPanelTaskStatus(ctx, input.TaskID)
			if err != nil {
				return nil, fmt.Errorf("poll panel task: %w", err)
			}
			out := &PollPanelStatusOutput{
				TaskID:          st.TaskID,
				Status:          st.Status,
				Progress:        st.Progress,
				CurrentStep:     st.CurrentStep,
				DraftFragmentID: st.DraftFragmentID,
				Error:           st.Error,
				CombinedContent: st.CombinedContent,
				TokensUsed:      st.TokensUsed,
			}
			for _, p := range st.Panels {
				out.Panels = append(out.Panels, PanelResult{
					Index:    p.Index,
					ImageURL: p.ImageURL,
					Caption:  p.Caption,
				})
			}
			return out, nil
		},
	)
}

func NewRetryPanelTaskTool(client *grapery_client.Client) (tool.InvokableTool, error) {
	return utils.InferTool(
		"retry_panel_task",
		"重试失败的参考图多面板碎片任务",
		func(ctx context.Context, input *PanelTaskIDInput) (*GeneratePanelOutput, error) {
			resp, err := client.RetryFragmentPanelTask(ctx, input.TaskID)
			if err != nil {
				return nil, fmt.Errorf("retry panel task: %w", err)
			}
			return &GeneratePanelOutput{
				TaskID:          resp.TaskID,
				Status:          resp.Status,
				DraftFragmentID: resp.DraftFragmentID,
			}, nil
		},
	)
}

func NewResumePanelTaskTool(client *grapery_client.Client) (tool.InvokableTool, error) {
	return utils.InferTool(
		"resume_panel_task",
		"从失败/中断处恢复参考图多面板碎片任务",
		func(ctx context.Context, input *PanelTaskIDInput) (*GeneratePanelOutput, error) {
			resp, err := client.ResumeFragmentPanelTask(ctx, input.TaskID)
			if err != nil {
				return nil, fmt.Errorf("resume panel task: %w", err)
			}
			return &GeneratePanelOutput{
				TaskID:          resp.TaskID,
				Status:          resp.Status,
				DraftFragmentID: resp.DraftFragmentID,
			}, nil
		},
	)
}

func AllTools(client *grapery_client.Client) ([]tool.BaseTool, error) {
	var tools []tool.BaseTool
	add := func(t tool.InvokableTool, err error) error {
		if err != nil {
			return err
		}
		tools = append(tools, t)
		return nil
	}
	if err := add(NewGeneratePanelFragmentTool(client)); err != nil {
		return nil, err
	}
	if err := add(NewPollPanelTaskStatusTool(client)); err != nil {
		return nil, err
	}
	if err := add(NewRetryPanelTaskTool(client)); err != nil {
		return nil, err
	}
	if err := add(NewResumePanelTaskTool(client)); err != nil {
		return nil, err
	}
	return tools, nil
}

type PanelTaskIDInput struct {
	TaskID string `json:"task_id" jsonschema:"description=任务 ID,required"`
}

func ToolInfos() string {
	return `可用工具：
1. generate_panel_fragment - 从参考图启动多面板碎片生成（异步）
2. poll_panel_task_status - 查询多面板任务进度与结果
3. retry_panel_task - 重试失败任务
4. resume_panel_task - 从中断处恢复任务`
}

type GeneratePanelInput struct {
	UserInput              string `json:"user_input" jsonschema:"description=用户文字创意,required"`
	ReferenceImageURL      string `json:"reference_image_url" jsonschema:"description=参考图 URL,required"`
	Style                  string `json:"style,omitempty"`
	PanelCount             int    `json:"panel_count,omitempty"`
	Visibility             string `json:"visibility,omitempty"`
	Topic                  string `json:"topic,omitempty"`
	AspectRatio            string `json:"aspect_ratio,omitempty"`
	DialogueMode           string `json:"dialogue_mode,omitempty"`
	ConsistencyLevel       string `json:"consistency_level,omitempty"`
	EnableReferenceAssets  *bool  `json:"enable_reference_assets,omitempty"`
	IncludeGenerationTrace bool   `json:"include_generation_trace,omitempty"`
}

type GeneratePanelOutput struct {
	TaskID          string `json:"task_id"`
	Status          string `json:"status"`
	DraftFragmentID string `json:"draft_fragment_id,omitempty"`
}

type PollPanelStatusInput struct {
	TaskID string `json:"task_id" jsonschema:"description=任务 ID,required"`
}

type PollPanelStatusOutput struct {
	TaskID          string        `json:"task_id"`
	Status          string        `json:"status"`
	Progress        float64       `json:"progress"`
	CurrentStep     string        `json:"current_step,omitempty"`
	DraftFragmentID string        `json:"draft_fragment_id,omitempty"`
	Error           string        `json:"error,omitempty"`
	CombinedContent string        `json:"combined_content,omitempty"`
	Panels          []PanelResult `json:"panels,omitempty"`
	TokensUsed      int           `json:"tokens_used,omitempty"`
}

type PanelResult struct {
	Index    int    `json:"index"`
	ImageURL string `json:"image_url"`
	Caption  string `json:"caption"`
}
