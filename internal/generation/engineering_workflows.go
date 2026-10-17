package generation

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/grapestree/fgrapery/grapery-agent/internal/domain"
)

//go:embed engineering_workflows.json
var engineeringWorkflowJSON []byte

func workflowReleaseChecksum(release *domain.WorkflowRelease) (string, error) {
	raw, err := json.Marshal(struct {
		Key          string                    `json:"key"`
		Version      int                       `json:"version"`
		Manifest     map[string]any            `json:"manifest,omitempty"`
		Definition   domain.WorkflowDefinition `json:"definition"`
		PromptBundle map[string]string         `json:"promptBundle,omitempty"`
		Policies     domain.WorkflowPolicies   `json:"policies,omitempty"`
	}{release.Key, release.Version, release.Manifest, release.Definition, release.PromptBundle, release.Policies})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}
func engineeringWorkflow(surface, action string) (*domain.WorkflowRelease, error) {
	if action != "generate" {
		return nil, fmt.Errorf("no engineering workflow for %s/%s", surface, action)
	}
	var releases []domain.WorkflowRelease
	if err := json.Unmarshal(engineeringWorkflowJSON, &releases); err != nil {
		return nil, err
	}
	for _, release := range releases {
		if release.Manifest["surface"] == surface {
			checksum, err := workflowReleaseChecksum(&release)
			if err != nil {
				return nil, err
			}
			release.Checksum = checksum
			return &release, nil
		}
	}
	return nil, fmt.Errorf("no engineering workflow for %s/%s", surface, action)
}
func isEngineeringWorkflow(release *domain.WorkflowRelease) bool {
	if release == nil || release.Version != 1 || release.Manifest["engineeringDefault"] != true {
		return false
	}
	for _, kind := range []string{"fragment", "storyboard", "character"} {
		if release.ID == "engineering."+kind+".v1" && release.Key == "engineering."+kind && release.Manifest["surface"] == "voyager."+kind {
			return true
		}
	}
	return false
}
func frozenEngineeringWorkflow(snapshot *domain.WorkflowRelease) (*domain.WorkflowRelease, error) {
	if !isEngineeringWorkflow(snapshot) {
		return nil, fmt.Errorf("missing frozen engineering workflow")
	}
	checksum, err := workflowReleaseChecksum(snapshot)
	if err != nil {
		return nil, err
	}
	if checksum != snapshot.Checksum {
		return nil, fmt.Errorf("frozen engineering workflow checksum mismatch")
	}
	raw, _ := json.Marshal(snapshot)
	var copy domain.WorkflowRelease
	if err := json.Unmarshal(raw, &copy); err != nil {
		return nil, err
	}
	return &copy, nil
}
