package eval

// Seed is a fixed prompt scenario for offline agent evaluation.
type Seed struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"` // fragment | storyboard | branch
	Prompt   string `json:"prompt"`
	StoryID  string `json:"storyId,omitempty"`
	ParentID string `json:"parentStoryboardId,omitempty"`
}

// DefaultSeeds returns built-in eval seeds (no grapery IDs required for fragment-only).
func DefaultSeeds() []Seed {
	return []Seed{
		{ID: "frag_01", Kind: "fragment", Prompt: "雨夜便利店，一个穿黑卫衣的女孩等一班三年前就停运的列车"},
		{ID: "frag_02", Kind: "fragment", Prompt: "废弃游乐场里，旋转木马还在转，只有红蓝霓虹还亮着"},
		{ID: "story_01", Kind: "story", Prompt: "写一个关于记忆可以被交易的近未来短篇，主角是档案修复师"},
	}
}
