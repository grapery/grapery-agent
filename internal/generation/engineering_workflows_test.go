package generation

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/grapestree/fgrapery/grapery-agent/internal/config"
	"github.com/grapestree/fgrapery/grapery-agent/internal/domain"
	"github.com/grapestree/fgrapery/grapery-agent/internal/grapery_client"
	"github.com/grapestree/fgrapery/grapery-agent/internal/runstore"
	workflowruntime "github.com/grapestree/fgrapery/grapery-agent/internal/workflow"
)

func TestEngineeringCreationDefaultsExecuteWithNoOperatorConfiguration(t *testing.T) {
	for _, kind := range []string{"fragment", "storyboard", "character"} {
		t.Run(kind, func(t *testing.T) {
			var attempts atomic.Int32
			backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == "/api/v1/agent-policy/workflow-resolve" {
					w.WriteHeader(404)
					w.Write([]byte(`{"code":-4,"message":"no_active_workflow","errorCode":"no_active_workflow"}`))
					return
				}
				if strings.Contains(r.URL.Path, "generation-checkpoints") {
					w.Write([]byte(`{"code":1}`))
					return
				}
				t.Errorf("unexpected dependency on operator configuration %s", r.URL.Path)
				w.WriteHeader(500)
			}))
			defer backend.Close()
			svc := NewService(grapery_client.NewClient(config.GraperyConfig{BaseURL: backend.URL}), runstore.NewMemoryStore(), "", "", false)
			svc.workflowActivities = workflowruntime.NewActivityRegistry()
			release, err := engineeringWorkflow("voyager."+kind, "generate")
			if err != nil {
				t.Fatal(err)
			}
			for _, node := range release.Definition.Nodes {
				activity := node.Activity
				if err := svc.workflowActivities.Register(activity, func(ctx context.Context, input, config map[string]any) (map[string]any, error) {
					controls, err := controlsForGeneration(ctx, nil)
					if err != nil {
						return nil, err
					}
					if controls.Tone != "neutral" {
						t.Error("default control was not frozen")
					}
					attempts.Add(1)
					return map[string]any{"accepted": true}, nil
				}); err != nil {
					t.Fatal(err)
				}
			}
			run, err := svc.StartWorkflow(context.Background(), domain.WorkflowStartInput{Surface: "voyager." + kind, Action: "generate", Input: map[string]any{}})
			if err != nil {
				t.Fatal(err)
			}
			deadline := time.Now().Add(3 * time.Second)
			for time.Now().Before(deadline) {
				current, _ := svc.store.GetRun(context.Background(), run.ID)
				if current.Status == domain.RunStatusFailed {
					t.Fatal(current.Error)
				}
				if current.Status == domain.RunStatusSucceeded {
					if int(attempts.Load()) != len(release.Definition.Nodes) {
						t.Fatalf("default graph incomplete %d", attempts.Load())
					}
					return
				}
				time.Sleep(5 * time.Millisecond)
			}
			t.Fatal("default graph did not complete")
		})
	}
}

