package http

import (
	"testing"

	"github.com/grapestree/fgrapery/grapery-agent/internal/agentauth"
)

func TestExpectedScopeForPath(t *testing.T) {
	agent, op, ok := expectedScopeForPath("POST", "/api/v1/generation/fragment-panels/stream")
	if !ok || agent != "fragment-panel" || op != "generate" {
		t.Fatalf("got %q %q %v", agent, op, ok)
	}
	agent, op, ok = expectedScopeForPath("POST", "/api/v1/generation/fragments/stream")
	if !ok || agent != "fragment" || op != "generate" {
		t.Fatalf("fragments stream scope got %q %q %v", agent, op, ok)
	}
	agent, op, ok = expectedScopeForPath("POST", "/api/v1/agent/fragment-panel/chat")
	if !ok || agent != "fragment-panel" || op != "chat" {
		t.Fatalf("chat scope got %q %q %v", agent, op, ok)
	}
	agent, op, ok = expectedScopeForPath("POST", "/api/v1/agent/fragment/chat/sync")
	if !ok || agent != "fragment" || op != "chat" {
		t.Fatalf("sync chat scope got %q %q %v", agent, op, ok)
	}
	agent, op, ok = expectedScopeForPath("POST", "/api/v1/agent/fragment/chat/cp_test/resume/sync")
	if !ok || agent != "fragment" || op != "chat" {
		t.Fatalf("sync resume scope got %q %q %v", agent, op, ok)
	}
	agent, op, ok = expectedScopeForPath("POST", "/api/v1/agent/creation/sessions/cs_1/messages/stream")
	if !ok || agent != "" || op != "generate" {
		t.Fatalf("creation scope got %q %q %v", agent, op, ok)
	}
}

func TestScopeMatches(t *testing.T) {
	claims := &agentauth.Claims{Scope: "agent:fragment-panel:generate"}
	if !scopeMatches(claims, "fragment-panel", "generate") {
		t.Fatal("scope should match")
	}
	if scopeMatches(claims, "fragment-panel", "chat") {
		t.Fatal("scope should not match chat")
	}
	legacy := &agentauth.Claims{Agent: "fragment-panel", Operation: "chat"}
	if !scopeMatches(legacy, "fragment-panel", "chat") {
		t.Fatal("legacy agent/op should match")
	}
	storyboard := &agentauth.Claims{Scope: "agent:storyboard:generate"}
	if !scopeMatches(storyboard, "", "generate") {
		t.Fatal("shared creation endpoint should accept storyboard generate operation")
	}
	if scopeMatches(storyboard, "", "chat") {
		t.Fatal("shared creation endpoint must still enforce operation")
	}
}
