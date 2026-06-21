package prompt

// FragmentCreatorInstruction is the Eino agent system instruction for FragmentCreator.
// LLM generation prompts execute in grapery when tools call the fragment API; see Catalog().
func FragmentCreatorInstruction(toolInfos string) string {
	return FragmentIntro + "\n\n" + toolInfos + "\n" + FragmentChatPlanningProtocol + "\n" + FragmentDomainKnowledge
}

// CharacterDesignerInstruction is the Eino agent system instruction for CharacterDesigner.
func CharacterDesignerInstruction(toolInfos string) string {
	return CharacterIntro + "\n\n" + toolInfos + "\n" + CharacterDomainKnowledge
}

// StoryboardDirectorInstruction is the Eino agent system instruction for StoryboardDirector.
func StoryboardDirectorInstruction(toolInfos string) string {
	return StoryboardIntro + "\n\n" + toolInfos + "\n" + StoryboardDomainKnowledge
}

// BranchExplorerInstruction is the Eino agent system instruction for BranchExplorer.
func BranchExplorerInstruction(toolInfos string) string {
	return BranchIntro + "\n\n" + toolInfos + "\n" + BranchDomainKnowledge
}

// FragmentPanelCreatorInstruction is the Eino agent system instruction for FragmentPanelCreator.
func FragmentPanelCreatorInstruction(toolInfos string) string {
	return FragmentPanelIntro + "\n\n" + toolInfos + "\n" + FragmentChatPlanningProtocol + "\n" + FragmentPanelDomainKnowledge
}

const FragmentChatPlanningProtocol = `
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Voyager 对话式创建协议
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

当你处于「询问灵感、澄清需求、总结创作方案」阶段时，先用自然语言简短回复用户，再在回复末尾附加一个机器可读 JSON 块。JSON 块必须用以下标记包裹，客户端会解析为 structured 字段，不会直接展示：

[[voyager_structured]]
{
  "type": "planning",
  "needsMoreInfo": true,
  "question": "还需要用户回答的问题，若不需要则为空字符串",
  "options": [
    { "id": "style", "label": "画风", "value": "fantasy" }
  ],
  "storyElements": [
    {
      "key": "prop_self_propelled_cannon",
      "label": "自走炮",
      "kind": "prop",
      "required": false,
      "inputType": "image",
      "helperText": "可上传自走炮参考图，让造型更稳定"
    }
  ],
  "generationIntent": {
    "agent": "fragment",
    "userInput": "用于正式生成的完整中文创作描述",
    "imageCount": 4,
    "style": "fantasy",
    "mood": "mysterious",
    "length": "medium",
    "language": "zh-Hans",
    "visibility": "private",
    "aspectRatio": "9:16",
    "topic": ""
  }
}
[[/voyager_structured]]

规则：
- 不要把真实生成进度、taskId、draftFragmentId、图片 URL 编造成文本；这些只能来自工具结果或 generation run。
- 如果用户信息足够生成，将 needsMoreInfo 设为 false，并给出完整 generationIntent。
- 如果用户只是在修改已完成结果，输出 type="revision" 并在 generationIntent 中写清用于重新生成的完整描述。
- 参考图多格漫画应把 generationIntent.agent 设为 "fragment-panel"；普通文字碎片设为 "fragment"。
- 第一阶段必须尽量给出 storyElements，用于客户端展示参考图输入槽位。每个元素必须有稳定 key、用户可读 label、kind（character/prop/location/scene/object）、required、inputType=image。若用户描述中出现关键道具、角色或地点，例如“自走炮 / 炮弹 / 对方阵地”，分别作为独立槽位输出。
- storyElements 是“可选参考图槽位”，不是生成结果；不要编造 imageUrl。客户端上传后会在 generation 请求中通过 referenceSlots 回传。
`
