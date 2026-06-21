package agents

import (
	"context"
	"fmt"

	"github.com/grapestree/fgrapery/grapery-agent/internal/grapery_client"
	graperymodel "github.com/grapestree/fgrapery/grapery-agent/internal/model"
	"github.com/grapestree/fgrapery/grapery-agent/internal/prompt"
	"github.com/grapestree/fgrapery/grapery-agent/internal/tools/branch"
	"github.com/grapestree/fgrapery/grapery-agent/internal/tools/character"
	"github.com/grapestree/fgrapery/grapery-agent/internal/tools/common"
	"github.com/grapestree/fgrapery/grapery-agent/internal/tools/fragment"
	"github.com/grapestree/fgrapery/grapery-agent/internal/tools/fragment_panel"
	"github.com/grapestree/fgrapery/grapery-agent/internal/tools/storyboard"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
)

// AgentRegistry 管理所有 Agent 实例
type AgentRegistry struct {
	FragmentAgent      *adk.ChatModelAgent
	FragmentPanelAgent *adk.ChatModelAgent
	CharacterAgent     *adk.ChatModelAgent
	StoryboardAgent    *adk.ChatModelAgent
	BranchExplorer     *adk.ChatModelAgent
}

// NewRegistry 创建所有 Agent
func NewRegistry(ctx context.Context, chatModel model.BaseChatModel, textModel *graperymodel.HuoshanTextModel, imageModel *graperymodel.HuoshanImageModel, videoModel *graperymodel.HuoshanVideoModel, client *grapery_client.Client, maxIterations int) (*AgentRegistry, error) {
	fragmentAgent, err := newFragmentAgent(ctx, chatModel, client, maxIterations)
	if err != nil {
		return nil, fmt.Errorf("create fragment agent: %w", err)
	}

	fragmentPanelAgent, err := newFragmentPanelAgent(ctx, chatModel, client, maxIterations)
	if err != nil {
		return nil, fmt.Errorf("create fragment panel agent: %w", err)
	}

	characterAgent, err := newCharacterAgent(ctx, chatModel, client, maxIterations)
	if err != nil {
		return nil, fmt.Errorf("create character agent: %w", err)
	}

	storyboardAgent, err := newStoryboardAgent(ctx, chatModel, textModel, imageModel, videoModel, client, maxIterations)
	if err != nil {
		return nil, fmt.Errorf("create storyboard agent: %w", err)
	}

	branchAgent, err := newBranchExplorerAgent(ctx, chatModel, client, maxIterations)
	if err != nil {
		return nil, fmt.Errorf("create branch explorer agent: %w", err)
	}

	return &AgentRegistry{
		FragmentAgent:      fragmentAgent,
		FragmentPanelAgent: fragmentPanelAgent,
		CharacterAgent:     characterAgent,
		StoryboardAgent:    storyboardAgent,
		BranchExplorer:     branchAgent,
	}, nil
}

func newFragmentAgent(ctx context.Context, chatModel model.BaseChatModel, client *grapery_client.Client, maxIter int) (*adk.ChatModelAgent, error) {
	tools, err := fragment.AllTools(client)
	if err != nil {
		return nil, fmt.Errorf("create fragment tools: %w", err)
	}
	fbTool, err := common.NewAskUserFeedbackTool()
	if err != nil {
		return nil, fmt.Errorf("create feedback tool: %w", err)
	}
	tools = append(tools, fbTool)

	return adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:          "FragmentCreator",
		Description:   "故事碎片创作 Agent，负责从用户输入生成完整的图文故事碎片，支持 AI 自主决策和人机协作",
		Instruction:   prompt.FragmentCreatorInstruction(fragment.ToolInfos()),
		Model:         chatModel,
		ToolsConfig:   adk.ToolsConfig{ToolsNodeConfig: compose.ToolsNodeConfig{Tools: tools}},
		MaxIterations: maxIter,
	})
}

