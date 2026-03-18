# Frontend Development Guidelines

> Execution standards for frontend development in MemoBase.

---

## Scope

These guidelines define frontend coding standards for the agreed stack.

Fixed by project memory (journal records):
- UI framework: `React`
- Language: `TypeScript`
- Build tool: `Vite`

Not yet fixed at team level (choose during scaffold and keep consistent):
- Router library
- HTTP client library
- Server-state caching library
- Runtime schema validation library

Constraint:
- Frontend must stay API-driven and cannot bypass backend service contracts.

Product non-goals in current scope:
- No mobile-first adaptation priority.
- No complex multi-role permission UI matrix.
- No multi-tenant UI architecture.
- No recommendation-system-oriented feature design.

Source references for current project agreements:
- `README.md`
- `doc/模块核心技术栈与任务边界.md`
- `doc/模块组织与系统架构说明.md`

---

## Guidelines Index

| Guide | Description | Status |
|-------|-------------|--------|
| [Directory Structure](./directory-structure.md) | Feature/module layout and boundaries | Defined |
| [Component Guidelines](./component-guidelines.md) | Component architecture and a11y | Defined |
| [Hook Guidelines](./hook-guidelines.md) | Data and reusable logic hooks | Defined |
| [State Management](./state-management.md) | Local/global/server/url state model | Defined |
| [Quality Guidelines](./quality-guidelines.md) | Lint/test/review standards | Defined |
| [Type Safety](./type-safety.md) | Type and runtime validation standards | Defined |

---

## Required Reading Order

1. `directory-structure.md`
2. `component-guidelines.md`
3. `type-safety.md`
4. `hook-guidelines.md`
5. `state-management.md`
6. `quality-guidelines.md`

---

## Core Rules

1. UI components are presentation-first; business orchestration lives in hooks/services.
2. API contracts are consumed via typed client methods, not ad-hoc fetch calls.
3. State ownership must be explicit (local vs global vs server vs URL).
4. Every async UI path has loading, empty, and error states.
5. Accessibility and type safety are release blockers, not optional polish.
