package generation

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/grapestree/fgrapery/grapery-agent/internal/characterutil"
	"github.com/grapestree/fgrapery/grapery-agent/internal/domain"
	"github.com/grapestree/fgrapery/grapery-agent/internal/grapery_client"
	"github.com/grapestree/fgrapery/grapery-agent/internal/runstore"
)

func (s *Service) StartCharacter(ctx context.Context, in domain.CharacterGenerateInput) (*domain.GenerationRun, error) {
	controls, err := controlsForGeneration(ctx, in.CreativeControls)
	if err != nil {
		return nil, err
	}
	in.CreativeControls = controls
	input := map[string]any{"creativeControls": controls, "clientRequestId": in.ClientRequestID, "storyId": in.StoryID, "prompt": in.Prompt, "name": in.Name}
	run, err := s.store.CreateRun(ctx, domain.RunKindCharacter, domain.AgentCharacterDesigner, in.Prompt, input)
	if err != nil {
		return nil, err
	}
	if run.Reused {
		return run, nil
	}
	go s.executeCharacter(context.WithoutCancel(ctx), run.ID, in)
	return run, nil
}

func (s *Service) executeCharacter(ctx context.Context, runID string, in domain.CharacterGenerateInput) {
	ctx = runstore.ContextWithRunID(ctx, runID)
	s.markRunning(ctx, runID)

	if in.UseAsyncTask {
		s.executeCharacterAsyncTask(ctx, runID, in)
		return
	}
	s.executeCharacterSync(ctx, runID, in)
}

func (s *Service) executeCharacterAsyncTask(ctx context.Context, runID string, in domain.CharacterGenerateInput) {
	sourceType := characterutil.ResolveGenTaskSourceType(in.SourceType, in.SourceFragmentID, in.SourceFragmentCharacterKey, in.Prompt)
	name := characterutil.ResolveCharacterName(in.Name, in.Prompt, sourceType)
	if characterutil.AsyncTaskNeedsName(in.Name, in.Prompt, sourceType) {
		_ = s.finishRun(ctx, runID, domain.RunStatusFailed, nil, domain.ContentRef{}, 0, "character name is required for async task (provide name or a descriptive prompt)")
		return
	}
	var suggestion *grapery_client.FragmentSuggestion
	if sourceType == characterutil.GenSourceFragment && in.SourceFragmentCharacterKey != "" {
		suggestion = &grapery_client.FragmentSuggestion{
			Key:  in.SourceFragmentCharacterKey,
			Name: name,
		}
	}
	taskOut, err := tracedClientCall(ctx, s.store, "start_character_gen_task", map[string]any{"storyId": in.StoryID}, func(c context.Context) (map[string]any, error) {
		task, err := s.client.StartCharacterGenTask(c, grapery_client.CharacterGenTaskRequest{CreativeControls: in.CreativeControls, WorkflowReleaseID: in.WorkflowReleaseID, WorkflowRunID: runID,
			StoryID:                    in.StoryID,
			SourceType:                 sourceType,
			Name:                       name,
			Prompt:                     in.Prompt,
			ReferenceImage:             in.ReferenceImage,
			SourceFragmentID:           in.SourceFragmentID,
			SourceFragmentCharacterKey: in.SourceFragmentCharacterKey,
			Suggestion:                 suggestion,
		})
		if err != nil {
			return nil, fmt.Errorf("start character gen task: %w", err)
		}
		return map[string]any{"taskId": task.ID, "status": task.Status}, nil
	})
	if err != nil {
		_ = s.finishRun(ctx, runID, domain.RunStatusFailed, nil, domain.ContentRef{}, 0, err.Error())
		return
	}

	taskID := str(taskOut["taskId"])
	content := domain.ContentRef{TaskID: taskID}
	_ = s.store.UpdateRun(ctx, runID, func(r *domain.GenerationRun) {
		r.Status = domain.RunStatusWaiting
		r.ContentIDs = content
	})

	deadline := time.Now().Add(generationPollTimeout(0))
	var final *grapery_client.CharacterGenTask
	for time.Now().Before(deadline) {
		task, pollErr := s.client.GetCharacterGenTask(ctx, taskID)
		if pollErr != nil {
			_ = s.finishRun(ctx, runID, domain.RunStatusFailed, nil, content, 0, pollErr.Error())
			return
		}
		switch task.Status {
		case "succeeded", "completed", "success", "partial":
			final = task
			goto doneAsync
		case "failed", "cancelled", "canceled":
			_ = s.finishRun(ctx, runID, domain.RunStatusFailed, map[string]any{"status": task.Status, "error": task.ErrorMessage}, content, 0, task.ErrorMessage)
			return
		}
		time.Sleep(3 * time.Second)
	}
	_ = s.finishRun(ctx, runID, domain.RunStatusFailed, nil, content, 0, "poll timeout")
	return

doneAsync:
	output := map[string]any{"taskId": taskID, "status": final.Status, "stages": final.Stages, "partial": final.Status == "partial"}
	content.CharacterID = final.CharacterID
	if final.Character != nil {
		content.CharacterID = final.Character.ID
		output["characterId"] = final.Character.ID
		output["characterName"] = final.Character.Name
		if final.Character.PortraitURL != "" {
			output["portraitUrl"] = final.Character.PortraitURL
		}
	}
	status, message := domain.RunStatusSucceeded, ""
	if final.Status == "partial" {
		status = domain.RunStatusFailed
		message = final.ErrorMessage
	}
	_ = s.finishRun(ctx, runID, status, output, content, 0, message)
	if run, ok := s.store.GetRun(ctx, runID); ok {
		s.traceArtifact(ctx, run)
	}
}

