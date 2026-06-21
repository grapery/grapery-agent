package characterutil

import "strings"

const (
	GenSourceFragment    = "fragment"
	GenSourceManualPrompt = "manual_prompt"
	GenSourceManualForm  = "manual_form"
)

// ResolveGenTaskSourceType maps inputs to grapery character-generation-tasks sourceType.
func ResolveGenTaskSourceType(explicit, sourceFragmentID, sourceFragmentCharacterKey, prompt string) string {
	if s := strings.TrimSpace(explicit); s != "" {
		return s
	}
	if strings.TrimSpace(sourceFragmentID) != "" || strings.TrimSpace(sourceFragmentCharacterKey) != "" {
		return GenSourceFragment
	}
	if strings.TrimSpace(prompt) != "" {
		return GenSourceManualPrompt
	}
	return GenSourceManualForm
}

// ResolveCharacterName returns a non-empty name for task validation, or empty if fragment path may supply it later.
func ResolveCharacterName(name, prompt string, sourceType string) string {
	name = strings.TrimSpace(name)
	if name != "" && !isPlaceholderName(name) {
		return name
	}
	prompt = strings.TrimSpace(prompt)
	if prompt != "" {
		runes := []rune(prompt)
		if len(runes) > 12 {
			runes = runes[:12]
		}
		return string(runes)
	}
	if sourceType == GenSourceFragment {
		return ""
	}
	return ""
}

func isPlaceholderName(name string) bool {
	switch strings.TrimSpace(name) {
	case "", "碎片角色", "角色", "未命名角色":
		return true
	default:
		return false
	}
}

// AsyncTaskNeedsName reports whether StartCharacterGenTask requires an explicit or derived name upfront.
func AsyncTaskNeedsName(name, prompt, sourceType string) bool {
	if ResolveCharacterName(name, prompt, sourceType) != "" {
		return false
	}
	return sourceType != GenSourceFragment
}
