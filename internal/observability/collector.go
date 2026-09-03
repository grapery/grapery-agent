package observability

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/callbacks"
	modelcomponent "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

const defaultRecentSpanLimit = 200

type ComponentStats struct {
	Started         int64 `json:"started"`
	Succeeded       int64 `json:"succeeded"`
	Failed          int64 `json:"failed"`
	TotalDurationMs int64 `json:"totalDurationMs"`
	TokensUsed      int64 `json:"tokensUsed"`
}

type Span struct {
	Agent        string    `json:"agent"`
	CheckpointID string    `json:"checkpointId,omitempty"`
	Component    string    `json:"component"`
	Name         string    `json:"name,omitempty"`
	Type         string    `json:"type,omitempty"`
	Status       string    `json:"status"`
	DurationMs   int64     `json:"durationMs"`
	TokensUsed   int       `json:"tokensUsed,omitempty"`
	Error        string    `json:"error,omitempty"`
	FinishedAt   time.Time `json:"finishedAt"`
}

type Snapshot struct {
	Components map[string]ComponentStats `json:"components"`
	Recent     []Span                    `json:"recent"`
}

type Collector struct {
	mu        sync.RWMutex
	stats     map[string]ComponentStats
	recent    []Span
	maxRecent int
}

func NewCollector(maxRecent int) *Collector {
	if maxRecent <= 0 {
		maxRecent = defaultRecentSpanLimit
	}
	return &Collector{stats: make(map[string]ComponentStats), maxRecent: maxRecent}
}

type startKey struct{}

func (c *Collector) Handler(agent, checkpointID string) callbacks.Handler {
	return callbacks.NewHandlerBuilder().
		OnStartFn(func(ctx context.Context, info *callbacks.RunInfo, _ callbacks.CallbackInput) context.Context {
			c.markStarted(componentName(info))
			return context.WithValue(ctx, startKey{}, time.Now())
		}).
		OnEndFn(func(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
			c.finish(agent, checkpointID, info, elapsed(ctx), modelTokens(output), nil)
			return ctx
		}).
		OnErrorFn(func(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
			c.finish(agent, checkpointID, info, elapsed(ctx), 0, err)
			return ctx
		}).
		OnEndWithStreamOutputFn(func(ctx context.Context, info *callbacks.RunInfo, output *schema.StreamReader[callbacks.CallbackOutput]) context.Context {
			if output == nil {
				c.finish(agent, checkpointID, info, elapsed(ctx), 0, errors.New("callback stream is nil"))
				return ctx
			}
			started := time.Now().Add(-elapsed(ctx))
			go func() {
				defer output.Close()
				tokens := 0
				var streamErr error
				for {
					item, err := output.Recv()
					if errors.Is(err, io.EOF) {
						break
					}
					if err != nil {
						streamErr = err
						break
					}
					tokens += modelTokens(item)
				}
				c.finish(agent, checkpointID, info, time.Since(started), tokens, streamErr)
			}()
			return ctx
		}).
		Build()
}

func elapsed(ctx context.Context) time.Duration {
	if started, ok := ctx.Value(startKey{}).(time.Time); ok {
		return time.Since(started)
	}
	return 0
}

func modelTokens(output callbacks.CallbackOutput) int {
	converted := modelcomponent.ConvCallbackOutput(output)
	if converted == nil {
		return 0
	}
	if converted.TokenUsage != nil {
		return converted.TokenUsage.TotalTokens
	}
	if converted.Message != nil && converted.Message.ResponseMeta != nil && converted.Message.ResponseMeta.Usage != nil {
		return converted.Message.ResponseMeta.Usage.TotalTokens
	}
	return 0
}

func componentName(info *callbacks.RunInfo) string {
	if info == nil || strings.TrimSpace(string(info.Component)) == "" {
		return "unknown"
	}
	return string(info.Component)
}

func (c *Collector) markStarted(component string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	stats := c.stats[component]
	stats.Started++
	c.stats[component] = stats
}

func (c *Collector) finish(agent, checkpointID string, info *callbacks.RunInfo, duration time.Duration, tokens int, err error) {
	component := componentName(info)
	span := Span{Agent: agent, CheckpointID: checkpointID, Component: component, Status: "succeeded", DurationMs: duration.Milliseconds(), TokensUsed: tokens, FinishedAt: time.Now().UTC()}
	if info != nil {
		span.Name, span.Type = info.Name, info.Type
	}
	if err != nil {
		span.Status = "failed"
		span.Error = truncate(err.Error(), 300)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	stats := c.stats[component]
	if err != nil {
		stats.Failed++
	} else {
		stats.Succeeded++
	}
	stats.TotalDurationMs += span.DurationMs
	stats.TokensUsed += int64(tokens)
	c.stats[component] = stats
	c.recent = append(c.recent, span)
	if len(c.recent) > c.maxRecent {
		c.recent = append([]Span(nil), c.recent[len(c.recent)-c.maxRecent:]...)
	}
}

func (c *Collector) Snapshot() Snapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := Snapshot{Components: make(map[string]ComponentStats, len(c.stats)), Recent: append([]Span(nil), c.recent...)}
	for key, value := range c.stats {
		result.Components[key] = value
	}
	return result
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}
