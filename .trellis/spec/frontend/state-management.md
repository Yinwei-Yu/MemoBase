# State Management

> State ownership model for MemoBase frontend.

---

## State Categories

1. Local UI state (`useState`/`useReducer`)
- Component-local concerns: modal open, tab selection, form draft.

2. Server state (query cache layer)
- Data fetched from backend: knowledge bases, docs, chat history, task status.
- Includes caching, background refetch, invalidation.

3. Global app state (single lightweight store solution)
- Cross-page session context: auth identity, selected workspace, UI preferences.

4. URL state (React Router search params)
- Filter/sort/page and deep-linkable view state.

---

## Promotion Rules

Promote local state to global only when at least one condition is true:
- Needed by 2+ distant route branches.
- Must persist across navigation.
- Represents app/session identity.

Do not promote temporary form or modal state to global.

---

## Server State Rules

- Never duplicate query data in global store by default.
- Mutations must invalidate or update related query caches explicitly.
- Keep query keys deterministic and typed.
- Use optimistic updates only with rollback path.

Tooling note:
- `TanStack Query` + `Zustand` is a practical default pair.
- If the team picks alternatives, document the choice in this file and keep one stack only.

---

## Common Mistakes (Forbidden)

- Storing server lists in local state and manually syncing.
- Global store used as a dump for all states.
- Route-filter state not reflected in URL.
- Mutations that change server data without cache invalidation.
