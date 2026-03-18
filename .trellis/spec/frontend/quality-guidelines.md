# Quality Guidelines

> Frontend quality gate for MemoBase.

---

## Baseline Tooling

- Lint: `eslint` with `typescript-eslint` and React hooks rules
- Format: `prettier`
- Unit/Component tests: `vitest` + `@testing-library/react` (recommended baseline)
- E2E (critical path): `playwright` (recommended)

All CI-critical checks must pass before merge.

---

## Forbidden Patterns

- Unhandled promise rejections in UI logic.
- Direct DOM querying/mutation outside React refs/effects.
- API calls inside component render body.
- Hardcoded backend URLs in components.
- Accessibility regressions (missing labels, unreachable controls).

---

## Required Patterns

- Every API interaction shows loading and error feedback.
- Reusable API access through unified client/service layer.
- Route-level error boundary for uncaught view errors.
- Explicit empty state for list and search pages.
- Keep component files focused; extract hooks/services when complexity grows.

---

## Testing Requirements

- Unit tests for pure utilities and state logic.
- Component tests for critical interactive components (chat input, citations panel, uploader).
- Integration tests for key flows:
  - login/logout
  - knowledge base create/delete
  - upload -> status update -> retrievable state
  - ask question -> answer + citations rendering
- Add regression tests for every bug fixed in production/demo.

---

## Code Review Checklist

- State ownership is correct (local/global/server/url).
- API contracts are typed and validated where needed.
- Error/loading/empty states are complete.
- Accessibility checks pass for new UI.
- No cross-feature coupling through private internals.
- Tests cover success and failure paths.
