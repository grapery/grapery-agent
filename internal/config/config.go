package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Server    ServerConfig
	Grapery   GraperyConfig
	Eino      EinoConfig
	Artifact  ArtifactConfig
	AgentAuth AgentAuthConfig
}

// AgentAuthConfig 控制 grapery-agent 对入站请求的 Agent Access Token 校验。
type AgentAuthConfig struct {
	TokenVerifyKey           string
	// InternalAPIKey 期望的内部服务密钥（env: GRAPERY_INTERNAL_API_KEY）。
	// 固定版本试运行请求必须携带 X-Internal-Api-Key 且与此一致。
	InternalAPIKey           string
	AccessTokenRequired      bool
	ReplayCacheEnabled       bool
	ExecFragmentPanelEnabled bool
	AuditSyncEnabled         bool
	DurableRuntimeRequired   bool
	ObservabilityToken       string
}

type ArtifactConfig struct {
	ExportDir string
}

type ServerConfig struct {
	Port string
}

type GraperyConfig struct {
	BaseURL string
	APIKey  string // 用于内部服务间认证，或复用 JWT
}

const (
	// SeedreamTimeoutFloor Seedream 5.0 组图流式最低超时（秒），与 grapery EffectiveHuoshanRequestTimeoutFloor 对齐。
	SeedreamTimeoutFloor = 600
	// DefaultHuoshanTextModel 与 grapery/internal/genai/providers/huoshan defaultTextModel 一致。
	DefaultHuoshanTextModel = "doubao-seed-2-0-lite-260215"
	// DefaultAgentTokenVerifyKey 与 grapery 的 AGENT_TOKEN_SIGNING_KEY 默认值一致。
	DefaultAgentTokenVerifyKey = "voyager1990"
)

type EinoConfig struct {
	TextModel          string // huoshan-endpoint-id 或 gemini-model-name
	TextProvider       string // huoshan | gemini
	HuoshanAPIKey      string
	HuoshanBaseURL     string
	GeminiAPIKey       string
	MaxIterations      int
	SessionMaxMessages int
	KnowledgeDir       string
	KnowledgeTopK      int
	RequestTimeout     int // seconds
	ImageModel         string
	VideoModel         string
}

// EffectiveImageTimeout returns HTTP timeout (seconds) for image/video models.
// Seedream 5.0 streaming image sets need well above the default 180s.
func (e EinoConfig) EffectiveImageTimeout() int {
	if e.RequestTimeout >= SeedreamTimeoutFloor {
		return e.RequestTimeout
	}
	return SeedreamTimeoutFloor
}

func Load() *Config {
	return &Config{
		Server: ServerConfig{
			Port: getEnv("SERVER_PORT", "9020"),
		},
		Grapery: GraperyConfig{
			BaseURL: getEnv("GRAPERY_BASE_URL", "http://localhost:9000"),
			APIKey:  getEnv("GRAPERY_API_KEY", ""),
		},
		Artifact: ArtifactConfig{
			ExportDir: getEnv("AGENT_ARTIFACT_DIR", "./data/agent-artifacts"),
		},
		AgentAuth: AgentAuthConfig{
			TokenVerifyKey:           effectiveAgentTokenVerifyKey(),
			AccessTokenRequired:      getEnvBool("AGENT_ACCESS_TOKEN_REQUIRED", false),
			InternalAPIKey:           getEnv("GRAPERY_INTERNAL_API_KEY", ""),
			ReplayCacheEnabled:       getEnvBool("AGENT_TOKEN_REPLAY_CACHE_ENABLED", false),
			ExecFragmentPanelEnabled: getEnvBool("AGENT_EXEC_FRAGMENT_PANEL_ENABLED", true),
			AuditSyncEnabled:         getEnvBool("AUDIT_SYNC_TO_GRAPERY_ENABLED", true),
			DurableRuntimeRequired:   getEnvBool("DURABLE_RUNTIME_REQUIRED", isProductionEnvironment()),
			ObservabilityToken:       getEnv("AGENT_OBSERVABILITY_TOKEN", ""),
		},
		Eino: EinoConfig{
			TextModel:          loadTextModel(),
			TextProvider:       getEnv("EINO_TEXT_PROVIDER", getEnv("AI_DEFAULT_PROVIDER", "huoshan")),
			HuoshanAPIKey:      getEnv("HUOSHAN_API_KEY", ""),
			HuoshanBaseURL:     getEnv("HUOSHAN_BASE_URL", "https://ark.cn-beijing.volces.com/api/v3"),
			GeminiAPIKey:       getEnv("GEMINI_API_KEY", ""),
			MaxIterations:      getEnvInt("EINO_MAX_ITERATIONS", 30),
			SessionMaxMessages: getEnvInt("EINO_SESSION_MAX_MESSAGES", 40),
			KnowledgeDir:       getEnv("EINO_KNOWLEDGE_DIR", ""),
			KnowledgeTopK:      getEnvInt("EINO_KNOWLEDGE_TOP_K", 4),
			RequestTimeout:     getEnvInt("EINO_REQUEST_TIMEOUT", 180),
			ImageModel:         getEnv("HUOSHAN_IMAGE_MODEL", "doubao-seedream-5-0-260128"),
			VideoModel:         getEnv("HUOSHAN_VIDEO_MODEL", "doubao-seedance-1-5-pro-251215"),
		},
	}
}

func isProductionEnvironment() bool {
	for _, key := range []string{"APP_ENV", "ENVIRONMENT", "GIN_MODE"} {
		value := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
		if value == "production" || value == "prod" || value == "release" {
			return true
		}
	}
	return false
}

func loadTextModel() string {
	if v := os.Getenv("EINO_TEXT_MODEL"); v != "" {
		return v
	}
	if v := os.Getenv("HUOSHAN_TEXT_MODEL"); v != "" {
		return v
	}
	return DefaultHuoshanTextModel
}

// effectiveAgentTokenVerifyKey 优先 AGENT_TOKEN_VERIFY_KEY；未设置时回落到
// AGENT_TOKEN_SIGNING_KEY（与 grapery 共用 .env 时 source 一次即可对齐）。
func effectiveAgentTokenVerifyKey() string {
	if v := strings.TrimSpace(os.Getenv("AGENT_TOKEN_VERIFY_KEY")); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv("AGENT_TOKEN_SIGNING_KEY")); v != "" {
		return v
	}
	return DefaultAgentTokenVerifyKey
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
		return v == "yes"
	}
	return fallback
}
