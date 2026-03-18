# Design Frontend-Backend API Contract

## Goal
Define a production-style API contract standard for MemoBase MVP, so frontend and backend can implement against a stable, testable, and versioned interface.

## Requirements
- Define unified API conventions: versioning, headers, auth, response envelope, error model.
- Define core endpoint contracts for MVP modules:
  - Auth
  - Knowledge Base
  - Document Upload/Index Task
  - Chat/Agent
  - Session/Message
  - Health/Readiness/Metrics
- Define cross-layer contract rules for async tasks, pagination, filtering, idempotency, and traceability.
- Align API design with confirmed project boundaries from memory:
  - Go + Gin
  - PostgreSQL + local filesystem
  - Qdrant + BM25 (jieba tokenization)
  - external LLM + Ollama
  - no OpenSearch/MinIO/Redis/Viper/Nginx in MVP
- Provide validation and error mapping guidance with concrete examples.

## Acceptance Criteria
- [ ] A single API spec document exists and can be used for frontend-backend parallel development.
- [ ] The document defines stable request/response schemas for all MVP endpoints.
- [ ] The document defines standard error codes and HTTP mapping.
- [ ] The document includes async task status flow and polling contracts.
- [ ] The document includes Good/Base/Bad examples for at least one representative endpoint.

## Technical Notes
- Use REST-style HTTP API under `/api/v1`.
- Include explicit field constraints and enum values.
- Keep schemas JSON-focused and implementation language-agnostic.
