# 知忆（MemoBase）接口文档（MVP）

## 1. 文档范围

本接口文档依据《[模块组织与系统架构说明](./模块组织与系统架构说明.md)》中的架构图编写，覆盖：

- 前端可直接调用的 `HTTP API`（Auth / KB / Doc / Chat / Session / Health）
- 后端内部服务契约（Hybrid Retrieval / Memory / Agent / Model Gateway / Document Processing）
- MVP 主链路：`上传 -> 建索引 -> 提问 -> 返回答案+引用+轨迹`

---

## 2. 通用约定

### 2.1 基础信息

- Base URL：`/api/v1`
- 数据格式：`application/json`
- 时间格式：`ISO 8601`（UTC），示例：`2026-03-13T08:30:00Z`
- 鉴权方式：`Bearer JWT`

### 2.2 通用请求头

| Header | 必填 | 说明 |
|---|---|---|
| `Authorization` | 是（除登录/健康检查外） | `Bearer <token>` |
| `Content-Type` | 是（JSON 接口） | `application/json` |
| `X-Request-Id` | 否 | 客户端请求追踪 ID |

### 2.3 统一响应结构

```json
{
  "code": "OK",
  "message": "success",
  "data": {},
  "request_id": "req_01HV....",
  "timestamp": "2026-03-13T08:30:00Z"
}
```

失败示例：

```json
{
  "code": "VALIDATION_ERROR",
  "message": "kb_id is required",
  "data": null,
  "request_id": "req_01HV....",
  "timestamp": "2026-03-13T08:30:00Z"
}
```

### 2.4 统一错误码

| code | HTTP | 说明 |
|---|---|---|
| `OK` | 200 | 成功 |
| `BAD_REQUEST` | 400 | 参数错误 |
| `UNAUTHORIZED` | 401 | 未登录或 token 无效 |
| `FORBIDDEN` | 403 | 无权限访问资源 |
| `NOT_FOUND` | 404 | 资源不存在 |
| `CONFLICT` | 409 | 资源冲突（重名、状态冲突） |
| `VALIDATION_ERROR` | 422 | 字段校验失败 |
| `RATE_LIMITED` | 429 | 请求频率过高 |
| `INTERNAL_ERROR` | 500 | 服务内部错误 |
| `UPSTREAM_ERROR` | 502 | 模型或外部依赖异常 |
| `SERVICE_UNAVAILABLE` | 503 | 服务暂不可用 |

---

## 3. 外部 HTTP API（前后端联调）

## 3.1 Auth API

### 3.1.1 登录

- `POST /auth/login`

请求：

```json
{
  "username": "demo",
  "password": "******"
}
```

返回 `data`：

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

### 3.1.2 刷新 Token

- `POST /auth/refresh`

### 3.1.3 退出登录

- `POST /auth/logout`

### 3.1.4 获取当前用户

- `GET /auth/me`

---

## 3.2 Knowledge Base API

### 3.2.1 创建知识库

- `POST /knowledge-bases`

请求：

```json
{
  "name": "操作系统期末复习",
  "description": "课程资料与历年题",
  "tags": ["OS", "exam"]
}
```

返回 `data`：

```json
{
  "kb_id": "kb_001",
  "name": "操作系统期末复习",
  "description": "课程资料与历年题",
  "tags": ["OS", "exam"],
  "doc_count": 0,
  "created_at": "2026-03-13T08:30:00Z"
}
```

### 3.2.2 知识库列表

- `GET /knowledge-bases?page=1&page_size=20&keyword=`

### 3.2.3 知识库详情

- `GET /knowledge-bases/{kb_id}`

### 3.2.4 更新知识库

- `PATCH /knowledge-bases/{kb_id}`

### 3.2.5 删除知识库

- `DELETE /knowledge-bases/{kb_id}`

---

## 3.3 Document API

### 3.3.1 上传文档

- `POST /knowledge-bases/{kb_id}/documents`
- `Content-Type: multipart/form-data`

表单字段：

| 字段 | 必填 | 说明 |
|---|---|---|
| `file` | 是 | 支持 `pdf/docx/txt/md` |
| `title` | 否 | 文档标题（默认文件名） |
| `chunk_size` | 否 | 切片大小，默认 500 |
| `chunk_overlap` | 否 | 切片重叠，默认 100 |
| `reindex` | 否 | 是否立即构建索引，默认 true |

返回 `data`：

```json
{
  "doc_id": "doc_001",
  "kb_id": "kb_001",
  "file_name": "os-notes.pdf",
  "status": "pending",
  "task_id": "task_001",
  "created_at": "2026-03-13T08:30:00Z"
}
```

### 3.3.2 文档列表

- `GET /knowledge-bases/{kb_id}/documents?page=1&page_size=20&status=`

