package runstore

import "context"

type runIDKey struct{}

// ContextWithRunID attaches a generation run ID for tool trace recording.
func ContextWithRunID(ctx context.Context, runID string) context.Context {
	return context.WithValue(ctx, runIDKey{}, runID)
}

// RunIDFromContext returns the active generation run ID, if any.
func RunIDFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(runIDKey{}).(string)
	return v, ok && v != ""
}