func TestWorkflowDefaultDoesNotHideAuthNetworkOrBrokenRelease(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
	}{
		{"unauthorized", 401, `{"message":"no_active_workflow","errorCode":"no_active_workflow"}`},
		{"database error", 500, `{"message":"database unavailable"}`},
		{"unmatched operator conditions", 404, `{"message":"no matching workflow"}`},
		{"invalid published release", 200, `{"code":1,"data":{"entry":{"release":{"id":"ops.bad","key":"ops","version":1,"status":"released","definition":{"nodes":[{"id":"bad","type":"activity","activity":"unsafe.unregistered"}]}}}}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.status); w.Write([]byte(tc.body)) }))
			defer server.Close()
			svc := NewService(grapery_client.NewClient(config.GraperyConfig{BaseURL: server.URL}), runstore.NewMemoryStore(), "", "", false)
			if _, err := svc.StartWorkflow(context.Background(), domain.WorkflowStartInput{Surface: "voyager.fragment", Action: "generate", Input: map[string]any{}}); err == nil {
				t.Fatal("silently replaced error with default graph")
			}
			if len(svc.store.ListRuns(context.Background(), domain.RunKindWorkflow, 10)) != 0 {
				t.Fatal("started work despite policy error")
			}
		})
	}
}

func TestFrozenEngineeringGraphAndControlsCannotDrift(t *testing.T) {
	release, err := engineeringWorkflow("voyager.storyboard", "generate")
	if err != nil {
		t.Fatal(err)
	}
	controls, err := domain.ParseCreativeControls(release.Manifest["creativeControls"])
	if err != nil {
		t.Fatal(err)
	}
	state := workflowCheckpoint{Version: workflowCheckpointVersion, ReleaseID: release.ID, EngineeringRelease: release, CreativeControls: &controls, Input: map[string]any{}}
	raw, _ := json.Marshal(state)
	var restored workflowCheckpoint
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	frozen, err := frozenEngineeringWorkflow(restored.EngineeringRelease)
	if err != nil {
		t.Fatal(err)
	}
	release.Definition.Nodes[0].Activity = "changed.after.checkpoint"
	if frozen.Definition.Nodes[0].Activity != "storyboard.ensure_draft" || restored.CreativeControls.Tone != "neutral" {
		t.Fatal("checkpoint adopted live changes")
	}
	restored.EngineeringRelease.Definition.Nodes[0].Activity = "tampered"
	if _, err := frozenEngineeringWorkflow(restored.EngineeringRelease); err == nil {
		t.Fatal("corrupt graph checksum accepted")
	}
	fake := *frozen
	fake.ID = "operator.custom"
	if isEngineeringWorkflow(&fake) {
		t.Fatal("operator manifest marker impersonated reserved engineering release")
	}
	other, err := os.ReadFile("../../../grapery/internal/service/engineering_workflows.json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(other, engineeringWorkflowJSON) {
		t.Fatal("Grapery resolution and Agent execution defaults diverged")
	}
}

func TestWorkflowClientsCannotOverrideOperatorControlsOrCorePrompts(t *testing.T) {
	svc := NewService(nil, runstore.NewMemoryStore(), "", "", false)
	for _, key := range []string{"creativeControls", "workflowSystemPrompt", "workflowUserPrompt", "workflowOutputSchema"} {
		if _, err := svc.StartWorkflow(context.Background(), domain.WorkflowStartInput{Surface: "voyager.fragment", Action: "generate", Input: map[string]any{key: "override"}}); err == nil {
			t.Fatalf("client overrode %s", key)
		}
	}
	controls := domain.DefaultCreativeControls()
	controls.Tone = "warm"
	ctx := withCreativeControls(context.Background(), controls)
	supplied := domain.DefaultCreativeControls()
	supplied.Tone = "tense"
	got, err := controlsForGeneration(ctx, &supplied)
	if err != nil {
		t.Fatal(err)
	}
	if got.Tone != "warm" {
		t.Fatal("client input replaced operator controls")
	}
	if _, err := controlsForGeneration(context.Background(), &supplied); err == nil {
		t.Fatal("direct generation forged operator authority")
	}
}


func TestWorkflowNetworkFailureDoesNotFallback(t *testing.T) {
 server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
 url := server.URL
 server.Close()
 svc := NewService(grapery_client.NewClient(config.GraperyConfig{BaseURL:url}), runstore.NewMemoryStore(), "", "", false)
 if _, err := svc.StartWorkflow(context.Background(), domain.WorkflowStartInput{Surface:"voyager.fragment", Action:"generate", Input:map[string]any{}}); err == nil { t.Fatal("network failure silently downgraded") }
 if len(svc.store.ListRuns(context.Background(), domain.RunKindWorkflow, 10)) != 0 { t.Fatal("started default after network failure") }
}

func TestManagedCreationDoesNotReadDeprecatedPromptRecords(t *testing.T) {
 for _, kind := range []string{"fragment", "storyboard", "character"} {
 t.Run(kind, func(t *testing.T) {
  release, _ := engineeringWorkflow("voyager."+kind, "generate")
  release.ID = "operator."+kind
  release.Key = "operator."+kind
  release.PromptBundle = map[string]string{"generate":"deleted-or-unpublished-template"}
  release.Manifest["creativeControls"] = map[string]any{"version":1,"tone":"warm"}
  server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
   if strings.Contains(r.URL.Path,"generation-checkpoints") { w.Write([]byte(`{"code":1}`)); return }
   if r.URL.Path == "/api/v1/agent-policy/workflow-resolve" { json.NewEncoder(w).Encode(map[string]any{"code":1,"data":domain.WorkflowResolution{Entry:domain.WorkflowCatalogEntry{Release:*release}}}); return }
   t.Errorf("managed core must not depend on deprecated template DB %s",r.URL.Path); w.WriteHeader(500)
  }))
  defer server.Close()
  svc := NewService(grapery_client.NewClient(config.GraperyConfig{BaseURL:server.URL}),runstore.NewMemoryStore(),"","",false)
  svc.workflowActivities = workflowruntime.NewActivityRegistry()
  for _, node := range release.Definition.Nodes { if err := svc.workflowActivities.Register(node.Activity,func(ctx context.Context,_,_ map[string]any)(map[string]any,error){ c,_:=controlsForGeneration(ctx,nil); if c.Tone!="warm" { t.Error("operator controls lost") }; return map[string]any{},nil }); err != nil { t.Fatal(err) } }
  run,err:=svc.StartWorkflow(context.Background(),domain.WorkflowStartInput{Surface:"voyager."+kind,Action:"generate",Input:map[string]any{}})
  if err!=nil { t.Fatal(err) }
  current:=waitCharacterRun(t,svc,run.ID)
  if current.Status != domain.RunStatusSucceeded { t.Fatal(current.Error) }
  if current.PromptBundle["generate"]!="deleted-or-unpublished-template" || len(current.PromptSnapshots)!=0 { t.Fatal("deprecated IDs must remain audit-only") }
 })
 }
}

func TestTestRunDoesNotUseNoBindingFallback(t *testing.T) {
 for _, id := range []string{"", "missing-release"} {
  server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){ if strings.Contains(r.URL.Path,"workflow-resolve") { t.Error("test run used routing") }; w.WriteHeader(404); w.Write([]byte(`{"message":"no_active_workflow","errorCode":"no_active_workflow"}`)) }))
  svc:=NewService(grapery_client.NewClient(config.GraperyConfig{BaseURL:server.URL}),runstore.NewMemoryStore(),"","",false)
  _,err:=svc.StartWorkflow(context.Background(),domain.WorkflowStartInput{Surface:"voyager.fragment",Action:"generate",TestRun:true,ReleaseID:id,Input:map[string]any{}})
  server.Close()
  if err==nil { t.Fatal("test run silently selected engineering default") }
 }
}

func TestReleaseControlsDistinguishAbsentFromExplicitNull(t *testing.T) {
 if _,err:=creativeControlsFromRelease(&domain.WorkflowRelease{}); err!=nil { t.Fatal(err) }
 if _,err:=creativeControlsFromRelease(&domain.WorkflowRelease{Manifest:map[string]any{"creativeControls":nil}}); err==nil { t.Fatal("explicit null treated as absent") }
}

func TestPlanningCoreCannotBeReplacedByLegacyTemplate(t *testing.T) {
 system,user,schema,err:=generationPlannerPrompts(map[string]any{"rawInput":"雨夜","sceneCount":2},map[string]any{"promptTemplate":map[string]any{"systemPrompt":"override","userPrompt":"replacement","outputSchema":map[string]any{"type":"string"}}})
 if err!=nil { t.Fatal(err) }
 if system!=generationPlannerSystemPrompt || !strings.Contains(user,"雨夜") || schema!=nil { t.Fatal("legacy template replaced engineering planner core") }
}
