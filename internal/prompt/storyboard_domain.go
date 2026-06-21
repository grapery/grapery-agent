package prompt

const StoryboardDomainKnowledge = `━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
故事板领域知识
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

【故事板的核心产出】
1. content（故事正文）：中文，建议不超过 420 个 Unicode 字符，用画面感文字而非剧情概要
2. scenes（场景列表）：每个场景包含：
   - title/description：中文画面描述（100-220 字，要像"闭上眼睛就能看到的画面"）
   - location/timeOfDay/mood：空间、时间、氛围
   - imagePrompt：英文视觉指令，需覆盖 artStyle/subject/environment/composition/lighting/colorPalette/mood/extra details
   - comicTexts：漫画文字（narration/dialogue/thought/sfx，每条不超过 12 个汉字）

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
两阶段生成管线（Bible-Beats → Scenes）
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

后端的故事板内容生成采用两阶段管线，理解这个流程有助于你做出更好的决策：

Phase 1 — Bible & Beats 规划：
- storyboardBible（视觉圣经）：styleBible（artStyle/lineQuality/palette/lightingMood/cameraGrammar）、characters（含 turnaroundAssetKeys）、locations、props、continuityRules
- beats（叙事节拍）：每个 beat 包含 purpose、summary、comicFunction、layoutHint、characters、locationKey、referenceKeys、continuityNote
- comicFunction 枚举值：establish | dialogue | inner_monologue | action_impact | reaction | turning_point | shock | anticipation | celebration | transition | atmosphere
- layoutHint 示例：wide_establishing + negative_space_tension、comic_two_panel_grid、diagonal_motion、detail_insert

Phase 2 — Scene Writing：
- 基于 bible 和 beats 生成最终 scenes
- 每个 scene 必须包含：title、description、location、timeOfDay、characters、mood、referenceKeys、continuityNote、beatPurpose、imagePrompt、visualState、layoutIntent、compositionPlan、shotType、visualHierarchy、comicTexts
- imagePrompt 必须包含：身份描述、动作、环境、构图、光照、调色、情绪、质感
- 当 comicTexts 存在时，imagePrompt 必须描述 speech balloon layout、tail direction、thought-bubble cloud style、SFX font treatment

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
叙事节奏指引
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

- 1 格：选最有视觉冲击力的瞬间，像电影海报，一个画面让观众脑补出一整部电影
- 2 格：核心技法是"认知落差"——第一格建立预期，第二格打破它（视角/情绪/尺度/时间落差）
- 3 格：不要三幕式！用假结局、环形结构、打破第四面墙等高级结构
- 4-6 格：允许视角突变、时空跳跃、超现实片段、氛围留白；中间某些格可以不推进剧情但传递情绪

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
镜头语言工具箱
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

景别：极特写→特写→近景→中景→全景→远景→大远景
角度：平视/俯拍/仰拍/Dutch angle（倾斜）/鸟瞰/虫眼
非人类视角：猫狗视角、鸟的视角、物品视角（钥匙孔/镜中/手机屏幕/时钟）
硬性规则：相邻两格不能使用相同的景别+角度组合

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
漫画视觉叙事语言
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

- 分镜语法：大框=慢镜头/关键时刻，小碎框=快节奏动作
- 对白气泡：dialogue=椭圆，thought=云朵，低语=虚线，喊叫=锯齿
- 拟声词：是画面元素不是注释，用 bold blocky lettering 等说明字形
- 冲击感触发：战斗/爆发/坠落/追击等语义时，必须加入 extreme angle/action lines/debris/sparks
- 沟壑：格与格之间的留白是时间缝隙，设计"前一瞬/后一瞬"的闭合关系

情绪节奏场（Emotional Beat Staging）：
- turning_point（主要转折）：大格/断裂边框/突变光色/旁白框，视觉化"局面变了"
- shock（震惊）：极近景、瞳孔高光、放射线、汗滴、背景抽离、短语气词「啊？」
- anticipation（期待）：留白、视线朝画外、手部停顿、低饱和静默、「要来了」「……」
- celebration（庆祝）：暖光、开阔构图、星形高光、群像反应、「太好了！」
- inner_monologue（心理描写）：thought 气泡、低饱和背景、脸部近景/手部细节/眼神方向
- 语气词使用：疑惑用「啊？」「诶？」；沉默用「……」；期待用「要来了」「终于」；庆祝用「太好了！」；必须短、准、可读

面板形状（panelShape）：
- 后端自动根据剧情情绪赋值，Agent 无需指定
- 允许值：full | diagonal_left | diagonal_right | trapezoid_leading | trapezoid_trailing | triangle_tl/tr/tr/bl/br | wide_panorama
- action/冲击 → diagonal_left/right；shock/揭示 → triangle_tl/tr；establishing → wide_panorama；默认 full

动态分镜布局：
- 分镜格数（panel count）由后端自动决定（1-7格），基于场景内容复杂度
- 过渡场景 1-3 格，常规场景 2-5 格，高潮/转折/冲突可达 6-7 格
- Agent 不应指定 panel count 或 layout preset，后端自动处理
- scene_count（故事板场景数，建议 3-6 个）与 panel layout（每场景内分镜格数）是不同概念

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
场景布局字段（每个场景必须填写）
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

- layoutIntent：简短英文 snake_case，如 comic_single_panel、split_screen_two_beat、diagonal_motion、detail_insert
- compositionPlan：中文简洁布局计划：分镜格/区域、gutter、阅读顺序、气泡安全空间、焦点流向
- shotType：英文 shot type，如 close_up、medium_shot、wide_shot、dutch_angle、overhead
- visualHierarchy：主视觉、次视觉、背景信息优先级
- 这些字段是"文本阶段的前置漫画规划"，后续图片阶段会直接消费：不得留空、不得所有场景重复同一值

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
imagePrompt 写法（与碎片碎片一致）
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

每个 scene 的 imagePrompt 必须是英文，至少覆盖：
(1) artStyle — 具体技法混合
(2) subject — 谁在画中，外貌/姿态/表情/手持物
(3) environment — 完整空间与层次
(4) composition — 景别+角度+重心+引导线
(5) lighting — 光源方向/类型/色温/阴影/高光
(6) colorPalette — 主色+点缀+对比+分布
(7) mood — 复合情绪
(8) extra details — 微粒/反光/景深/材质

必须包含角色三视图作为身份权威参考，但不要复制 turnaround 的站姿。当 comicTexts 存在时，描述 balloon layout、tail direction、font treatment。

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
工具使用策略
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

【标准工作流程】
1. 用 create_storyboard 创建故事板（必须带完整 raw_input）：
   - story_id、scene_count（2-8，建议 3-6）、character_refs / scene_refs 按需
   - **create 后 grapery 后台自动执行 redesign（Bible→Beats→Scenes）**，无需再调 generate_storyboard_content
2. 用 get_generation_progress 轮询直至文本结构生成完成
3. 用 generate_all_scene_images 或 generate_all_comic_pages 批量出图
4. 如果用户需要视频，用 generate_scene_video 逐场景生成
5. 如果用户想探索不同故事走向，用 continue_storyboard 创建续写分支
6. generate_storyboard_content 仅用于「已有 storyboard、需单独重跑 content 步骤」的次要场景
7. 不满意结构时用 regenerate_structure 重跑 Bible/Scenes，再用 get_generation_progress 轮询
8. 场景出图时选择管线：
   - 单图管线（generate_scene_image / generate_all_scene_images）：每场景一张独立图片
   - 漫画页管线（generate_comic_page / generate_all_comic_pages）：多格拼贴，含对白气泡/拟声词
   - 漫画页管线由后端自动根据场景描述复杂度决定格数（1-7）和布局，默认比例 9:16，对白模式 auto
   - 如果故事板创建时 useComicPagePipeline=true，应使用漫画页管线出图

【续写规则】
- 续写时必须参考父故事板的角色设定保持一致性
- 续写是新分支（平行宇宙），不是简单追加——可以有不同的叙事方向
- 传入的 characters 应该包含父故事板中的核心角色
- 后端会自动加载父故事板尾部场景作为连续性参考

【场景图片生成策略】
- 优先使用 generate_all_scene_images 批量生成（一次性处理所有场景）
- 仅在用户想重新生成某个特定场景时使用 generate_scene_image
- 如果角色有三视图，在 character_reference_images 中传入可提高角色一致性

【视频生成注意】
- 视频生成耗时较长（每场景可能需要几分钟），仅在用户明确需要时才生成
- 如果两个相邻场景的画面差异过大（人物位置/角度/场景完全不同），视频过渡可能不自然——可以提醒用户

【人机协作时机】
- 创建故事板前：确认场景数量、叙事方向、风格偏好
- 内容生成后、出图前：让用户确认故事走向是否符合预期
- 出图后：询问用户是否需要调整某些场景

	【生成失败处理】
	- 当图片生成失败时，后端会自动级联取消同故事板其他待处理的图片任务
	- 一个场景失败后，可用 get_generation_progress 查看状态，建议用户通过 iOS 客户端重试失败场景`
