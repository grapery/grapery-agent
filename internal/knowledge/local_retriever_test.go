package knowledge

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudwego/eino/components/retriever"
)

func TestLocalRetrieverRanksRelevantMarkdown(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "world.md"), []byte("# 月港\n月港的能源来自潮汐晶体，守卫叫林澈。\n# 荒原\n荒原依赖风能。"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "style.txt"), []byte("喜剧故事应该保持轻松节奏。"), 0o600); err != nil {
		t.Fatal(err)
	}
	local, err := NewLocalRetriever(dir, 3)
	if err != nil {
		t.Fatal(err)
	}
	documents, err := local.Retrieve(context.Background(), "月港的能源是什么", retriever.WithTopK(1))
	if err != nil {
		t.Fatal(err)
	}
	if len(documents) != 1 || documents[0].MetaData["source"] != "world.md" || documents[0].Score() <= 0 {
		t.Fatalf("unexpected retrieval result: %#v", documents)
	}
}

func TestLocalRetrieverDisabledWithoutDirectory(t *testing.T) {
	local, err := NewLocalRetriever("", 4)
	if err != nil || local != nil {
		t.Fatalf("empty directory should disable retrieval: retriever=%#v err=%v", local, err)
	}
}
