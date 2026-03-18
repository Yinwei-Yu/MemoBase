# 知忆（MemoBase）前后端 API 规范（MVP）

## 1. 目的与边界

本文档定义前后端联调所使用的 HTTP API 规范，目标是让前端与后端在并行开发时基于同一契约实现。

固定技术边界（来自当前项目已确认决策）：
- 后端：`Go + Gin`
- 前端：`React + TypeScript + Vite`
- 存储：`PostgreSQL + 本地文件存储`
- 检索：`BM25 + Qdrant`（中文分词基线：`jieba`）
- 模型：外部模型 API + 本地 `Ollama`
- 编排：轻量 `ReAct`

MVP 禁止引入（除非团队批准 RFC）：
- `OpenSearch`
- `MinIO`
- `Redis`
- `Viper`
- `Nginx`

---

## 2. 全局协议

### 2.1 Base URL 与版本

- Base URL：`/api/v1`
- 版本策略：
  - 非破坏性变更：仅新增字段
  - 破坏性变更：发布 `/api/v2`

### 2.2 数据与命名约定

- Content-Type：`application/json`（文件上传除外）
- 字段命名：`snake_case`
- 时间格式：`ISO 8601 UTC`（例：`2026-03-18T12:00:00Z`）
- ID 类型：字符串（建议前缀：`kb_`、`doc_`、`sess_`、`task_`、`trace_`）

### 2.3 鉴权与请求头

| Header | 必填 | 说明 |
|---|---|---|
| `Authorization` | 除登录外必填 | `Bearer <access_token>` |
| `Content-Type` | JSON 接口必填 | `application/json` |
| `X-Request-Id` | 可选（建议） | 客户端请求 ID，用于链路排障 |
| `Idempotency-Key` | 幂等写接口建议 | 防止重试重复提交 |

### 2.4 统一响应结构

成功：

```json
{
  "data": {},
  "request_id": "req_01H...",
  "timestamp": "2026-03-18T12:00:00Z"
}
```