`status`：`pending | processing | indexed | failed`

### 3.3.3 文档详情

- `GET /knowledge-bases/{kb_id}/documents/{doc_id}`

### 3.3.4 删除文档（含索引同步删除）

- `DELETE /knowledge-bases/{kb_id}/documents/{doc_id}`

### 3.3.5 触发重建索引

- `POST /knowledge-bases/{kb_id}/documents/{doc_id}/reindex`

---

## 3.4 Chat API（问答 + 智能体）

### 3.4.1 发起问答

- `POST /chat/completions`

请求：

```json
{
  "kb_id": "kb_001",
  "session_id": "sess_001",
  "question": "进程和线程的核心区别是什么？",
  "model": "gpt-4o-mini",
  "use_agent": true,
  "top_k": 6,
  "include_trace": true
}
```

返回 `data`：

```json
{
  "answer": "进程是资源分配的基本单位，线程是调度执行的基本单位。",
  "citations": [
    {
      "doc_id": "doc_001",
      "doc_title": "os-notes.pdf",
      "chunk_id": "ck_101",
      "snippet": "线程共享进程地址空间...",
      "score": 0.87
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
  "latency_ms": 1832,
  "token_usage": {
    "prompt_tokens": 1231,
    "completion_tokens": 214,
    "total_tokens": 1445
  }
}
```

### 3.4.2 获取执行轨迹

- `GET /chat/traces/{trace_id}`

返回 `data` 关键字段：

| 字段 | 说明 |
|---|---|
| `steps[]` | ReAct 执行步骤 |
| `steps[].tool` | `search_knowledge/search_memory/summarize_context` |
| `steps[].input` | 工具输入参数 |
| `steps[].observation` | 工具输出摘要 |
| `steps[].latency_ms` | 每步耗时 |

---

## 3.5 Session API

### 3.5.1 创建会话

- `POST /sessions`

请求：

```json
{
  "kb_id": "kb_001",
  "title": "操作系统复习对话"
}
```

### 3.5.2 会话列表

- `GET /sessions?page=1&page_size=20&kb_id=`

### 3.5.3 会话详情

- `GET /sessions/{session_id}`

### 3.5.4 会话消息列表

- `GET /sessions/{session_id}/messages?page=1&page_size=50`

### 3.5.5 删除会话

- `DELETE /sessions/{session_id}`

---

## 3.6 Health & Observability API

### 3.6.1 存活检查

- `GET /healthz`

### 3.6.2 就绪检查（依赖检查）

- `GET /readyz`

返回 `data` 示例：

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

### 3.6.3 指标接口

- `GET /metrics`（Prometheus 抓取）

---

## 4. 后端内部服务契约（Service Layer）

以下接口不直接暴露给前端，用于保证架构图中的模块解耦与复用。

## 4.1 Document Processing Service

```go
ProcessDocument(ctx, req) (taskID string, err error)
ReindexDocument(ctx, docID string) (taskID string, err error)
GetDocumentTask(ctx, taskID string) (DocumentTaskStatus, error)
```

## 4.2 Hybrid Retrieval Service

```go
Retrieve(ctx, req) (results []RetrievedChunk, err error)
```

`req` 关键字段：
- `kb_id`
- `query`
- `top_k`
- `bm25_weight`
- `vector_weight`

## 4.3 Memory Service

```go
Recall(ctx, req) (memories []MemoryItem, err error)
Upsert(ctx, req) error
SummarizeSession(ctx, sessionID string) (summary string, err error)
```

## 4.4 Model Gateway Service

```go
Generate(ctx, req) (LLMResponse, error)
ListModels(ctx) ([]ModelMeta, error)
```

约束：
- 业务层必须通过网关调用模型
- 支持外置与本地模型统一协议

## 4.5 Agent Orchestrator Service（ReAct）

```go
Run(ctx, req) (AgentResult, error)
GetTrace(ctx, traceID string) (AgentTrace, error)
```

工具白名单：
- `search_knowledge`
- `search_memory`
- `summarize_context`

---

## 5. 主链路时序与状态

1. 前端上传文档，`Document API` 返回 `task_id`。
2. `Document Processing Service` 完成解析/切片/索引，状态流转：`pending -> processing -> indexed | failed`。
3. 前端发起问答，`Chat API` 调用编排层。
4. 编排层按需调用检索、记忆、模型网关、智能体。
5. 返回统一结果：`answer + citations + (optional) trace`。

---

## 6. 版本与兼容策略

- 当前版本：`v1`（MVP）
- 破坏性变更：通过 `/api/v2` 发布
- 非破坏性新增：仅新增字段，不删除既有字段
- 前端兼容建议：忽略未知字段、按 `code` 而非 `message` 判断错误
