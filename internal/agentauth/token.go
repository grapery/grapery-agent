// Package agentauth 校验 grapery 签发的短期 Agent Access Token。
//
// 线格式（必须与 grapery internal/service/agent_access_token.go 完全一致）：
//
//	base64url(payloadJSON) + "." + base64url(HMAC_SHA256(payloadJSON, key))
//
// 校验：对 base64 解码后的原始 payload 字节重算 HMAC 并比较，再解析 claims、
// 校验 audience 与过期时间。grapery 是 auth/quota 权威方，agent 只验证入站访问。
package agentauth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

const (
	TokenIssuer   = "grapery"
	TokenAudience = "grapery-agent"
)

var (
	ErrNotConfigured = errors.New("agent access token verification is not configured")
	ErrMalformed     = errors.New("malformed agent access token")
	ErrBadSignature  = errors.New("invalid agent access token signature")
	ErrBadAudience   = errors.New("invalid agent access token audience")
	ErrExpired       = errors.New("agent access token expired")
	ErrBadClaims     = errors.New("invalid agent access token claims")
)

// Claims 是 Agent Access Token 的载荷（字段需与 grapery 侧保持一致）。
type Claims struct {
	Version            string `json:"v"`
	Issuer             string `json:"iss"`
	Audience           string `json:"aud"`
	UserID             string `json:"userId"`
	RequestID          string `json:"requestId"`
	SessionID          string `json:"sessionId,omitempty"`
	Agent              string `json:"agent"`
	Operation          string `json:"operation"`
	Scope              string `json:"scope,omitempty"`
	Kind               string `json:"kind,omitempty"`
	QuotaMode          string `json:"quotaMode,omitempty"`
	QuotaReservationID string `json:"quotaReservationId,omitempty"`
	MaxTokens          int    `json:"maxTokens,omitempty"`
	MaxImages          int    `json:"maxImages,omitempty"`
	JTI                string `json:"jti"`
	IssuedAt           int64  `json:"iat"`
	ExpiresAt          int64  `json:"exp"`
}

// Verifier 校验对称密钥签发的 token。
type Verifier struct {
	secret []byte
}

// NewVerifier 创建校验器。
func NewVerifier(key string) *Verifier {
	return &Verifier{secret: []byte(strings.TrimSpace(key))}
}

// IsConfigured 仅当密钥已设置时返回 true。
func (v *Verifier) IsConfigured() bool {
	return v != nil && len(v.secret) > 0
}

// Verify 校验 token 并返回 claims。
func (v *Verifier) Verify(token string) (*Claims, error) {
	if !v.IsConfigured() {
		return nil, ErrNotConfigured
	}
	token = strings.TrimSpace(token)
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return nil, ErrMalformed
	}
	enc := base64.RawURLEncoding
	payload, err := enc.DecodeString(parts[0])
	if err != nil {
		return nil, ErrMalformed
	}
	sig, err := enc.DecodeString(parts[1])
	if err != nil {
		return nil, ErrMalformed
	}
	mac := hmac.New(sha256.New, v.secret)
	_, _ = mac.Write(payload)
	if !hmac.Equal(mac.Sum(nil), sig) {
		return nil, ErrBadSignature
	}
	var claims Claims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, ErrBadClaims
	}
	if claims.Audience != TokenAudience {
		return nil, ErrBadAudience
	}
	if claims.ExpiresAt > 0 && time.Now().Unix() > claims.ExpiresAt {
		return nil, ErrExpired
	}
	return &claims, nil
}

// ---- context helpers ----

type claimsKey struct{}

// ContextWithClaims 将 claims 注入 context，供下游用量统计/审计使用。
func ContextWithClaims(ctx context.Context, claims *Claims) context.Context {
	return context.WithValue(ctx, claimsKey{}, claims)
}

// ClaimsFromContext 取出 claims。
func ClaimsFromContext(ctx context.Context) (*Claims, bool) {
	v, ok := ctx.Value(claimsKey{}).(*Claims)
	return v, ok && v != nil
}
