package character

import (
	"context"
	"fmt"

	"github.com/grapestree/fgrapery/grapery-agent/internal/grapery_client"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

func NewGenerateAttrsTool(client *grapery_client.Client) (tool.InvokableTool, error) {
	return utils.InferTool(
		"generate_character_attrs",
		"使用 AI 根据提示词生成角色的完整属性（性格、背景、外貌、能力等）",
		func(ctx context.Context, input *GenerateAttrsInput) (*GenerateAttrsOutput, error) {
			resp, err := client.GenerateCharacterAttrs(ctx, grapery_client.GenerateCharacterAttrsRequest{
				Prompt: input.Prompt,
				Name:   input.Name,
			})
			if err != nil {
				return nil, fmt.Errorf("generate character attrs: %w", err)
			}
			return &GenerateAttrsOutput{
				Description:     resp.Description,
				Personality:     resp.Personality,
				Background:      resp.Background,
				ShortTermGoal:   resp.ShortTermGoal,
				LongTermGoal:    resp.LongTermGoal,
				HandlingStyle:   resp.HandlingStyle,
				CognitionRange:  resp.CognitionRange,
				AbilityFeatures: resp.AbilityFeatures,
				Appearance:      resp.Appearance,
				DressPreference: resp.DressPreference,
			}, nil
		},
	)
}

func NewCreateCharacterTool(client *grapery_client.Client) (tool.InvokableTool, error) {
	return utils.InferTool(
		"create_character",
		"在指定故事中创建一个角色记录",
		func(ctx context.Context, input *CreateCharacterInput) (*CreateCharacterOutput, error) {
			resp, err := client.CreateCharacter(ctx, grapery_client.CreateCharacterRequest{
				Name:            input.Name,
				Description:     input.Description,
				Personality:     input.Personality,
				Background:      input.Background,
				ShortTermGoal:   input.ShortTermGoal,
				LongTermGoal:    input.LongTermGoal,
				HandlingStyle:   input.HandlingStyle,
				CognitionRange:  input.CognitionRange,
				AbilityFeatures: input.AbilityFeatures,
				Appearance:      input.Appearance,
				DressPreference: input.DressPreference,
				StoryID:         input.StoryID,
				IsPublic:        input.IsPublic,
				SourceType:      input.SourceType,
				ReferenceImage:  input.ReferenceImage,
				NeedsPortrait:   input.NeedsPortrait,
			})
			if err != nil {
				return nil, fmt.Errorf("create character: %w", err)
			}
			return &CreateCharacterOutput{ID: resp.ID, Name: resp.Name}, nil
		},
	)
}

func NewGeneratePortraitTool(client *grapery_client.Client) (tool.InvokableTool, error) {
	return utils.InferTool(
		"generate_portrait",
		"为指定角色生成全身肖像图",
		func(ctx context.Context, input *GeneratePortraitInput) (*GeneratePortraitOutput, error) {
			resp, err := client.GenerateCharacterPortrait(ctx, input.CharacterID, grapery_client.GeneratePortraitRequest{
				CustomPrompt:   input.CustomPrompt,
				ReferenceImage: input.ReferenceImage,
				AspectRatio:    input.AspectRatio,
			})
			if err != nil {
				return nil, fmt.Errorf("generate portrait: %w", err)
			}
			return &GeneratePortraitOutput{PortraitURL: resp.PortraitURL, RecordID: resp.RecordID}, nil
		},
	)
}

func NewGenerateThreeViewsTool(client *grapery_client.Client) (tool.InvokableTool, error) {
	return utils.InferTool(
		"generate_three_views",
		"为指定角色生成三视图（正面/侧面/背面）合成图",
		func(ctx context.Context, input *GenerateThreeViewsInput) (*GenerateThreeViewsOutput, error) {
			resp, err := client.GenerateCharacterThreeViews(ctx, input.CharacterID, grapery_client.GenerateThreeViewsRequest{
				RegenerateAll:  input.RegenerateAll,
				ReferenceImage: input.ReferenceImage,
			})
			if err != nil {
				return nil, fmt.Errorf("generate three views: %w", err)
			}
			return &GenerateThreeViewsOutput{SheetURL: resp.Views.Sheet}, nil
		},
	)
}

func NewGenerateAvatarTool(client *grapery_client.Client) (tool.InvokableTool, error) {
	return utils.InferTool(
		"generate_avatar",
		"为指定角色生成头像",
		func(ctx context.Context, input *GenerateAvatarInput) (*GenerateAvatarOutput, error) {
			resp, err := client.GenerateCharacterAvatar(ctx, input.CharacterID, grapery_client.GenerateAvatarRequest{
				AspectRatio: input.AspectRatio,
			})
			if err != nil {
				return nil, fmt.Errorf("generate avatar: %w", err)
			}
			return &GenerateAvatarOutput{AvatarURL: resp.AvatarURL, RecordID: resp.RecordID}, nil
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

	if err := add(NewGenerateAttrsTool(client)); err != nil {
		return nil, fmt.Errorf("create tool: %w", err)
	}
	if err := add(NewCreateCharacterTool(client)); err != nil {
		return nil, fmt.Errorf("create tool: %w", err)
	}
	if err := add(NewGeneratePortraitTool(client)); err != nil {
		return nil, fmt.Errorf("create tool: %w", err)
	}
	if err := add(NewGenerateThreeViewsTool(client)); err != nil {
		return nil, fmt.Errorf("create tool: %w", err)
	}
	if err := add(NewGenerateAvatarTool(client)); err != nil {
		return nil, fmt.Errorf("create tool: %w", err)
	}
	return tools, nil
}

// ToolInfos 返回所有工具的信息摘要（用于 Agent Instruction）
func ToolInfos() string {
	return `可用工具：
1. generate_character_attrs - 根据提示词生成角色的完整属性
2. create_character - 在故事中创建角色记录
3. generate_portrait - 生成角色肖像图
4. generate_three_views - 生成角色三视图
5. generate_avatar - 生成角色头像
6. ask_user_feedback - 人机协作，征求用户意见`
}

// ============ 类型定义 ============

type GenerateAttrsInput struct {
	Prompt string `json:"prompt" jsonschema:"description=角色生成提示词,required"`
	Name   string `json:"name,omitempty" jsonschema:"description=角色名称"`
}

type GenerateAttrsOutput struct {
	Description     string `json:"description"`
	Personality     string `json:"personality"`
	Background      string `json:"background"`
	ShortTermGoal   string `json:"short_term_goal"`
	LongTermGoal    string `json:"long_term_goal"`
	HandlingStyle   string `json:"handling_style"`
	CognitionRange  string `json:"cognition_range"`
	AbilityFeatures string `json:"ability_features"`
	Appearance      string `json:"appearance"`
	DressPreference string `json:"dress_preference"`
}

type CreateCharacterInput struct {
	Name            string `json:"name" jsonschema:"description=角色名称,required"`
	StoryID         string `json:"story_id" jsonschema:"description=所属故事 ID,required"`
	Description     string `json:"description,omitempty"`
	Personality     string `json:"personality,omitempty"`
	Background      string `json:"background,omitempty"`
	ShortTermGoal   string `json:"short_term_goal,omitempty"`
	LongTermGoal    string `json:"long_term_goal,omitempty"`
	HandlingStyle   string `json:"handling_style,omitempty"`
	CognitionRange  string `json:"cognition_range,omitempty"`
	AbilityFeatures string `json:"ability_features,omitempty"`
	Appearance      string `json:"appearance,omitempty"`
	DressPreference string `json:"dress_preference,omitempty"`
	IsPublic        bool   `json:"is_public,omitempty"`
	SourceType      string `json:"source_type,omitempty"`
	ReferenceImage  string `json:"reference_image,omitempty"`
	NeedsPortrait   bool   `json:"needs_portrait,omitempty" jsonschema:"description=是否自动生成角色肖像"`
}

type CreateCharacterOutput struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type GeneratePortraitInput struct {
	CharacterID    string `json:"character_id" jsonschema:"description=角色 ID,required"`
	CustomPrompt   string `json:"custom_prompt,omitempty" jsonschema:"description=自定义肖像提示词"`
	ReferenceImage string `json:"reference_image,omitempty" jsonschema:"description=参考图 URL"`
	AspectRatio    string `json:"aspect_ratio,omitempty" jsonschema:"description=图片比例,2:3/3:4"`
}

type GeneratePortraitOutput struct {
	PortraitURL string `json:"portrait_url"`
	RecordID    string `json:"record_id"`
}

type GenerateThreeViewsInput struct {
	CharacterID    string `json:"character_id" jsonschema:"description=角色 ID,required"`
	RegenerateAll  bool   `json:"regenerate_all,omitempty"`
	ReferenceImage string `json:"reference_image,omitempty"`
}

type GenerateThreeViewsOutput struct {
	SheetURL string `json:"sheet_url"`
}

type GenerateAvatarInput struct {
	CharacterID string `json:"character_id" jsonschema:"description=角色 ID,required"`
	AspectRatio string `json:"aspect_ratio,omitempty" jsonschema:"description=比例,1:1/16:9/9:16"`
}

type GenerateAvatarOutput struct {
	AvatarURL string `json:"avatar_url"`
	RecordID  string `json:"record_id"`
}
