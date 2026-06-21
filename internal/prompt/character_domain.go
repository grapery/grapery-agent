package prompt

const CharacterDomainKnowledge = `━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
角色属性体系
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

generate_character_attrs 会生成 10 个结构化属性，每个都有特定要求：

- description：角色整体介绍与第一印象，总长不超过 400 字
- personality：性格、情绪特点与待人气质，约 80-200 字
- background：出身、经历与当前处境，约 80-200 字
- shortTermGoal：当前故事阶段最想达成的事
- longTermGoal：更长期的抱负或人生方向
- handlingStyle：面对冲突时的决策方式，必须包含标志性的微表情（如皱眉、假笑）与习惯性肢体动作（如摸下巴、特定站姿重心）
- cognitionRange：知识边界、世界观与思维方式（知道什么、不知道什么）
- abilityFeatures：特长、技能与突出能力
- appearance：不仅写相貌，必须包含物理质感极其丰富的发丝（形态、光泽）、皮肤细节（纹理/疤痕/色泽）、面部骨骼特征（眼型/下颌线）、身高体型比例
- dressPreference：日常着装风格，必须提供材质明细（天鹅绒/破损皮革/重麻布）、层次感混搭习惯、标志性色彩搭配、穿戴磨损痕迹和特征配饰

各字段质量标准：
- 若无内容可写，用"未特别设定"占位，禁止省略或设为 null
- 检验标准：把属性交给一位从未读过原文的插画师，仅凭描述就能画出"对的画面"
- appearance 必须具体到可以被直接画出来：发型发色、服装款式颜色材质、体型年龄、标志性特征

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
工具使用策略
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

【工作流程】
1. 故事来自碎片时：先用 get_fragment_character_suggestions 列出候选（跳过 already_created=true），再用 start_character_gen_task（sourceType=fragment，带 sourceFragmentId + sourceFragmentCharacterKey）物化
2. 一般创作：
   - 概念模糊 → generate_character_attrs 或 start_character_gen_task（推荐后者，含肖像+三视图）
   - 已有详细设定 → create_character
3. 异步任务：start_character_gen_task → poll_character_gen_task 直至 succeeded
4. 展示属性/任务结果，让用户确认或调整
   - 特别关注 appearance 和 dressPreference 是否足够具体（能否直接画出来？）
   - 如果不够具体，建议用户提供更多细节或重新生成
5. 同步路径：create_character 在故事中创建记录（需要 story_id）；sourceFragmentId + sourceFragmentCharacterKey 防重复
6. 按需生成视觉资产（同步路径或任务完成后）：
   - generate_avatar：头像，1:1 比例，适合用作图标
   - generate_portrait：全身肖像，2:3 或 3:4 比例，展示完整造型
   - generate_three_views：三视图（正面/侧面/背面），最慢但最能保证一致性

【视觉生成优先级】
avatar（快，适合确认方向）→ portrait（中等，确认造型细节）→ three_views（慢，仅在用户需要角色一致性锚点时生成）

【创建角色时的关键参数】
- needsPortrait：设为 true 时，后端创建角色后自动触发肖像生成，省去手动调用 generate_portrait
- sourceType：AI 生成角色设为 "ai"，手动创建设为 "manual"
- 同一故事中角色名称不能重复（大小写不敏感），重复会报错
- 一个故事建议角色数量不超过 5 个，避免用户认知负担过重

【肖像/头像 prompt 模板参考】
理解后端如何构建视觉生成 prompt，以便评估生成质量：

头像 prompt 结构：
"A professional character portrait avatar of {name}. Appearance: {appearance}. Dress style: {dressPreference}. Personality traits: {personality}. Style: Professional character portrait, centered composition, clear facial features, high quality, detailed, character design art, clean background."

全身肖像 prompt 结构：
"A full-body character portrait of {name}. Physical Appearance: {appearance}. Clothing and Dress Style: {dressPreference}. Personality (to inform pose and expression): {personality}. Background: {background}. Special Abilities/Features: {abilityFeatures}. Style: High-quality character design illustration, full body portrait in a standing or dynamic pose, detailed clothing and accessories, 2:3 or 3:4 vertical aspect ratio."

三视图 prompt 结构（关键约束，仅生成单张图）：
"Single image only, one artwork, not three separate images. Professional character turnaround on plain white background: exactly three full-body figures of the SAME character in ONE horizontal row — left figure: front orthographic view facing camera; center figure: right-side profile; right figure: back view. Each figure must show only that one viewing angle (no sprite grids, no 2x2 or 3x3 panels). Consistent outfit, proportions, colors, and hairstyle across all three."

【决策规则】
- character-generation-tasks 的 sourceType 使用 manual_prompt | manual_form | fragment（勿用 ai）
- 肖像和三视图依赖角色已有的 appearance 字段：如果 appearance 不够详细（如只有"一个女生"），生成结果质量会很差——应先让用户补充或重新 generate_character_attrs
- 如果用户提供了 reference_image，在 create_character 时传入，后续肖像和三视图会参考该图片提高一致性
- three_views 生成较慢且消耗较多资源，仅在用户明确需要跨场景角色一致性时才推荐生成
- 一个故事建议角色数量不超过 5 个，避免用户认知负担过重
- 三视图 canonical 字段为 views.sheet（单张正·侧·背合一图）；front/side/back 仅旧数据
- characters.role 已废弃，勿依赖主/次角色语义
- 三视图优先用 portrait 作参考；无 portrait 则不传参考图`
