"""Tool definitions for the agent graph.

Tools are plain async functions that the agent can invoke.
For qwen2.5:3b which may not support native tool calling,
the decide node uses prompt-based routing instead of binding tools to the LLM.
"""

from __future__ import annotations

import logging
from typing import Any

import psycopg2

from config import settings
from retriever.hybrid import HybridRetriever

logger = logging.getLogger("agent-service.tools")


# Module-level retriever singleton
_retriever: HybridRetriever | None = None


def get_retriever() -> HybridRetriever:
    """Get or create the global HybridRetriever instance."""
    global _retriever
    if _retriever is None:
        _retriever = HybridRetriever()
    return _retriever


async def search_knowledge_base(
    query: str,
    kb_id: str,
    top_k: int = 6,
) -> tuple[list[dict[str, Any]], bool]:
    """Search the knowledge base using hybrid retrieval.

    Args:
        query: The search query.
        kb_id: Knowledge base ID.
        top_k: Number of results to return.

    Returns:
        Tuple of (results, degraded).
    """
    logger.info(
        "search_knowledge_base: kb_id=%s top_k=%d query=%.80s", kb_id, top_k, query
    )
    retriever = get_retriever()
    return retriever.search(query=query, kb_id=kb_id, top_k=top_k)


async def search_session_memory(
    session_id: str,
    limit: int = 5,
) -> list[dict[str, str]]:
    """Fetch session memories from PostgreSQL.

    Args:
        session_id: Session identifier.
        limit: Maximum number of memories to return.

    Returns:
        List of memory dicts with 'type' and 'summary' keys.
    """
    if not session_id:
        return []

    try:
        conn = psycopg2.connect(settings.database_url)
        cur = conn.cursor()
        cur.execute(
            """
            SELECT type, summary
            FROM session_memories
            WHERE session_id = %s
            ORDER BY created_at DESC
            LIMIT %s
            """,
            (session_id, limit),
        )
        rows = cur.fetchall()
        cur.close()
        conn.close()
        return [{"type": r[0], "summary": r[1]} for r in rows]
    except Exception as e:
        logger.warning("Failed to fetch session memories: %s", e)
        return []


async def fetch_chat_history(
    session_id: str,
    limit: int = 20,
) -> list[dict[str, str]]:
    """Fetch recent chat messages from PostgreSQL.

    Args:
        session_id: Session identifier.
        limit: Maximum number of messages.

    Returns:
        List of message dicts with 'role', 'content', 'created_at'.
    """
    if not session_id:
        return []

    try:
        conn = psycopg2.connect(settings.database_url)
        cur = conn.cursor()
        cur.execute(
            """
            SELECT role, content, created_at
            FROM messages
            WHERE session_id = %s
            ORDER BY created_at DESC
            LIMIT %s
            """,
            (session_id, limit),
        )
        rows = cur.fetchall()
        cur.close()
        conn.close()
        # Reverse to chronological order
        messages = [
            {
                "role": r[0],
                "content": r[1],
                "created_at": r[2].isoformat()
                if hasattr(r[2], "isoformat")
                else str(r[2]),
            }
            for r in reversed(rows)
        ]
        return messages
    except Exception as e:
        logger.warning("Failed to fetch chat history: %s", e)
        return []
