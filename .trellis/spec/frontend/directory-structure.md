# Directory Structure

> Frontend project structure for MemoBase.

---

## Overview

Frontend is organized by feature-first modules, with shared layers for reusable UI and infrastructure.

---

## Standard Layout

```text
frontend/
├── src/
│   ├── app/                    # app bootstrap, router, providers
│   ├── pages/                  # route-level pages (thin composition)
│   ├── features/               # feature modules (kb, doc, chat, auth, ops)
│   │   └── <feature>/
│   │       ├── components/
│   │       ├── hooks/
│   │       ├── services/
│   │       ├── types/
│   │       └── index.ts
│   ├── components/             # shared presentational components
│   ├── hooks/                  # shared generic hooks
│   ├── lib/                    # api client, utils, constants
│   ├── stores/                 # global state stores
│   ├── styles/                 # global styles and tokens
│   └── test/                   # shared test utilities
├── public/
└── vite.config.ts
```

---

## Module Boundaries

- `pages`: compose feature components; avoid direct API logic.
- `features/*/services`: typed API calls and DTO mapping.
- `features/*/hooks`: behavior orchestration and side effects.
- `components`: no feature-specific business dependency.
- `stores`: global app-level state only.

Forbidden boundary violations:
- page/component making raw HTTP calls directly
- feature module importing from unrelated feature internals
- shared components depending on feature stores

---

## Naming Conventions

- Component files: `PascalCase.tsx`
- Hook files: `useXxx.ts`
- Service files: `<feature>Api.ts`
- Type files: `<feature>.types.ts`
- Utility files: `camelCase.ts`

Use explicit barrel files (`index.ts`) per feature for public exports.

---

## Current Project Anchors

Use these docs as source of truth until app code is fully scaffolded:
- `README.md`
- `doc/模块组织与系统架构说明.md`
