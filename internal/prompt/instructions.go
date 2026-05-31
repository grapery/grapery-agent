package prompt

// FragmentCreatorInstruction is the Eino agent system instruction for FragmentCreator.
// LLM generation prompts execute in grapery when tools call the fragment API; see Catalog().
func FragmentCreatorInstruction(toolInfos string) string {
	return FragmentIntro + "\n\n" + toolInfos + "\n" + FragmentDomainKnowledge
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
