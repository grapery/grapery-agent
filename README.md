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

- 开发（仅当 `.env` 不存在时复制，避免覆盖已有配置）：
  ```bash
  [ ! -f .env ] && cp env.grapery-agent.dev.example .env
  ```
- 可选：`make sync-env-from-grapery` — 若无 `.env` 则创建；已有文件只填空字段，不覆盖已有值
- 生产字段参考：`env.grapery-agent.prod.example`（CI 自动生成远端 `.env`）

关键变量见 `env.grapery-agent.dev.example`：`HUOSHAN_API_KEY`、`AGENT_TOKEN_VERIFY_KEY`（= grapery `AGENT_TOKEN_SIGNING_KEY`）、`GRAPERY_API_KEY`（= `GRAPERY_INTERNAL_API_KEY`）、`GRAPERY_BASE_URL`。

## 运行

```bash
[ ! -f .env ] && cp env.grapery-agent.dev.example .env   # 仅首次
# 或：make sync-env-from-grapery
make run                                # 或 make run-agent（loads .env if present）

# 等价
go run ./cmd/server
```

也可从 `grapery/`：`make run-agent` / `make sync-agent-env`。

## 部署

Dev/prod 通过 GitHub Actions 构建镜像并部署到 ECS；公网入口由 ngx 反向代理。详见 [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md)。
