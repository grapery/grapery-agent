package http

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/grapestree/fgrapery/grapery-agent/internal/agents"
	"github.com/grapestree/fgrapery/grapery-agent/internal/grapery_client"
	"github.com/grapestree/fgrapery/grapery-agent/internal/runstore"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	"github.com/gin-gonic/gin"
)

// Handler 管理 HTTP 路由和 Agent 调用
type Handler struct {
	registry   *agents.AgentRegistry
	client     *grapery_client.Client
	checkpoint adk.CheckPointStore
	genHandler *GenerationHandler
}

func NewHandler(registry *agents.AgentRegistry, client *grapery_client.Client, checkpoint adk.CheckPointStore, genHandler *GenerationHandler) *Handler {
	return &Handler{
		registry:   registry,
		client:     client,
		checkpoint: checkpoint,
		genHandler: genHandler,
	}
}

func (h *Handler) RegisterRoutes(r *gin.Engine) {
	api := r.Group("/api/v1/agent")
	api.Use(h.authMiddleware())
	{
		api.POST("/fragment/chat", h.agentChat(h.registry.FragmentAgent))
		api.POST("/fragment/chat/:checkpointID/resume", h.agentResume(h.registry.FragmentAgent))

		api.POST("/character/chat", h.agentChat(h.registry.CharacterAgent))
		api.POST("/character/chat/:checkpointID/resume", h.agentResume(h.registry.CharacterAgent))

		api.POST("/storyboard/chat", h.agentChat(h.registry.StoryboardAgent))
		api.POST("/storyboard/chat/:checkpointID/resume", h.agentResume(h.registry.StoryboardAgent))

		api.POST("/branch/chat", h.agentChat(h.registry.BranchExplorer))
		api.POST("/branch/chat/:checkpointID/resume", h.agentResume(h.registry.BranchExplorer))
	}
	if h.genHandler != nil {
		h.genHandler.RegisterRoutes(r)
	}
}

// authMiddleware 从 Authorization header 提取 JWT 并注入 context
func (h *Handler) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		if token := c.GetHeader("Authorization"); token != "" && len(token) > 7 && token[:7] == "Bearer " {
			ctx = grapery_client.ContextWithAuthToken(ctx, token[7:])
		}
		if runID := c.GetHeader("X-Generation-Run-Id"); runID != "" {
			ctx = runstore.ContextWithRunID(ctx, runID)
		}
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// ============ 请求/响应类型 ============

type ChatRequest struct {
	Message     string `json:"message" binding:"required"`
	InterruptID string `json:"interruptId,omitempty"`
}

type ChatResponse struct {
	Message      string `json:"message"`
	Finished     bool   `json:"finished"`
	Interrupted  bool   `json:"interrupted,omitempty"`
	Question     string `json:"question,omitempty"`
	InterruptID  string `json:"interruptId,omitempty"`
	CheckPointID string `json:"checkpointId,omitempty"`
}

// ============ 通用 Agent 处理器 ============

func (h *Handler) agentChat(agent *adk.ChatModelAgent) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req ChatRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
			return
		}

		ctx := c.Request.Context()
		cpID := fmt.Sprintf("cp_%d", time.Now().UnixMilli())
		enableStreaming := c.Query("stream") == "true"

		runner := adk.NewRunner(ctx, adk.RunnerConfig{
			Agent:           agent,
			CheckPointStore: h.checkpoint,
			EnableStreaming: enableStreaming,
		})

		messages := []adk.Message{schema.UserMessage(req.Message)}
		iter := runner.Run(ctx, messages, adk.WithCheckPointID(cpID))
		h.processEvents(c, iter, cpID)
	}
}

func (h *Handler) agentResume(agent *adk.ChatModelAgent) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req ChatRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
			return
		}

		ctx := c.Request.Context()
		cpID := c.Param("checkpointID")

		runner := adk.NewRunner(ctx, adk.RunnerConfig{
			Agent:           agent,
			CheckPointStore: h.checkpoint,
		})

		// 通过 ResumeWithParams 将用户回复传递给被 interrupt 的工具
		targets := map[string]any{}
		if req.InterruptID != "" {
			targets[req.InterruptID] = req.Message
		}

		iter, err := runner.ResumeWithParams(ctx, cpID, &adk.ResumeParams{
			Targets: targets,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": -5, "message": fmt.Sprintf("resume failed: %v", err)})
			return
		}

		h.processEvents(c, iter, cpID)
	}
}

// ============ 事件处理 ============

func (h *Handler) processEvents(c *gin.Context, iter *adk.AsyncIterator[*adk.AgentEvent], checkpointID string) {
	if c.Query("stream") == "true" {
		h.processEventsSSE(c, iter, checkpointID)
		return
	}
	h.processEventsSync(c, iter, checkpointID)
}

