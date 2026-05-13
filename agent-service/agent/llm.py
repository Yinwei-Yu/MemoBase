"""LLM initialization for the agent service."""

from __future__ import annotations

import logging
from functools import lru_cache

from langchain_ollama import ChatOllama, OllamaEmbeddings

from config import settings

logger = logging.getLogger("agent-service.llm")


@lru_cache(maxsize=4)
def get_chat_llm(model: str | None = None) -> ChatOllama:
    """Get or create the ChatOllama instance (cached)."""
    model_name = model or settings.ollama_chat_model
    logger.info("Creating ChatOllama: model=%s url=%s", model_name, settings.ollama_url)
    return ChatOllama(
        model=model_name,
        base_url=settings.ollama_url,
        temperature=0.1,
        num_predict=2048,
    )


@lru_cache(maxsize=1)
def get_embeddings() -> OllamaEmbeddings:
    """Get or create the OllamaEmbeddings instance (cached)."""
    logger.info(
        "Creating OllamaEmbeddings: model=%s url=%s",
        settings.ollama_embed_model,
        settings.ollama_url,
    )
    return OllamaEmbeddings(
        model=settings.ollama_embed_model,
        base_url=settings.ollama_url,
    )


def clear_llm_cache() -> None:
    """Clear cached LLM/embedding instances (useful for testing)."""
    get_chat_llm.cache_clear()
    get_embeddings.cache_clear()
