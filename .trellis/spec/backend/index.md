# Backend Development Guidelines

> Execution standards for backend development in MemoBase.

---

## Scope

These guidelines define backend coding standards for the agreed stack.

Fixed by project memory (journal records):
- Language: `Go`
- API framework: `Gin` (MVP fixed)
- Storage: `PostgreSQL` + local filesystem
- Vector retrieval: `Qdrant`
- Retrieval mode: hybrid (`BM25` + vector)
- Agent flow: lightweight `ReAct`
- Model access: external LLM APIs + local `Ollama`

Planned by project scope:
- Deployment baseline: `Docker`/`Compose`
- Engineering target: `Kubernetes + Prometheus + Grafana`

Explicitly out of MVP unless scope change is approved:
- `OpenSearch`
- `MinIO`
- `Redis`
- `Viper`
- `Nginx`

Product non-goals in current scope:
- No complex multi-role permission architecture.
- No multi-tenant architecture.
- No recommendation-system-oriented backend module.

Source references for current project agreements:
- `README.md`
- `doc/模块核心技术栈与任务边界.md`
- `doc/模块组织与系统架构说明.md`

---

## Guidelines Index

| Guide | Description | Status |
|-------|-------------|--------|
| [Directory Structure](./directory-structure.md) | Module boundaries and package layout | Defined |
| [Database Guidelines](./database-guidelines.md) | Relational + vector DB conventions | Defined |
| [Error Handling](./error-handling.md) | Error model and HTTP mapping | Defined |
| [Quality Guidelines](./quality-guidelines.md) | Lint/test/review standards | Defined |
| [Logging Guidelines](./logging-guidelines.md) | Structured logging standards | Defined |
| [Type Safety](./type-safety.md) | Type rules and null-safety boundaries | Defined |

---

## Required Reading Order

1. `directory-structure.md`
2. `error-handling.md`
3. `database-guidelines.md`
4. `logging-guidelines.md`
5. `quality-guidelines.md`

---

## Core Rules

1. API contracts are stable and explicit; no hidden field changes.
2. Service layer owns orchestration; handlers do transport only.
3. Repository layer owns all storage access; no cross-layer DB calls.
4. Every request path must be observable (request id, duration, status, errors).
5. New behavior must include tests or a clear reason for temporary test debt.
6. Do not introduce out-of-MVP infrastructure dependencies without a documented RFC.
