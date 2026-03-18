# Journal - mirai (Part 1)

> AI development session journal
> Started: 2026-03-12

---

## 2026-03-12 - Memory Synchronization

- Synced long-term project memory into `AGENTS.md`.
- Recorded stable team context:
  - members `L`, `G`, `W`, `Z`
  - `L` acts as team leader and primary delivery owner
  - uneven participation is expected
- Recorded stable project direction:
  - practical knowledge-base intelligent agent platform
  - backend `Go`
  - frontend `React`
  - vector database `Qdrant`
  - support for external and local models
- Recorded required technical scope:
  - hybrid retrieval with vector retrieval and `BM25`
  - memory management referencing `OpenClaw`
  - `ReAct`-style agent orchestration
  - `Kubernetes` deployment
  - `Prometheus` and `Grafana` observability
- Recorded explicit non-goals for early scope:
  - no complex multi-role permissions
  - no multi-tenant architecture
  - no mobile adaptation priority
  - no recommendation system priority

## 2026-03-12 - School Materials Location

- Confirmed school-related materials are stored under `.school-doc/`.
- Current files in `.school-doc/` include:
  - `homework.md`
  - `知识库智能体开发任务清单.md`
  - `知识库智能体项目参考文档.md`
  - `知识库智能体项目立项书.md`
  - `计算机综合项目实践-课程介绍.txt`
  - `计算机综合项目实践-项目选题.txt`
  - `课题名称.pdf`
- Future course-related work should prefer `.school-doc/` as the reference source.


## Session 1: Project bootstrap, memory sync, and README setup

**Date**: 2026-03-12
**Task**: Project bootstrap, memory sync, and README setup

### Summary

Initialized project-level memory in AGENTS.md, established .school-doc as the source of truth for school materials, and wrote the repository README for 知忆 MemoBase.

### Main Changes



### Git Commits

| Hash | Message |
|------|---------|
| `47668fb` | (see git log) |

### Testing

- [OK] (Add test results)

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 2: Finalize MVP tech stack selection

**Date**: 2026-03-18
**Task**: Finalize MVP tech stack selection

### Summary

Confirmed the course-project MVP tech stack and recorded key architectural decisions: frontend uses React + TypeScript + Vite; backend uses Go + Gin; storage uses PostgreSQL plus local filesystem; retrieval uses Qdrant only with hybrid retrieval and jieba-based Chinese tokenization; model access uses external LLM APIs plus Ollama; agent orchestration follows a lightweight ReAct design. Explicitly decided not to introduce OpenSearch, MinIO, Redis, Viper, or Nginx for the current MVP unless future scope expands.

### Main Changes



### Git Commits

| Hash | Message |
|------|---------|
| `55dee01` | (see git log) |

### Testing

- [OK] (Add test results)

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 3: Finalize and enforce project stack boundaries/specs

**Date**: 2026-03-18
**Task**: Finalize and enforce project stack boundaries/specs

### Summary

Reworked frontend/backend spec docs from templates into executable standards, then revised them against workspace memory: fixed MVP stack boundaries, explicit non-goals, and dependency constraints.

### Main Changes



### Git Commits

| Hash | Message |
|------|---------|
| `8edb33b` | (see git log) |

### Testing

- [OK] (Add test results)

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 4: Design frontend-backend API contract for MVP

**Date**: 2026-03-18
**Task**: Design frontend-backend API contract for MVP

### Summary

Defined and refined the MVP frontend-backend API contract document, including unified response/error model, endpoint contracts, async task protocol, and stack-boundary constraints for implementation alignment.

### Main Changes



### Git Commits

| Hash | Message |
|------|---------|
| `e93075e` | (see git log) |

### Testing

- [OK] (Add test results)

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 5: MVP重构进度与待办（WIP）

**Date**: 2026-03-18
**Task**: MVP重构进度与待办（WIP）

### Summary

记录MVP重构阶段进度：已完成全栈MVP主链路与三项规范检查修复，补充当前待办（服务层重构、hooks拆分、端到端测试、文档解析能力校准与可观测性强化）。

### Main Changes

| 模块 | 当前进度 |
|------|----------|
| 后端MVP | 已完成 Go+Gin API 主链路（auth/kb/doc/task/chat/session/health），接入 PostgreSQL、Qdrant、Ollama，支持文档分块与索引 |
| 前端MVP | 已完成 React+TS+Vite 页面与核心逻辑（登录、知识库、文档、聊天、会话、运维状态） |
| 规范检查 | 已执行 check-backend / check-frontend / check-cross-layer，并修复日志字段、错误泄露、参数校验、无障碍与错误边界等问题 |
| 构建验证 | `go build ./...`、`npm run build`、`docker compose config` 已通过 |

**当前状态**:
- 当前工作区包含未提交改动，处于重构迭代中（commit 锚点为 `e93075e`，后续提交待补充）。

**待办事项（按优先级）**:
1. 后端分层重构：将 handlers 中编排逻辑进一步下沉到 service/usecase 层，降低路由层复杂度。
2. 前端结构重构：将页面内请求逻辑拆分为 `hooks/useXxxQuery.ts` 与 `hooks/useXxxMutation.ts`，统一状态与错误处理。
3. 关键链路测试：补齐后端集成测试（上传->索引->问答）与前端核心流程测试（登录/KB CRUD/上传/聊天引用渲染）。
4. 文档解析能力对齐：补充真实 PDF/DOCX 解析，或在 README/API 文档中明确当前能力边界。
5. 可观测性增强：将 metrics 从占位实现升级为 Prometheus 指标采集与导出。

**风险提示**:
- 若不尽快补齐测试与解析能力声明，后续联调阶段容易出现“接口可用但结果质量不稳定”的问题。


### Git Commits

| Hash | Message |
|------|---------|
| `e93075e` | (see git log) |

### Testing

- [OK] (Add test results)

### Status

[OK] **Completed**

### Next Steps

- None - task complete
