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

- 开发：`cp env.grapery-agent.dev.example .env`
- 生产字段参考：`env.grapery-agent.prod.example`（CI 自动生成远端 `.env`）

## 运行

```bash
go run cmd/server/main.go
```

## 部署

Dev/prod 通过 GitHub Actions 构建镜像并部署到 ECS；公网入口由 ngx 反向代理。详见 [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md)。
