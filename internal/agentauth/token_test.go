package agentauth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"
)

// signLikeGrapery 复制 grapery 侧 internal/service/agent_access_token.go 的线格式，
// 以验证 grapery-agent 校验器与 grapery 签发器跨服务互通。
func signLikeGrapery(key string, claims Claims) string {
	payload, _ := json.Marshal(claims)
	mac := hmac.New(sha256.New, []byte(key))
	_, _ = mac.Write(payload)
	enc := base64.RawURLEncoding
	return enc.EncodeToString(payload) + "." + enc.EncodeToString(mac.Sum(nil))
}

func validClaims() Claims {
	now := time.Now()
	return Claims{
		Version:   "v1",
		Issuer:    TokenIssuer,
		Audience:  TokenAudience,
		UserID:    "user-1",
		RequestID: "req_1",
		Agent:     "fragment-panel",
		Operation: "generate",
		JTI:       "jti_1",
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(5 * time.Minute).Unix(),
	}
}

func TestVerifier_VerifyValid(t *testing.T) {
	const key = "shared-secret"
	v := NewVerifier(key)
	token := signLikeGrapery(key, validClaims())

	claims, err := v.Verify(token)
	if err != nil {
		t.Fatalf("verify failed: %v", err)
	}
	if claims.UserID != "user-1" || claims.Agent != "fragment-panel" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

func TestVerifier_RejectsBadSignature(t *testing.T) {
	v := NewVerifier("right-key")
	token := signLikeGrapery("wrong-key", validClaims())
	if _, err := v.Verify(token); err == nil {
		t.Fatal("expected bad signature rejection")
	}
}

func TestVerifier_RejectsBadAudience(t *testing.T) {
	const key = "k"
	v := NewVerifier(key)
	c := validClaims()
	c.Audience = "someone-else"
	if _, err := v.Verify(signLikeGrapery(key, c)); err != ErrBadAudience {
		t.Fatalf("expected ErrBadAudience, got %v", err)
	}
}

func TestVerifier_RejectsExpired(t *testing.T) {
	const key = "k"
	v := NewVerifier(key)
	c := validClaims()
	c.ExpiresAt = time.Now().Add(-time.Minute).Unix()
	if _, err := v.Verify(signLikeGrapery(key, c)); err != ErrExpired {
		t.Fatalf("expected ErrExpired, got %v", err)
	}
}

func TestVerifier_Malformed(t *testing.T) {
	v := NewVerifier("k")
	for _, tok := range []string{"", "noseparator", "a.b.c"} {
		if _, err := v.Verify(tok); err == nil {
			t.Fatalf("expected malformed rejection for %q", tok)
		}
	}
}

func TestVerifier_NotConfigured(t *testing.T) {
	v := NewVerifier("   ")
	if v.IsConfigured() {
		t.Fatal("blank key should not be configured")
	}
	if _, err := v.Verify("x.y"); err != ErrNotConfigured {
		t.Fatalf("expected ErrNotConfigured, got %v", err)
	}
}
