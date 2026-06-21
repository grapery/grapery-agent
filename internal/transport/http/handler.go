package http

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/grapestree/fgrapery/grapery-agent/internal/agents"
	"github.com/grapestree/fgrapery/grapery-agent/internal/config"
	"github.com/grapestree/fgrapery/grapery-agent/internal/grapery_client"
	"github.com/grapestree/fgrapery/grapery-agent/internal/runstore"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Handler 管理 HTTP 路由和 Agent 调用
type Handler struct {
	registry   *agents.AgentRegistry
	client     *grapery_client.Client
	checkpoint adk.CheckPointStore
	genHandler *GenerationHandler
	agentAuth  agentAuthDeps
}

func NewHandler(registry *agents.AgentRegistry, client *grapery_client.Client, checkpoint adk.CheckPointStore, genHandler *GenerationHandler, agentAuth config.AgentAuthConfig) *Handler {
	return &Handler{
		registry:   registry,
		client:     client,
		checkpoint: checkpoint,
		genHandler: genHandler,
		agentAuth:  newAgentAuthDeps(agentAuth, client),
	}
}

func (h *Handler) RegisterRoutes(r *gin.Engine) {
	api := r.Group("/api/v1/agent")
	api.Use(h.agentAuth.agentAccessTokenMiddleware())
	api.Use(forwardUserJWTMiddleware(h.client))
	api.Use(h.runIDMiddleware())
	{
		api.POST("/fragment/chat", h.agentChat(h.registry.FragmentAgent, "fragment"))
		api.POST("/fragment/chat/sync", h.agentChatSync(h.registry.FragmentAgent, "fragment"))
		api.POST("/fragment/chat/:checkpointID/resume", h.agentResume(h.registry.FragmentAgent, "fragment"))
		api.POST("/fragment/chat/:checkpointID/resume/sync", h.agentResumeSync(h.registry.FragmentAgent, "fragment"))

		api.POST("/fragment-panel/chat", requireAgentClaims(h.agentAuth), h.agentChat(h.registry.FragmentPanelAgent, "fragment-panel"))
		api.POST("/fragment-panel/chat/sync", requireAgentClaims(h.agentAuth), h.agentChatSync(h.registry.FragmentPanelAgent, "fragment-panel"))
		api.POST("/fragment-panel/chat/:checkpointID/resume", requireAgentClaims(h.agentAuth), h.agentResume(h.registry.FragmentPanelAgent, "fragment-panel"))
		api.POST("/fragment-panel/chat/:checkpointID/resume/sync", requireAgentClaims(h.agentAuth), h.agentResumeSync(h.registry.FragmentPanelAgent, "fragment-panel"))

		api.POST("/character/chat", h.agentChat(h.registry.CharacterAgent, "character"))
		api.POST("/character/chat/sync", h.agentChatSync(h.registry.CharacterAgent, "character"))
		api.POST("/character/chat/:checkpointID/resume", h.agentResume(h.registry.CharacterAgent, "character"))
		api.POST("/character/chat/:checkpointID/resume/sync", h.agentResumeSync(h.registry.CharacterAgent, "character"))

		api.POST("/storyboard/chat", h.agentChat(h.registry.StoryboardAgent, "storyboard"))
		api.POST("/storyboard/chat/sync", h.agentChatSync(h.registry.StoryboardAgent, "storyboard"))
		api.POST("/storyboard/chat/:checkpointID/resume", h.agentResume(h.registry.StoryboardAgent, "storyboard"))
		api.POST("/storyboard/chat/:checkpointID/resume/sync", h.agentResumeSync(h.registry.StoryboardAgent, "storyboard"))

		api.POST("/branch/chat", h.agentChat(h.registry.BranchExplorer, "branch"))
		api.POST("/branch/chat/sync", h.agentChatSync(h.registry.BranchExplorer, "branch"))
		api.POST("/branch/chat/:checkpointID/resume", h.agentResume(h.registry.BranchExplorer, "branch"))
		api.POST("/branch/chat/:checkpointID/resume/sync", h.agentResumeSync(h.registry.BranchExplorer, "branch"))
	}
	if h.genHandler != nil {
		h.genHandler.RegisterRoutes(r, h.agentAuth, h.client)
	}
}

func (h *Handler) runIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
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
	Structured   any    `json:"structured,omitempty"`
}

type chatDelivery int

const (
	chatDeliveryAuto chatDelivery = iota
	chatDeliverySync
)

// ============ 通用 Agent 处理器 ============

func (h *Handler) agentChat(agent *adk.ChatModelAgent, agentName string) gin.HandlerFunc {
	return h.bindAgentChat(agent, agentName, chatDeliveryAuto, false)
}

func (h *Handler) agentChatSync(agent *adk.ChatModelAgent, agentName string) gin.HandlerFunc {
	return h.bindAgentChat(agent, agentName, chatDeliverySync, false)
}

func (h *Handler) agentResume(agent *adk.ChatModelAgent, agentName string) gin.HandlerFunc {
	return h.bindAgentChat(agent, agentName, chatDeliveryAuto, true)
}

func (h *Handler) agentResumeSync(agent *adk.ChatModelAgent, agentName string) gin.HandlerFunc {
	return h.bindAgentChat(agent, agentName, chatDeliverySync, true)
}

