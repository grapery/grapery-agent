package http

import (
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/grapestree/fgrapery/grapery-agent/internal/agentauth"
	"github.com/grapestree/fgrapery/grapery-agent/internal/config"
	"github.com/grapestree/fgrapery/grapery-agent/internal/grapery_client"
)

const agentBearerUsedKey = "agentBearerUsed"

// AgentAccessHeader 是携带 grapery 签发的 Agent Access Token 的专用头。
const AgentAccessHeader = "X-Agent-Access-Token"

type agentAuthDeps struct {
	verifier       *agentauth.Verifier
	client         *grapery_client.Client
	accessRequired bool
	replayEnabled  bool
}

func newAgentAuthDeps(cfg config.AgentAuthConfig, client *grapery_client.Client) agentAuthDeps {
	return agentAuthDeps{
		verifier:       agentauth.NewVerifier(cfg.TokenVerifyKey),
		accessRequired: cfg.AccessTokenRequired,
		replayEnabled:  cfg.ReplayCacheEnabled,
		client:         client,
	}
}

func (d agentAuthDeps) agentAccessTokenMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if d.verifier == nil || !d.verifier.IsConfigured() {
			c.Next()
			return
		}

		raw, fromBearer := extractAgentAccessToken(c)
		if raw == "" {
			if d.accessRequired {
				log.Printf("[agent-auth] reject path=%s reason=missing_agent_access_token", c.Request.URL.Path)
				abortUnauthorized(c, "missing agent access token")
				return
			}
			c.Next()
			return
		}

		claims, err := d.verifier.Verify(raw)
		if err != nil {
			// Bearer 可能是用户 JWT：校验失败则交给后续 JWT 中间件（非强制模式）。
			if fromBearer && !d.accessRequired {
				c.Next()
				return
			}
			log.Printf("[agent-auth] reject path=%s fromBearer=%v reason=%s", c.Request.URL.Path, fromBearer, err.Error())
			abortUnauthorized(c, "invalid agent access token: "+err.Error())
			return
		}

		if d.replayEnabled && d.client != nil && claims.JTI != "" && strings.EqualFold(claims.Operation, "generate") {
			if err := d.client.ConsumeAgentTokenJTI(c.Request.Context(), claims.JTI); err != nil {
				abortUnauthorized(c, "agent access token replay check failed: "+err.Error())
				return
			}
		}

		if expAgent, expOp, check := expectedScopeForPath(c.Request.Method, c.Request.URL.Path); check {
			if !scopeMatches(claims, expAgent, expOp) {
				log.Printf(
					"[agent-auth] reject path=%s userID=%s tokenScope=%s wantAgent=%s wantOp=%s",
					c.Request.URL.Path, claims.UserID, claims.Scope, expAgent, expOp,
				)
				abortUnauthorized(c, "token scope does not match endpoint")
				return
			}
		}

		if fromBearer {
			c.Set(agentBearerUsedKey, true)
		}
		c.Set("agentUserID", claims.UserID)
		c.Set("agentScope", claims.Scope)
		c.Request = c.Request.WithContext(agentauth.ContextWithClaims(c.Request.Context(), claims))
		c.Next()
	}
}

func extractAgentAccessToken(c *gin.Context) (token string, fromBearer bool) {
	raw := strings.TrimSpace(c.GetHeader(AgentAccessHeader))
	if raw != "" {
		return raw, false
	}
	auth := c.GetHeader("Authorization")
	if strings.HasPrefix(auth, "AgentAccess ") {
		return strings.TrimSpace(strings.TrimPrefix(auth, "AgentAccess ")), false
	}
	if strings.HasPrefix(auth, "Bearer ") {
		bearer := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
		return bearer, true
	}
	return "", false
}

func expectedScopeForPath(method, path string) (agent, operation string, enforce bool) {
	path = strings.TrimSuffix(path, "/")
	switch {
	case method == http.MethodPost && strings.HasPrefix(path, "/api/v1/agent/creation/"):
		return "fragment", "generate", true
	case strings.HasSuffix(path, "/chat") || strings.Contains(path, "/chat/"):
		parts := strings.Split(strings.TrimPrefix(path, "/api/v1/agent/"), "/")
		if len(parts) > 0 && parts[0] != "" {
			return parts[0], "chat", true
		}
	case method == http.MethodPost && strings.HasPrefix(path, "/api/v1/generation/"):
		rest := strings.TrimPrefix(path, "/api/v1/generation/")
		seg := strings.Split(rest, "/")[0]
		switch seg {
		case "fragment-panels":
			return "fragment-panel", "generate", true
		case "fragments":
			return "fragment", "generate", true
		case "characters":
			return "character", "generate", true
		case "storyboards":
			return "storyboard", "generate", true
		case "stories":
			return "story", "generate", true
		case "branches":
			return "branch", "generate", true
		case "workflows":
			return "", "", false
		}
	case strings.Contains(path, "/generation/") && strings.HasSuffix(path, "/stream"):
		rest := strings.TrimPrefix(path, "/api/v1/generation/")
		seg := strings.Split(rest, "/")[0]
		return generationSegToAgent(seg), "generate", true
	}
	return "", "", false
}

func generationSegToAgent(seg string) string {
	switch seg {
	case "fragment-panels":
		return "fragment-panel"
	case "fragments":
		return "fragment"
	case "characters":
		return "character"
	case "storyboards":
		return "storyboard"
	case "stories":
		return "story"
	case "branches":
		return "branch"
	default:
		return seg
	}
}

func scopeMatches(claims *agentauth.Claims, agent, operation string) bool {
	if claims == nil {
		return false
	}
	if claims.Scope != "" {
		want := "agent:" + agent + ":" + operation
		return strings.EqualFold(claims.Scope, want)
	}
	return strings.EqualFold(claims.Agent, agent) && strings.EqualFold(claims.Operation, operation)
}

func abortUnauthorized(c *gin.Context, msg string) {
	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": -2, "message": msg})
}

func requireAgentClaims(deps agentAuthDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.verifier == nil || !deps.verifier.IsConfigured() {
			c.Next()
			return
		}
		if _, ok := agentauth.ClaimsFromContext(c.Request.Context()); !ok {
			abortUnauthorized(c, "agent access token required for this endpoint")
			return
		}
		c.Next()
	}
}

// forwardUserJWTMiddleware 从 Authorization Bearer 提取用户 JWT（agent token 已占用 Bearer 时跳过）。
func forwardUserJWTMiddleware(client *grapery_client.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		if c.GetBool(agentBearerUsedKey) {
			c.Request = c.Request.WithContext(ctx)
			c.Next()
			return
		}
		if token := c.GetHeader("Authorization"); len(token) > 7 && token[:7] == "Bearer " {
			ctx = grapery_client.ContextWithAuthToken(ctx, token[7:])
		}
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
