package storyboard

import (
	"context"
	"fmt"

	"github.com/grapestree/fgrapery/grapery-agent/internal/grapery_client"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

func NewCreateStoryboardTool(client *grapery_client.Client) (tool.InvokableTool, error) {
	return utils.InferTool(
		"create_storyboard",
		"创建一个新的故事板",
		func(ctx context.Context, input *CreateStoryboardInput) (*CreateStoryboardOutput, error) {
			var charRefs []grapery_client.CharacterRef
			for _, cr := range input.CharacterRefs {
				charRefs = append(charRefs, grapery_client.CharacterRef{
					CharacterID: cr.CharacterID,
					Role:        cr.Role,
					Order:       cr.Order,
				})
			}

			var sceneRefs []grapery_client.SceneRef
			for _, sr := range input.SceneRefs {
				sceneRefs = append(sceneRefs, grapery_client.SceneRef{
					StorySceneID:   sr.StorySceneID,
					Sequence:       sr.Sequence,
					IsPrimaryScene: sr.IsPrimaryScene,
				})
			}

			resp, err := client.CreateStoryboard(ctx, grapery_client.CreateStoryboardRequest{
				StoryID:      input.StoryID,
				Title:        input.Title,
				RawInput:     input.RawInput,
				SceneCount:   input.SceneCount,
				CharacterRefs: charRefs,
				SceneRefs:           sceneRefs,
				Tags:                input.Tags,
						UseComicPagePipeline: input.UseComicPagePipeline,
			})
			if err != nil {
				return nil, fmt.Errorf("create storyboard: %w", err)
			}
			return &CreateStoryboardOutput{ID: resp.ID, StoryID: resp.StoryID}, nil
		},
	)
}

func NewGenerateContentTool(client *grapery_client.Client) (tool.InvokableTool, error) {
	return utils.InferTool(
		"generate_storyboard_content",
		"为故事板生成故事内容（第一步）",
		func(ctx context.Context, input *GenerateContentInput) (*GenerateContentOutput, error) {
			err := client.GenerateStoryboardContent(ctx, input.StoryboardID, grapery_client.GenerateStoryboardContentRequest{
				RawInput:     input.RawInput,
				CharacterIDs: input.CharacterIDs,
				SceneIDs:     input.SceneIDs,
				Style:        input.Style,
			})
			if err != nil {
				return nil, fmt.Errorf("generate content: %w", err)
			}
			return &GenerateContentOutput{Success: true}, nil
		},
	)
}

func NewGenerateSceneImageTool(client *grapery_client.Client) (tool.InvokableTool, error) {
	return utils.InferTool(
		"generate_scene_image",
		"为故事板的单个场景生成图片",
		func(ctx context.Context, input *GenerateSceneImageInput) (*GenerateSceneImageOutput, error) {
			err := client.GenerateSceneImage(ctx, input.StoryboardID, grapery_client.GenerateSceneImageRequest{
				SceneID:                  input.SceneID,
				SceneTitle:               input.SceneTitle,
				SceneDescription:         input.SceneDescription,
				ReferenceImages:          input.ReferenceImages,
				SceneCharacters:          input.SceneCharacters,
				CharacterReferenceImages: input.CharacterReferenceImages,
			})
			if err != nil {
				return nil, fmt.Errorf("generate scene image: %w", err)
			}
			return &GenerateSceneImageOutput{Success: true}, nil
		},
	)
}

func NewGenerateAllImagesTool(client *grapery_client.Client) (tool.InvokableTool, error) {
	return utils.InferTool(
		"generate_all_scene_images",
		"批量为故事板的所有场景生成图片",
		func(ctx context.Context, input *GenerateAllImagesInput) (*GenerateAllImagesOutput, error) {
			err := client.GenerateAllSceneImages(ctx, input.StoryboardID, grapery_client.GenerateAllImagesRequest{
				RegenerateAll: input.RegenerateAll,
			})
			if err != nil {
				return nil, fmt.Errorf("generate all images: %w", err)
			}
			return &GenerateAllImagesOutput{Success: true}, nil
		},
	)
}

func NewGenerateVideoTool(client *grapery_client.Client) (tool.InvokableTool, error) {
	return utils.InferTool(
		"generate_scene_video",
		"为故事板场景生成视频",
		func(ctx context.Context, input *GenerateVideoInput) (*GenerateVideoOutput, error) {
			err := client.GenerateSceneVideo(ctx, input.StoryboardID, grapery_client.StoryboardVideoRequest{
				SceneID:          input.SceneID,
				SceneTitle:       input.SceneTitle,
				InputDescription: input.InputDescription,
				ReferenceImageURL: input.ReferenceImageURL,
				EndFrameURL:      input.EndFrameURL,
			})
			if err != nil {
				return nil, fmt.Errorf("generate video: %w", err)
			}
			return &GenerateVideoOutput{Success: true}, nil
		},
	)
}

func NewContinueStoryboardTool(client *grapery_client.Client) (tool.InvokableTool, error) {
	return utils.InferTool(
		"continue_storyboard",
		"创建故事板的续写（平行宇宙分支）",
		func(ctx context.Context, input *ContinueInput) (*ContinueOutput, error) {
			resp, err := client.ContinueStoryboard(ctx, input.StoryboardID, grapery_client.ContinueStoryboardRequest{
				RawInput:      input.RawInput,
				SceneCount:    input.SceneCount,
				Characters:    input.Characters,
				GenerateVideo: input.GenerateVideo,
				ComicStyle:    input.ComicStyle,
			})
			if err != nil {
				return nil, fmt.Errorf("continue storyboard: %w", err)
			}
			if resp.NewStoryboard == nil {
					return nil, fmt.Errorf("continue storyboard: no storyboard in response")
				}
				return &ContinueOutput{ID: resp.NewStoryboard.ID, StoryID: resp.NewStoryboard.StoryID}, nil
		},
	)
}

func NewGenerateStructureTool(client *grapery_client.Client) (tool.InvokableTool, error) {
	return utils.InferTool(
		"regenerate_structure",
		"重新生成故事板的结构（视觉圣经+场景规划）。已有场景时同步返回结果；无场景时异步执行。",
		func(ctx context.Context, input *GenerateStructureInput) (*GenerateStructureOutput, error) {
			resp, err := client.GenerateStructure(ctx, input.StoryboardID)
			if err != nil {
				return nil, fmt.Errorf("regenerate structure: %w", err)
			}
			msg := "结构已同步生成完成"
			if resp.AsyncAccepted {
				msg = "结构重新生成已提交，正在后台执行，请稍后查询进度"
			}
			return &GenerateStructureOutput{
				AsyncAccepted: resp.AsyncAccepted,
				Message:       msg,
			}, nil
		},
	)
}


func NewGenerateComicPageTool(client *grapery_client.Client) (tool.InvokableTool, error) {
	return utils.InferTool(
		"generate_comic_page",
		"为单个场景生成漫画页（多格拼贴），包含对白气泡、拟声词等漫画文字层",
		func(ctx context.Context, input *GenerateComicPageInput) (*GenerateComicPageOutput, error) {
			err := client.GenerateComicPage(ctx, input.StoryboardID, grapery_client.GenerateComicPageRequest{
				SceneID:                  input.SceneID,
				SceneTitle:               input.SceneTitle,
				SceneDescription:         input.SceneDescription,
				ReferenceImages:          input.ReferenceImages,
				SceneCharacters:          input.SceneCharacters,
				CharacterReferenceImages: input.CharacterReferenceImages,
				LayoutPreset:             input.LayoutPreset,
				PanelCount:               input.PanelCount,
				PageAspectRatio:          input.PageAspectRatio,
				DialogueMode:             input.DialogueMode,
			})
			if err != nil {
				return nil, fmt.Errorf("generate comic page: %w", err)
			}
			return &GenerateComicPageOutput{Success: true}, nil
		},
	)
}

func NewGenerateAllComicPagesTool(client *grapery_client.Client) (tool.InvokableTool, error) {
	return utils.InferTool(
		"generate_all_comic_pages",
		"批量为故事板所有场景生成漫画页（多格拼贴），后端自动决定每场景的格数和布局",
		func(ctx context.Context, input *GenerateAllComicPagesInput) (*GenerateAllComicPagesOutput, error) {
			resp, err := client.GenerateAllComicPages(ctx, input.StoryboardID, grapery_client.GenerateAllComicPagesRequest{
				RegenerateAll:   input.RegenerateAll,
				LayoutPreset:    input.LayoutPreset,
				PanelCount:      input.PanelCount,
				PageAspectRatio: input.PageAspectRatio,
				DialogueMode:    input.DialogueMode,
			})
			if err != nil {
				return nil, fmt.Errorf("generate all comic pages: %w", err)
			}
			return &GenerateAllComicPagesOutput{
				Total:        resp.Total,
				SuccessCount: resp.SuccessCount,
				FailedCount:  resp.FailedCount,
			}, nil
		},
	)
}

func NewGetGenerationProgressTool(client *grapery_client.Client) (tool.InvokableTool, error) {
	return utils.InferTool(
		"get_generation_progress",
		"查询故事板的生成进度，包括当前步骤、是否正在生成、工作流状态等",
		func(ctx context.Context, input *GetGenerationProgressInput) (*GetGenerationProgressOutput, error) {
			resp, err := client.GetGenerationProgress(ctx, input.StoryboardID)
			if err != nil {
				return nil, fmt.Errorf("get generation progress: %w", err)
			}
			return &GetGenerationProgressOutput{
				WorkflowStatus:    resp.WorkflowStatus,
				CurrentStep:       resp.CurrentStep,
				IsGenerating:      resp.IsGenerating,
				GenerationMessage: resp.GenerationMessage,
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

	if err := add(NewCreateStoryboardTool(client)); err != nil {
		return nil, fmt.Errorf("create tool: %w", err)
	}
	if err := add(NewGenerateContentTool(client)); err != nil {
		return nil, fmt.Errorf("create tool: %w", err)
	}
	if err := add(NewGenerateSceneImageTool(client)); err != nil {
		return nil, fmt.Errorf("create tool: %w", err)
	}
	if err := add(NewGenerateAllImagesTool(client)); err != nil {
		return nil, fmt.Errorf("create tool: %w", err)
	}
	if err := add(NewGenerateVideoTool(client)); err != nil {
		return nil, fmt.Errorf("create tool: %w", err)
	}
	if err := add(NewContinueStoryboardTool(client)); err != nil {
		return nil, fmt.Errorf("create tool: %w", err)
	}
	if err := add(NewGenerateStructureTool(client)); err != nil {
		return nil, fmt.Errorf("create tool: %w", err)
	}
	if err := add(NewGenerateComicPageTool(client)); err != nil {
		return nil, fmt.Errorf("create tool: %w", err)
	}
	if err := add(NewGenerateAllComicPagesTool(client)); err != nil {
		return nil, fmt.Errorf("create tool: %w", err)
	}
	if err := add(NewGetGenerationProgressTool(client)); err != nil {
		return nil, fmt.Errorf("create tool: %w", err)
	}
	return tools, nil
}

// ToolInfos 返回所有工具的信息摘要（用于 Agent Instruction）
func ToolInfos() string {
	return `可用工具：
1. create_storyboard - 创建故事板（带 raw_input；后台自动执行 redesign 文本结构）
2. get_generation_progress - 轮询故事板文本结构生成进度（create 后的主路径）
3. regenerate_structure - 重新生成 Bible/场景结构（不满意时）
4. generate_storyboard_content - 单独重跑 content 步骤（次要；已有 storyboard 时用）
5. generate_scene_image - 为单个场景生成图片
6. generate_all_scene_images - 批量为所有场景生成图片
7. generate_comic_page / generate_all_comic_pages - 漫画页拼贴出图
8. generate_scene_video - 为场景生成视频
9. continue_storyboard - 创建续写分支（平行宇宙）
10. ask_user_feedback - 人机协作`
}

// ============ 类型定义 ============

type CharRef struct {
	CharacterID string `json:"character_id"`
	Role        string `json:"role,omitempty"`
	Order       int    `json:"order,omitempty"`
}

type SceneRefInput struct {
	StorySceneID   string `json:"story_scene_id" jsonschema:"description=故事场景 ID,required"`
	Sequence       *int   `json:"sequence,omitempty"`
	IsPrimaryScene bool   `json:"is_primary_scene,omitempty"`
}

type CreateStoryboardInput struct {
	StoryID              string         `json:"story_id" jsonschema:"description=所属故事 ID,required"`
	Title                string         `json:"title,omitempty" jsonschema:"description=故事板标题"`
	RawInput             string         `json:"raw_input" jsonschema:"description=用户原始输入,required"`
	SceneCount           int            `json:"scene_count,omitempty" jsonschema:"description=场景数量,2-8"`
	CharacterRefs        []CharRef      `json:"character_refs,omitempty" jsonschema:"description=关联角色"`
	SceneRefs            []SceneRefInput `json:"scene_refs,omitempty" jsonschema:"description=关联故事场景"`
	Tags                 []string       `json:"tags,omitempty" jsonschema:"description=标签"`
	UseComicPagePipeline bool           `json:"use_comic_page_pipeline,omitempty" jsonschema:"description=是否使用漫画页管线（多格拼贴出图）"`
}

type CreateStoryboardOutput struct {
	ID      string `json:"id"`
	StoryID string `json:"story_id"`
}

type GenerateContentInput struct {
	StoryboardID string   `json:"storyboard_id" jsonschema:"description=故事板 ID,required"`
	RawInput     string   `json:"raw_input" jsonschema:"description=内容描述,required"`
	CharacterIDs []string `json:"character_ids,omitempty" jsonschema:"description=关联角色 ID"`
	SceneIDs     []string `json:"scene_ids,omitempty" jsonschema:"description=关联场景 ID"`
	Style        string   `json:"style,omitempty" jsonschema:"description=风格"`
}

type GenerateContentOutput struct {
	Success bool `json:"success"`
}

type GenerateSceneImageInput struct {
	StoryboardID             string   `json:"storyboard_id" jsonschema:"description=故事板 ID,required"`
	SceneID                  string   `json:"scene_id" jsonschema:"description=场景 ID,required"`
	SceneTitle               string   `json:"scene_title,omitempty"`
	SceneDescription         string   `json:"scene_description" jsonschema:"description=场景描述,required"`
	ReferenceImages          []string `json:"reference_images,omitempty"`
	SceneCharacters          []string `json:"scene_characters,omitempty"`
	CharacterReferenceImages []string `json:"character_reference_images,omitempty"`
}

type GenerateSceneImageOutput struct {
	Success bool `json:"success"`
}

type GenerateAllImagesInput struct {
	StoryboardID  string `json:"storyboard_id" jsonschema:"description=故事板 ID,required"`
	RegenerateAll bool   `json:"regenerate_all,omitempty"`
}

type GenerateAllImagesOutput struct {
	Success bool `json:"success"`
}

type GenerateVideoInput struct {
	StoryboardID      string `json:"storyboard_id" jsonschema:"description=故事板 ID,required"`
	SceneID           string `json:"scene_id" jsonschema:"description=场景 ID,required"`
	SceneTitle        string `json:"scene_title,omitempty"`
	InputDescription  string `json:"input_description" jsonschema:"description=视频描述,required"`
	ReferenceImageURL string `json:"reference_image_url,omitempty"`
	EndFrameURL       string `json:"end_frame_url,omitempty"`
}

type GenerateVideoOutput struct {
	Success bool `json:"success"`
}

type ContinueInput struct {
	StoryboardID  string   `json:"storyboard_id" jsonschema:"description=父故事板 ID,required"`
	RawInput      string   `json:"raw_input" jsonschema:"description=续写方向描述,required"`
	SceneCount    int      `json:"scene_count,omitempty"`
	Characters    []string `json:"characters,omitempty"`
	GenerateVideo bool     `json:"generate_video,omitempty"`
	ComicStyle    string   `json:"comic_style,omitempty"`
}

type ContinueOutput struct {
	ID      string `json:"id"`
	StoryID string `json:"story_id"`
}

type GenerateStructureInput struct {
	StoryboardID string `json:"storyboard_id" jsonschema:"description=故事板 ID,required"`
}

type GenerateStructureOutput struct {
	AsyncAccepted bool   `json:"async_accepted"`
	Message       string `json:"message"`
}

type GenerateComicPageInput struct {
	StoryboardID             string   `json:"storyboard_id" jsonschema:"description=故事板 ID,required"`
	SceneID                  string   `json:"scene_id" jsonschema:"description=场景 ID,required"`
	SceneTitle               string   `json:"scene_title,omitempty"`
	SceneDescription         string   `json:"scene_description" jsonschema:"description=场景描述,required"`
	ReferenceImages          []string `json:"reference_images,omitempty"`
	SceneCharacters          []string `json:"scene_characters,omitempty"`
	CharacterReferenceImages []string `json:"character_reference_images,omitempty"`
	LayoutPreset             string   `json:"layout_preset,omitempty"`
	PanelCount               int      `json:"panel_count,omitempty"`
	PageAspectRatio          string   `json:"page_aspect_ratio,omitempty"`
	DialogueMode             string   `json:"dialogue_mode,omitempty"`
}

type GenerateComicPageOutput struct {
	Success bool `json:"success"`
}

type GenerateAllComicPagesInput struct {
	StoryboardID    string `json:"storyboard_id" jsonschema:"description=故事板 ID,required"`
	RegenerateAll   bool   `json:"regenerate_all,omitempty"`
	LayoutPreset    string `json:"layout_preset,omitempty"`
	PanelCount      int    `json:"panel_count,omitempty"`
	PageAspectRatio string `json:"page_aspect_ratio,omitempty"`
	DialogueMode    string `json:"dialogue_mode,omitempty"`
}

type GenerateAllComicPagesOutput struct {
	Total        int `json:"total"`
	SuccessCount int `json:"success_count"`
	FailedCount  int `json:"failed_count"`
}

type GetGenerationProgressInput struct {
	StoryboardID string `json:"storyboard_id" jsonschema:"description=故事板 ID,required"`
}

type GetGenerationProgressOutput struct {
	WorkflowStatus    string `json:"workflow_status"`
	CurrentStep       int    `json:"current_step"`
	IsGenerating      bool   `json:"is_generating"`
	GenerationMessage string `json:"generation_message"`
}
