package prompt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGraperyPromptAnchors verifies grapery source files still contain strings
// the agent instruction summaries were derived from. Update Catalog().Version and
// agent domain knowledge when intentionally changing grapery prompts.
func TestGraperyPromptAnchors(t *testing.T) {
	root := findMonorepoRoot(t)
	for _, ref := range Catalog() {
		if len(ref.DriftAnchors) == 0 {
			continue
		}
		path := filepath.Join(root, ref.Path)
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", ref.Path, err)
		}
		text := string(body)
		for _, anchor := range ref.DriftAnchors {
			if !strings.Contains(text, anchor) {
				t.Errorf("drift: %s (%s) missing anchor %q", ref.Path, ref.Symbol, anchor)
			}
		}
	}
}

func findMonorepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if fileExists(filepath.Join(dir, "grapery", "go.mod")) && fileExists(filepath.Join(dir, "grapery-agent", "go.mod")) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("monorepo root not found (expected grapery/ and grapery-agent/ siblings)")
		}
		dir = parent
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
