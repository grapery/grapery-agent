package http

import (
	"net/http"
	"strconv"

	"github.com/grapestree/fgrapery/grapery-agent/internal/artifact"
	"github.com/grapestree/fgrapery/grapery-agent/internal/domain"
	"github.com/grapestree/fgrapery/grapery-agent/internal/eval"
	"github.com/grapestree/fgrapery/grapery-agent/internal/generation"
	"github.com/grapestree/fgrapery/grapery-agent/internal/grapery_client"
	"github.com/grapestree/fgrapery/grapery-agent/internal/runstore"
	"github.com/gin-gonic/gin"
)

// GenerationHandler serves non-chat generation and RL artifact APIs.
type GenerationHandler struct {
	gen      *generation.Service
	store    runstore.Store
	exporter *artifact.Exporter
	eval     *eval.Harness
}

func NewGenerationHandler(gen *generation.Service, store runstore.Store, artifactDir string) *GenerationHandler {
	return &GenerationHandler{
		gen:      gen,
		store:    store,
		exporter: artifact.NewExporter(store, artifactDir),
		eval:     eval.NewHarness(gen, store),
	}
}

func (h *GenerationHandler) RegisterRoutes(r *gin.Engine) {
	g := r.Group("/api/v1/generation")
	g.Use(h.authMiddleware())
	{
		g.POST("/fragments", h.startFragment)
		g.POST("/stories", h.startStory)
		g.POST("/storyboards", h.startStoryboard)
		g.POST("/characters", h.startCharacter)
		g.POST("/branches", h.startBranches)

		g.GET("/runs/:id", h.getRun)
		g.GET("/runs", h.listRuns)

		g.POST("/artifacts/preference-pair", h.recordPreferencePair)
		g.POST("/artifacts/branch-selection", h.recordBranchSelection)
		g.GET("/artifacts/export", h.exportArtifacts)

		g.POST("/eval/run", h.runEval)
		g.GET("/eval/seeds", h.listSeeds)
	}
}

func (h *GenerationHandler) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		ctx := c.Request.Context()
		if token != "" && len(token) > 7 && token[:7] == "Bearer " {
			ctx = grapery_client.ContextWithAuthToken(ctx, token[7:])
		}
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
