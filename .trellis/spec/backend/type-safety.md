# Type Safety

> Type-safety standards for Go backend in MemoBase.

---

## Core Principles

- Prefer concrete domain structs over `map[string]interface{}` in service/repo boundaries.
- Use typed request/response DTOs in handlers.
- Keep nullable DB columns represented explicitly (`sql.Null*` or pointers) at storage boundary.
- Convert nullable storage types to API-safe shapes before returning JSON.

---

## Error and Optional Handling

- Never ignore returned errors from parse/convert operations.
- Treat empty string and null as different states when business semantics differ.
- Avoid panics from unsafe string/rune slicing or unchecked type assertions.

---

## JSON and Interface Boundaries

- Use stable struct tags for API contracts (`snake_case`).
- Minimize untyped `interface{}` usage; confine it to extensible metadata fields.
- When unmarshalling dynamic JSON, validate required keys before use.

---

## Forbidden Patterns

- Blind type assertions without `ok` check.
- Returning internal storage structs directly when they contain nullable/internal-only fields.
- Byte-based substring operations for user-facing Unicode text.

---

## Review Checklist

- Request/response structs are explicit and validated.
- Optional fields are represented safely across DB -> service -> API.
- No unsafe casts or unchecked dynamic payload reads.
- Unicode text truncation/summary logic is rune-safe.
