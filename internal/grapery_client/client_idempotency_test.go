package grapery_client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateStoryboardForwardsIdempotencyKey(t *testing.T) {
	var received string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = r.Header.Get("Idempotency-Key")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":1,"data":{"id":"sb_1","storyId":"story_1"}}`))
	}))
	defer server.Close()

	client := &Client{baseURL: server.URL, httpClient: server.Client()}
	ctx := ContextWithIdempotencyKey(context.Background(), "workflow:run_1")
	if _, err := client.CreateStoryboard(ctx, CreateStoryboardRequest{StoryID: "story_1", RawInput: "hello"}); err != nil {
		t.Fatal(err)
	}
	if received != "workflow:run_1" {
		t.Fatalf("idempotency key not forwarded: %q", received)
	}
}

func TestExecuteStoryboardWorkflowStagePinsRunAndRevisionIdentity(t *testing.T) {
	var received StoryboardWorkflowStageRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/storyboards/sb_1/generate/stages/scene_plan" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":1,"data":{"storyboardId":"sb_1","generationRunId":"sgr_1","stage":"scene_plan","progress":65}}`))
	}))
	defer server.Close()

	client := &Client{baseURL: server.URL, httpClient: server.Client()}
	request := StoryboardWorkflowStageRequest{GenerationRunID: "sgr_1", ClientRequestID: "turn_2", RegenerateStructure: true, UserDirective: "改成雨夜"}
	result, err := client.ExecuteStoryboardWorkflowStage(context.Background(), "sb_1", "scene_plan", request)
	if err != nil {
		t.Fatal(err)
	}
	if received.GenerationRunID != "sgr_1" || received.ClientRequestID != "turn_2" || result.GenerationRunID != "sgr_1" {
		t.Fatalf("stage identity was not preserved: request=%#v result=%#v", received, result)
	}
}
