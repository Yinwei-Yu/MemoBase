# Hook Guidelines

> Standards for reusable hooks and data hooks.

---

## Custom Hook Patterns

Use hooks for reusable stateful logic and side effects.

Rules:
- Hook names must start with `use`.
- One primary responsibility per hook.
- Return stable, typed object API.
- Keep side effects explicit and cleanup-safe.

---

## Data Fetching Standard

Server state must be managed through a single query abstraction + unified API client.

- API calls live in `services/*Api.ts`.
- Query hooks live in `hooks/useXxxQuery.ts`.
- Mutation hooks live in `hooks/useXxxMutation.ts`.
- Query keys must be centralized and stable.

Library choice note:
- `TanStack Query` is a recommended baseline, not a hard requirement.
- If another library is selected, keep the same hook contract shape and cache invalidation discipline.

Minimum behavior for every async hook:
- loading state
- error state
- empty state handling path
- retry policy (explicit)

---

## Hook Return Contract

Prefer object return values for readability:

```ts
return {
  data,
  isLoading,
  error,
  refetch,
};
```

Avoid tuple returns unless the API is naturally pair-like and obvious.

---

## Dependency and Effect Rules

- Respect exhaustive dependency checks.
- Do not suppress dependency warnings without code comment and reason.
- Avoid `useEffect` for pure derivations; use memoized selectors.

---

## Common Mistakes (Forbidden)

- Hook containing direct DOM manipulation that belongs in component.
- Copy-pasted fetch logic across multiple hooks.
- Hidden global writes inside generic hooks.
- Network requests triggered from render path.
