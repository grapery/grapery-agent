package grapery_client

import (
	"encoding/json"
	"testing"
)

func TestFragmentStatusAcceptsFractionalLedgerCostAndQuality(t *testing.T) {
	var status FragmentTaskStatus
	if err := json.Unmarshal([]byte(`{"taskId":"t","cost":{"amount":0.0123,"tokens":123,"state":"recorded"},"result":{"quality":{"status":"unreviewed","reason":"parse_failed"}}}`), &status); err != nil {
		t.Fatal(err)
	}
	if status.Cost == nil || status.Cost.Amount == nil || *status.Cost.Amount != 0.0123 || status.Cost.State != "recorded" || status.Result.Quality.Status != "unreviewed" {
		t.Fatalf("lost quality/ledger precision %#v", status)
	}
	if err := json.Unmarshal([]byte(`{"cost":{"state":"unavailable"}}`), &status); err != nil {
		t.Fatal(err)
	}
	var unavailable FragmentTaskStatus
	_ = json.Unmarshal([]byte(`{"cost":{"state":"unavailable"}}`), &unavailable)
	if unavailable.Cost.Amount != nil {
		t.Fatal("missing historical cost invented a zero charge")
	}
}
