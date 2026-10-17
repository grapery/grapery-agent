package generation

import (
	"context"
	"fmt"
	"strings"

	"github.com/grapestree/fgrapery/grapery-agent/internal/domain"
	"github.com/grapestree/fgrapery/grapery-agent/internal/runstore"
)

// Only accepted backend data can serve as an outcome artifact. Progress=100 or
// a successful provider URL is insufficient evidence that a story was saved.
func (s *Service) storyboardAcceptedArtifact(ctx context.Context, id string, requireImages bool) (map[string]any, error) {
	board, err := s.client.GetStoryboard(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("verify accepted storyboard: %w", err)
	}
	if board == nil || board.ID != id || strings.TrimSpace(board.Content) == "" || len(board.Scenes) == 0 || strings.TrimSpace(board.ContentRevision) == "" {
		return nil, fmt.Errorf("accepted storyboard verification failed: missing content, scenes or revision")
	}
	scenes := make([]map[string]any, 0, len(board.Scenes))
	for _, scene := range board.Scenes {
		if scene.ID == "" || strings.TrimSpace(scene.Title) == "" || strings.TrimSpace(scene.Description) == "" {
			return nil, fmt.Errorf("accepted storyboard verification failed: incomplete scene")
		}
		if requireImages && strings.TrimSpace(scene.Image) == "" {
			return nil, fmt.Errorf("accepted storyboard verification failed: missing image for scene %s", scene.ID)
		}
		scenes = append(scenes, map[string]any{"id": scene.ID, "sequence": scene.Sequence, "title": scene.Title, "description": scene.Description, "image": scene.Image, "continuityNote": scene.ContinuityNote})
	}
	artifact := map[string]any{"storyboardId": board.ID, "storyId": board.StoryID, "title": board.Title, "acceptedContent": board.Content, "contentRevision": board.ContentRevision, "scenes": scenes, "sceneCount": len(scenes)}
	// The final workflow snapshot clones run.Output. Keep the same verified
	// artifact at the top level, independently of node ordering in nodeOutputs.
	if runID, ok := runstore.RunIDFromContext(ctx); ok {
		if err := s.store.UpdateRun(ctx, runID, func(run *domain.GenerationRun) {
			if run.Output == nil {
				run.Output = map[string]any{}
			}
			run.Output["artifact"] = artifact
		}); err != nil {
			return nil, fmt.Errorf("save accepted storyboard artifact: %w", err)
		}
	}
	return artifact, nil
}
