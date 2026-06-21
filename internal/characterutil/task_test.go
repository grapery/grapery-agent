package characterutil

import "testing"

func TestResolveGenTaskSourceType(t *testing.T) {
	if got := ResolveGenTaskSourceType("", "frag1", "char_main", ""); got != GenSourceFragment {
		t.Fatalf("want fragment, got %q", got)
	}
	if got := ResolveGenTaskSourceType("", "", "", "prompt"); got != GenSourceManualPrompt {
		t.Fatalf("want manual_prompt, got %q", got)
	}
	if got := ResolveGenTaskSourceType("", "", "", ""); got != GenSourceManualForm {
		t.Fatalf("want manual_form, got %q", got)
	}
}

func TestResolveCharacterName(t *testing.T) {
	if got := ResolveCharacterName("", "雨夜便利店女孩", GenSourceManualPrompt); got == "" {
		t.Fatal("expected derived name from prompt")
	}
	if got := ResolveCharacterName("", "", GenSourceFragment); got != "" {
		t.Fatalf("fragment without name should be empty, got %q", got)
	}
}
