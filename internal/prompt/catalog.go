package prompt

// Role describes what the agent instruction block is for (orchestration vs generation).
type Role string

const (
	RoleOrchestration Role = "orchestration" // teaches tool use + product rules; does not replace grapery LLM prompts
	RoleStrategy      Role = "strategy"      // agent-only branch axes; grapery applies narrator continuation separately
	RoleReference     Role = "reference"     // documented grapery prompt; no agent instruction yet
)

// GraperyRef points to the authoritative prompt location in the grapery repo (read-only).
type GraperyRef struct {
	Path         string   // relative to monorepo root, e.g. grapery/internal/service/...
	Symbol       string   // function or API entry
	Version      string   // bump when agent summary is re-synced from grapery
	Role         Role
	AgentVersion string   // matches domain.AgentVersion when applicable
	Summary      string   // what the agent block covers vs what grapery still owns
	NotInAgent   []string // grapery-only capabilities not mirrored in agent instruction
	DriftAnchors []string // substrings that must exist in grapery source (drift_check_test)
}

// Catalog returns the Agent Instruction ↔ grapery prompt mapping.
func Catalog() []GraperyRef {
	return []GraperyRef{
		{
			Path:         "grapery/internal/service/fragment_generation_service.go",
			Symbol:       "(*FragmentGenerationService).buildExtractionAndStoryPrompt",
			Version:      "2026-06-11",
			Role:         RoleOrchestration,
			AgentVersion: "fragment_creator:v1",
			Summary:      "文本碎片：元素示例、八层 imagePrompt、漫画叙事、参考图四阶段；JSON/schema 与 scene 扩写在 grapery",
			NotInAgent: []string{
				"fragment_generation_huoshan_scenes.go 场景扩写细节",
				"fragment_comic_style_service.go 风格命名",
			},
			DriftAnchors: []string{
				"第一件事：元素提取",
				"每个 visualBible.characters[] 条目必须填写 name",
				"第四阶段——冲突裁决",
			},
		},
		{
			Path:         "grapery/internal/service/fragment_panel_plan_prompts.go",
			Symbol:       "buildFragmentPanelPlanPrompt",
			Version:      "2026-06-11",
			Role:         RoleOrchestration,
			AgentVersion: "fragment_panel_creator:v1",
			Summary:      "参考图多面板：panel plan JSON、visualBible、layout_intent/comic_texts；出图在 fragment_panel_generation_service",
			NotInAgent: []string{
				"runPanelImageBatchHuoshan 组图出图细节",
			},
			DriftAnchors: []string{
				"参考图多模态视觉事实",
				"visualBible 必须存在且包含 styleBible.artStyle",
				"layout_intent",
			},
		},
		{
			Path:         "grapery/internal/service/character.go",
			Symbol:       "(*Service).GenerateCharacterWithAI",
			Version:      "2026-06-11",
			Role:         RoleOrchestration,
			AgentVersion: "character_designer:v1",
			Summary:      "10 字段语义；异步 character-generation-tasks 由 agent tools 暴露",
			NotInAgent:   nil,
			DriftAnchors: []string{
				"你是一个专业的故事角色设计师",
				"dressPreference：日常着装风格与偏好",
				"handlingStyle：面对冲突或难题时的决策方式",
			},
		},
		{
			Path:         "grapery/internal/service/character.go",
			Symbol:       "(*Service).StartCharacterGenerationTask / runCharacterGenerationTask",
			Version:      "2026-06-11",
			Role:         RoleReference,
			AgentVersion: "character_designer:v1",
			Summary:      "异步任务：extract→create→portrait→three-views；sourceType fragment|manual_prompt|manual_form",
			NotInAgent:   nil,
			DriftAnchors: []string{
				"character name is required",
				"CharacterGenerationStepExtract",
			},
		},
		{
			Path:         "grapery/internal/service/storyboard_redesign_prompts.go",
			Symbol:       "buildStoryboardBiblePlanSystemPrompt / buildStoryboardSceneWriterSystemPrompt",
			Version:      "2026-06-11",
			Role:         RoleOrchestration,
			AgentVersion: "storyboard_director:v1",
			Summary:      "Bible-Beats-Scenes、comicFunction、panelShape、layout 字段；grapery 英文 strict JSON",
			NotInAgent: []string{
				"validateStoryboardBiblePlan 完整校验",
				"storyboard_comic_page.go 拼贴出图细节",
			},
			DriftAnchors: []string{
				"You are a cinematic storyboard writer and manga/comic panel director",
				"inner_monologue|action_impact|reaction|turning_point|shock",
				"panelShape encodes the clipping shape",
			},
		},
		{
			Path:         "grapery/internal/service/storyboard.go",
			Symbol:       "GenerateStoryboardScenes (legacy Chinese scene writer)",
			Version:      "2026-06-11",
			Role:         RoleReference,
			AgentVersion: "storyboard_director:v1",
			Summary:      "旧版中文分镜编剧；主路径为 create 后台 redesign + regenerate_structure",
			NotInAgent:   nil,
			DriftAnchors: []string{
				"你是一位专业的故事分镜编剧",
			},
		},
		{
			Path:         "grapery/internal/service/narrator_pipeline.go",
			Symbol:       "generateStoryboardContent (continuation)",
			Version:      "2026-06-11",
			Role:         RoleReference,
			AgentVersion: "branch_explorer:v1",
			Summary:      "续写正文 800-1500 字；agent 提供 strategy 前缀",
			NotInAgent: []string{
				"generateScenesFromContent JSON 场景规划",
			},
			DriftAnchors: []string{
				"你是一位专业的小说作家。请根据以下上下文和用户输入，续写一个引人入胜的故事章节",
				"篇幅控制在 800-1500 字",
			},
		},
		{
			Path:         "grapery/internal/service/story.go",
			Symbol:       "enrichStoryDescription / AI story generation",
			Version:      "2026-06-11",
			Role:         RoleReference,
			AgentVersion: "story_generator:v1",
			Summary:      "故事 enrich 与 generate-story；无 Story chat agent",
			NotInAgent: []string{
				"StoryCreator chat agent",
			},
			DriftAnchors: []string{
				"enrichedDescription",
				"你是一位资深故事创作作家兼编辑",
			},
		},
		{
			Path:         "grapery/internal/transport/http/ai_handler.go",
			Symbol:       "POST /api/v1/ai/generate-story",
			Version:      "2026-06-11",
			Role:         RoleReference,
			AgentVersion: "story_generator:v1",
			Summary:      "generation/stories 调用入口",
			NotInAgent:   nil,
			DriftAnchors: []string{
				"generate-story",
			},
		},
	}
}
