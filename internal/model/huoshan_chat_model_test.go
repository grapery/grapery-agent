package model

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cloudwego/eino/schema"
	"github.com/grapestree/fgrapery/grapery-agent/internal/config"
)

func TestHuoshanChatModelPreservesResponseMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"message": map[string]any{"role": "assistant", "content": "done"}, "finish_reason": "stop"}},
			"usage":   map[string]any{"prompt_tokens": 11, "completion_tokens": 7, "total_tokens": 18},
		})
	}))
	defer server.Close()
	model := NewHuoshanChatModel(config.EinoConfig{HuoshanAPIKey: "test", HuoshanBaseURL: server.URL, TextModel: "test-model", RequestTimeout: 5})
	message, err := model.Generate(context.Background(), []*schema.Message{schema.UserMessage("hello")})
	if err != nil {
		t.Fatal(err)
	}
	if message.ResponseMeta == nil || message.ResponseMeta.FinishReason != "stop" || message.ResponseMeta.Usage == nil || message.ResponseMeta.Usage.TotalTokens != 18 {
		t.Fatalf("response metadata was lost: %#v", message.ResponseMeta)
	}
}
