package http

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/grapestree/fgrapery/grapery-agent/internal/artifact"
	"github.com/grapestree/fgrapery/grapery-agent/internal/config"
	"github.com/grapestree/fgrapery/grapery-agent/internal/domain"
	"github.com/grapestree/fgrapery/grapery-agent/internal/eval"
	"github.com/grapestree/fgrapery/grapery-agent/internal/generation"
	"github.com/grapestree/fgrapery/grapery-agent/internal/grapery_client"
	"github.com/grapestree/fgrapery/grapery-agent/internal/runstore"
)

// GenerationHandler serves non-chat generation and RL artifact APIs.
type GenerationHandler struct {
	gen      *generation.Service
	client   *grapery_client.Client
	store    runstore.Store
	exporter *artifact.Exporter
	eval     *eval.Harness
}

func NewGenerationHandler(gen *generation.Service, store runstore.Store, artifactDir string, _ config.AgentAuthConfig) *GenerationHandler {
	return &GenerationHandler{
		gen:      gen,
		store:    store,
		exporter: artifact.NewExporter(store, artifactDir),
		eval:     eval.NewHarness(gen, store),
	}
}

func (h *GenerationHandler) RegisterRoutes(r *gin.Engine, auth agentAuthDeps, client *grapery_client.Client) {
	h.client = client
	g := r.Group("/api/v1/generation")
	g.Use(auth.agentAccessTokenMiddleware())
	g.Use(forwardUserJWTMiddleware(client))
	g.Use(runIDHeaderMiddleware())
	{
		g.POST("/fragments", h.startFragment)
		g.POST("/fragments/stream", requireAgentClaims(auth), h.streamFragment)
		g.POST("/fragment-panels", requireAgentClaims(auth), h.startFragmentPanel)
		g.POST("/fragment-panels/stream", requireAgentClaims(auth), h.streamFragmentPanel)
		g.POST("/stories", h.startStory)
		g.POST("/storyboards", h.startStoryboard)
		g.POST("/characters", h.startCharacter)
		g.POST("/branches", h.startBranches)
		g.POST("/workflows", requireAgentClaims(auth), h.startWorkflow)

		g.GET("/runs/:id", h.getRun)
		g.GET("/runs", h.listRuns)

		g.POST("/artifacts/preference-pair", h.recordPreferencePair)
		g.POST("/artifacts/branch-selection", h.recordBranchSelection)
		g.GET("/artifacts/export", h.exportArtifacts)

		g.POST("/eval/run", h.runEval)
		g.GET("/eval/seeds", h.listSeeds)
	}

	ag := r.Group("/api/v1/agent")
	ag.Use(auth.agentAccessTokenMiddleware())
	ag.Use(forwardUserJWTMiddleware(client))
	ag.Use(runIDHeaderMiddleware())
	{
		ag.GET("/runs/:id", h.getRun)
		ag.POST("/runs/:id/cancel", h.cancelRun)
		ag.POST("/creation/sessions", h.createCreationSession)
		ag.POST("/creation/sessions/:id/messages/stream", h.streamCreationMessage)
	}
}

func runIDHeaderMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		if runID := c.GetHeader("X-Generation-Run-Id"); runID != "" {
			ctx = runstore.ContextWithRunID(ctx, runID)
		}
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func (h *GenerationHandler) ok(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{"code": 1, "message": "success", "data": data})
}

func (h *GenerationHandler) fail(c *gin.Context, status int, msg string) {
	c.JSON(status, gin.H{"code": -1, "message": msg})
}

