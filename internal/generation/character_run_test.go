package generation

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/grapestree/fgrapery/grapery-agent/internal/config"
	"github.com/grapestree/fgrapery/grapery-agent/internal/domain"
	"github.com/grapestree/fgrapery/grapery-agent/internal/grapery_client"
	"github.com/grapestree/fgrapery/grapery-agent/internal/runstore"
)

func characterRunTestService(t *testing.T, handler http.HandlerFunc) *Service {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return NewService(grapery_client.NewClient(config.GraperyConfig{BaseURL: server.URL, APIKey: "fallback-test-only"}), runstore.NewMemoryStore(), "test", "test", false)
}
func waitCharacterRun(t *testing.T, s *Service, id string) *domain.GenerationRun {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		run, err := s.GetRun(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		if run.Status == domain.RunStatusSucceeded || run.Status == domain.RunStatusFailed {
			return run
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("character run did not finish")
	return nil
}
func TestCharacterAsyncPartialIsTerminalAndPreservesContentAndStageErrors(t *testing.T) {
	service := characterRunTestService(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer request-user-test" {
			t.Error("request identity was lost across background execution")
		}
		switch r.URL.Path {
		case "/api/v1/character-generation-tasks":
			json.NewEncoder(w).Encode(map[string]any{"code": 1, "data": map[string]any{"id": "task", "status": "pending"}})
		case "/api/v1/character-generation-tasks/task":
			json.NewEncoder(w).Encode(map[string]any{"code": 1, "data": map[string]any{"id": "task", "status": "partial", "characterId": "character", "errorMessage": "threeViews failed", "character": map[string]any{"id": "character", "name": "雨", "portrait": "portrait"}, "stages": map[string]any{"portrait": map[string]any{"status": "completed", "attempts": 1}, "threeViews": map[string]any{"status": "failed", "error": "timeout", "attempts": 1}}}})
		default:
			t.Errorf("unexpected endpoint %s", r.URL.Path)
			http.Error(w, "unexpected", 500)
		}
	})
	ctx := grapery_client.ContextWithAuthToken(context.Background(), "request-user-test")
	run, err := service.StartCharacter(ctx, domain.CharacterGenerateInput{Name: "雨", Prompt: "雨夜", StoryID: "story", UseAsyncTask: true})
	if err != nil {
		t.Fatal(err)
	}
	got := waitCharacterRun(t, service, run.ID)
	if got.Status != domain.RunStatusFailed || got.ContentIDs.CharacterID != "character" || got.Output["partial"] != true || got.Output["portraitUrl"] != "portrait" {
		t.Fatalf("partial lost: %+v", got)
	}
	if got.Output["stages"] == nil {
		t.Fatal("stage failures absent")
	}
}
func TestCharacterSyncVisualFailuresDoNotReportSuccess(t *testing.T) {
	service := characterRunTestService(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/characters/generate":
			var input map[string]any
			_ = json.NewDecoder(r.Body).Decode(&input)
			if input["storyId"] != "story" {
				t.Error("story context missing")
			}
			json.NewEncoder(w).Encode(map[string]any{"code": 1, "data": map[string]any{"description": "角色介绍", "appearance": "银发"}})
		case "/api/v1/characters":
			json.NewEncoder(w).Encode(map[string]any{"code": 1, "data": map[string]any{"id": "character", "name": "雨"}})
		case "/api/v1/characters/character/generate-portrait", "/api/v1/characters/character/generate-three-views":
			http.Error(w, "provider unavailable", 500)
		default:
			t.Errorf("unexpected endpoint %s", r.URL.Path)
			http.Error(w, "unexpected", 500)
		}
	})
	run, err := service.StartCharacter(context.Background(), domain.CharacterGenerateInput{Name: "雨", Prompt: "雨夜", StoryID: "story", CreateRecord: true, GeneratePortrait: true, GenerateThreeViews: true})
	if err != nil {
		t.Fatal(err)
	}
	got := waitCharacterRun(t, service, run.ID)
	if got.Status != domain.RunStatusFailed || got.Output["partial"] != true || got.ContentIDs.CharacterID != "character" {
		t.Fatalf("false success: %+v", got)
	}
}
