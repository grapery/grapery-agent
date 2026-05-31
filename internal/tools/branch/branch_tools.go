package branch

import (
	"context"
	"fmt"

	"github.com/grapestree/fgrapery/grapery-agent/internal/grapery_client"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

func NewContinueBranchTool(client *grapery_client.Client) (tool.InvokableTool, error) {
	return utils.InferTool(
		"continue_storyboard_branch",
		"从父故事板创建一条平行宇宙续写分支，需指定续写方向 raw_input 和可选 strategy 标签",
		func(ctx context.Context, input *ContinueBranchInput) (*ContinueBranchOutput, error) {
			resp, err := client.ContinueStoryboard(ctx, input.ParentStoryboardID, grapery_client.ContinueStoryboardRequest{
				RawInput:   input.RawInput,
				SceneCount: input.SceneCount,
				Characters: input.Characters,
				ComicStyle: input.ComicStyle,
			})
			if err != nil {
				return nil, fmt.Errorf("continue branch: %w", err)
			}
			if resp.NewStoryboard == nil {
				return nil, fmt.Errorf("continue branch: empty storyboard")
			}
			return &ContinueBranchOutput{
				StoryboardID: resp.NewStoryboard.ID,
				StoryID:      resp.NewStoryboard.StoryID,
				Strategy:     input.Strategy,
				TokensUsed:   resp.TokensUsed,
			}, nil
		},
	)
}

func AllTools(client *grapery_client.Client) ([]tool.BaseTool, error) {
	t, err := NewContinueBranchTool(client)
	if err != nil {
		return nil, err
	}
	return []tool.BaseTool{t}, nil
}

func ToolInfos() string {
	return `可用工具：
1. continue_storyboard_branch - 创建平行宇宙续写分支
2. ask_user_feedback - 人机协作`
}

type ContinueBranchInput struct {
	ParentStoryboardID string   `json:"parent_storyboard_id" jsonschema:"description=父故事板 ID,required"`
	RawInput           string   `json:"raw_input" jsonschema:"description=续写方向描述,required"`
	Strategy           string   `json:"strategy,omitempty" jsonschema:"description=变体策略标签,hopeful_turn/darker_twist等"`
	SceneCount         int      `json:"scene_count,omitempty"`
	Characters         []string `json:"characters,omitempty"`
	ComicStyle         string   `json:"comic_style,omitempty"`
}

type ContinueBranchOutput struct {
	StoryboardID string `json:"storyboard_id"`
	StoryID      string `json:"story_id"`
	Strategy     string `json:"strategy,omitempty"`
	TokensUsed   int    `json:"tokens_used,omitempty"`
}