func (h *GenerationHandler) startFragment(c *gin.Context) {
	var in domain.FragmentGenerateInput
	if err := c.ShouldBindJSON(&in); err != nil {
		h.fail(c, http.StatusBadRequest, err.Error())
		return
	}
	run, err := h.gen.StartFragment(c.Request.Context(), in)
	if err != nil {
		h.fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	h.ok(c, run)
}

func (h *GenerationHandler) startFragmentPanel(c *gin.Context) {
	if err := h.requireFragmentPanelExec(c); err != nil {
		return
	}
	var in domain.FragmentPanelGenerateInput
	if err := c.ShouldBindJSON(&in); err != nil {
		h.fail(c, http.StatusBadRequest, err.Error())
		return
	}
	run, err := h.gen.StartFragmentPanel(c.Request.Context(), in)
	if err != nil {
		h.fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	h.ok(c, run)
}

func (h *GenerationHandler) startStory(c *gin.Context) {
	var in domain.StoryGenerateInput
	if err := c.ShouldBindJSON(&in); err != nil {
		h.fail(c, http.StatusBadRequest, err.Error())
		return
	}
	run, err := h.gen.StartStory(c.Request.Context(), in)
	if err != nil {
		h.fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	h.ok(c, run)
}

func (h *GenerationHandler) startStoryboard(c *gin.Context) {
	var in domain.StoryboardGenerateInput
	if err := c.ShouldBindJSON(&in); err != nil {
		h.fail(c, http.StatusBadRequest, err.Error())
		return
	}
	run, err := h.gen.StartStoryboard(c.Request.Context(), in)
	if err != nil {
		h.fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	h.ok(c, run)
}

func (h *GenerationHandler) startCharacter(c *gin.Context) {
	var in domain.CharacterGenerateInput
	if err := c.ShouldBindJSON(&in); err != nil {
		h.fail(c, http.StatusBadRequest, err.Error())
		return
	}
	run, err := h.gen.StartCharacter(c.Request.Context(), in)
	if err != nil {
		h.fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	h.ok(c, run)
}

func (h *GenerationHandler) startBranches(c *gin.Context) {
	var in domain.BranchExploreInput
	if err := c.ShouldBindJSON(&in); err != nil {
		h.fail(c, http.StatusBadRequest, err.Error())
		return
	}
	run, err := h.gen.StartBranchBatch(c.Request.Context(), in)
	if err != nil {
		h.fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	h.ok(c, run)
}

func (h *GenerationHandler) startWorkflow(c *gin.Context) {
	var in domain.WorkflowStartInput
	if err := c.ShouldBindJSON(&in); err != nil {
		h.fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if !workflowScopeMatches(c, in.Surface) {
		abortUnauthorized(c, "agent token cannot start this workflow surface")
		return
	}
	run, err := h.gen.StartWorkflow(c.Request.Context(), in)
	if err != nil {
		h.fail(c, http.StatusBadRequest, err.Error())
		return
	}
	h.ok(c, run)
}

func workflowScopeMatches(c *gin.Context, surface string) bool {
	target := strings.TrimSpace(strings.ToLower(surface))
	if idx := strings.LastIndex(target, "."); idx >= 0 {
		target = target[idx+1:]
	}
	switch target {
	case "fragment", "storyboard", "story", "character", "branch":
		return creationTargetScopeMatches(c, target)
	default:
		return false
	}
}

func (h *GenerationHandler) getRun(c *gin.Context) {
	run, err := h.gen.GetRun(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.fail(c, http.StatusNotFound, err.Error())
		return
	}
	h.ok(c, run)
}

func (h *GenerationHandler) listRuns(c *gin.Context) {
	kind := domain.RunKind(c.Query("kind"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	h.ok(c, h.gen.ListRuns(c.Request.Context(), kind, limit))
}

func (h *GenerationHandler) recordPreferencePair(c *gin.Context) {
	var req domain.PreferencePairRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.fail(c, http.StatusBadRequest, err.Error())
		return
	}
	art, err := h.exporter.RecordPreferencePair(c.Request.Context(), req)
	if err != nil {
		h.fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	h.ok(c, art)
}

func (h *GenerationHandler) recordBranchSelection(c *gin.Context) {
	var req domain.BranchSelectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.fail(c, http.StatusBadRequest, err.Error())
		return
	}
	art, err := h.exporter.RecordBranchSelection(c.Request.Context(), req)
	if err != nil {
		h.fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	h.ok(c, art)
}

func (h *GenerationHandler) exportArtifacts(c *gin.Context) {
	typ := domain.RLArtifactType(c.DefaultQuery("type", string(domain.ArtifactTypeGenerationTrace)))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "500"))
	path, count, err := h.exporter.ExportJSONL(c.Request.Context(), typ, limit)
	if err != nil {
		h.fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	h.ok(c, gin.H{"path": path, "count": count, "type": typ})
}

func (h *GenerationHandler) runEval(c *gin.Context) {
	var body struct {
		AgentVersion domain.AgentVersion `json:"agentVersion"`
		SeedIDs      []string            `json:"seedIds,omitempty"`
		WaitSec      int                 `json:"waitSec,omitempty"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		h.fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if body.AgentVersion == "" {
		body.AgentVersion = domain.AgentFragmentCreator
	}
	record, err := h.eval.RunEval(c.Request.Context(), body.AgentVersion, body.SeedIDs, body.WaitSec)
	if err != nil {
		h.fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	h.ok(c, record)
}

func (h *GenerationHandler) listSeeds(c *gin.Context) {
	h.ok(c, eval.DefaultSeeds())
}

func (h *GenerationHandler) requireFragmentPanelExec(c *gin.Context) error {
	if !c.GetBool(agentBearerUsedKey) {
		return nil
	}
	if h.gen.ExecFragmentPanelEnabled() {
		return nil
	}
	h.fail(c, http.StatusForbidden, "AGENT_EXEC_FRAGMENT_PANEL_ENABLED must be true for agent-token-only fragment-panel generation")
	return errFragmentPanelExecDisabled
}

var errFragmentPanelExecDisabled = errors.New("fragment panel exec disabled")
