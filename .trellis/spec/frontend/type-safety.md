# Type Safety

> TypeScript and runtime validation standards.

---

## TypeScript Baseline

Required compiler posture:
- `strict: true`
- `noImplicitAny: true`
- `strictNullChecks: true`
- `noUncheckedIndexedAccess: true` (recommended)

No new code should require lowering strictness.

---

## Type Organization

- API DTO types: colocated in `features/<feature>/types`.
- Shared cross-feature types: `src/lib/types`.
- Component-local types: same file or nearby `*.types.ts`.
- Avoid giant global type files.

---

## Runtime Validation

Use one runtime schema validation library for external boundary validation (for example `zod`):
- API response parsing for unstable/third-party payloads.
- URL query param parsing.
- Complex form schemas.

Type-only checks are not enough for untrusted runtime input.

Constraint:
- Pick one schema library and keep it consistent project-wide.

---

## Safe Patterns

- Prefer `unknown` over `any` for uncertain input.
- Use discriminated unions for state machines and request status.
- Narrow nullable values before render usage.
- Encode API error shape as explicit type.

---

## Forbidden Patterns

- `any` in business logic paths.
- Double assertion (`as unknown as T`) without boundary justification.
- Ignoring nullable cases with non-null assertion (`!`) in hot paths.
- Silent `JSON.parse` usage without schema validation.
