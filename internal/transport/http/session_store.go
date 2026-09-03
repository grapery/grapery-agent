package http

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

const sessionRecordVersion = 1

type sessionMessage struct {
	Role    schema.RoleType `json:"role"`
	Content string          `json:"content"`
}

type sessionRecord struct {
	Version   int              `json:"version"`
	Messages  []sessionMessage `json:"messages"`
	UpdatedAt time.Time        `json:"updatedAt"`
}

// chatSessionStore persists bounded conversation history through the same
// durable backend used by Eino checkpoints. Locks serialize turns for one
// session so concurrent requests cannot overwrite each other's history.
type chatSessionStore struct {
	checkpoint  adk.CheckPointStore
	maxMessages int
	locks       sync.Map
}

func newChatSessionStore(checkpoint adk.CheckPointStore, maxMessages int) *chatSessionStore {
	if maxMessages <= 0 {
		maxMessages = 40
	}
	return &chatSessionStore{checkpoint: checkpoint, maxMessages: maxMessages}
}

func (s *chatSessionStore) lock(key string) func() {
	value, _ := s.locks.LoadOrStore(key, &sync.Mutex{})
	mu := value.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

func (s *chatSessionStore) load(ctx context.Context, key string) ([]adk.Message, error) {
	if s == nil || s.checkpoint == nil || strings.TrimSpace(key) == "" {
		return nil, nil
	}
	raw, found, err := s.checkpoint.Get(ctx, key)
	if err != nil || !found || len(raw) == 0 {
		return nil, err
	}
	var record sessionRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		return nil, err
	}
	if record.Version != sessionRecordVersion {
		return nil, errors.New("unsupported agent session version")
	}
	messages := make([]adk.Message, 0, len(record.Messages))
	for _, item := range record.Messages {
		content := strings.TrimSpace(item.Content)
		if content == "" || (item.Role != schema.User && item.Role != schema.Assistant) {
			continue
		}
		messages = append(messages, &schema.Message{Role: item.Role, Content: content})
	}
	return messages, nil
}

func (s *chatSessionStore) append(ctx context.Context, key string, messages ...adk.Message) error {
	if s == nil || s.checkpoint == nil || strings.TrimSpace(key) == "" {
		return nil
	}
	existing, err := s.load(ctx, key)
	if err != nil {
		return err
	}
	existing = append(existing, messages...)
	if len(existing) > s.maxMessages {
		existing = existing[len(existing)-s.maxMessages:]
	}
	record := sessionRecord{Version: sessionRecordVersion, UpdatedAt: time.Now().UTC()}
	for _, message := range existing {
		if message == nil || strings.TrimSpace(message.Content) == "" || (message.Role != schema.User && message.Role != schema.Assistant) {
			continue
		}
		record.Messages = append(record.Messages, sessionMessage{Role: message.Role, Content: strings.TrimSpace(message.Content)})
	}
	raw, err := json.Marshal(record)
	if err != nil {
		return err
	}
	return s.checkpoint.Set(ctx, key, raw)
}

func (s *chatSessionStore) clear(ctx context.Context, key string) error {
	if s == nil || s.checkpoint == nil || strings.TrimSpace(key) == "" {
		return nil
	}
	return s.checkpoint.Set(ctx, key, nil)
}
