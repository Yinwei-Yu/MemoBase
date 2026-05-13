---
name: deploy-memo
description: Deploy MemoBase (知忆) knowledge-base platform. Start all services, pull models, verify health, open frontend.
---

# MemoBase 部署技能

按照以下步骤完成 MemoBase 项目的环境配置、服务启动和验证。

## 项目架构

```
浏览器 → 前端 (nginx:5173→8080)
              ↓
         后端 (Go:8080) → PostgreSQL, Qdrant, Ollama
              ↓
         Agent 服务 (Python gRPC:50051) → PostgreSQL, Qdrant, Ollama
```

**6 个服务：** postgres (5432), qdrant (6333), ollama (11434), agent-service (50051), backend (8080), frontend (5173)

## 部署步骤

### 步骤 1：前置条件

```bash
docker --version        # >= 24.0
docker compose version  # >= 2.20
```

### 步骤 2：清理旧容器

```bash
docker compose down --remove-orphans 2>/dev/null || true
```

容器名冲突时：
```bash
docker rm -f memobase-postgres memobase-qdrant memobase-ollama memobase-agent-service memobase-backend memobase-frontend 2>/dev/null || true
```

### 步骤 3：构建并启动所有服务

在项目根目录执行：

```bash
docker compose up -d --build
```

启动顺序由健康检查保证：
1. postgres (`pg_isready`)
2. qdrant (TCP 6333)
3. ollama (TCP 11434)
4. agent-service (TCP 50051) — 依赖 1-3 全部 healthy
5. backend (`wget /api/v1/readyz`) — 依赖 1-4 全部 healthy
6. frontend (`wget http://127.0.0.1:8080/healthz`) — 依赖 backend healthy

### 步骤 4：等待服务就绪

```bash
docker compose ps
```

约 2-3 分钟全部 `healthy`。

### 步骤 5：拉取 Ollama 模型（首次必须）

```bash
curl http://localhost:11434/api/pull -d '{"name":"qwen2.5:3b"}'
curl http://localhost:11434/api/pull -d '{"name":"nomic-embed-text"}'
```

约 2GB。未拉取则文档索引和 AI 对话失败。

### 步骤 6：验证

```bash
curl http://localhost:8080/api/v1/readyz   # → {"status":"ready",...}
python -c "import socket; s=socket.socket(); s.connect(('localhost',50051)); print('OK'); s.close()"
curl http://localhost:5173/healthz
```

### 步骤 7：打开前端

```
browser_navigate: http://localhost:5173
```

**默认账号：** `demo` / `demo123`

## 端口

| 服务 | 主机端口 |
|------|---------|
| 前端 | 5173 |
| 后端 API | 8080 |
| Agent gRPC | 50051 |
| PostgreSQL | 5432 |
| Qdrant | 6333 |
| Ollama | 11434 |

## 验证清单

- [ ] `docker compose ps` 全部 healthy
- [ ] `curl localhost:8080/api/v1/readyz` → `"status":"ready"`
- [ ] `curl localhost:5173` 返回 HTML
- [ ] 模型已拉取：`curl localhost:11434/api/tags | grep -E "qwen2.5:3b|nomic-embed-text"`
- [ ] 浏览器 `localhost:5173` 显示登录页
- [ ] `demo` / `demo123` 可登录
- [ ] 可创建知识库并上传文档索引
- [ ] AI 问答正常

## 运维命令

```bash
docker compose logs -f backend           # 后端日志
docker compose logs -f agent-service     # Agent 日志
docker compose restart agent-service     # 重启 Agent
docker compose build agent-service && docker compose up -d agent-service  # 重建
docker exec -it memobase-postgres psql -U memo -d memo  # 连接数据库
docker compose down                      # 停止
docker compose down -v                   # 停止并删除数据
```

## 已知问题

所有已知问题已在当前代码中修复：

| 问题 | 修复位置 |
|------|---------|
| Ollama 模型未拉取 → `MODEL_UNAVAILABLE` | 执行步骤 5 |
| 容器名冲突 | 执行步骤 2 |
| 前端 healthcheck IPv6 问题 | `docker-compose.yml` 使用 `127.0.0.1` |
| Agent healthcheck grpc 模块找不到 | `docker-compose.yml` 使用 TCP socket |
| BM25 检索表名 `chunks` → `document_chunks` | `agent-service/retriever/hybrid.py:120` |
| Qdrant `search()` → `query_points()` | `agent-service/retriever/hybrid.py:191` |
| proto 裸导入 → 相对导入 | `agent_pb2_grpc.py:6` 使用 `from . import` |

## 生产部署

```bash
export POSTGRES_PASSWORD="强密码"
export JWT_SECRET="长随机密钥"
export CORS_ORIGIN="https://域名"

docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d --build
```

生产差异：端口不外曝、禁用 demo 用户、资源限制。

## 备份恢复

```bash
./scripts/backup.sh                       # 备份
./scripts/restore.sh <文件.tar.gz>         # 恢复
```
