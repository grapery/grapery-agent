package http

import (
	"fmt"
	"log"
	"time"

	"github.com/gin-gonic/gin"
)

func chatModeLabel(stream bool) string {
	if stream {
		return "sse"
	}
	return "sync"
}

func logChatStart(c *gin.Context, agent, mode, checkpointID string, messageLen int) {
	userID, _ := c.Get("agentUserID")
	scope, _ := c.Get("agentScope")
	log.Printf(
		"[agent-chat] start agent=%s mode=%s checkpoint=%s userID=%v scope=%v msgLen=%d path=%s",
		agent, mode, checkpointID, userID, scope, messageLen, c.Request.URL.Path,
	)
}

func logChatComplete(agent, mode, checkpointID string, resp ChatResponse, started time.Time) {
	log.Printf(
		"[agent-chat] complete agent=%s mode=%s checkpoint=%s duration=%s interrupted=%v finished=%v msgLen=%d hasStructured=%v",
		agent, mode, checkpointID, time.Since(started).Round(time.Millisecond),
		resp.Interrupted, resp.Finished, len(resp.Message), resp.Structured != nil,
	)
}

func logChatAgentError(agent, mode, checkpointID string, started time.Time, err error) {
	log.Printf(
		"[agent-chat] agent_error agent=%s mode=%s checkpoint=%s duration=%s err=%v",
		agent, mode, checkpointID, time.Since(started).Round(time.Millisecond), err,
	)
}

func logChatError(agent, mode, checkpointID string, err error) {
	log.Printf("[agent-chat] request_error agent=%s mode=%s checkpoint=%s err=%v", agent, mode, checkpointID, err)
}

func logSSEEvent(agent, checkpointID, event string, detail string) {
	if detail == "" {
		log.Printf("[agent-chat] sse agent=%s checkpoint=%s event=%s", agent, checkpointID, event)
		return
	}
	log.Printf("[agent-chat] sse agent=%s checkpoint=%s event=%s %s", agent, checkpointID, event, detail)
}

func sseDetail(resp ChatResponse) string {
	return fmt.Sprintf("msgLen=%d interrupted=%v finished=%v hasStructured=%v",
		len(resp.Message), resp.Interrupted, resp.Finished, resp.Structured != nil)
}
