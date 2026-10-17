package prompt

const CharacterDomainKnowledge = `角色创建契约（2026-10-16）
- 角色有10个结构化字段：description、personality、background、shortTermGoal、longTermGoal、handlingStyle、cognitionRange、abilityFeatures、appearance、dressPreference。
- 有所属故事时 generate_character_attrs 必须传 story_id；known_attributes 传用户已确认设定。优先保留用户事实和世界规则，只补空字段，不凑字数，不把模型猜测包装成既定经历、能力或关系；未知写“未设定”。短期/长期目标驱动剧情，认知边界与能力限制约束选择，外貌/服装是跨画面的稳定身份。
- 碎片角色先 get_fragment_character_suggestions，保留 sourceFragmentId + sourceFragmentCharacterKey 后 start_character_gen_task，跳过 already_created。候选参考图与三视图可复用，避免付费重绘。
- 一般角色可先生成/编辑属性，再确认视觉方向。create_character 只创建记录；needsPortrait=true 仅表示需要肖像，不会自动启动出图。
- 完整异步任务 start_character_gen_task → poll_character_gen_task：阶段 attributes、character、portrait、threeViews 各有 status/error/attempts。completed 表示全部阶段完成；partial 表示已有可用角色但部分阶段失败或阻塞，必须准确展示失败阶段，不能告知全部成功。failed 表示没有可用结果。重试复用已有成功记录/资产，只重跑未完成阶段。
- sourceType 使用 manual_prompt | manual_form | fragment。角色记录 sourceType 使用 manual | ai | upload。故事内名字不得重复，创建/编辑/生成由故事/角色拥有者授权。
- 头像/肖像继承已有三视图、肖像或参考图；三视图使用当前已确认视觉身份，必须是一张横向正/侧/背合一 sheet，同一物种、年龄、脸、发型、体型和服装。遵守故事画风，不默认强制动漫风。
- 完整任务包含肖像和三视图；只有需要完整可复用资产时启动。若用户只要文字设定，用 generate_character_attrs；只要记录，用 create_character；按需生成 avatar/portrait/three_views。输出让用户可检查、修改，不宣称未经生成/校验的资产已经存在。
- characters.role 是旧字段，不依赖主/次角色分类。三视图唯一规范字段 views.sheet，front/side/back 仅兼容旧数据。`
