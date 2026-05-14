"""Configuration for the agent service."""

from pydantic_settings import BaseSettings


class Settings(BaseSettings):
    """Agent service settings, loaded from environment variables."""

    grpc_port: int = 50051

    # Ollama
    ollama_url: str = "http://host.docker.internal:11434"
    ollama_chat_model: str = ""
    ollama_embed_model: str = "nomic-embed-text"

    # Embedding Provider (ollama or openai_compatible)
    embed_provider: str = "ollama"
    embed_api_base_url: str = ""
    embed_api_key: str = ""
    embed_model: str = ""

    # Qdrant
    qdrant_url: str = "http://qdrant:6333"
    qdrant_collection_prefix: str = "kb_chunks"

    # PostgreSQL
    database_url: str = "postgres://memo:memo@postgres:5432/memo"

    # Retrieval
    bm25_weight: float = 0.5
    vector_weight: float = 0.5
    top_k: int = 6
    retrieve_candidate_limit: int = 5000
    max_retrieval_attempts: int = 2

    # MCP
    mcp_qdrant_enabled: bool = True
    mcp_qdrant_url: str = "http://mcp-qdrant:8000/sse"

    model_config = {"env_prefix": "", "case_sensitive": False}


settings = Settings()
