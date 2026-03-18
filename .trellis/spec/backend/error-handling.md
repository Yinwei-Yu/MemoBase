# Error Handling

> Error model, propagation, and API mapping for MemoBase backend.

---

## Error Model

All API errors use a stable envelope:

```json
{
  "error": {
    "code": "KB_NOT_FOUND",
    "message": "knowledge base not found",
    "details": {
      "kb_id": "..."
    },
    "request_id": "..."
  }
}
```

Rules:
- `code` is machine-readable and stable.
- `message` is user-safe and concise.
- `details` is optional and sanitized.
- `request_id` must always be included.

---

## Propagation Pattern

- Wrap internal errors with `%w`.
- Convert infrastructure errors to domain errors at repository boundary.
- Map domain errors to HTTP status only in handler layer.
- Never `panic` for expected business errors.

Recommended categories:
- Validation (`INVALID_ARGUMENT`, `BAD_REQUEST`)
- Auth/Authz (`UNAUTHORIZED`, `FORBIDDEN`)
- Not found (`*_NOT_FOUND`)
- Conflict (`CONFLICT`, `ALREADY_EXISTS`)
- Dependency (`UPSTREAM_TIMEOUT`, `QDRANT_UNAVAILABLE`, `MODEL_UNAVAILABLE`)
- Internal (`INTERNAL`)

---

## HTTP Mapping Baseline

- `400`: invalid input
- `401`: not authenticated
- `403`: authenticated but forbidden
- `404`: resource missing
- `409`: conflict or duplicate operation
- `422`: semantically invalid payload
- `429`: rate limited
- `500`: internal error
- `502/503/504`: dependency unavailable/timeout

---

## Logging and Error Coupling

- Log full internal error once at the failure boundary.
- API response must not include stack traces or secrets.
- Include `error_code`, `request_id`, and relevant ids (`kb_id`, `doc_id`, `session_id`).

---

## Common Mistakes (Forbidden)

- Returning raw `err.Error()` to clients.
- Swallowing errors and returning success with partial failure.
- Inconsistent error codes for same failure class.
- Mixing transport status logic into service/repository layers.
