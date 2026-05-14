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


def get_provider_llm(
    api_base_url: str,
    api_key: str,
    model: str,
) -> "ChatOpenAI":
    """Get or create a ChatOpenAI instance for an external provider.

    This is NOT cached — each unique provider config gets its own instance.
    """
    from langchain_openai import ChatOpenAI

    logger.info(
        "Creating ChatOpenAI: model=%s base_url=%s",
        model,
        api_base_url,
    )
    return ChatOpenAI(
        model=model,
        api_key=api_key,
        base_url=api_base_url,
        temperature=0.1,
        max_tokens=2048,
    )


def get_llm_for_request(
    provider_api_base_url: str = "",
    provider_api_key: str = "",
    provider_model: str = "",
    fallback_model: str = "",
):
    """Return the appropriate LLM for a chat request.

    If provider credentials are given, use ChatOpenAI (OpenAI-compatible).
    Otherwise fall back to local Ollama.
    """
    if provider_api_base_url and provider_model:
        logger.info(
            "Using external provider: model=%s base_url=%s",
            provider_model,
            provider_api_base_url,
        )
        return get_provider_llm(
            api_base_url=provider_api_base_url,
            api_key=provider_api_key,
            model=provider_model,
        )
    logger.info("Using default Ollama: model=%s", fallback_model or settings.ollama_chat_model)
    return get_chat_llm(fallback_model)


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
