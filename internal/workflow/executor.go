package workflow

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/grapestree/fgrapery/grapery-agent/internal/domain"
)

type Activity func(ctx context.Context, input map[string]any, config map[string]any) (map[string]any, error)

type ActivityRegistry struct {
	mu         sync.RWMutex
	activities map[string]Activity
}

func NewActivityRegistry() *ActivityRegistry {
	return &ActivityRegistry{activities: make(map[string]Activity)}
}

func (r *ActivityRegistry) Register(name string, activity Activity) error {
	name = strings.TrimSpace(name)
	if name == "" || activity == nil {
		return errors.New("activity name and implementation are required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.activities[name]; exists {
		return fmt.Errorf("activity already registered: %s", name)
	}
	r.activities[name] = activity
	return nil
}

func (r *ActivityRegistry) get(name string) (Activity, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	activity, ok := r.activities[name]
	return activity, ok
}

type CompiledWorkflow struct {
	Release *domain.WorkflowRelease
	Batches [][]domain.WorkflowNode
}

// Compile validates the graph and produces deterministic topological batches.
// Nodes in one batch may execute concurrently in a later durable scheduler.
func Compile(release *domain.WorkflowRelease, registry *ActivityRegistry) (*CompiledWorkflow, error) {
	if release == nil || release.ID == "" || release.Key == "" || release.Version <= 0 {
		return nil, errors.New("valid workflow release is required")
	}
	if release.Status != "released" && release.Status != "active" {
		return nil, fmt.Errorf("workflow release %s is not executable", release.ID)
	}
	nodes := release.Definition.Nodes
	if len(nodes) == 0 {
		return nil, errors.New("workflow contains no nodes")
	}
	byID := make(map[string]domain.WorkflowNode, len(nodes))
	indegree := make(map[string]int, len(nodes))
	children := make(map[string][]string, len(nodes))
	for _, node := range nodes {
		if node.ID == "" {
			return nil, errors.New("workflow contains an empty node id")
		}
		if _, exists := byID[node.ID]; exists {
			return nil, fmt.Errorf("duplicate workflow node id: %s", node.ID)
		}
		if node.Type != "activity" && node.Type != "persist" {
			return nil, fmt.Errorf("workflow node type %s is not executable in runtime v1", node.Type)
		}
		if registry != nil {
			if _, ok := registry.get(node.Activity); !ok {
				return nil, fmt.Errorf("workflow activity is not registered: %s", node.Activity)
			}
		}
		byID[node.ID] = node
	}
	for _, node := range nodes {
		seen := map[string]bool{}
		for _, dependency := range node.DependsOn {
			if _, ok := byID[dependency]; !ok {
				return nil, fmt.Errorf("node %s depends on unknown node %s", node.ID, dependency)
			}
			if dependency == node.ID {
				return nil, fmt.Errorf("node %s depends on itself", node.ID)
			}
			if !seen[dependency] {
				seen[dependency] = true
				indegree[node.ID]++
				children[dependency] = append(children[dependency], node.ID)
			}
		}
	}
	ready := make([]string, 0)
	for id := range byID {
		if indegree[id] == 0 {
			ready = append(ready, id)
		}
	}
	var batches [][]domain.WorkflowNode
	visited := 0
	for len(ready) > 0 {
		sort.Strings(ready)
		current := ready
		ready = nil
		batch := make([]domain.WorkflowNode, 0, len(current))
		for _, id := range current {
			batch = append(batch, byID[id])
			visited++
			for _, child := range children[id] {
				indegree[child]--
				if indegree[child] == 0 {
					ready = append(ready, child)
				}
			}
		}
		batches = append(batches, batch)
	}
	if visited != len(nodes) {
		return nil, errors.New("workflow graph contains a cycle")
	}
	return &CompiledWorkflow{Release: release, Batches: batches}, nil
}

type ExecutionResult struct {
	NodeOutputs map[string]map[string]any `json:"nodeOutputs"`
}

type ExecutionOptions struct {
	// NodeOutputs contains outputs restored from a durable checkpoint. Nodes
	// present in the map are not executed again.
	NodeOutputs map[string]map[string]any
	// AttemptCounts is restored from the durable checkpoint. BeforeNodeAttempt
	// must persist the new count before the activity performs side effects.
	AttemptCounts     map[string]int
	BeforeNodeAttempt func(node domain.WorkflowNode, attempt int) error
	AfterNode         func(node domain.WorkflowNode, outputs map[string]map[string]any) error
	OnNodeRetry       func(node domain.WorkflowNode, nextAttempt int, err error)
	RetryDelay        func(nextAttempt int) time.Duration
}

// Execute runs v1 activity/persist nodes. The executor applies release retry
// policy; checkpoint persistence remains owned by the caller. Every Activity
// must therefore be idempotent for its run/node key.
func Execute(ctx context.Context, compiled *CompiledWorkflow, registry *ActivityRegistry, input map[string]any) (*ExecutionResult, error) {
	return ExecuteWithOptions(ctx, compiled, registry, input, ExecutionOptions{})
}

func ExecuteWithOptions(ctx context.Context, compiled *CompiledWorkflow, registry *ActivityRegistry, input map[string]any, options ExecutionOptions) (*ExecutionResult, error) {
	if compiled == nil || registry == nil {
		return nil, errors.New("compiled workflow and activity registry are required")
	}
	result := &ExecutionResult{NodeOutputs: cloneNodeOutputs(options.NodeOutputs)}
	for _, batch := range compiled.Batches {
		// Runtime v1 executes deterministically. Parallel scheduling is introduced
		// only after node attempts/checkpoints are wired to the durable run store.
		for _, node := range batch {
			if _, completed := result.NodeOutputs[node.ID]; completed {
				continue
			}
			activity, ok := registry.get(node.Activity)
			if !ok {
				return nil, fmt.Errorf("activity unavailable at execution: %s", node.Activity)
			}
			nodeInput := mergeNodeInput(input, node.DependsOn, result.NodeOutputs)
			maxAttempts := compiled.Release.Policies.MaxAttempts
			if maxAttempts <= 0 {
				maxAttempts = 1
			}
			completedAttempts := options.AttemptCounts[node.ID]
			if completedAttempts >= maxAttempts {
				return nil, fmt.Errorf("workflow node %s exhausted %d attempt(s) before recovery", node.ID, maxAttempts)
			}
			var output map[string]any
			var err error
			for attempt := completedAttempts + 1; attempt <= maxAttempts; attempt++ {
				if options.BeforeNodeAttempt != nil {
					if err := options.BeforeNodeAttempt(node, attempt); err != nil {
						return nil, fmt.Errorf("checkpoint workflow node %s attempt %d: %w", node.ID, attempt, err)
					}
				}
				output, err = activity(ctx, nodeInput, node.Config)
				if err == nil {
					break
				}
				if attempt == maxAttempts {
					return nil, fmt.Errorf("workflow node %s failed after %d attempt(s): %w", node.ID, attempt, err)
				}
				nextAttempt := attempt + 1
				if options.OnNodeRetry != nil {
					options.OnNodeRetry(node, nextAttempt, err)
				}
				delay := workflowRetryDelay(nextAttempt)
				if options.RetryDelay != nil {
					delay = options.RetryDelay(nextAttempt)
				}
				if delay > 0 {
					timer := time.NewTimer(delay)
					select {
					case <-ctx.Done():
						timer.Stop()
						return nil, fmt.Errorf("workflow node %s retry cancelled: %w", node.ID, ctx.Err())
					case <-timer.C:
					}
				}
			}
			result.NodeOutputs[node.ID] = output
			if options.AfterNode != nil {
				if err := options.AfterNode(node, cloneNodeOutputs(result.NodeOutputs)); err != nil {
					return nil, fmt.Errorf("checkpoint workflow node %s: %w", node.ID, err)
				}
			}
		}
	}
	return result, nil
}

func workflowRetryDelay(nextAttempt int) time.Duration {
	seconds := 1 << max(nextAttempt-2, 0)
	if seconds > 30 {
		seconds = 30
	}
	return time.Duration(seconds) * time.Second
}

func cloneNodeOutputs(input map[string]map[string]any) map[string]map[string]any {
	out := make(map[string]map[string]any, len(input))
	for nodeID, values := range input {
		cloned := make(map[string]any, len(values))
		for key, value := range values {
			cloned[key] = value
		}
		out[nodeID] = cloned
	}
	return out
}

func mergeNodeInput(root map[string]any, dependencies []string, outputs map[string]map[string]any) map[string]any {
	input := make(map[string]any, len(root)+1)
	for key, value := range root {
		input[key] = value
	}
	upstream := make(map[string]any, len(dependencies))
	for _, dependency := range dependencies {
		upstream[dependency] = outputs[dependency]
	}
	input["upstream"] = upstream
	return input
}
