package http

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	"github.com/gin-gonic/gin"
)

func TestProcessEventsSSEConsumesMessageStreamAndPersistsAssistant(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest("POST", "/api/v1/agent/fragment/chat?stream=true", nil)

	checkpoint := NewInMemoryCheckPointStore()
	handler := &Handler{sessions: newChatSessionStore(checkpoint, 10)}
	iterator, generator := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	stream := schema.StreamReaderFromArray([]*schema.Message{
		schema.AssistantMessage("hello", nil),
		schema.AssistantMessage(" world", nil),
		{Role: schema.Assistant, ResponseMeta: &schema.ResponseMeta{Usage: &schema.TokenUsage{TotalTokens: 2}}},
	})
	generator.Send(adk.EventFromMessage(nil, stream, schema.Assistant, ""))
	generator.Close()

	const sessionKey = "eino-session:user:fragment:sse-test"
	handler.processEventsSSE(ctx, iterator, "cp-test", "fragment", "sse-test", sessionKey, time.Now())

	body := recorder.Body.String()
	if !strings.Contains(body, `"message":"hello"`) || !strings.Contains(body, `"message":" world"`) {
		t.Fatalf("stream chunks were not emitted: %s", body)
	}
	if strings.Count(body, "event:message") != 2 {
		t.Fatalf("usage-only chunk should not emit an empty message event: %s", body)
	}
	if !strings.Contains(body, "event:done") {
		t.Fatalf("done event missing: %s", body)
	}

	messages, err := handler.sessions.load(context.Background(), sessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 || messages[0].Content != "hello world" {
		t.Fatalf("assistant stream was not persisted: %#v", messages)
	}
}

func TestProcessEventsSSEPersistsInterruptQuestion(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest("POST", "/api/v1/agent/fragment/chat?stream=true", nil)

	checkpoint := NewInMemoryCheckPointStore()
	handler := &Handler{sessions: newChatSessionStore(checkpoint, 10)}
	iterator, generator := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	generator.Send(&adk.AgentEvent{Action: &adk.AgentAction{Interrupted: &adk.InterruptInfo{Data: "请选择一个方向"}}})
	generator.Close()

	const sessionKey = "eino-session:user:fragment:interrupt-test"
	handler.processEventsSSE(ctx, iterator, "cp-test", "fragment", "interrupt-test", sessionKey, time.Now())

	if !strings.Contains(recorder.Body.String(), `"question":"请选择一个方向"`) {
		t.Fatalf("interrupt event missing question: %s", recorder.Body.String())
	}
	messages, err := handler.sessions.load(context.Background(), sessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 || messages[0].Role != schema.Assistant || messages[0].Content != "请选择一个方向" {
		t.Fatalf("interrupt question was not persisted: %#v", messages)
	}
}
