package character

import (
	"context"
	"fmt"

	"github.com/grapestree/fgrapery/grapery-agent/internal/characterutil"
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
				SourceType:                 input.SourceType,
				ReferenceImage:             input.ReferenceImage,
				SourceFragmentID:           input.SourceFragmentID,
				SourceFragmentCharacterKey: input.SourceFragmentCharacterKey,
				NeedsPortrait:              input.NeedsPortrait,
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

func NewGetFragmentCharacterSuggestionsTool(client *grapery_client.Client) (tool.InvokableTool, error) {
	return utils.InferTool(
		"get_fragment_character_suggestions",
		"列出故事来源碎片中可物化为故事角色的候选",
		func(ctx context.Context, input *FragmentSuggestionsInput) (*FragmentSuggestionsOutput, error) {
			resp, err := client.GetFragmentCharacterSuggestions(ctx, input.StoryID)
			if err != nil {
				return nil, fmt.Errorf("fragment character suggestions: %w", err)
			}
			out := &FragmentSuggestionsOutput{
				StoryID:    resp.StoryID,
				FragmentID: resp.FragmentID,
			}
			for _, s := range resp.Suggestions {
				out.Suggestions = append(out.Suggestions, FragmentSuggestionItem{
					Key:                 s.Key,
					Name:                s.Name,
					Role:                s.Role,
					Description:         s.Description,
					Appearance:          s.Appearance,
					Background:          s.Background,
					ReferenceImageURL:   s.ReferenceImageURL,
					AlreadyCreated:      s.AlreadyCreated,
					ExistingCharacterID: s.ExistingCharacterID,
				})
			}
			return out, nil
		},
	)
}

func NewStartCharacterGenTaskTool(client *grapery_client.Client) (tool.InvokableTool, error) {
	return utils.InferTool(
		"start_character_gen_task",
		"启动角色异步生成任务（属性→创建→肖像→三视图）",
		func(ctx context.Context, input *StartCharacterGenTaskInput) (*StartCharacterGenTaskOutput, error) {
			sourceType := characterutil.ResolveGenTaskSourceType(
				input.SourceType, input.SourceFragmentID, input.SourceFragmentCharacterKey, input.Prompt,
			)
			name := characterutil.ResolveCharacterName(input.Name, input.Prompt, sourceType)
			if name == "" && input.SuggestionName != "" {
				name = characterutil.ResolveCharacterName(input.SuggestionName, "", sourceType)
			}
			if characterutil.AsyncTaskNeedsName(name, input.Prompt, sourceType) {
				return nil, fmt.Errorf("character name is required (provide name, suggestion_name, or a descriptive prompt)")
			}
			var suggestion *grapery_client.FragmentSuggestion
			if input.SuggestionKey != "" || input.SuggestionName != "" {
				suggestion = &grapery_client.FragmentSuggestion{
					Key:         input.SuggestionKey,
					Name:        input.SuggestionName,
					Description: input.SuggestionDescription,
					Appearance:  input.SuggestionAppearance,
				}
			} else if sourceType == characterutil.GenSourceFragment && input.SourceFragmentCharacterKey != "" {
				suggestion = &grapery_client.FragmentSuggestion{Key: input.SourceFragmentCharacterKey, Name: name}
			}
			task, err := client.StartCharacterGenTask(ctx, grapery_client.CharacterGenTaskRequest{
				StoryID:                    input.StoryID,
				SourceType:                 sourceType,
				Name:                       name,
				Prompt:                     input.Prompt,
				Description:                input.Description,
				Background:                 input.Background,
				Personality:                input.Personality,
				ShortTermGoal:              input.ShortTermGoal,
				LongTermGoal:               input.LongTermGoal,
				HandlingStyle:              input.HandlingStyle,
				CognitionRange:             input.CognitionRange,
				AbilityFeatures:            input.AbilityFeatures,
				Appearance:                 input.Appearance,
				DressPreference:            input.DressPreference,
				ReferenceImage:             input.ReferenceImage,
				SourceFragmentID:           input.SourceFragmentID,
				SourceFragmentCharacterKey: input.SourceFragmentCharacterKey,
				Suggestion:                 suggestion,
			})
			if err != nil {
				return nil, fmt.Errorf("start character gen task: %w", err)
			}
			return &StartCharacterGenTaskOutput{
				TaskID:  task.ID,
				Status:  task.Status,
				StoryID: task.StoryID,
			}, nil
		},
	)
}

func NewPollCharacterGenTaskTool(client *grapery_client.Client) (tool.InvokableTool, error) {
	return utils.InferTool(
		"poll_character_gen_task",
		"查询角色异步生成任务状态",
		func(ctx context.Context, input *PollCharacterGenTaskInput) (*PollCharacterGenTaskOutput, error) {
			task, err := client.GetCharacterGenTask(ctx, input.TaskID)
			if err != nil {
				return nil, fmt.Errorf("poll character gen task: %w", err)
			}
			out := &PollCharacterGenTaskOutput{
				TaskID:       task.ID,
				Status:       task.Status,
				Progress:     task.Progress,
				CurrentStep:  task.CurrentStep,
				CharacterID:  task.CharacterID,
				ErrorMessage: task.ErrorMessage,
			}
			if task.Character != nil {
				out.CharacterName = task.Character.Name
				out.PortraitURL = task.Character.PortraitURL
			}
			return out, nil
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
	if err := add(NewGetFragmentCharacterSuggestionsTool(client)); err != nil {
		return nil, fmt.Errorf("create tool: %w", err)
	}
	if err := add(NewStartCharacterGenTaskTool(client)); err != nil {
		return nil, fmt.Errorf("create tool: %w", err)
	}
	if err := add(NewPollCharacterGenTaskTool(client)); err != nil {
		return nil, fmt.Errorf("create tool: %w", err)
	}
	return tools, nil
}

// ToolInfos 返回所有工具的信息摘要（用于 Agent Instruction）
func ToolInfos() string {
	return `可用工具：
1. generate_character_attrs - 根据提示词生成角色的完整属性（同步）
2. create_character - 在故事中创建角色记录
3. start_character_gen_task - 启动异步角色生成（推荐：含肖像+三视图）
4. poll_character_gen_task - 查询异步角色任务
5. get_fragment_character_suggestions - 从故事来源碎片列出角色候选
6. generate_portrait - 生成角色肖像图
7. generate_three_views - 生成角色三视图（views.sheet 单图）
8. generate_avatar - 生成角色头像
9. ask_user_feedback - 人机协作，征求用户意见`
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
	SourceType                 string `json:"source_type,omitempty"`
	ReferenceImage             string `json:"reference_image,omitempty"`
	SourceFragmentID           string `json:"source_fragment_id,omitempty"`
	SourceFragmentCharacterKey string `json:"source_fragment_character_key,omitempty"`
	NeedsPortrait              bool   `json:"needs_portrait,omitempty" jsonschema:"description=是否自动生成角色肖像"`
}

type FragmentSuggestionsInput struct {
	StoryID string `json:"story_id" jsonschema:"description=故事 ID,required"`
}

type FragmentSuggestionsOutput struct {
	StoryID      string                   `json:"story_id"`
	FragmentID   string                   `json:"fragment_id,omitempty"`
	Suggestions  []FragmentSuggestionItem `json:"suggestions,omitempty"`
}

type FragmentSuggestionItem struct {
	Key                 string `json:"key"`
	Name                string `json:"name"`
	Role                string `json:"role,omitempty"`
	Description         string `json:"description,omitempty"`
	Appearance          string `json:"appearance,omitempty"`
	Background          string `json:"background,omitempty"`
	ReferenceImageURL   string `json:"reference_image_url,omitempty"`
	AlreadyCreated      bool   `json:"already_created,omitempty"`
	ExistingCharacterID string `json:"existing_character_id,omitempty"`
}

type StartCharacterGenTaskInput struct {
	StoryID                    string `json:"story_id" jsonschema:"required"`
	SourceType                 string `json:"source_type,omitempty" jsonschema:"description=ai|fragment|manual_form"`
	Name                       string `json:"name,omitempty"`
	Prompt                     string `json:"prompt,omitempty"`
	Description                string `json:"description,omitempty"`
	Background                 string `json:"background,omitempty"`
	Personality                string `json:"personality,omitempty"`
	ShortTermGoal              string `json:"short_term_goal,omitempty"`
	LongTermGoal               string `json:"long_term_goal,omitempty"`
	HandlingStyle              string `json:"handling_style,omitempty"`
	CognitionRange             string `json:"cognition_range,omitempty"`
	AbilityFeatures            string `json:"ability_features,omitempty"`
	Appearance                 string `json:"appearance,omitempty"`
	DressPreference            string `json:"dress_preference,omitempty"`
	ReferenceImage             string `json:"reference_image,omitempty"`
	SourceFragmentID           string `json:"source_fragment_id,omitempty"`
	SourceFragmentCharacterKey string `json:"source_fragment_character_key,omitempty"`
	SuggestionKey              string `json:"suggestion_key,omitempty"`
	SuggestionName             string `json:"suggestion_name,omitempty"`
	SuggestionDescription      string `json:"suggestion_description,omitempty"`
	SuggestionAppearance       string `json:"suggestion_appearance,omitempty"`
}

type StartCharacterGenTaskOutput struct {
	TaskID  string `json:"task_id"`
	Status  string `json:"status"`
	StoryID string `json:"story_id"`
}

type PollCharacterGenTaskInput struct {
	TaskID string `json:"task_id" jsonschema:"required"`
}

type PollCharacterGenTaskOutput struct {
	TaskID        string `json:"task_id"`
	Status        string `json:"status"`
	Progress      int    `json:"progress"`
	CurrentStep   string `json:"current_step,omitempty"`
	CharacterID   string `json:"character_id,omitempty"`
	CharacterName string `json:"character_name,omitempty"`
	PortraitURL   string `json:"portrait_url,omitempty"`
	ErrorMessage  string `json:"error_message,omitempty"`
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
