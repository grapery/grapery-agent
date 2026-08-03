package main

import (
	"context"
	"fmt"
	"log"

	"github.com/cloudwego/eino/adk"
	"github.com/grapestree/fgrapery/grapery-agent/internal/agents"
	"github.com/grapestree/fgrapery/grapery-agent/internal/config"
	"github.com/grapestree/fgrapery/grapery-agent/internal/generation"
	"github.com/grapestree/fgrapery/grapery-agent/internal/grapery_client"
	graperymodel "github.com/grapestree/fgrapery/grapery-agent/internal/model"
	"github.com/grapestree/fgrapery/grapery-agent/internal/runstore"
	"github.com/grapestree/fgrapery/grapery-agent/internal/transport/http"

	"github.com/cloudwego/eino/components/model"
	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.Load()

	// 1. 创建 grapery HTTP 客户端
	client := grapery_client.NewClient(cfg.Grapery)

	// 2. 创建 Eino ChatModel（根据配置选择 Huoshan 或 Gemini）
	chatModel := graperymodel.NewChatModel(cfg.Eino)

	// 3. 创建 Huoshan 图片/视频生成模型（仅 huoshan provider）
	var imageModel *graperymodel.HuoshanImageModel
	var videoModel *graperymodel.HuoshanVideoModel
	var textModel *graperymodel.HuoshanTextModel
	if cfg.Eino.TextProvider == "huoshan" || cfg.Eino.TextProvider == "" {
		imageTimeout := cfg.Eino.EffectiveImageTimeout()
		imageModel = graperymodel.NewImageModel(cfg.Eino.HuoshanAPIKey, cfg.Eino.HuoshanBaseURL, cfg.Eino.ImageModel, imageTimeout)
		videoModel = graperymodel.NewVideoModel(cfg.Eino.HuoshanAPIKey, cfg.Eino.HuoshanBaseURL, cfg.Eino.VideoModel, imageTimeout)
		textModel = graperymodel.NewTextModel(cfg.Eino.HuoshanAPIKey, cfg.Eino.HuoshanBaseURL, cfg.Eino.TextModel, cfg.Eino.RequestTimeout)
	}

	// 编译期验证接口实现
	_ = model.BaseChatModel(chatModel)

	// 4. 创建 Agent Registry
	ctx := context.Background()
	registry, err := agents.NewRegistry(ctx, chatModel, textModel, imageModel, videoModel, client, cfg.Eino.MaxIterations)
	if err != nil {
		log.Fatalf("Failed to create agent registry: %v", err)
	}

	// 5. 创建 CheckPoint Store 与 Generation Run Store
	var checkpoint = adk.CheckPointStore(http.NewInMemoryCheckPointStore())
	var runStore runstore.Store = runstore.NewMemoryStore()
	if cfg.Grapery.APIKey != "" {
		checkpoint = http.NewGraperyCheckPointStore(client)
		runStore = runstore.NewGraperyStore(client)
	} else {
		if cfg.AgentAuth.DurableRuntimeRequired {
			log.Fatal("GRAPERY_API_KEY is required when DURABLE_RUNTIME_REQUIRED is enabled")
		}
		log.Printf("WARNING: GRAPERY_API_KEY is empty; generation runs and checkpoints are not durable")
	}
	genSvc := generation.NewService(client, runStore, cfg.Eino.TextProvider, cfg.Eino.TextModel, cfg.AgentAuth.ExecFragmentPanelEnabled)
	if cfg.Grapery.APIKey != "" {
		if err := genSvc.ResumeWorkflows(ctx); err != nil {
			log.Printf("workflow recovery scan failed: %v", err)
		}
	}
	genHandler := http.NewGenerationHandler(genSvc, runStore, cfg.Artifact.ExportDir, cfg.AgentAuth)

	// 6. 创建 HTTP Handler
	handler := http.NewHandler(registry, client, checkpoint, genHandler, cfg.AgentAuth)

	// 7. 启动 HTTP 服务
	r := gin.Default()
	handler.RegisterRoutes(r)

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "grapery-agent"})
	})

	addr := fmt.Sprintf(":%s", cfg.Server.Port)
	log.Printf("grapery-agent starting on %s", addr)
	log.Printf("Grapery backend: %s", cfg.Grapery.BaseURL)
	log.Printf("Eino text provider: %s", cfg.Eino.TextProvider)
	log.Printf("Agent access token: verifyKeySet=%t required=%t", cfg.AgentAuth.TokenVerifyKey != "", cfg.AgentAuth.AccessTokenRequired)
	log.Printf("Agent chat: sync routes enabled at POST /api/v1/agent/{agent}/chat/sync")

	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
