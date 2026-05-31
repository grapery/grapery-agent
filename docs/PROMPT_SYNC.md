# Agent Instruction ↔ grapery Prompt 对照表

本文档说明 **grapery-agent** 中 Eino Agent 的 `Instruction` 与 **grapery** 服务端真实 LLM 提示词的关系，以及如何降低两处漂移。

## 架构原则

| 路径 | 谁执行 LLM | Agent Instruction 作用 |
|------|------------|-------------------------|
| `POST /api/v1/agent/*/chat` | **grapery**（经工具调 HTTP API） | 教 Agent 何时调哪个工具、解释字段与产品规则 |
| `POST /api/v1/generation/*` | **grapery**（`generation.Service` 直连 client） | **不经过** Agent LLM；Instruction 不参与 |

**单一事实来源（生成质量）**：`grapery/internal/service/*.go` 内嵌的 system/user prompt。  
**Agent 侧单一事实来源（编排说明）**：`grapery-agent/internal/prompt/*_domain.go` + `instructions.go`。

## 代码位置

| Agent | Instruction 组装 | 领域知识常量 |
|-------|-------------------|--------------|
| FragmentCreator | `prompt.FragmentCreatorInstruction` | `FragmentDomainKnowledge` |
| CharacterDesigner | `prompt.CharacterDesignerInstruction` | `CharacterDomainKnowledge` |
| StoryboardDirector | `prompt.StoryboardDirectorInstruction` | `StoryboardDomainKnowledge` |
| BranchExplorer | `prompt.BranchExplorerInstruction` | `BranchDomainKnowledge` |
| （无）Story | — | 见 `Catalog()` story 条目 |

机器可读映射：`internal/prompt/catalog.go` 的 `Catalog()`。  
漂移检测：`go test ./internal/prompt/ -run TestGraperyPromptAnchors`。

## 对照表

### 故事碎片（Fragment）

| 项 | grapery | grapery-agent |
|----|---------|---------------|
| 权威 prompt | `fragment_generation_service.go` → `buildExtractionAndStoryPrompt` | `FragmentDomainKnowledge`（浓缩） |
| 场景扩写 / 出图 | `fragment_generation_huoshan_scenes.go`, `buildFragmentSceneImagePrompt` | 未收录 |
| 多面板规划 | `fragment_panel_plan_prompts.go` | 未收录 |
| Agent 版本 | — | `fragment_creator:v1` |
| 对齐程度 | **高**（元素示例、八层 imagePrompt、漫画语言、参考图四阶段） | |
| 已知差距 | JSON 硬性 schema、panel 管线 | |

### 故事角色（Character）

| 项 | grapery | grapery-agent |
|----|---------|---------------|
| 权威 prompt | `character.go` → `GenerateCharacterWithAI` systemPrompt | `CharacterDomainKnowledge` |
| 出图模板 | character 服务内英文 portrait/avatar/三视图 | Instruction 内模板说明 |
| Agent 版本 | — | `character_designer:v1` |
| 对齐程度 | **高**（10 字段语义一致） | |
| 已知差距 | strict JSON 输出约束；`character-generation-tasks` 未暴露为 tool | |

### 故事板（Storyboard）

| 项 | grapery | grapery-agent |
|----|---------|---------------|
| 权威 prompt（主路径） | `storyboard_redesign_prompts.go`（英文 JSON） | `StoryboardDomainKnowledge`（中文编排） |
| 续写 / 旧管线 | `storyboard.go`, `narrator_pipeline.go` | 工具流程说明 |
| Agent 版本 | — | `storyboard_director:v1` |
| 对齐程度 | **中**（概念对齐，无完整 schema） | |
| 已知差距 | `comicFunction` 枚举在 agent 中不完整；chat 默认 `generate_storyboard_content` 与 redesign `generate/structure` 易混淆 | |

### 多分支（Branch）

| 项 | grapery | grapery-agent |
|----|---------|---------------|
| 续写正文 | `narrator_pipeline.go`（800–1500 字、FateSnapshot） | **不复制** |
| 策略轴 | — | `branch_strategies.go`（hopeful_turn 等） |
| Agent 版本 | — | `branch_explorer:v1` |
| 对齐程度 | **低**（策略层自研；正文仍走 grapery） | |
| 建议 | `BuildBranchRawInput` 的 seed 应包含完整用户前提，策略仅作前缀 | |

### 故事（Story）

| 项 | grapery | grapery-agent |
|----|---------|---------------|
| enrich | `story.go` enrich systemPrompt | 无 Agent |
| 生成任务 | `ai_handler.go` `/api/v1/ai/generate-story` | `generation/story_run.go` |
| Agent 版本 | — | `story_generator:v1`（run 元数据） |
| 对齐程度 | **无 Agent Instruction** | |

## 维护流程

1. 修改 grapery 生成 prompt 后，运行：
   ```bash
   cd grapery-agent && go test ./internal/prompt/ -run TestGraperyPromptAnchors
   ```
2. 若 anchor 失败：更新 `internal/prompt/*_domain.go` 与 `catalog.go` 的 `Version`、`DriftAnchors`。
3. 大改时在本文件「已知差距」补一行说明。

## 非目标（当前阶段）

- 不把 grapery prompt 全文复制进 agent（避免双份维护巨型字符串）。
- 不修改 `grapery/` 代码（由 grapery-agent 单独演进；后续可选共享 `prompt/` 子模块）。