失败：

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "question is required",
    "details": {
      "field": "question"
    }
  },
  "request_id": "req_01H...",
  "timestamp": "2026-03-18T12:00:00Z"
}
```

### 2.5 错误码与 HTTP 映射

| code | HTTP | 说明 |
|---|---|---|
| `INVALID_ARGUMENT` | 400 | 参数格式错误 |
| `UNAUTHORIZED` | 401 | 未登录或 token 无效 |
| `FORBIDDEN` | 403 | 无权限 |
| `KB_NOT_FOUND` / `DOC_NOT_FOUND` / `SESSION_NOT_FOUND` | 404 | 资源不存在 |
| `CONFLICT` | 409 | 状态冲突或重复资源 |
| `VALIDATION_ERROR` | 422 | 业务字段校验失败 |
| `RATE_LIMITED` | 429 | 请求频率受限 |
| `QDRANT_UNAVAILABLE` | 503 | 向量检索不可用 |
| `MODEL_UNAVAILABLE` | 503 | 模型依赖不可用 |
| `UPSTREAM_TIMEOUT` | 504 | 外部依赖超时 |
| `INTERNAL` | 500 | 内部错误 |

---

## 3. 通用设计规则

### 3.1 分页协议

查询参数：
- `page`（从 1 开始）
- `page_size`（默认 20，最大 100）

分页响应：

```json
{
  "data": {
    "items": [],
    "pagination": {
      "page": 1,
      "page_size": 20,
      "total": 132
    }
  },
  "request_id": "req_...",
  "timestamp": "2026-03-18T12:00:00Z"
}
```

### 3.2 排序与过滤

- 排序：`sort_by` + `sort_order=asc|desc`
- 过滤：统一使用 query 参数，不在 body 中混用

### 3.3 幂等与重试

以下接口建议支持 `Idempotency-Key`：
- 创建知识库
- 上传文档
- 触发重建索引

服务端应在幂等窗口内返回同一语义结果，避免重复任务。

### 3.4 长任务协议

所有长耗时操作（文档解析/建索引/重建索引）走任务化：
- 提交接口返回 `task_id`
- 通过任务查询接口轮询状态

任务状态流转：
- `pending -> processing -> succeeded | failed`

---

## 4. 外部 HTTP API（前后端联调）

## 4.1 Auth

### POST `/auth/login`

请求：

```json
{
  "username": "demo",
  "password": "******"
}
```

响应 `data`：

```json
{
  "access_token": "jwt_access",
  "refresh_token": "jwt_refresh",
  "expires_in": 7200,
  "user": {
    "user_id": "u_001",
    "username": "demo",
    "display_name": "Demo User"
  }
}
```

### POST `/auth/refresh`

请求：`{ "refresh_token": "..." }`

### POST `/auth/logout`

### GET `/auth/me`

---

## 4.2 Knowledge Base

### POST `/knowledge-bases`

请求：

```json
{
  "name": "操作系统期末复习",
  "description": "课程资料与历年题",
  "tags": ["OS", "exam"]
}
```

字段约束：
- `name`：1-64 字符
- `description`：0-512 字符
- `tags`：最多 10 个

### GET `/knowledge-bases?page=1&page_size=20&keyword=`

### GET `/knowledge-bases/{kb_id}`

### PATCH `/knowledge-bases/{kb_id}`

### DELETE `/knowledge-bases/{kb_id}`

---

## 4.3 Document

### POST `/knowledge-bases/{kb_id}/documents`

- Content-Type：`multipart/form-data`

表单字段：

| 字段 | 必填 | 约束 |
|---|---|---|
| `file` | 是 | `pdf/docx/txt/md`，<= 20MB |
| `title` | 否 | 1-128 字符 |
| `chunk_size` | 否 | 200-1200，默认 500 |
| `chunk_overlap` | 否 | 0-300，默认 100 |
| `reindex` | 否 | 默认 `true` |

响应 `data`：

```json
{
  "doc_id": "doc_001",
  "kb_id": "kb_001",
  "status": "pending",
  "task_id": "task_001",
  "created_at": "2026-03-18T12:00:00Z"
}
```

### GET `/knowledge-bases/{kb_id}/documents?page=1&page_size=20&status=`

`status`：`pending | processing | indexed | failed`

### GET `/knowledge-bases/{kb_id}/documents/{doc_id}`

### DELETE `/knowledge-bases/{kb_id}/documents/{doc_id}`

### POST `/knowledge-bases/{kb_id}/documents/{doc_id}/reindex`

响应：返回新的 `task_id`

---

## 4.4 Task（长任务状态）

### GET `/tasks/{task_id}`

响应 `data`：

```json
{
  "task_id": "task_001",
  "type": "document_index",
  "status": "processing",
  "progress": 65,
  "error_code": null,
  "error_message": null,
  "created_at": "2026-03-18T12:00:00Z",
  "updated_at": "2026-03-18T12:00:20Z"
}
```

---

## 4.5 Chat / Agent

### POST `/chat/completions`

请求：

```json
{
  "kb_id": "kb_001",
  "session_id": "sess_001",
  "question": "进程和线程的核心区别是什么？",
  "model": "llm_default",
  "use_agent": true,
  "top_k": 6,
  "include_trace": true
}
```

字段约束：
- `kb_id`：必填
- `session_id`：可选，缺失时后端可创建匿名会话或返回错误（二选一，实施前固定）
- `question`：1-2000 字符
- `model`：模型网关注册模型标识
- `top_k`：1-20，默认 6
- `use_agent`：默认 `false`
- `include_trace`：默认 `false`

响应 `data`：

```json
{
  "answer": "进程是资源分配的基本单位，线程是调度执行的基本单位。",
  "citations": [
    {
      "doc_id": "doc_001",
      "doc_title": "os-notes.pdf",
      "chunk_id": "ck_101",
      "snippet": "线程共享进程地址空间...",
      "score": 0.87,
      "retrieval_source": "fused"
    }
  ],
  "memory_used": [
    {
      "memory_id": "mem_01",
      "type": "short_term",
      "summary": "用户正在准备操作系统考试"
    }
  ],
  "trace_id": "trace_001",
  "degraded": false,
  "latency_ms": 1832,
  "token_usage": {
    "prompt_tokens": 1231,
    "completion_tokens": 214,
    "total_tokens": 1445
  }
}
```

### GET `/chat/traces/{trace_id}`

`steps[].tool` 仅允许：
- `search_knowledge`
- `search_memory`
- `summarize_context`

---

## 4.6 Session

### POST `/sessions`

请求：

```json
{
  "kb_id": "kb_001",
  "title": "操作系统复习对话"
}
```

### GET `/sessions?page=1&page_size=20&kb_id=`

### GET `/sessions/{session_id}`

### GET `/sessions/{session_id}/messages?page=1&page_size=50`

### DELETE `/sessions/{session_id}`

---

## 4.7 Health / Observability

### GET `/healthz`

### GET `/readyz`

响应 `data` 示例：

```json
{
  "status": "ready",
  "checks": {
    "db": "up",
    "qdrant": "up",
    "storage": "up",
    "model_gateway": "up"
  }
}
```

### GET `/metrics`

用于 Prometheus 抓取。

---

## 5. 内部服务契约（后端模块间）

> 以下不直接暴露给前端，用于后端分层解耦。

```go
type RetrievalService interface {
    Retrieve(ctx context.Context, req RetrievalRequest) ([]Chunk, error)
}

