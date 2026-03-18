# Quality Guidelines

> Backend quality gate for changes in MemoBase.

---

## Baseline Tooling

- Formatting: `gofmt` + `goimports`
- Lint: `golangci-lint`
- Tests: `go test ./...`
- Security check (recommended): `gosec ./...`

All CI-critical checks must pass before merge.

---

## Forbidden Patterns

- Business logic in handlers.
- Global mutable state without synchronization.
- Context-free DB/network operations.
- Silent error ignore (`_ = err` without explicit reason).
- Hardcoded secrets or environment-dependent constants in code.
- Cross-module direct calls that bypass service/repository contracts.

---

## Required Patterns

- Constructor-based dependency injection.
- Small interfaces at capability boundaries.
- Explicit timeout/retry policy for external dependencies.
- Idempotent design for indexing and async job endpoints.
- Stable API response envelopes and version-safe contracts.
- Dependency boundary checks: MVP cannot introduce `OpenSearch`, `MinIO`, `Redis`, `Viper`, or `Nginx` without team-approved RFC.

---

## Testing Requirements

- Unit tests for service logic and error mapping.
- Repository tests for query correctness (with test DB).
- Integration tests for MVP critical path:
  - document upload -> parse/chunk/index
  - ask question -> hybrid retrieval -> model answer envelope
- Regression test for every production bug fix.

---

## Code Review Checklist

- Layer boundary respected (`api/app/repo/infra`).
- Error codes and statuses are consistent.
- Logs include required context fields.
- Migrations and rollback path are safe.
- Contract changes reflected in API docs.
- Tests cover normal + failure paths.
