package grapery_client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDoJSONIncludesPolicyErrorMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"code":-1,"message":"no active workflow binding","data":null}`))
	}))
	defer server.Close()

	client := &Client{baseURL: server.URL, httpClient: server.Client()}
	err := client.getInternalJSON(context.Background(), "/api/v1/agent-policy/workflow-resolve", &struct{}{})
	if err == nil {
		t.Fatal("expected policy request to fail")
	}
	if !strings.Contains(err.Error(), "no active workflow binding") {
		t.Fatalf("expected response message in error, got %q", err.Error())
	}
}

func TestDoJSONKeepsStatusWhenPolicyErrorBodyIsNotJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("upstream unavailable"))
	}))
	defer server.Close()

	client := &Client{baseURL: server.URL, httpClient: server.Client()}
	err := client.getInternalJSON(context.Background(), "/api/v1/agent-policy/workflow-resolve", &struct{}{})
	if err == nil || !strings.Contains(err.Error(), "status 503") {
		t.Fatalf("expected status fallback, got %v", err)
	}
}