type MemoryService interface {
    Recall(ctx context.Context, req RecallRequest) ([]MemoryItem, error)
    Upsert(ctx context.Context, req UpsertMemoryRequest) error
    SummarizeSession(ctx context.Context, sessionID string) (string, error)
}

type ModelGateway interface {
    Generate(ctx context.Context, req LLMRequest) (LLMResponse, error)
    ListModels(ctx context.Context) ([]ModelMeta, error)
}

type AgentOrchestrator interface {
    Run(ctx context.Context, req AgentRunRequest) (AgentResult, error)
    GetTrace(ctx context.Context, traceID string) (AgentTrace, error)
}
```

约束：
- 业务层必须经 `ModelGateway` 调模型
- 智能体工具不得直连数据库
- API 层不得直连检索/向量库/文件系统

---

## 6. Good / Base / Bad 示例（`POST /chat/completions`）

### Good（完整且推荐）

请求：

```json
{
  "kb_id": "kb_001",
  "session_id": "sess_001",
  "question": "进程和线程的核心区别是什么？",
  "model": "llm_default",
  "use_agent": true,
  "top_k": 6,
  "include_trace": true
}
```

结果：`200`，返回 `answer + citations + trace_id + token_usage`。

### Base（最小可用）

请求：

```json
{
  "kb_id": "kb_001",
  "question": "什么是死锁？"
}
```

结果：`200`，后端使用默认 `model/top_k/use_agent/include_trace`。

### Bad（非法）

请求：

```json
{
  "kb_id": "",
  "question": ""
}
```

结果：`422`

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "kb_id and question are required",
    "details": {
      "fields": ["kb_id", "question"]
    }
  },
  "request_id": "req_01H...",
  "timestamp": "2026-03-18T12:00:00Z"
}
```

---

## 7. 前后端联调最小清单

1. 前端按 `snake_case` 字段消费接口。
2. 前端只基于 `error.code` 分支，不依赖 `message` 文案。
3. 文件上传后必须轮询 `GET /tasks/{task_id}`，不要假设同步完成。
4. 聊天响应中的 `citations` 必须可渲染来源片段。
5. `trace_id` 仅在 `include_trace=true` 或 `use_agent=true` 时要求返回。

---

## 8. 变更管理

- 任何字段删除/改名：视为破坏性变更，走 `/api/v2`。
- 新增可选字段：允许在 `v1` 内发布。
- 接口契约变更必须同步更新本文档与前端类型定义。