func (s *Service) executeCharacterSync(ctx context.Context, runID string, in domain.CharacterGenerateInput) {
	attrsOut, err := tracedClientCall(ctx, s.store, "generate_character_attrs", map[string]any{"prompt": in.Prompt}, func(c context.Context) (map[string]any, error) {
		resp, err := s.client.GenerateCharacterAttrs(c, grapery_client.GenerateCharacterAttrsRequest{CreativeControls: in.CreativeControls, WorkflowReleaseID: in.WorkflowReleaseID, WorkflowRunID: runID,
			Prompt:  in.Prompt,
			StoryID: in.StoryID,
			Name:    in.Name,
		})
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"description":     resp.Description,
			"personality":     resp.Personality,
			"background":      resp.Background,
			"shortTermGoal":   resp.ShortTermGoal,
			"longTermGoal":    resp.LongTermGoal,
			"handlingStyle":   resp.HandlingStyle,
			"cognitionRange":  resp.CognitionRange,
			"abilityFeatures": resp.AbilityFeatures,
			"appearance":      resp.Appearance,
			"dressPreference": resp.DressPreference,
		}, nil
	})
	if err != nil {
		_ = s.finishRun(ctx, runID, domain.RunStatusFailed, nil, domain.ContentRef{}, 0, err.Error())
		return
	}

	output := map[string]any{"attributes": attrsOut}
	stages := map[string]any{"attributes": map[string]any{"status": "completed"}}
	output["stages"] = stages
	var assetErrors []string
	content := domain.ContentRef{}

	if in.CreateRecord {
		name := in.Name
		if name == "" {
			name = "角色"
		}
		sourceType := in.SourceType
		if sourceType == "" {
			sourceType = "ai"
		}
		charOut, createErr := tracedClientCall(ctx, s.store, "create_character", map[string]any{"storyId": in.StoryID, "name": name}, func(c context.Context) (map[string]any, error) {
			resp, err := s.client.CreateCharacter(c, grapery_client.CreateCharacterRequest{
				Name:                       name,
				StoryID:                    in.StoryID,
				Description:                str(attrsOut["description"]),
				Personality:                str(attrsOut["personality"]),
				Background:                 str(attrsOut["background"]),
				ShortTermGoal:              str(attrsOut["shortTermGoal"]),
				LongTermGoal:               str(attrsOut["longTermGoal"]),
				HandlingStyle:              str(attrsOut["handlingStyle"]),
				CognitionRange:             str(attrsOut["cognitionRange"]),
				AbilityFeatures:            str(attrsOut["abilityFeatures"]),
				Appearance:                 str(attrsOut["appearance"]),
				DressPreference:            str(attrsOut["dressPreference"]),
				SourceType:                 sourceType,
				ReferenceImage:             in.ReferenceImage,
				SourceFragmentID:           in.SourceFragmentID,
				SourceFragmentCharacterKey: in.SourceFragmentCharacterKey,
				NeedsPortrait:              in.GeneratePortrait,
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
			avOut, assetErr := tracedClientCall(ctx, s.store, "generate_avatar", map[string]any{"characterId": content.CharacterID}, func(c context.Context) (map[string]any, error) {
				resp, err := s.client.GenerateCharacterAvatar(c, content.CharacterID, grapery_client.GenerateAvatarRequest{AspectRatio: "1:1"})
				if err != nil {
					return nil, err
				}
				return map[string]any{"avatarUrl": resp.AvatarURL}, nil
			})
			output["avatar"] = avOut
			if assetErr != nil {
				assetErrors = append(assetErrors, "avatar: "+assetErr.Error())
				stages["avatar"] = map[string]any{"status": "failed", "error": assetErr.Error()}
			} else {
				stages["avatar"] = map[string]any{"status": "completed"}
			}
		}
		if in.GeneratePortrait && content.CharacterID != "" {
			pOut, assetErr := tracedClientCall(ctx, s.store, "generate_portrait", map[string]any{"characterId": content.CharacterID}, func(c context.Context) (map[string]any, error) {
				resp, err := s.client.GenerateCharacterPortrait(c, content.CharacterID, grapery_client.GeneratePortraitRequest{
					ReferenceImage: in.ReferenceImage,
				})
				if err != nil {
					return nil, err
				}
				return map[string]any{"portraitUrl": resp.PortraitURL}, nil
			})
			output["portrait"] = pOut
			if assetErr != nil {
				assetErrors = append(assetErrors, "portrait: "+assetErr.Error())
				stages["portrait"] = map[string]any{"status": "failed", "error": assetErr.Error()}
			} else {
				stages["portrait"] = map[string]any{"status": "completed"}
			}
		}
		if in.GenerateThreeViews && content.CharacterID != "" {
			tvOut, assetErr := tracedClientCall(ctx, s.store, "generate_three_views", map[string]any{"characterId": content.CharacterID}, func(c context.Context) (map[string]any, error) {
				resp, err := s.client.GenerateCharacterThreeViews(c, content.CharacterID, grapery_client.GenerateThreeViewsRequest{
					ReferenceImage: in.ReferenceImage,
				})
				if err != nil {
					return nil, err
				}
				return map[string]any{"sheetUrl": resp.Views.Sheet}, nil
			})
			output["threeViews"] = tvOut
			if assetErr != nil {
				assetErrors = append(assetErrors, "threeViews: "+assetErr.Error())
				stages["threeViews"] = map[string]any{"status": "failed", "error": assetErr.Error()}
			} else {
				stages["threeViews"] = map[string]any{"status": "completed"}
			}
		}
	}

	status, message := domain.RunStatusSucceeded, ""
	if len(assetErrors) > 0 {
		status = domain.RunStatusFailed
		message = strings.Join(assetErrors, "; ")
		output["partial"] = true
		output["status"] = "partial"
	}
	_ = s.finishRun(ctx, runID, status, output, content, 0, message)
	if run, ok := s.store.GetRun(ctx, runID); ok {
		s.traceArtifact(ctx, run)
	}
}
