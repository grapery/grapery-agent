package config

import (
	"os"
	"testing"
)

func TestEffectiveAgentTokenVerifyKey(t *testing.T) {
	t.Setenv("AGENT_TOKEN_VERIFY_KEY", "")
	t.Setenv("AGENT_TOKEN_SIGNING_KEY", "")
	if got := effectiveAgentTokenVerifyKey(); got != DefaultAgentTokenVerifyKey {
		t.Fatalf("default = %q", got)
	}

	t.Setenv("AGENT_TOKEN_SIGNING_KEY", "from-signing")
	if got := effectiveAgentTokenVerifyKey(); got != "from-signing" {
		t.Fatalf("signing fallback = %q", got)
	}

	t.Setenv("AGENT_TOKEN_VERIFY_KEY", "from-verify")
	if got := effectiveAgentTokenVerifyKey(); got != "from-verify" {
		t.Fatalf("verify wins = %q", got)
	}
}

func TestLoadUsesSigningKeyFallback(t *testing.T) {
	t.Setenv("AGENT_TOKEN_VERIFY_KEY", "")
	t.Setenv("AGENT_TOKEN_SIGNING_KEY", "shared-secret")
	cfg := Load()
	if cfg.AgentAuth.TokenVerifyKey != "shared-secret" {
		t.Fatalf("TokenVerifyKey = %q", cfg.AgentAuth.TokenVerifyKey)
	}
	os.Unsetenv("AGENT_TOKEN_SIGNING_KEY")
}
