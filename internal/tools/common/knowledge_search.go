package common

import (
	"context"

	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

type KnowledgeSearchInput struct {
	Query string `json:"query" jsonschema:"description=要检索的故事设定、创作规范或产品知识问题,required"`
	TopK  int    `json:"topK,omitempty" jsonschema:"description=返回结果数，建议 1 到 8"`
}

type KnowledgeSearchResult struct {
	Content string         `json:"content"`
	Source  string         `json:"source,omitempty"`
	Score   float64        `json:"score"`
	Meta    map[string]any `json:"meta,omitempty"`
}

func NewKnowledgeSearchTool(knowledge retriever.Retriever, defaultTopK int) (tool.InvokableTool, error) {
	return utils.InferTool(
		"search_domain_knowledge",
		"检索经过运营维护的故事世界观、角色设定、创作规范和产品知识。涉及既有事实或连续性时，应先检索再回答或生成。",
		func(ctx context.Context, input *KnowledgeSearchInput) ([]KnowledgeSearchResult, error) {
			topK := input.TopK
			if topK <= 0 {
				topK = defaultTopK
			}
			if topK > 8 {
				topK = 8
			}
			documents, err := knowledge.Retrieve(ctx, input.Query, retriever.WithTopK(topK))
			if err != nil {
				return nil, err
			}
			results := make([]KnowledgeSearchResult, 0, len(documents))
			for _, document := range documents {
				if document == nil {
					continue
				}
				source, _ := document.MetaData["source"].(string)
				results = append(results, KnowledgeSearchResult{Content: document.Content, Source: source, Score: document.Score(), Meta: document.MetaData})
			}
			return results, nil
		},
	)
}
