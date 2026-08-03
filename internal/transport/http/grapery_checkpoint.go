package http

import (
	"context"

	"github.com/cloudwego/eino/adk"
	"github.com/grapestree/fgrapery/grapery-agent/internal/grapery_client"
)

type GraperyCheckPointStore struct{ client *grapery_client.Client }

func NewGraperyCheckPointStore(client *grapery_client.Client) *GraperyCheckPointStore {
	return &GraperyCheckPointStore{client: client}
}

func (s *GraperyCheckPointStore) Get(ctx context.Context, id string) ([]byte, bool, error) {
	return s.client.GetGenerationCheckpoint(ctx, id)
}

func (s *GraperyCheckPointStore) Set(ctx context.Context, id string, state []byte) error {
	return s.client.SaveGenerationCheckpoint(ctx, id, state)
}

var _ adk.CheckPointStore = (*GraperyCheckPointStore)(nil)
