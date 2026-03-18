# Database Guidelines

> Data access standards for relational DB and Qdrant in MemoBase.

---

## Storage Roles

- Relational DB (default: `PostgreSQL`): users, knowledge bases, docs, chunks metadata, sessions, messages, memories, task logs.
- `Qdrant`: chunk vectors + retrieval metadata.
- Local filesystem storage: original files and intermediate processing artifacts.

MVP boundary:
- No `OpenSearch` as BM25 backend.
- No `MinIO` object storage dependency.

No layer bypass is allowed. All persistence goes through repository interfaces.

---

## Query Patterns

- Every DB operation must accept `context.Context`.
- Use repository methods; do not build SQL in handlers.
- Use explicit column lists for read queries on hot paths.
- Batch writes for chunk ingest; avoid one-row-per-request loops.
- Use pagination (`limit`, `cursor` or `id >`) for list endpoints.
- Use idempotency keys for retriable ingest/index operations.

For hybrid retrieval:
1. Run BM25 and vector search independently.
2. Normalize scores and fuse in retrieval service.
3. Deduplicate by chunk id.
4. Return citation-ready metadata (doc_id, chunk_id, offsets, score source).
5. Chinese text tokenization for BM25 uses `jieba` baseline.

---

## Transactions

Use transaction blocks when operations must be atomic across relational tables:
- create knowledge base + bootstrap records
- document delete + chunk metadata delete + task status update
- session/message writes that must remain consistent

Rules:
- Keep transactions short.
- No remote calls (LLM, Qdrant HTTP) inside SQL transaction.
- Rollback on first error and wrap root cause.

---

## Migrations

Migration files are SQL-first and live in `migrations/`.
Tooling may use `golang-migrate` or equivalent, but the team must keep one tool only across environments.

Rules:
- One migration per logical schema change.
- Include both `up` and `down`.
- Never edit a migration already applied in shared env; create a new corrective migration.
- Index additions must be reviewed with expected query usage.

---

## Naming Conventions

- Tables: plural snake_case (`knowledge_bases`, `task_logs`).
- Columns: snake_case.
- Primary keys: `id` (UUID preferred for external-facing entities).
- Foreign keys: `<entity>_id`.
- Timestamps: `created_at`, `updated_at`, optional `deleted_at`.
- Index names: `idx_<table>_<column_list>`.

Qdrant conventions:
- Collection: `<kb_id>_chunks` or centralized `kb_chunks` with `kb_id` payload.
- Required payload fields: `kb_id`, `doc_id`, `chunk_id`, `source`, `created_at`.

---

## Common Mistakes (Forbidden)

- Building SQL in handler/service directly.
- Performing N+1 queries for list/detail aggregation.
- Returning raw DB errors to API response.
- Writing to relational DB and Qdrant without retry/reconciliation strategy.
- Storing secrets or full user PII in vector payload.
- Adding Redis/MinIO/OpenSearch just to simplify short-term implementation.
