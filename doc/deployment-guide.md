# MemoBase 部署指南

## 环境要求

- Docker >= 24.0
- Docker Compose >= 2.20
- （可选）NVIDIA GPU 用于加速 LLM 推理

## 快速启动（开发环境）

```bash
# 进入项目目录
cd /path/to/memo

# 构建并启动所有服务
docker compose up -d --build

# 等待所有服务就绪（约 2-3 分钟）
docker compose ps

# 拉取 Ollama 模型（首次必须，总计约 2GB）
curl http://localhost:11434/api/pull -d '{"name":"qwen2.5:3b"}'
curl http://localhost:11434/api/pull -d '{"name":"nomic-embed-text"}'

# 验证所有服务就绪
curl http://localhost:8080/api/v1/readyz
```

**默认账号：** `demo` / `demo123`

**服务地址：**
| 服务 | 地址 |
|------|------|
| 前端 | http://localhost:5173 |
| 后端 API | http://localhost:8080/api/v1 |
| Agent gRPC | localhost:50051 |
| PostgreSQL | localhost:5432 |
| Qdrant | http://localhost:6333 |
| Ollama | http://localhost:11434 |

## 生产环境部署

```bash
# 设置必需的环境变量
export POSTGRES_PASSWORD="你的强密码"
export JWT_SECRET="你的长随机密钥（至少32字符）"
export CORS_ORIGIN="https://你的域名"

# 使用生产配置启动
docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d --build
```

生产配置变更：
- 移除 PostgreSQL、Qdrant、Ollama 的外部端口暴露
- 禁用 demo 用户
- 添加资源限制
- 必须显式设置 CORS_ORIGIN

## 系统架构

```
浏览器 → 前端 (nginx:8080)
              ↓
         后端 (Go:8080) → PostgreSQL, Qdrant, Ollama
              ↓
         Agent 服务 (Python:50051) → PostgreSQL, Qdrant, Ollama
```

## 服务详情

### 前端（React + Vite）
- 构建：`node:24-alpine` → `nginx-unprivileged:1.27-alpine`
- 使用 nginx 提供 SPA 服务，支持前端路由回退
- 健康检查：`wget http://127.0.0.1:8080/healthz`

### 后端（Go + Gin）
- 构建：`golang:1.25-alpine` → `alpine:3.20`
- REST API 路径：`/api/v1/*`
- JWT 认证
- 健康检查：`wget http://localhost:8080/api/v1/healthz`

### Agent 服务（Python + LangGraph）
- 构建：`python:3.12-slim` + `uv` 包管理器
- gRPC 服务端口：50051
- ReAct 智能体 + 混合检索（BM25 + 向量）
- 健康检查：TCP Socket 连接 50051 端口

### PostgreSQL 16
- 数据库：`memo`，用户：`memo`
- 启动时自动创建 demo 用户（`ENABLE_DEMO_USER=true`）

### Qdrant v1.16.3
- 向量数据库，用于文档嵌入存储
- 按知识库自动创建集合

### Ollama
- 本地 LLM 网关
- 必需模型：`qwen2.5:3b`（对话）、`nomic-embed-text`（嵌入）

## 环境变量

### 后端
| 变量 | 默认值 | 说明 |
|------|--------|------|
| `APP_ENV` | `dev` | 环境名称 |
| `PORT` | `8080` | HTTP 端口 |
| `CORS_ORIGIN` | `http://localhost:5173` | 允许的 CORS 来源 |
| `JWT_SECRET` | `memo-dev-secret` | JWT 签名密钥（生产环境必须修改！） |
| `TOKEN_TTL_HOURS` | `2` | Token 有效期（小时） |
| `DATABASE_URL` | （compose 设置） | PostgreSQL 连接字符串 |
| `STORAGE_DIR` | `/app/storage` | 文件存储路径 |
| `QDRANT_URL` | （compose 设置） | Qdrant 地址 |
| `QDRANT_COLLECTION` | `kb_chunks` | 集合名前缀 |
| `OLLAMA_URL` | （compose 设置） | Ollama 地址 |
| `OLLAMA_CHAT_MODEL` | `qwen2.5:3b` | 对话模型名称 |
| `OLLAMA_EMBED_MODEL` | `nomic-embed-text` | 嵌入模型名称 |
| `OLLAMA_TIMEOUT_SEC` | `120` | Ollama 请求超时（秒） |
| `ENABLE_DEMO_USER` | `true` | 自动创建 demo 用户 |
| `AGENT_SERVICE_URL` | （compose 设置） | Agent gRPC 地址 |

### Agent 服务
| 变量 | 默认值 | 说明 |
|------|--------|------|
| `GRPC_PORT` | `50051` | gRPC 监听端口 |
| `OLLAMA_URL` | （compose 设置） | Ollama 地址 |
| `OLLAMA_CHAT_MODEL` | `qwen2.5:3b` | 对话模型 |
| `OLLAMA_EMBED_MODEL` | `nomic-embed-text` | 嵌入模型 |
| `QDRANT_URL` | （compose 设置） | Qdrant 地址 |
| `QDRANT_COLLECTION_PREFIX` | `kb_chunks` | 集合名前缀 |
| `DATABASE_URL` | （compose 设置） | PostgreSQL 连接字符串 |
| `BM25_WEIGHT` | `0.5` | RRF 融合中 BM25 权重 |
| `VECTOR_WEIGHT` | `0.5` | RRF 融合中向量权重 |
| `TOP_K` | `6` | 默认检索数量 |

## 问题排查

常见问题及修复方法请参阅 [问题排查指南](./qa-troubleshooting.md)。

## 备份与恢复

```bash
# 备份
./scripts/backup.sh

# 恢复
./scripts/restore.sh <备份文件.tar.gz>
```

## 常用命令

```bash
# 查看日志
docker compose logs -f backend
docker compose logs -f agent-service

# 重启单个服务
docker compose restart agent-service

# 重新构建单个服务
docker compose build agent-service && docker compose up -d agent-service

# 检查服务健康状态
docker compose ps

# 连接 PostgreSQL
docker exec -it memobase-postgres psql -U memo -d memo

# 停止所有服务
docker compose down

# 停止并删除数据卷（会丢失数据！）
docker compose down -v
```
