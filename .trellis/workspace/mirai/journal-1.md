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
