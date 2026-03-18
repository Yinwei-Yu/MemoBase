# Directory Structure

> Backend package and module organization for MemoBase.

---

## Overview

Backend follows layered architecture:
`API -> Application Service -> Core Capability -> Repository/Infra`

The purpose is to keep transport, business orchestration, and storage concerns separated.

---

## Standard Layout

```text
backend/
├── cmd/
│   └── server/                # app entrypoint
├── internal/
│   ├── api/
│   │   ├── handler/           # gin handlers only
│   │   ├── middleware/        # auth, request-id, recovery, logging
│   │   └── router/            # route registration
│   ├── app/                   # application services (use-case orchestration)
│   ├── domain/                # domain entities and business interfaces
│   ├── repo/                  # relational and qdrant repositories
│   ├── retrieval/             # BM25 + vector fusion service
│   ├── memory/                # short-term/long-term memory logic
│   ├── agent/                 # react loop + tool registry
│   ├── modelgateway/          # external/local model adapters
│   └── infra/                 # config, db clients, qdrant client, file storage
├── migrations/                # SQL migration files
├── configs/                   # env templates and config defaults
├── test/                      # integration tests and test fixtures
└── go.mod
```

---

## Module Boundaries

- `handler`: parse/validate request, call one app service, map response.
- `app`: orchestration only; no direct SQL or HTTP framework dependency.
- `repo`: persistence details; return domain types and repository errors.
- `retrieval/memory/agent/modelgateway`: reusable capability modules with stable interfaces.
- `infra`: library wrappers and shared clients; no business decisions.

Forbidden boundary violations:
- handler importing repo directly
- repo importing handler or gin context
- app depending on concrete vendor SDK types when interface abstraction is possible

---

## Naming Conventions

- Package names: lowercase, short, singular (`handler`, `repo`, `memory`).
- File names: `snake_case.go`.
- Interface names: behavior-first (`MemoryStore`, `Retriever`, `ModelClient`).
- Constructor functions: `NewXxx`.
- HTTP handlers: `<resource>_<action>.go` (for example `knowledgebase_create.go`).

---

## Current Project Anchors

Use these docs as architecture source of truth until code modules are complete:
- `doc/模块组织与系统架构说明.md`
- `doc/模块核心技术栈与任务边界.md`
