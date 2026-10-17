package generation

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/grapestree/fgrapery/grapery-agent/internal/config"
	"github.com/grapestree/fgrapery/grapery-agent/internal/grapery_client"
	"github.com/grapestree/fgrapery/grapery-agent/internal/runstore"
)

func TestStoryboardWorkflowPassesStableRevisionAndTargetsToBackend(t *testing.T) {
	var received grapery_client.StoryboardWorkflowStageRequest
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/storyboards/board/generate/stages/bible_plan" {
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(404)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Error(err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"code":1,"data":{"storyboardId":"board","generationRunId":"generation","stage":"bible_plan"}}`))
	}))
	defer backend.Close()
	svc := NewService(grapery_client.NewClient(config.GraperyConfig{BaseURL: backend.URL}), runstore.NewMemoryStore(), "", "", false)
	_, err := svc.executeGenerateStoryboardBiblePlanActivity(context.Background(), map[string]any{"draftStoryboardId": "board", "rawInput": "only change page two", "regenerateStructure": true, "sceneCount": 3, "baseRevision": "accepted-hash", "targetSceneIds": []string{"scene-two"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if received.BaseRevision != "accepted-hash" || len(received.TargetSceneIDs) != 1 || received.TargetSceneIDs[0] != "scene-two" {
		t.Fatalf("lost edit contract: %+v", received)
	}
}

func TestStoryboardArtifactRequiresActualPersistedResult(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		wantError  bool
	}{
		{"accepted", `{"code":1,"data":{"id":"board","storyId":"story","content":"accepted plot","contentRevision":"hash","storyboardScenes":[{"id":"scene","title":"Arrival","description":"She arrives in rain","image":"https://example.invalid/scene.png"}]}}`, false},
		{"empty scenes", `{"code":1,"data":{"id":"board","content":"plot","contentRevision":"hash","storyboardScenes":[]}}`, true},
		{"missing image", `{"code":1,"data":{"id":"board","content":"plot","contentRevision":"hash","storyboardScenes":[{"id":"scene","title":"Arrival","description":"She arrives"}]}}`, true},
		{"backend failure", `{"code":500,"message":"unavailable"}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(tc.body))
			}))
			defer backend.Close()
			svc := NewService(grapery_client.NewClient(config.GraperyConfig{BaseURL: backend.URL}), runstore.NewMemoryStore(), "", "", false)
			artifact, err := svc.storyboardAcceptedArtifact(context.Background(), "board", true)
			if (err != nil) != tc.wantError {
				t.Fatalf("artifact=%+v err=%v", artifact, err)
			}
			if !tc.wantError && (artifact["acceptedContent"] != "accepted plot" || artifact["contentRevision"] != "hash") {
				t.Fatalf("artifact lost accepted data: %+v", artifact)
			}
		})
	}
}
