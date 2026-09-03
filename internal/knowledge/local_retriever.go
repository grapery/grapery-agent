package knowledge

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
)

type indexedDocument struct {
	document *schema.Document
	terms    map[string]int
}

// LocalRetriever is a read-only Eino Retriever for curated Markdown/text
// knowledge. It is intentionally dependency-free and can later be replaced by
// RedisSearch/VikingDB without changing Agent or Tool contracts.
type LocalRetriever struct {
	documents   []indexedDocument
	defaultTopK int
}

func NewLocalRetriever(root string, defaultTopK int) (*LocalRetriever, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, nil
	}
	if defaultTopK <= 0 {
		defaultTopK = 4
	}
	var documents []indexedDocument
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".md" && ext != ".mdx" && ext != ".txt" {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, _ := filepath.Rel(root, path)
		for index, chunk := range splitKnowledgeDocument(string(raw), 5000) {
			digest := sha256.Sum256([]byte(relative + fmt.Sprint(index) + chunk))
			doc := (&schema.Document{
				ID:       fmt.Sprintf("%x", digest[:12]),
				Content:  chunk,
				MetaData: map[string]any{"source": relative, "chunk": index},
			})
			documents = append(documents, indexedDocument{document: doc, terms: knowledgeTerms(chunk)})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &LocalRetriever{documents: documents, defaultTopK: defaultTopK}, nil
}

func (r *LocalRetriever) Retrieve(ctx context.Context, query string, opts ...retriever.Option) (docs []*schema.Document, err error) {
	options := retriever.GetCommonOptions(&retriever.Options{}, opts...)
	topK := r.defaultTopK
	if options.TopK != nil && *options.TopK > 0 {
		topK = *options.TopK
	}
	ctx = callbacks.OnStart(ctx, &retriever.CallbackInput{Query: query, TopK: topK, ScoreThreshold: options.ScoreThreshold})
	defer func() {
		if err != nil {
			_ = callbacks.OnError(ctx, err)
		}
	}()
	queryTerms := knowledgeTerms(query)
	if len(queryTerms) == 0 {
		return nil, fmt.Errorf("knowledge query is empty")
	}
	type match struct {
		doc   *schema.Document
		score float64
	}
	matches := make([]match, 0, len(r.documents))
	for _, candidate := range r.documents {
		score := lexicalScore(queryTerms, candidate.terms)
		if score <= 0 || (options.ScoreThreshold != nil && score < *options.ScoreThreshold) {
			continue
		}
		clone := *candidate.document
		clone.MetaData = cloneStringAnyMap(candidate.document.MetaData)
		clone.WithScore(score)
		matches = append(matches, match{doc: &clone, score: score})
	}
	sort.SliceStable(matches, func(i, j int) bool { return matches[i].score > matches[j].score })
	if len(matches) > topK {
		matches = matches[:topK]
	}
	for _, item := range matches {
		docs = append(docs, item.doc)
	}
	_ = callbacks.OnEnd(ctx, &retriever.CallbackOutput{Docs: docs, Extra: map[string]any{"backend": "local_lexical"}})
	return docs, nil
}

func (r *LocalRetriever) IsCallbacksEnabled() bool { return true }

func splitKnowledgeDocument(content string, maxRunes int) []string {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil
	}
	sections := strings.Split(content, "\n#")
	var chunks []string
	for index, section := range sections {
		if index > 0 {
			section = "#" + section
		}
		runes := []rune(strings.TrimSpace(section))
		for len(runes) > 0 {
			end := maxRunes
			if len(runes) < end {
				end = len(runes)
			}
			chunks = append(chunks, strings.TrimSpace(string(runes[:end])))
			runes = runes[end:]
		}
	}
	return chunks
}

func knowledgeTerms(value string) map[string]int {
	terms := make(map[string]int)
	var ascii strings.Builder
	var previousCJK rune
	flushASCII := func() {
		if ascii.Len() > 1 {
			terms[ascii.String()]++
		}
		ascii.Reset()
	}
	for _, current := range []rune(strings.ToLower(value)) {
		if unicode.Is(unicode.Han, current) {
			flushASCII()
			terms[string(current)]++
			if previousCJK != 0 {
				terms[string([]rune{previousCJK, current})] += 2
			}
			previousCJK = current
			continue
		}
		previousCJK = 0
		if unicode.IsLetter(current) || unicode.IsDigit(current) {
			ascii.WriteRune(current)
		} else {
			flushASCII()
		}
	}
	flushASCII()
	return terms
}

func lexicalScore(query, document map[string]int) float64 {
	matched, total := 0, 0
	for term, weight := range query {
		total += weight
		if count := document[term]; count > 0 {
			if count < weight {
				matched += count
			} else {
				matched += weight
			}
		}
	}
	if total == 0 {
		return 0
	}
	return float64(matched) / float64(total)
}

func cloneStringAnyMap(source map[string]any) map[string]any {
	result := make(map[string]any, len(source)+1)
	for key, value := range source {
		result[key] = value
	}
	return result
}
