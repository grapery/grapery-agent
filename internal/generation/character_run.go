package generation

import (
	"context"

	"github.com/grapestree/fgrapery/grapery-agent/internal/domain"
	"github.com/grapestree/fgrapery/grapery-agent/internal/grapery_client"
	"github.com/grapestree/fgrapery/grapery-agent/internal/runstore"
)

func (s *Service) StartCharacter(ctx context.Context, in domain.CharacterGenerateInput) (*domain.GenerationRun, error) {
	input := map[string]any{"storyId": in.StoryID, "prompt": in.Prompt, "name": in.Name}
	run, err := s.store.CreateRun(ctx, domain.RunKindCharacter, domain.AgentCharacterDesigner, in.Prompt, input)
	if err != nil {
		return nil, err
	}
	go s.executeCharacter(context.Background(), run.ID, in)
	return run, nil
}

func (s *Service) executeCharacter(ctx context.Context, runID string, in domain.CharacterGenerateInput) {
	ctx = runstore.ContextWithRunID(ctx, runID)
	s.markRunning(ctx, runID)

	attrsOut, err := tracedClientCall(ctx, s.store, "generate_character_attrs", map[string]any{"prompt": in.Prompt}, func(c context.Context) (map[string]any, error) {
		resp, err := s.client.GenerateCharacterAttrs(c, grapery_client.GenerateCharacterAttrsRequest{
			Prompt: in.Prompt,
			Name:   in.Name,
		})
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"description":     resp.Description,
			"personality":     resp.Personality,
			"background":      resp.Background,
			"appearance":      resp.Appearance,
			"dressPreference": resp.DressPreference,
		}, nil
	})
	if err != nil {
		_ = s.finishRun(ctx, runID, domain.RunStatusFailed, nil, domain.ContentRef{}, 0, err.Error())
		return
	}

	output := map[string]any{"attributes": attrsOut}
	content := domain.ContentRef{}

	if in.CreateRecord {
		name := in.Name
		if name == "" {
			name = "角色"
		}
		charOut, createErr := tracedClientCall(ctx, s.store, "create_character", map[string]any{"storyId": in.StoryID, "name": name}, func(c context.Context) (map[string]any, error) {
			resp, err := s.client.CreateCharacter(c, grapery_client.CreateCharacterRequest{
				Name:            name,
				StoryID:         in.StoryID,
				Description:     str(attrsOut["description"]),
				Personality:     str(attrsOut["personality"]),
				Background:      str(attrsOut["background"]),
				Appearance:      str(attrsOut["appearance"]),
				DressPreference: str(attrsOut["dressPreference"]),
				SourceType:      "ai",
				ReferenceImage:  in.ReferenceImage,
				NeedsPortrait:   in.GeneratePortrait,
			})
			if err != nil {
				return nil, err
			}
			return map[string]any{"id": resp.ID, "name": resp.Name}, nil
		})
		if createErr != nil {
			_ = s.finishRun(ctx, runID, domain.RunStatusFailed, output, content, 0, createErr.Error())
			return
		}
		content.CharacterID = str(charOut["id"])
		output["characterId"] = content.CharacterID

		if in.GenerateAvatar && content.CharacterID != "" {
			avOut, _ := tracedClientCall(ctx, s.store, "generate_avatar", map[string]any{"characterId": content.CharacterID}, func(c context.Context) (map[string]any, error) {
				resp, err := s.client.GenerateCharacterAvatar(c, content.CharacterID, grapery_client.GenerateAvatarRequest{AspectRatio: "1:1"})
				if err != nil {
					return nil, err
				}
				return map[string]any{"avatarUrl": resp.AvatarURL}, nil
			})
			output["avatar"] = avOut
		}
		if in.GeneratePortrait && content.CharacterID != "" {
			pOut, _ := tracedClientCall(ctx, s.store, "generate_portrait", map[string]any{"characterId": content.CharacterID}, func(c context.Context) (map[string]any, error) {
				resp, err := s.client.GenerateCharacterPortrait(c, content.CharacterID, grapery_client.GeneratePortraitRequest{
					ReferenceImage: in.ReferenceImage,
				})
				if err != nil {
					return nil, err
				}
				return map[string]any{"portraitUrl": resp.PortraitURL}, nil
			})
			output["portrait"] = pOut
		}
	}

	_ = s.finishRun(ctx, runID, domain.RunStatusSucceeded, output, content, 0, "")
	if run, ok := s.store.GetRun(ctx, runID); ok {
		s.traceArtifact(ctx, run)
	}
}
