# Build MVP Product End-to-End

## Goal
Implement a runnable MemoBase MVP with complete frontend pages and backend business flow, including local Ollama protocol integration, and provide end-to-end deployment guide in README.

## Requirements
- Backend (Go + Gin):
  - Implement API modules: Auth, Knowledge Base, Document, Task, Chat, Session, Health.
  - Use PostgreSQL for relational data persistence.
  - Use local filesystem for uploaded document files.
  - Use Qdrant for vector point upsert/search.
  - Implement hybrid retrieval (keyword + vector fusion).
  - Integrate local Ollama protocol for embeddings and chat completion.
  - Provide standardized response envelope and error mapping.
- Frontend (React + TS + Vite):
  - Implement complete pages: login, knowledge base list/create/delete, document upload/list/reindex/delete, chat with citations and trace display, session list/messages, ops status page.
  - Implement API client, auth state, async loading/error/empty states.
- Deployment:
  - Provide docker-compose based startup for postgres/qdrant/ollama/backend/frontend.
  - Provide README with full setup and run instructions.

## Acceptance Criteria
- [ ] `docker compose up` starts all required services.
- [ ] User can login, create KB, upload doc, wait for indexing task, ask question, get answer and citations.
- [ ] Backend can call local Ollama protocol endpoints.
- [ ] README contains full deployment and troubleshooting guide.

## Technical Notes
- Respect project stack boundaries and non-goals.
- Keep API v1 under `/api/v1`.
- Use snake_case API fields and standardized error codes.