func (h *Handler) bindAgentChat(agent *adk.ChatModelAgent, agentName string, delivery chatDelivery, resume bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		started := time.Now()
		var req ChatRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			logChatError(agentName, chatModeLabel(false), "", err)
			c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
			return
		}

		ctx := c.Request.Context()
		var cpID string
		if resume {
			cpID = c.Param("checkpointID")
		} else {
			cpID = "cp_" + uuid.New().String()
		}

		stream := delivery == chatDeliveryAuto && c.Query("stream") == "true"
		mode := chatModeLabel(stream)
		logChatStart(c, agentName, mode, cpID, len(req.Message))

		runner := adk.NewRunner(ctx, adk.RunnerConfig{
			Agent:           agent,
			CheckPointStore: h.checkpoint,
			EnableStreaming: stream,
		})

		var iter *adk.AsyncIterator[*adk.AgentEvent]
		if resume {
			targets := map[string]any{}
			if req.InterruptID != "" {
				targets[req.InterruptID] = req.Message
			}
			var err error
			iter, err = runner.ResumeWithParams(ctx, cpID, &adk.ResumeParams{
				Targets: targets,
			})
			if err != nil {
				logChatError(agentName, mode, cpID, fmt.Errorf("resume failed: %w", err))
				c.JSON(http.StatusInternalServerError, gin.H{"code": -5, "message": fmt.Sprintf("resume failed: %v", err)})
				return
			}
		} else {
			messages := []adk.Message{schema.UserMessage(req.Message)}
			iter = runner.Run(ctx, messages, adk.WithCheckPointID(cpID))
		}

		if stream {
			h.processEventsSSE(c, iter, cpID, agentName, started)
			return
		}
		h.processEventsSync(c, iter, cpID, agentName, started)
	}
}

// ============ 事件处理 ============

// processEventsSync 同步收集所有事件后返回单个 JSON（与 grapery 信封一致）。
func (h *Handler) processEventsSync(c *gin.Context, iter *adk.AsyncIterator[*adk.AgentEvent], checkpointID, agentName string, started time.Time) {
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
			logChatAgentError(agentName, "sync", checkpointID, started, event.Err)
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

	cleanMessage, structured := extractStructuredPayload(finalMessage)
	resp := ChatResponse{
		Message:      cleanMessage,
		Finished:     !interrupted,
		Interrupted:  interrupted,
		Question:     question,
		InterruptID:  interruptID,
		CheckPointID: checkpointID,
		Structured:   structured,
	}
	logChatComplete(agentName, "sync", checkpointID, resp, started)
	c.JSON(http.StatusOK, gin.H{
		"code": 1, "message": "success",
		"data": resp,
	})
}

// processEventsSSE 以 Server-Sent Events 流式推送事件
func (h *Handler) processEventsSSE(c *gin.Context, iter *adk.AsyncIterator[*adk.AgentEvent], checkpointID, agentName string, started time.Time) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	writeSSE := func(event string, data any) {
		jsonData, err := json.Marshal(data)
		if err != nil {
			log.Printf("[agent-chat] sse_marshal_error agent=%s checkpoint=%s event=%s err=%v", agentName, checkpointID, event, err)
			jsonData = []byte(`{"error":"internal marshal error"}`)
		}
		c.SSEvent(event, string(jsonData))
		c.Writer.Flush()
	}

	writeSSE("start", gin.H{"checkpointId": checkpointID})
	logSSEEvent(agentName, checkpointID, "start", "")

	for {
		event, ok := iter.Next()
		if !ok {
			break
		}

		if event.Err != nil {
			logChatAgentError(agentName, "sse", checkpointID, started, event.Err)
			writeSSE("error", gin.H{"error": event.Err.Error()})
			return
		}

		if event.Action != nil && event.Action.Interrupted != nil {
			info := event.Action.Interrupted
			resp := ChatResponse{
				Interrupted:  true,
				Question:     extractQuestion(info),
				InterruptID:  extractInterruptID(info),
				CheckPointID: checkpointID,
			}
			logSSEEvent(agentName, checkpointID, "interrupt", sseDetail(resp))
			writeSSE("interrupt", resp)
			return
		}

		if event.Output != nil && event.Output.MessageOutput != nil {
			mv := event.Output.MessageOutput
			if mv.Message != nil {
				cleanMessage, structured := extractStructuredPayload(mv.Message.Content)
				resp := ChatResponse{
					Message:      cleanMessage,
					CheckPointID: checkpointID,
					Structured:   structured,
				}
				logSSEEvent(agentName, checkpointID, "message", sseDetail(resp))
				writeSSE("message", resp)
			}
		}
	}

	done := ChatResponse{Finished: true, CheckPointID: checkpointID}
	logChatComplete(agentName, "sse", checkpointID, done, started)
	writeSSE("done", done)
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

const (
	structuredStartMarker = "[[voyager_structured]]"
	structuredEndMarker   = "[[/voyager_structured]]"
)

func extractStructuredPayload(message string) (string, any) {
	start := strings.Index(message, structuredStartMarker)
	if start < 0 {
		return message, nil
	}
	payloadStart := start + len(structuredStartMarker)
	endRel := strings.Index(message[payloadStart:], structuredEndMarker)
	if endRel < 0 {
		return message, nil
	}
	end := payloadStart + endRel
	raw := strings.TrimSpace(message[payloadStart:end])
	var structured any
	if err := json.Unmarshal([]byte(raw), &structured); err != nil {
		return message, nil
	}
	clean := strings.TrimSpace(message[:start] + message[end+len(structuredEndMarker):])
	return clean, structured
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