func newFragmentPanelAgent(ctx context.Context, chatModel model.BaseChatModel, client *grapery_client.Client, maxIter int) (*adk.ChatModelAgent, error) {
	tools, err := fragment_panel.AllTools(client)
	if err != nil {
		return nil, fmt.Errorf("create fragment panel tools: %w", err)
	}
	fbTool, err := common.NewAskUserFeedbackTool()
	if err != nil {
		return nil, fmt.Errorf("create feedback tool: %w", err)
	}
	tools = append(tools, fbTool)

	return adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:          "FragmentPanelCreator",
		Description:   "参考图多面板故事碎片 Agent，从单张参考图生成多格漫画式碎片",
		Instruction:   prompt.FragmentPanelCreatorInstruction(fragment_panel.ToolInfos()),
		Model:         chatModel,
		ToolsConfig:   adk.ToolsConfig{ToolsNodeConfig: compose.ToolsNodeConfig{Tools: tools}},
		MaxIterations: maxIter,
	})
}

func newCharacterAgent(ctx context.Context, chatModel model.BaseChatModel, client *grapery_client.Client, maxIter int) (*adk.ChatModelAgent, error) {
	tools, err := character.AllTools(client)
	if err != nil {
		return nil, fmt.Errorf("create character tools: %w", err)
	}
	fbTool, err := common.NewAskUserFeedbackTool()
	if err != nil {
		return nil, fmt.Errorf("create feedback tool: %w", err)
	}
	tools = append(tools, fbTool)

	return adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:          "CharacterDesigner",
		Description:   "角色设计 Agent，负责 AI 角色属性生成、创建角色、生成肖像和三视图",
		Instruction:   prompt.CharacterDesignerInstruction(character.ToolInfos()),
		Model:         chatModel,
		ToolsConfig:   adk.ToolsConfig{ToolsNodeConfig: compose.ToolsNodeConfig{Tools: tools}},
		MaxIterations: maxIter,
	})
}

func newStoryboardAgent(ctx context.Context, chatModel model.BaseChatModel, textModel *graperymodel.HuoshanTextModel, imageModel *graperymodel.HuoshanImageModel, videoModel *graperymodel.HuoshanVideoModel, client *grapery_client.Client, maxIter int) (*adk.ChatModelAgent, error) {
	tools, err := storyboard.AllTools(client)
	if err != nil {
		return nil, fmt.Errorf("create storyboard tools: %w", err)
	}
	fbTool, err := common.NewAskUserFeedbackTool()
	if err != nil {
		return nil, fmt.Errorf("create feedback tool: %w", err)
	}
	tools = append(tools, fbTool)

	return adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:          "StoryboardDirector",
		Description:   "故事板导演 Agent，负责故事板的创建、内容生成、场景出图、视频生成和续写",
		Instruction:   prompt.StoryboardDirectorInstruction(storyboard.ToolInfos()),
		Model:         chatModel,
		ToolsConfig:   adk.ToolsConfig{ToolsNodeConfig: compose.ToolsNodeConfig{Tools: tools}},
		MaxIterations: maxIter,
	})
}

func newBranchExplorerAgent(ctx context.Context, chatModel model.BaseChatModel, client *grapery_client.Client, maxIter int) (*adk.ChatModelAgent, error) {
	tools, err := branch.AllTools(client)
	if err != nil {
		return nil, fmt.Errorf("create branch tools: %w", err)
	}
	fbTool, err := common.NewAskUserFeedbackTool()
	if err != nil {
		return nil, fmt.Errorf("create feedback tool: %w", err)
	}
	tools = append(tools, fbTool)

	return adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:          "BranchExplorer",
		Description:   "多分支探索 Agent，从同一父故事板生成多条平行宇宙候选，用于社区筛选与 RL 数据沉淀",
		Instruction:   prompt.BranchExplorerInstruction(branch.ToolInfos()),
		Model:         chatModel,
		ToolsConfig:   adk.ToolsConfig{ToolsNodeConfig: compose.ToolsNodeConfig{Tools: tools}},
		MaxIterations: maxIter,
	})
}
