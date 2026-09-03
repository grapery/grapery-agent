package http

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/grapestree/fgrapery/grapery-agent/internal/agentauth"
	"github.com/grapestree/fgrapery/grapery-agent/internal/agents"
	"github.com/grapestree/fgrapery/grapery-agent/internal/config"
	"github.com/grapestree/fgrapery/grapery-agent/internal/grapery_client"
	"github.com/grapestree/fgrapery/grapery-agent/internal/observability"
	"github.com/grapestree/fgrapery/grapery-agent/internal/runstore"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Handler 管理 HTTP 路由和 Agent 调用
type Handler struct {
	registry      *agents.AgentRegistry
	client        *grapery_client.Client
	checkpoint    adk.CheckPointStore
	genHandler    *GenerationHandler
	agentAuth     agentAuthDeps
	sessions      *chatSessionStore
	observer      *observability.Collector
	observerToken string
}

func NewHandler(registry *agents.AgentRegistry, client *grapery_client.Client, checkpoint adk.CheckPointStore, genHandler *GenerationHandler, agentAuth config.AgentAuthConfig, sessionMaxMessages int) *Handler {
	return &Handler{
		registry:      registry,
		client:        client,
		checkpoint:    checkpoint,
		genHandler:    genHandler,
		agentAuth:     newAgentAuthDeps(agentAuth, client),
		sessions:      newChatSessionStore(checkpoint, sessionMaxMessages),
		observer:      observability.NewCollector(200),
		observerToken: strings.TrimSpace(agentAuth.ObservabilityToken),
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
		api.DELETE("/sessions/:agent/:sessionID", h.clearSession)
		api.GET("/observability", h.getObservability)
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
	SessionID   string `json:"sessionId,omitempty" binding:"omitempty,max=128"`
}

type ChatResponse struct {
	Message      string `json:"message"`
	Finished     bool   `json:"finished"`
	Interrupted  bool   `json:"interrupted,omitempty"`
	Question     string `json:"question,omitempty"`
	InterruptID  string `json:"interruptId,omitempty"`
	CheckPointID string `json:"checkpointId,omitempty"`
	Structured   any    `json:"structured,omitempty"`
	SessionID    string `json:"sessionId,omitempty"`
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
		sessionKey, err := h.resolveSessionKey(c, agentName, req.SessionID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
			return
		}
		if sessionKey != "" {
			unlock := h.sessions.lock(sessionKey)
			defer unlock()
		}
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
		callbackOption := adk.WithCallbacks(h.observer.Handler(agentName, cpID))

		var iter *adk.AsyncIterator[*adk.AgentEvent]
		if resume {
			targets := map[string]any{}
			if req.InterruptID != "" {
				targets[req.InterruptID] = req.Message
			}
			var err error
			iter, err = runner.ResumeWithParams(ctx, cpID, &adk.ResumeParams{
				Targets: targets,
			}, callbackOption)
			if err != nil {
				logChatError(agentName, mode, cpID, fmt.Errorf("resume failed: %w", err))
				c.JSON(http.StatusInternalServerError, gin.H{"code": -5, "message": fmt.Sprintf("resume failed: %v", err)})
				return
			}
			if err := h.sessions.append(ctx, sessionKey, schema.UserMessage(req.Message)); err != nil {
				// Resume has already started, so keep consuming its iterator rather
				// than abandoning a live Agent run because auxiliary memory failed.
				log.Printf("[agent-chat] session_save_error agent=%s session=%s err=%v", agentName, req.SessionID, err)
			}
		} else {
			messages, err := h.sessions.load(ctx, sessionKey)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"code": -5, "message": "load session: " + err.Error()})
				return
			}
			userMessage := schema.UserMessage(req.Message)
			messages = append(messages, userMessage)
			if err := h.sessions.append(ctx, sessionKey, userMessage); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"code": -5, "message": "save session: " + err.Error()})
				return
			}
			iter = runner.Run(ctx, messages, adk.WithCheckPointID(cpID), callbackOption)
		}

		if stream {
			h.processEventsSSE(c, iter, cpID, agentName, req.SessionID, sessionKey, started)
			return
		}
		h.processEventsSync(c, iter, cpID, agentName, req.SessionID, sessionKey, started)
	}
}

// ============ 事件处理 ============

