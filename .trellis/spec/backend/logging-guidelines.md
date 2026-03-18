# Logging Guidelines

> Structured logging standards for backend observability.

---

## Logging Stack

- Logger: Go `slog` with JSON handler.
- Correlation: request id middleware required.
- Metrics: Prometheus for latency/error counters (logs are not a metrics substitute).

---

## Required Log Fields

Every request-scoped log should include:
- `timestamp`
- `level`
- `service`
- `env`
- `request_id`
- `operation`
- `duration_ms` (when operation finishes)

Add when available:
- `user_id`
- `kb_id`
- `doc_id`
- `session_id`
- `error_code`

---

## Level Usage

- `DEBUG`: local diagnosis only; disabled by default in production.
- `INFO`: lifecycle milestones and key state transitions.
- `WARN`: recoverable anomalies and degraded behavior.
- `ERROR`: request or task failure requiring attention.

No business-as-usual logs at `ERROR`.

---

## What to Log

- API entry/exit summary (method, path, status, duration).
- Indexing task start/finish/fail with identifiers.
- Retrieval path decisions (bm25/vector/fused) and hit counts.
- Model gateway provider choice, timeout, retries, fallback.
- Agent step transitions and tool execution outcome.

---

## What NOT to Log

- API keys, tokens, secrets.
- Raw user documents or full prompts/responses in production logs.
- PII fields unless explicitly masked and approved.
- High-cardinality debug payloads on hot paths.

---

## Common Mistakes (Forbidden)

- Unstructured string logs with parse-hostile text.
- Missing request id on error logs.
- Logging same error in every stack layer (log once at boundary).
