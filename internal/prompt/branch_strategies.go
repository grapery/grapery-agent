package prompt

import "strings"

// DefaultBranchStrategies are narrative variation axes for batch branch exploration.
// These are agent-layer strategy labels; grapery narrator continuation uses its own
// full-context prompt in narrator_pipeline.go — pass SeedPrompt as user continuation context.
var DefaultBranchStrategies = []string{
	"hopeful_turn",
	"darker_twist",
	"comedic_detour",
	"mystery_reveal",
}

// BuildBranchRawInput prefixes grapery continue_storyboard raw_input with a strategy hint.
func BuildBranchRawInput(seed, strategy string) string {
	seed = strings.TrimSpace(seed)
	prefix := StrategyPromptPrefix(strategy)
	if seed == "" {
		return prefix
	}
	return prefix + "\n\n延续前提：\n" + seed
}

// StrategyPromptPrefix returns Chinese strategy guidance for branch raw_input.
func StrategyPromptPrefix(strategy string) string {
	switch strategy {
	case "hopeful_turn":
		return "续写方向：在保持角色一致的前提下，让局势出现意外转机，情绪从压抑转向微弱希望。"
	case "darker_twist":
		return "续写方向：引入一个更黑暗、更不可逆的转折，但转折必须来自前文已埋伏的细节。"
	case "comedic_detour":
		return "续写方向：用荒诞但合理的喜剧插曲打破紧张感，不改变主线因果。"
	case "mystery_reveal":
		return "续写方向：揭露一个隐藏信息，让读者产生「原来如此」的闭合感。"
	default:
		return "续写方向：" + strategy
	}
}

// StrategyNarrativeHook is a short label for branch candidate metadata.
func StrategyNarrativeHook(strategy string) string {
	switch strategy {
	case "hopeful_turn":
		return "微弱希望点亮僵局"
	case "darker_twist":
		return "不可逆的黑暗转折"
	case "comedic_detour":
		return "荒诞插曲打破节奏"
	case "mystery_reveal":
		return "隐藏信息被揭开"
	default:
		return strategy
	}
}

// StrategyDiff returns a machine-readable variation axis tag.
func StrategyDiff(strategy string) string {
	return "variation_axis=" + strategy
}

// StrategyAppeal returns an expected community appeal label (metadata only, not LLM-scored).
func StrategyAppeal(strategy string) string {
	switch strategy {
	case "hopeful_turn":
		return "治愈/共鸣向"
	case "darker_twist":
		return "悬疑/冲击向"
	case "comedic_detour":
		return "轻松/传播向"
	case "mystery_reveal":
		return "烧脑/讨论向"
	default:
		return "general"
	}
}
