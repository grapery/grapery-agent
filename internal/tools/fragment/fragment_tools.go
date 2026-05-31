package fragment

import (
	"context"
	"fmt"

	"github.com/grapestree/fgrapery/grapery-agent/internal/grapery_client"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

// NewExtractElementsTool 元素提取 + 内容生成 + VisualBible
// 调用 grapery 后端的碎片生成 API 来启动整个流程
func NewExtractElementsTool(client *grapery_client.Client) (tool.InvokableTool, error) {
	return utils.InferTool(
		"extract_elements",
		"从用户输入中提取故事元素，生成内容大纲和 VisualBible（风格圣经、角色定义、道具、场景设定）",
		func(ctx context.Context, input *ExtractElementsInput) (*ExtractElementsOutput, error) {
			resp, err := client.GenerateFragment(ctx, grapery_client.GenerateFragmentRequest{
				UserInput:  input.UserInput,
				ImageUrls:  input.ReferenceImages,
				ImageCount: input.ImageCount,
				Style:      input.Style,
				Mood:       input.Mood,
				Length:     input.Length,
				Language:   input.Language,
				Visibility: input.Visibility,
				AspectRatio:       input.AspectRatio,
				ConsistencyLevel:  input.ConsistencyLevel,
			})
			if err != nil {
				return nil, fmt.Errorf("extract elements failed: %w", err)
			}

			return &ExtractElementsOutput{
				TaskID:          resp.TaskID,
				Status:          resp.Status,
				DraftFragmentID: resp.DraftFragmentID,
			}, nil
		},
	)
}

// NewPollTaskStatusTool 轮询任务状态
func NewPollTaskStatusTool(client *grapery_client.Client) (tool.InvokableTool, error) {
	return utils.InferTool(
		"poll_task_status",
		"查询碎片生成任务的当前进度和状态",
		func(ctx context.Context, input *PollStatusInput) (*PollStatusOutput, error) {
			status, err := client.GetFragmentTaskStatus(ctx, input.TaskID)
			if err != nil {
				return nil, fmt.Errorf("poll status failed: %w", err)
			}
			out := &PollStatusOutput{
				TaskID:      status.TaskID,
				Status:      status.Status,
				Progress:    status.Progress,
				CurrentStep: status.CurrentStep,
				Error:       status.Error,
			}
			if status.Result != nil {
				out.Content = status.Result.Content
				out.ImageUrls = status.Result.ImageUrls
				out.TokensUsed = status.Result.TokensUsed
				out.AspectRatio = status.Result.AspectRatio
			}
			return out, nil
		},
	)
}

// NewCancelTaskTool 取消任务
func NewCancelTaskTool(client *grapery_client.Client) (tool.InvokableTool, error) {
	return utils.InferTool(
		"cancel_task",
		"取消正在进行的碎片生成任务",
		func(ctx context.Context, input *CancelTaskInput) (*CancelTaskOutput, error) {
			if err := client.CancelFragmentTask(ctx, input.TaskID); err != nil {
				return nil, fmt.Errorf("cancel task failed: %w", err)
			}
			return &CancelTaskOutput{Success: true, TaskID: input.TaskID}, nil
		},
	)
}

// NewEnhancePromptTool 增强提示词
func NewEnhancePromptTool(client *grapery_client.Client) (tool.InvokableTool, error) {
	return utils.InferTool(
		"enhance_prompt",
		"使用 AI 增强用户的原始提示词，使其更适合故事创作",
		func(ctx context.Context, input *EnhancePromptInput) (*EnhancePromptOutput, error) {
			resp, err := client.EnhancePrompt(ctx, grapery_client.EnhancePromptRequest{
				OriginalPrompt: input.OriginalPrompt,
				TargetType:     input.TargetType,
				Style:          input.Style,
				DetailLevel:    input.DetailLevel,
			})
			if err != nil {
				return nil, fmt.Errorf("enhance prompt failed: %w", err)
			}
			return &EnhancePromptOutput{
				TaskID:  resp.TaskID,
				Status:  resp.Status,
				Message: resp.Message,
			}, nil
		},
	)
}

// NewGenerateImageTool 独立的图片生成工具
func NewGenerateImageTool(client *grapery_client.Client) (tool.InvokableTool, error) {
	return utils.InferTool(
		"generate_image",
		"根据提示词独立生成一张图片（不依赖碎片管线）",
		func(ctx context.Context, input *GenerateImageInput) (*GenerateImageOutput, error) {
			resp, err := client.GenerateImage(ctx, grapery_client.GenerateImageRequest{
				Prompt:  input.Prompt,
				Size:    input.Size,
				Quality: input.Quality,
				Style:   input.Style,
				N:       input.N,
			})
			if err != nil {
				return nil, fmt.Errorf("generate image failed: %w", err)
			}
			return &GenerateImageOutput{TaskID: resp.TaskID, Status: resp.Status, Message: resp.Message}, nil
		},
	)
}

// NewPollAITaskTool 通用 AI 任务轮询工具
func NewPollAITaskTool(client *grapery_client.Client) (tool.InvokableTool, error) {
	return utils.InferTool(
		"poll_ai_task",
		"查询 AI 任务（enhance_prompt、generate_image 等）的当前状态",
		func(ctx context.Context, input *PollAITaskInput) (*PollAITaskOutput, error) {
			resp, err := client.GetAITaskStatus(ctx, input.TaskID)
			if err != nil {
				return nil, fmt.Errorf("poll AI task failed: %w", err)
			}
			return &PollAITaskOutput{
				TaskID:  resp.TaskID,
				Status:  resp.Status,
				Message: resp.Message,
			}, nil
		},
	)
}

// ============ Tool 输入输出类型 ============

type ExtractElementsInput struct {
	UserInput         string   `json:"user_input" jsonschema:"description=用户的故事创意描述,required"`
	ReferenceImages   []string `json:"reference_images,omitempty" jsonschema:"description=参考图片 URL 列表"`
	ImageCount        int      `json:"image_count,omitempty" jsonschema:"description=生成图片数量,1-10"`
	Style             string   `json:"style,omitempty" jsonschema:"description=故事风格,fantasy/sci-fi/romance/thriller等"`
	Mood              string   `json:"mood,omitempty" jsonschema:"description=故事氛围,happy/sad/mysterious/romantic"`
	Length            string   `json:"length,omitempty" jsonschema:"description=故事长度,short/medium/long"`
	Language          string   `json:"language,omitempty" jsonschema:"description=语言,zh-Hans/en/ja"`
	Visibility        string   `json:"visibility,omitempty" jsonschema:"description=可见性,public/followers/private"`
	AspectRatio       string   `json:"aspect_ratio,omitempty" jsonschema:"description=图片比例,1:1/16:9/9:16/3:4/4:3"`
	ConsistencyLevel  string   `json:"consistency_level,omitempty" jsonschema:"description=一致性级别,off/standard/strong"`
}

type ExtractElementsOutput struct {
	TaskID          string `json:"task_id"`
	Status          string `json:"status"`
	DraftFragmentID string `json:"draft_fragment_id,omitempty"`
}

type PollStatusInput struct {
	TaskID string `json:"task_id" jsonschema:"description=任务 ID,required"`
}

type PollStatusOutput struct {
	TaskID      string   `json:"task_id"`
	Status      string   `json:"status"`
	Progress    float64  `json:"progress"`
	CurrentStep string   `json:"current_step"`
	Error       string   `json:"error,omitempty"`
	Content     string   `json:"content,omitempty"`
	ImageUrls   []string `json:"image_urls,omitempty"`
	TokensUsed  int      `json:"tokens_used,omitempty"`
	AspectRatio string   `json:"aspect_ratio,omitempty"`
}

type CancelTaskInput struct {
	TaskID string `json:"task_id" jsonschema:"description=要取消的任务 ID,required"`
}

type CancelTaskOutput struct {
	Success bool   `json:"success"`
	TaskID  string `json:"task_id"`
}

type EnhancePromptInput struct {
	OriginalPrompt string `json:"original_prompt" jsonschema:"description=原始提示词,required"`
	TargetType     string `json:"target_type,omitempty" jsonschema:"description=目标类型,image/video/storyboard"`
	Style          string `json:"style,omitempty" jsonschema:"description=风格"`
	DetailLevel    string `json:"detail_level,omitempty" jsonschema:"description=细节级别,low/medium/high"`
}

type EnhancePromptOutput struct {
	TaskID  string `json:"task_id"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type GenerateImageInput struct {
	Prompt  string `json:"prompt" jsonschema:"description=图片描述提示词,required"`
	Size    string `json:"size,omitempty" jsonschema:"description=图片尺寸,1024x1024等"`
	Quality string `json:"quality,omitempty" jsonschema:"description=质量,standard/hd"`
	Style   string `json:"style,omitempty" jsonschema:"description=风格,vivid/natural"`
	N       int    `json:"n,omitempty" jsonschema:"description=生成数量,1-10"`
}

type GenerateImageOutput struct {
	TaskID  string `json:"task_id"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type PollAITaskInput struct {
	TaskID string `json:"task_id" jsonschema:"description=AI 任务 ID,required"`
}

type PollAITaskOutput struct {
	TaskID  string `json:"task_id"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type ConvertToStoryInput struct {
	FragmentID        string `json:"fragment_id" jsonschema:"description=碎片 ID,required"`
	Title             string `json:"title" jsonschema:"description=故事标题,required"`
	Description       string `json:"description,omitempty" jsonschema:"description=故事描述"`
	Genre             string `json:"genre,omitempty" jsonschema:"description=故事类型"`
	CoverImage        string `json:"cover_image,omitempty" jsonschema:"description=封面图片 URL"`
	SceneCount        int    `json:"scene_count,omitempty" jsonschema:"description=场景数量,2-8"`
	UseAI             bool   `json:"use_ai,omitempty" jsonschema:"description=是否使用 AI 续写"`
	CollaborationType string `json:"collaboration_type,omitempty" jsonschema:"description=协作类型,open/restricted/closed"`
}

type ConvertToStoryOutput struct {
	StoryID    string `json:"story_id"`
	StoryTitle string `json:"story_title"`
	FragmentID string `json:"fragment_id"`
}

type PrefillStoryInput struct {
	FragmentID string `json:"fragment_id" jsonschema:"description=碎片 ID,required"`
	SceneCount int    `json:"scene_count,omitempty" jsonschema:"description=场景数量,2-8"`
}

type PrefillStoryOutput struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Summary     string   `json:"summary,omitempty"`
	Style       string   `json:"style"`
	Genre       string   `json:"genre,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

func NewConvertToStoryTool(client *grapery_client.Client) (tool.InvokableTool, error) {
	return utils.InferTool(
		"convert_to_story",
		"将已生成的碎片转换为完整故事，创建故事记录（不包含故事板，需另建）",
		func(ctx context.Context, input *ConvertToStoryInput) (*ConvertToStoryOutput, error) {
			resp, err := client.ConvertFragmentToStory(ctx, input.FragmentID, grapery_client.ConvertFragmentToStoryRequest{
				Title:             input.Title,
				Description:       input.Description,
				Genre:             input.Genre,
				CoverImage:        input.CoverImage,
				SceneCount:        input.SceneCount,
				UseAI:             input.UseAI,
				CollaborationType: input.CollaborationType,
			})
			if err != nil {
				return nil, fmt.Errorf("convert to story: %w", err)
			}
			out := &ConvertToStoryOutput{FragmentID: resp.FragmentID}
			if resp.Story != nil {
				out.StoryID = resp.Story.ID
				out.StoryTitle = resp.Story.Title
			}
			return out, nil
		},
	)
}

func NewPrefillStoryTool(client *grapery_client.Client) (tool.InvokableTool, error) {
	return utils.InferTool(
		"prefill_story",
		"AI 生成碎片转故事的建议信息（标题、描述、角色、风格等），用于预填创建表单",
		func(ctx context.Context, input *PrefillStoryInput) (*PrefillStoryOutput, error) {
			resp, err := client.GetFragmentStoryPrefill(ctx, input.FragmentID, grapery_client.StoryPrefillAIRequest{
				SceneCount: input.SceneCount,
			})
			if err != nil {
				return nil, fmt.Errorf("prefill story: %w", err)
			}
			return &PrefillStoryOutput{
				Title:               resp.Title,
				Description:         resp.Description,
				Summary:             resp.Summary,
				Style:               resp.Style,
				Genre:               resp.Genre,
				Tags:                resp.Tags,
			}, nil
		},
	)
}

// AllTools 返回碎片生成的所有工具
func AllTools(client *grapery_client.Client) ([]tool.BaseTool, error) {
	var tools []tool.BaseTool

	add := func(t tool.InvokableTool, err error) error {
		if err != nil {
			return err
		}
		tools = append(tools, t)
		return nil
	}

	if err := add(NewExtractElementsTool(client)); err != nil {
		return nil, err
	}
	if err := add(NewPollTaskStatusTool(client)); err != nil {
		return nil, err
	}
	if err := add(NewCancelTaskTool(client)); err != nil {
		return nil, err
	}
	if err := add(NewEnhancePromptTool(client)); err != nil {
		return nil, err
	}
	if err := add(NewGenerateImageTool(client)); err != nil {
		return nil, err
	}
	if err := add(NewPollAITaskTool(client)); err != nil {
		return nil, err
	}
	if err := add(NewConvertToStoryTool(client)); err != nil {
		return nil, err
	}
	if err := add(NewPrefillStoryTool(client)); err != nil {
		return nil, err
	}
	return tools, nil
}

// ToolInfos 返回所有工具的信息摘要（用于 Agent Instruction）
func ToolInfos() string {
	return `可用工具：
1. extract_elements - 提取故事元素，启动碎片生成管线（异步）
2. poll_task_status - 查询碎片任务进度
3. cancel_task - 取消任务
4. enhance_prompt - 增强用户提示词
5. generate_image - 独立生成图片
6. poll_ai_task - 查询 AI 任务状态（enhance_prompt、generate_image 的结果）
	7. convert_to_story - 将碎片转换为完整故事
	8. prefill_story - AI 生成碎片转故事的建议信息`
}
