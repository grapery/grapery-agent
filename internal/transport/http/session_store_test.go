package http

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestChatSessionStorePersistsAndTrimsHistory(t *testing.T) {
	checkpoint := NewInMemoryCheckPointStore()
	store := newChatSessionStore(checkpoint, 3)
	ctx := context.Background()
	key := "eino-session:user:fragment:test"
	unlock := store.lock(key)
	defer unlock()
	if err := store.append(ctx, key,
		schema.UserMessage("one"), schema.AssistantMessage("two", nil),
		schema.UserMessage("three"), schema.AssistantMessage("four", nil),
	); err != nil {
		t.Fatal(err)
	}
	messages, err := store.load(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 3 || messages[0].Content != "two" || messages[2].Content != "four" {
		t.Fatalf("unexpected bounded session history: %#v", messages)
	}
	if err := store.clear(ctx, key); err != nil {
		t.Fatal(err)
	}
	messages, err = store.load(ctx, key)
	if err != nil || len(messages) != 0 {
		t.Fatalf("session was not cleared: messages=%#v err=%v", messages, err)
	}
}