// processEventsSync 同步收集所有事件后返回单个 JSON
func (h *Handler) processEventsSync(c *gin.Context, iter *adk.AsyncIterator[*adk.AgentEvent], checkpointID string) {
	var finalMessage string
	var interrupted bool
	var question string
	var interruptID string

	for {
		event, ok := iter.Next()
		if !ok {
			break
		}

		if event.Err != nil {
			log.Printf("agent error: %v", event.Err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"code": -5, "message": fmt.Sprintf("Agent error: %v", event.Err),
			})
			return
		}

		if event.Action != nil && event.Action.Interrupted != nil {
			interrupted = true
			question = extractQuestion(event.Action.Interrupted)
			interruptID = extractInterruptID(event.Action.Interrupted)
			continue
		}

		if event.Output != nil && event.Output.MessageOutput != nil {
			mv := event.Output.MessageOutput
			if mv.Message != nil {
				finalMessage = mv.Message.Content
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 1, "message": "success",
		"data": ChatResponse{
			Message:      finalMessage,
			Finished:     !interrupted,
			Interrupted:  interrupted,
			Question:     question,
			InterruptID:  interruptID,
			CheckPointID: checkpointID,
		},
	})
}

// processEventsSSE 以 Server-Sent Events 流式推送事件
func (h *Handler) processEventsSSE(c *gin.Context, iter *adk.AsyncIterator[*adk.AgentEvent], checkpointID string) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	writeSSE := func(event string, data any) {
		jsonData, err := json.Marshal(data)
		if err != nil {
			log.Printf("SSE marshal error: %v", err)
			jsonData = []byte(`{"error":"internal marshal error"}`)
		}
		c.SSEvent(event, string(jsonData))
		c.Writer.Flush()
	}

	writeSSE("start", gin.H{"checkpointId": checkpointID})

	for {
		event, ok := iter.Next()
		if !ok {
			break
		}

		if event.Err != nil {
			writeSSE("error", gin.H{"error": event.Err.Error()})
			return
		}

		if event.Action != nil && event.Action.Interrupted != nil {
			info := event.Action.Interrupted
			writeSSE("interrupt", ChatResponse{
				Interrupted:  true,
				Question:     extractQuestion(info),
				InterruptID:  extractInterruptID(info),
				CheckPointID: checkpointID,
			})
			return
		}

		if event.Output != nil && event.Output.MessageOutput != nil {
			mv := event.Output.MessageOutput
			if mv.Message != nil {
				writeSSE("message", ChatResponse{
					Message:      mv.Message.Content,
					CheckPointID: checkpointID,
				})
			}
		}
	}

	writeSSE("done", ChatResponse{Finished: true, CheckPointID: checkpointID})
}

// ============ 辅助函数 ============

func extractQuestion(info *adk.InterruptInfo) string {
	if info == nil {
		return ""
	}
	if data, ok := info.Data.(string); ok {
		return data
	}
	// tool.Interrupt(ctx, question) 将问题文本存在 InterruptContexts[].Info 中
	if len(info.InterruptContexts) > 0 {
		if data, ok := info.InterruptContexts[0].Info.(string); ok {
			return data
		}
	}
	if data, _ := json.Marshal(info.Data); len(data) > 0 && string(data) != "null" {
		return string(data)
	}
	return ""
}

// extractInterruptID 从 InterruptContexts 中取出第一个 interrupt ID
func extractInterruptID(info *adk.InterruptInfo) string {
	if info == nil || len(info.InterruptContexts) == 0 {
		return ""
	}
	return info.InterruptContexts[0].ID
}

// ============ InMemoryCheckPointStore ============

const maxCheckPoints = 10000

type InMemoryCheckPointStore struct {
	mu   sync.RWMutex
	data map[string][]byte
	keys []string // insertion order for eviction
}

func NewInMemoryCheckPointStore() *InMemoryCheckPointStore {
	return &InMemoryCheckPointStore{data: make(map[string][]byte)}
}

func (s *InMemoryCheckPointStore) Get(ctx context.Context, id string) ([]byte, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d, ok := s.data[id]
	return d, ok, nil
}

func (s *InMemoryCheckPointStore) Set(ctx context.Context, id string, state []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.data[id]; !exists {
		s.keys = append(s.keys, id)
	}
	s.data[id] = state
	for len(s.data) > maxCheckPoints {
		evictID := s.keys[0]
		s.keys = s.keys[1:]
		delete(s.data, evictID)
	}
	return nil
}

var _ adk.CheckPointStore = (*InMemoryCheckPointStore)(nil)