// processEventsSync 同步收集所有事件后返回单个 JSON（与 grapery 信封一致）。
func (h *Handler) processEventsSync(c *gin.Context, iter *adk.AsyncIterator[*adk.AgentEvent], checkpointID, agentName, sessionID, sessionKey string, started time.Time) {
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
	if interrupted && strings.TrimSpace(question) != "" {
		if err := h.sessions.append(c.Request.Context(), sessionKey, schema.AssistantMessage(question, nil)); err != nil {
			log.Printf("[agent-chat] session_save_error agent=%s session=%s err=%v", agentName, sessionID, err)
		}
	} else if cleanMessage != "" {
		if err := h.sessions.append(c.Request.Context(), sessionKey, schema.AssistantMessage(cleanMessage, nil)); err != nil {
			log.Printf("[agent-chat] session_save_error agent=%s session=%s err=%v", agentName, sessionID, err)
		}
	}
	resp := ChatResponse{
		Message:      cleanMessage,
		Finished:     !interrupted,
		Interrupted:  interrupted,
		Question:     question,
		InterruptID:  interruptID,
		CheckPointID: checkpointID,
		Structured:   structured,
		SessionID:    sessionID,
	}
	logChatComplete(agentName, "sync", checkpointID, resp, started)
	c.JSON(http.StatusOK, gin.H{
		"code": 1, "message": "success",
		"data": resp,
	})
}

// processEventsSSE 以 Server-Sent Events 流式推送事件
func (h *Handler) processEventsSSE(c *gin.Context, iter *adk.AsyncIterator[*adk.AgentEvent], checkpointID, agentName, sessionID, sessionKey string, started time.Time) {
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
	var assistantText strings.Builder
	emitMessage := func(message *schema.Message, role schema.RoleType) {
		if message == nil {
			return
		}
		cleanMessage, structured := extractStructuredPayload(message.Content)
		if role == schema.Assistant || message.Role == schema.Assistant {
			assistantText.WriteString(message.Content)
		}
		// Some providers send a final usage-only stream chunk. Preserve its
		// callback metadata, but do not expose an empty chat event to clients.
		if cleanMessage == "" && structured == nil {
			return
		}
		resp := ChatResponse{Message: cleanMessage, CheckPointID: checkpointID, Structured: structured, SessionID: sessionID}
		logSSEEvent(agentName, checkpointID, "message", sseDetail(resp))
		writeSSE("message", resp)
	}

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
			question := extractQuestion(info)
			if strings.TrimSpace(question) != "" {
				if err := h.sessions.append(c.Request.Context(), sessionKey, schema.AssistantMessage(question, nil)); err != nil {
					log.Printf("[agent-chat] session_save_error agent=%s session=%s err=%v", agentName, sessionID, err)
				}
			}
			resp := ChatResponse{
				Interrupted:  true,
				Question:     question,
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
				emitMessage(mv.Message, mv.Role)
			}
			if mv.MessageStream != nil {
				for {
					message, err := mv.MessageStream.Recv()
					if errors.Is(err, io.EOF) {
						break
					}
					if err != nil {
						mv.MessageStream.Close()
						logChatAgentError(agentName, "sse", checkpointID, started, err)
						writeSSE("error", gin.H{"error": err.Error()})
						return
					}
					emitMessage(message, mv.Role)
				}
				mv.MessageStream.Close()
			}
		}
	}

	cleanAssistant, structured := extractStructuredPayload(strings.TrimSpace(assistantText.String()))
	if message := strings.TrimSpace(cleanAssistant); message != "" {
		if err := h.sessions.append(c.Request.Context(), sessionKey, schema.AssistantMessage(message, nil)); err != nil {
			log.Printf("[agent-chat] session_save_error agent=%s session=%s err=%v", agentName, sessionID, err)
		}
	}
	done := ChatResponse{Finished: true, CheckPointID: checkpointID, SessionID: sessionID, Structured: structured}
	logChatComplete(agentName, "sse", checkpointID, done, started)
	writeSSE("done", done)
}

func (h *Handler) resolveSessionKey(c *gin.Context, agentName, sessionID string) (string, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return "", nil
	}
	for _, r := range sessionID {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.') {
			return "", errors.New("sessionId contains unsupported characters")
		}
	}
	owner := ""
	if claims, ok := agentauth.ClaimsFromContext(c.Request.Context()); ok {
		owner = strings.TrimSpace(claims.UserID)
	}
	if owner == "" {
		if token, ok := grapery_client.AuthTokenFromContext(c.Request.Context()); ok {
			digest := sha256.Sum256([]byte(token))
			owner = fmt.Sprintf("jwt-%x", digest[:8])
		}
	}
	if owner == "" {
		return "", errors.New("sessionId requires authenticated user identity")
	}
	return "eino-session:" + owner + ":" + agentName + ":" + sessionID, nil
}

func (h *Handler) clearSession(c *gin.Context) {
	key, err := h.resolveSessionKey(c, strings.TrimSpace(c.Param("agent")), c.Param("sessionID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}
	unlock := h.sessions.lock(key)
	defer unlock()
	if err := h.sessions.clear(c.Request.Context(), key); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": -5, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 1, "message": "success"})
}

func (h *Handler) getObservability(c *gin.Context) {
	provided := strings.TrimSpace(c.GetHeader("X-Agent-Observability-Token"))
	if h.observerToken == "" || subtle.ConstantTimeCompare([]byte(provided), []byte(h.observerToken)) != 1 {
		c.JSON(http.StatusNotFound, gin.H{"code": -4, "message": "not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 1, "message": "success", "data": h.observer.Snapshot()})
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
