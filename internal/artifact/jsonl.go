package artifact

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/grapestree/fgrapery/grapery-agent/internal/domain"
	"github.com/grapestree/fgrapery/grapery-agent/internal/runstore"
)

// Exporter writes RL artifacts to JSONL files.
type Exporter struct {
	store  runstore.Store
	dir    string
}

func NewExporter(store runstore.Store, dir string) *Exporter {
	return &Exporter{store: store, dir: dir}
}

// ExportJSONL writes artifacts of the given type to a timestamped file under dir.
func (e *Exporter) ExportJSONL(ctx context.Context, typ domain.RLArtifactType, limit int) (string, int, error) {
	if e.dir == "" {
		e.dir = os.TempDir()
	}
	if err := os.MkdirAll(e.dir, 0o755); err != nil {
		return "", 0, err
	}
	arts := e.store.ListArtifacts(ctx, typ, limit)
	name := fmt.Sprintf("artifacts_%s_%d.jsonl", typ, time.Now().Unix())
	path := filepath.Join(e.dir, name)
	f, err := os.Create(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, a := range arts {
		if err := enc.Encode(a); err != nil {
			return path, 0, err
		}
	}
	return path, len(arts), nil
}

// RecordPreferencePair stores a branch A/B preference sample.
func (e *Exporter) RecordPreferencePair(ctx context.Context, req domain.PreferencePairRequest) (*domain.RLArtifact, error) {
	art := &domain.RLArtifact{
		Type:            domain.ArtifactTypeBranchPair,
		RunID:           req.RunID,
		Prompt:          req.Prompt,
		BranchA:         req.BranchA,
		BranchB:         req.BranchB,
		Preferred:       req.Preferred,
		SimulatedReward: req.Reward,
	}
	if err := e.store.AppendArtifact(ctx, art); err != nil {
		return nil, err
	}
	return art, nil
}

// RecordBranchSelection stores winner/losers preference.
func (e *Exporter) RecordBranchSelection(ctx context.Context, req domain.BranchSelectionRequest) (*domain.RLArtifact, error) {
	art := &domain.RLArtifact{
		Type:             domain.ArtifactTypeBranchSelection,
		RunID:            req.RunID,
		Prompt:           req.Prompt,
		SelectedBranchID: req.SelectedBranchID,
		RejectedIDs:      req.RejectedIDs,
		SimulatedReward:  req.Reward,
	}
	if err := e.store.AppendArtifact(ctx, art); err != nil {
		return nil, err
	}
	return art, nil
}
