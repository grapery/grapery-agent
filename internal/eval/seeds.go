package eval

// Seed is a fixed prompt scenario for offline agent evaluation.
type Seed struct {
	ID                string   `json:"id"`
	Kind              string   `json:"kind"` // fragment | fragment_panel | storyboard | branch
	ReferenceImageURL string   `json:"referenceImageUrl,omitempty"`
	ExpectedImages    int      `json:"expectedImages,omitempty"`
	ExpectedScenes    int      `json:"expectedScenes,omitempty"`
	Rubric            []string `json:"rubric,omitempty"`
	Prompt            string   `json:"prompt"`
	StoryID           string   `json:"storyId,omitempty"`
	ParentID          string   `json:"parentStoryboardId,omitempty"`
}

// DefaultSeeds returns built-in eval seeds (no grapery IDs required for fragment-only).
func DefaultSeeds() []Seed {
	return []Seed{
		{ID: "frag_01", Kind: "fragment", Prompt: "雨夜便利店，一个穿黑卫衣的女孩等一班三年前就停运的列车"},
		{ID: "frag_02", Kind: "fragment", Prompt: "废弃游乐场里，旋转木马还在转，只有红蓝霓虹还亮着"},
		{ID: "story_01", Kind: "story", Prompt: "写一个关于记忆可以被交易的近未来短篇，主角是档案修复师"},
		// fragment_panel seed requires a reachable reference image URL in target environment.
		{ID: "panel_01", Kind: "fragment_panel", Prompt: "从这张参考图展开三格雨夜叙事", ReferenceImageURL: ""},
	}
}

// CreationContractSeeds requires a test story owned by the caller. It is opt-in
// because executing this suite can create drafts and incur provider charges.
func CreationContractSeeds(storyID string) []Seed {
	return []Seed{
		{ID: "fragment_canon", Kind: "fragment", ExpectedImages: 1, Prompt: "雨夜便利店，戴圆框眼镜、穿黄色雨衣的档案修复师发现自己的旧照片。只画一页四格，前两格不出现照片，最后一格才揭示照片。", Rubric: []string{"眼镜与黄色雨衣跨格一致", "四格且阅读顺序明确", "照片仅在最后揭示", "对白无无意义填充"}},
		{ID: "storyboard_motivation", Kind: "storyboard", StoryID: storyID, ExpectedScenes: 3, Prompt: "写三场：修复师为保护妹妹隐瞒照片来源，第二场被同伴质疑，第三场付出代价。遵守故事世界与已有角色的能力和认知范围。", Rubric: []string{"目标驱动选择而非旁白重复设定", "三场因果递进", "知识边界没有凭空突破", "结尾包含选择及代价"}},
		{ID: "character_world", Kind: "character", StoryID: storyID, Prompt: "补全一个档案修复师，短期目标是找到失踪妹妹，长期目标是证明记忆交易黑市存在；保留已给出的目标，遵守故事世界的能力边界。", Rubric: []string{"十字段完整且不互相矛盾", "用户目标不被覆盖", "外貌可被绘制并有辨识度", "职业与世界规则一致"}},
	}
}
