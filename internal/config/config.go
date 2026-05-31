package config

import (
	"os"
	"strconv"
)

type Config struct {
	Server   ServerConfig
	Grapery  GraperyConfig
	Eino     EinoConfig
	Artifact ArtifactConfig
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
)

type EinoConfig struct {
	TextModel      string // huoshan-endpoint-id 或 gemini-model-name
	TextProvider   string // huoshan | gemini
	HuoshanAPIKey  string
	HuoshanBaseURL string
	GeminiAPIKey   string
	MaxIterations  int
	RequestTimeout int // seconds
	ImageModel     string
	VideoModel     string
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
		Eino: EinoConfig{
			TextModel:      getEnv("EINO_TEXT_MODEL", ""),
			TextProvider:   getEnv("EINO_TEXT_PROVIDER", "huoshan"),
			HuoshanAPIKey:  getEnv("HUOSHAN_API_KEY", ""),
			HuoshanBaseURL: getEnv("HUOSHAN_BASE_URL", "https://ark.cn-beijing.volces.com/api/v3"),
			GeminiAPIKey:   getEnv("GEMINI_API_KEY", ""),
			MaxIterations:  getEnvInt("EINO_MAX_ITERATIONS", 30),
			RequestTimeout: getEnvInt("EINO_REQUEST_TIMEOUT", 180),
			ImageModel:     getEnv("HUOSHAN_IMAGE_MODEL", "doubao-seedream-5-0-260128"),
			VideoModel:     getEnv("HUOSHAN_VIDEO_MODEL", "doubao-seedance-1-5-pro-251215"),
		},
	}
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
