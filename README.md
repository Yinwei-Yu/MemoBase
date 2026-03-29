# 知忆 MemoBase（MVP）

基于混合检索、记忆管理与 ReAct 编排的知识库智能体平台 MVP。当前仓库已提供可运行的前后端实现、Ollama 本地模型调用协议，以及 Docker 一键部署方案。

## 1. 技术栈与边界

- 前端：React + TypeScript + Vite
- 后端：Go + Gin
- 关系数据：PostgreSQL
- 向量检索：Qdrant
- 检索策略：关键词（BM25-like）+ 向量融合检索
- 模型网关：本地 Ollama（`/api/chat` + `/api/embeddings`）
- 编排：轻量 ReAct 工具轨迹

MVP 不引入：OpenSearch / MinIO / Redis / Viper / Nginx。

## 2. 功能清单（MVP）

- 用户登录（默认 demo 账号）
- 知识库创建、列表、删除
- 文档上传、索引任务状态轮询、重建索引、删除
- 问答页面：答案 + 引用 + 执行轨迹
- 会话列表与消息查看
- 健康检查与依赖状态页面

当前解析能力边界：文档内容解析仅支持 `.txt/.md` 文本文件。

## 3. 目录结构

```text
.
├── backend/                  # Go API 服务
│   ├── cmd/server
│   ├── internal/
│   │   ├── api               # 路由与 handler
│   │   ├── core              # 文档处理、检索、聊天编排
│   │   ├── infra             # DB / Qdrant / Ollama 客户端
│   │   └── store             # PostgreSQL 数据访问
│   └── migrations/           # 初始化 SQL
├── frontend/                 # React 前端
│   └── src/pages             # 登录/知识库/文档/问答/会话/运维页面
├── docker-compose.yml
└── doc/接口文档（MVP）.md     # API 契约文档
```

## 4. 快速启动（推荐：Docker Compose）

### 4.1 前置条件

- Docker Desktop >= 24
- 可用网络（首次拉取镜像与 Ollama 模型）

### 4.2 启动服务（开发环境）

```bash
docker compose up -d --build
```

启动后服务端口：
- 前端：<http://localhost:5173>
- 后端：<http://localhost:8080>
- Postgres：`localhost:5432`
- Qdrant：<http://localhost:6333>
- Ollama：<http://localhost:11434>

### 4.3 启动服务（生产配置覆盖）

```bash
POSTGRES_PASSWORD=replace-me \
JWT_SECRET=replace-with-long-random-secret \
CORS_ORIGIN=https://your-domain.example \
docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d --build
```

说明：
- `docker-compose.prod.yml` 会关闭 demo 用户、要求强密钥、并限制部分资源。
- 生产覆盖默认不对宿主机暴露 Postgres/Qdrant/Ollama 端口。

### 4.4 拉取 Ollama 模型（必须）

首次运行请在宿主机执行：

```bash
# 聊天模型
curl http://localhost:11434/api/pull -d '{"name":"qwen2.5:3b"}'

# 向量模型
curl http://localhost:11434/api/pull -d '{"name":"nomic-embed-text"}'
```

> 如果你希望换模型，请同时修改 `docker-compose.yml` 中 backend 的 `OLLAMA_CHAT_MODEL` 和 `OLLAMA_EMBED_MODEL`。

### 4.5 默认账号（仅开发环境）

- 用户名：`demo`
- 密码：`demo123`（`ENABLE_DEMO_USER=true` 时自动创建）

## 5. 本地开发模式（不使用 Compose）

## 5.1 启动基础依赖

你需要自行准备并启动：
- PostgreSQL（数据库 `memo`，用户 `memo`，密码 `memo`）
- Qdrant（`http://localhost:6333`）
- Ollama（`http://localhost:11434`，并已拉取模型）

## 5.2 启动后端

```bash
cd backend
go mod tidy
cp .env.example .env  # 可选，或直接设置环境变量
go run ./cmd/server
```

## 5.3 启动前端

```bash
cd frontend
npm install
npm run dev
```

如需指定 API 地址：

```bash
VITE_API_BASE=http://localhost:8080/api/v1 npm run dev
```

## 6. MVP 使用流程

1. 登录系统
2. 创建知识库
3. 进入文档页上传 `.txt/.md`
4. 等待任务状态变为 `succeeded`
5. 进入问答页提问，查看答案与引用
6. 在会话页查看消息历史
7. 在运维页查看依赖健康状态

## 7. Ollama 协议实现说明

后端已实现以下本地协议调用：

- 聊天：`POST /api/chat`
  - 入参：`model`, `messages`, `stream=false`
  - 用于生成最终回答
- 向量：`POST /api/embeddings`
  - 入参：`model`, `prompt`
  - 用于写入 Qdrant 与向量检索

相关环境变量（backend）：

```env
OLLAMA_URL=http://ollama:11434
OLLAMA_CHAT_MODEL=qwen2.5:3b
OLLAMA_EMBED_MODEL=nomic-embed-text
OLLAMA_TIMEOUT_SEC=120
```

## 8. API 与观测说明

- 主入口：`/api/v1`
- 统一成功结构：`{ data, request_id, timestamp }`
- 统一失败结构：`{ error: { code, message, details }, request_id, timestamp }`
- 指标端点：`/metrics`（兼容 `/api/v1/metrics`）

详细契约见：
- [doc/接口文档（MVP）.md](doc/接口文档（MVP）.md)

## 9. 常见问题

1. `MODEL_UNAVAILABLE`
- 原因：Ollama 未启动或模型未拉取。
- 处理：检查 `http://localhost:11434`，执行模型 pull。

2. 文档一直 `processing`
- 原因：向量化或 Qdrant 写入失败。
- 处理：查看 backend 日志与 `GET /api/v1/tasks/{task_id}`。

3. 前端请求 401
- 原因：token 失效。
- 处理：重新登录。

4. `readyz` 失败
- 原因：DB/Qdrant/Ollama 任一依赖不可用。
- 处理：先确保 compose 服务都已启动。

## 10. 开发建议

- 每次改 API 契约后，先更新 `doc/接口文档（MVP）.md`。
- 前后端联调时，优先用 `request_id` 排查链路。
- 如需扩展到课程增强目标（K8s/Prometheus/Grafana），建议在当前 compose 验证稳定后再拆分。

## 11. 备份与恢复（基础版）

```bash
# 备份（默认输出到 backups/<timestamp>）
./scripts/backup.sh

# 恢复（支持目录或 .tar.gz）
./scripts/restore.sh ./backups/<timestamp>.tar.gz
```
