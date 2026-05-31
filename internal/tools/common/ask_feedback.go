package common

import (
	"context"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

// NewAskUserFeedbackTool 创建人机协作工具
// 当 Agent 需要向用户确认方向或收集反馈时使用
// 使用 Eino 的 tool.Interrupt/tool.GetResumeContext 机制
func NewAskUserFeedbackTool() (tool.InvokableTool, error) {
	return utils.InferOptionableTool(
		"ask_user_feedback",
		"暂停当前生成流程，向用户展示进度并征求反馈。用于：需要确认创作方向、选择风格偏好、确认角色设计等场景。",
		func(ctx context.Context, input *AskFeedbackInput, opts ...tool.Option) (string, error) {
			// 检查是否是 resume 流程，且携带了用户回复数据
			if isResume, hasData, data := tool.GetResumeContext[string](ctx); isResume && hasData {
				return data, nil
			}

			// 首次调用：触发 Interrupt，暂停等待用户输入
			return "", tool.Interrupt(ctx, input.Question)
		},
	)
}

type AskFeedbackInput struct {
	Question string `json:"question" jsonschema:"description=向用户提出的问题,required"`
	Context  string `json:"context,omitempty" jsonschema:"description=当前进度的简要说明"`
	Options  string `json:"options,omitempty" jsonschema:"description=可选的建议选项"`
}
