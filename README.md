# Grapery Agent

Grapery AI Agent 服务，用于故事、角色、分镜、片段等内容的 AI 生成。

## 项目结构

```
cmd/server/          - 服务入口
internal/
  agents/            - Agent 编排
  artifact/          - RL artifact JSONL 导出
  config/            - 配置管理
  domain/            - 领域模型
  eval/              - 评估框架
  generation/        - 生成服务（story, character, storyboard, fragment, branch）
  grapery_client/    - Grapery 后端 API 客户端
  model/             - AI 模型适配（火山方舟等）
  prompt/            - Prompt 模板
  runstore/          - 运行时上下文与记忆
  tools/             - Agent 工具集
  transport/http/    - HTTP 传输层
```

## 配置

复制 `.env.example` 为 `.env` 并填入实际配置值。

## 运行

```bash
go run cmd/server/main.go
```
