package generation

import (
	"testing"
	"time"
)

func TestGenerationPollTimeoutSupportsLongRunningTasks(t *testing.T) {
	if got := generationPollTimeout(0); got != 12*time.Hour {
		t.Fatalf("default timeout = %s, want 12h", got)
	}
	if got := generationPollTimeout(20 * 60); got != 20*time.Minute {
		t.Fatalf("explicit timeout = %s, want 20m", got)
	}
	if got := generationPollTimeout(24 * 60 * 60); got != 12*time.Hour {
		t.Fatalf("capped timeout = %s, want 12h", got)
	}
}
