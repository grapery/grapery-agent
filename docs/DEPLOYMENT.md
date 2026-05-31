# grapery-agent 部署说明

## 架构

- **容器名**：`grapery-agent`，监听 `9020`
- **Docker 网络**：`grapery-network`（与 `server`、`ngx` 等共用）
- **上游**：`GRAPERY_BASE_URL=http://server:8080`（容器内 DNS）
- **公网入口**：由 [ngx](https://github.com/grapery/ngx) 反向代理（见下方路由）

创作额度与会员校验由 **grapery server** 在 API 层完成；agent 透传用户 JWT 调用 grapery，无需单独配置 VIPPay。

## 部署顺序

1. **server**（`grapery` 仓库 `server-ci.yml`）
2. **grapery-agent**（本仓库 `agent-ci.yml`）
3. **ngx**（路由变更后，在 `ngx` 仓库手动触发 `ngx-ci.yml` → `workflow_dispatch`）

## GitHub Actions

工作流：[`.github/workflows/agent-ci.yml`](../.github/workflows/agent-ci.yml)

| 事件 | 行为 |
|------|------|
| PR | 构建 + prompt 漂移测试 |
| `develop` push | dev 镜像 + 部署到 `DEV_DEPLOY_HOST` |
| `main` push | prod 镜像 + 部署（**默认关闭**，见下） |
| `workflow_dispatch` | 选择 dev / prod（prod 需开关） |

**生产部署开关**：仓库 Variables 设置 `ENABLE_AGENT_PROD_DEPLOY=true` 后，`main` 推送或手动选 prod 才会构建/部署生产。未设置时仅走 develop/dev。

远端目录：`/home/ubuntu/grapery/grapery-agent/`

### 需在仓库配置的 Variables（与 grapery 对齐，勿用 Secrets）

在 **grapery-agent** 仓库 [Settings → Variables → Actions](https://github.com/grapery/grapery-agent/settings/variables/actions) 中，从 [grapery 同名页](https://github.com/grapery/grapery/settings/variables/actions) 复制整套 **Repository variables**（名称与值一致），至少包含：

| 类别 | 变量（示例） |
|------|----------------|
| 镜像 / 部署 | `ACR_*`, `DEV_DEPLOY_HOST`, `PROD_DEPLOY_HOST`, `SSH_USER`, `SSH_KEY` |
| 数据库 / 缓存 | `DB_ADDRESS`, `DB_DATABASE`, `DB_USERNAME`, `DB_PASSWORD`, `REDIS_ADDRESS`, `REDIS_PASSWORD`, `REDIS_DATABASE` |
| AI / Agent | `HUOSHAN_*`, `GEMINI_*`, `AI_DEFAULT_PROVIDER`, `EINO_*`, `GRAPERY_AGENT_API_KEY`（可与 `JWT_SECRET` 同源） |
| 阿里云 | `ALIYUN_*`, `ALIYUN_OSS_*`, `ALIYUN_SMS_*` |
| 鉴权 | `JWT_SECRET`, `JWT_EXPIRY_HOURS` |

`agent-ci.yml` 部署生成的 `.env` 会写入 DB、Redis、JWT、阿里云 OSS/SMS 等，与 server 侧配置一致，便于后续能力扩展或与 grapery 共用基础设施。

### Prompt 漂移测试

CI 会 checkout `grapery/grapery` 与 `grapery-agent` 为同级目录后执行：

```bash
cd grapery-agent && go test ./internal/prompt/ -run TestGraperyPromptAnchors
```

## Nginx 路由（ngx）

| 公网路径 | 后端 |
|----------|------|
| `/api/agent/*` | `grapery-agent:9020`（rewrite → `/api/v1/agent/*`） |
| `/api/v1/agent/*` | `grapery-agent:9020` |
| `/api/v1/generation/*` | `grapery-agent:9020` |

原 `chatmcp:8082` 已由 grapery-agent 承接 `/api/agent/` 入口。

## Dev / Prod 区分

与 `grapery` 的 `server-ci.yml` 相同：**一套 GitHub Variables**，由 CI 在生成 `.env` 时按环境写入不同默认值。

| 项 | Development (`develop` / dispatch `dev`) | Production (`main` / dispatch `prod`) |
|----|------------------------------------------|----------------------------------------|
| 部署主机 | `DEV_DEPLOY_HOST` | `PROD_DEPLOY_HOST` |
| 镜像 | `…/grapery-dev/grapery-agent:dev` | `…/grapery-prod/grapery-agent:prod` |
| `ENVIRONMENT` / `GRAPERY_ENV` | `development` | `production` |
| `DB_DATABASE` 默认 | `grapery_dev` | `grapery` |
| `ALIYUN_BUCKET` 默认 | `grapery-dev` | `grapery-prod` |
| `LOG_LEVEL` 默认 | `debug` | `info` |
| compose 项目名 | `grapery-agent` | `grapery-agent` |

模板文件（本地手工部署）：

- `env.grapery-agent.dev.example` → 复制为 `.env` 联调开发
- `env.grapery-agent.prod.example` → 生产字段清单（勿提交密钥）

Docker 相关文件：

| 文件 | 作用 |
|------|------|
| `Dockerfile` | 多阶段构建，暴露 `9020`，内置 `/health` |
| `docker-compose.grapery-agent.yml` | `IMAGE` / `SERVICE_NAME` / `SERVER_PORT` 由 `.env` 注入；挂载 `data/agent-artifacts` |
| `.dockerignore` | 排除 `.env`、`data/`、文档等 |

## agent-ci 与 server-ci 变量对照

| 区块 | server-ci | agent-ci（deploy .env） |
|------|-----------|------------------------|
| 环境元数据 | `ENVIRONMENT`, `GRAPERY_ENV`, `SERVICE_NAME`, `IMAGE` | 同左（`GRAPERY_HTTP_PORT` → `SERVER_PORT`） |
| DB / Redis | ✅ | ✅ |
| AI | `HUOSHAN_*`, `GEMINI_*`, `AI_DEFAULT_PROVIDER` | + `EINO_*`, `HUOSHAN_IMAGE/VIDEO_MODEL` |
| JWT | ✅ | ✅ |
| 阿里云 OSS/SMS/SLS | ✅ | ✅（SMS `USE_DEFAULT_CREDENTIAL` 可来自 var） |
| APNs / Apple IAP | ✅（server + p8 文件） | ❌（agent 不经手推送） |
| VIPPay / 微信 / Google | vippay-ci | ❌ |

## 本地 Docker

```bash
cp env.grapery-agent.dev.example .env
# 编辑 HUOSHAN_API_KEY、DB_* 等；联调 server 时保持 GRAPERY_BASE_URL=http://server:8080

docker build -t grapery-agent:local .
docker network create grapery-network 2>/dev/null || true
docker compose -f docker-compose.grapery-agent.yml -p grapery-agent up -d
curl -s http://127.0.0.1:9020/health
```

注意：本地若未加入 `grapery-network` 且无 `server` 容器，工具调用会失败；与 server 联调时请使用同一 network。
