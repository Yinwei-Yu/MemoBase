"""Hybrid retriever: BM25 + Qdrant vector search + RRF fusion.

Ports the Go retrieval logic (BM25, Qdrant search, RRF) to Python.
"""

from __future__ import annotations

import logging
import re
from typing import Any

import psycopg2
from qdrant_client import QdrantClient
from rank_bm25 import BM25Okapi

from config import settings
from text_processor.tokenizer import tokenize

logger = logging.getLogger("agent-service.retriever")


# ── RRF (Reciprocal Rank Fusion) ─────────────────────────────────────────────


def reciprocal_rank_fusion(
    bm25_scores: dict[str, float],
    vector_scores: dict[str, float],
    k: float = 60.0,
) -> dict[str, float]:
    """Combine BM25 and vector scores using RRF.

    Formula: score(d) = Σ 1/(k + rank_i(d))
    """
    bm25_ranks = _scores_to_ranks(bm25_scores)
    vector_ranks = _scores_to_ranks(vector_scores)

    all_ids: set[str] = set(bm25_ranks) | set(vector_ranks)
    results: dict[str, float] = {}
    for chunk_id in all_ids:
        score = 0.0
        if chunk_id in bm25_ranks:
            score += 1.0 / (k + bm25_ranks[chunk_id])
        if chunk_id in vector_ranks:
            score += 1.0 / (k + vector_ranks[chunk_id])
        if score > 0:
            results[chunk_id] = score
    return results


def _scores_to_ranks(scores: dict[str, float]) -> dict[str, float]:
    """Convert scores to 1-based ranks (ties get same rank, descending)."""
    if not scores:
        return {}
    sorted_items = sorted(scores.items(), key=lambda x: x[1], reverse=True)
    ranks: dict[str, float] = {}
    for i, (chunk_id, score) in enumerate(sorted_items):
        rank = float(i + 1)
        if i > 0 and score == sorted_items[i - 1][1]:
            rank = ranks[sorted_items[i - 1][0]]
        ranks[chunk_id] = rank
    return ranks


# ── Hybrid Retriever ─────────────────────────────────────────────────────────


class HybridRetriever:
    """Combines BM25 keyword search with Qdrant vector search via RRF."""

    def __init__(
        self,
        db_url: str | None = None,
        qdrant_url: str | None = None,
        qdrant_collection_prefix: str | None = None,
        bm25_weight: float | None = None,
        vector_weight: float | None = None,
        rrf_k: float = 60.0,
        top_k: int | None = None,
    ):
        self.db_url = db_url or settings.database_url
        self.qdrant_url = qdrant_url or settings.qdrant_url
        self.collection_prefix = (
            qdrant_collection_prefix or settings.qdrant_collection_prefix
        )
        self.bm25_weight = (
            bm25_weight if bm25_weight is not None else settings.bm25_weight
        )
        self.vector_weight = (
            vector_weight if vector_weight is not None else settings.vector_weight
        )
        self.rrf_k = rrf_k
        self.default_top_k = top_k or settings.top_k

        self._qdrant: QdrantClient | None = None
        self._bm25_cache: dict[str, tuple[list[str], BM25Okapi]] = {}

    @property
    def qdrant(self) -> QdrantClient:
        if self._qdrant is None:
            self._qdrant = QdrantClient(url=self.qdrant_url)
        return self._qdrant

    def _collection_name(self, kb_id: str) -> str:
        """Build Qdrant collection name: {prefix}__{sanitized_kb_id}."""
        prefix = (
            re.sub(r"[^a-zA-Z0-9_-]+", "_", self.collection_prefix).strip("_")
            or "kb_chunks"
        )
        kb_part = re.sub(r"[^a-zA-Z0-9_-]+", "_", kb_id).strip("_") or "default"
        return f"{prefix}__{kb_part}"

    def _fetch_chunks(self, kb_id: str) -> list[dict[str, Any]]:
        """Fetch all chunks for a KB from PostgreSQL."""
        try:
            conn = psycopg2.connect(self.db_url)
            cur = conn.cursor()
            cur.execute(
                """
                SELECT id, doc_id, kb_id, chunk_index, content, created_at
                FROM document_chunks
                WHERE kb_id = %s
                ORDER BY created_at DESC
                LIMIT 5000
                """,
                (kb_id,),
            )
            rows = cur.fetchall()
            cur.close()
            conn.close()
            return [
                {
                    "id": r[0],
                    "doc_id": r[1],
                    "kb_id": r[2],
                    "chunk_index": r[3],
                    "content": r[4],
                    "created_at": r[5].isoformat()
                    if hasattr(r[5], "isoformat")
                    else str(r[5]),
                }
                for r in rows
            ]
        except Exception as e:
            logger.warning("Failed to fetch chunks from DB: %s", e)
            return []

    def _get_or_build_bm25(self, kb_id: str) -> tuple[list[str], BM25Okapi] | None:
        """Get cached BM25 or build a new one from DB chunks."""
        if kb_id in self._bm25_cache:
            return self._bm25_cache[kb_id]

        chunks = self._fetch_chunks(kb_id)
        if not chunks:
            return None

        corpus = [c["content"] for c in chunks]
        tokenized = [tokenize(doc) for doc in corpus]
        bm25 = BM25Okapi(tokenized)
        chunk_ids = [c["id"] for c in chunks]

        self._bm25_cache[kb_id] = (chunk_ids, bm25)
        return (chunk_ids, bm25)

    def _search_bm25(self, query: str, kb_id: str, top_k: int) -> dict[str, float]:
        """BM25 keyword search."""
        bm25_data = self._get_or_build_bm25(kb_id)
        if bm25_data is None:
            return {}

        chunk_ids, bm25 = bm25_data
        tokenized_query = tokenize(query)
        if not tokenized_query:
            return {}

        scores = bm25.get_scores(tokenized_query)
        results: dict[str, float] = {}
        for idx, score in enumerate(scores):
            if score > 0:
                results[chunk_ids[idx]] = float(score)
        return results

    def _search_vector(self, query: str, kb_id: str, top_k: int) -> dict[str, float]:
        """Vector search via Qdrant with embedding from Ollama."""
        try:
            from agent.llm import get_embeddings

            embeddings = get_embeddings()
            query_vector = embeddings.embed_query(query)

            collection = self._collection_name(kb_id)
            response = self.qdrant.query_points(
                collection_name=collection,
                query=query_vector,
                limit=top_k * 3,
                with_payload=True,
            )
            scores: dict[str, float] = {}
            for hit in response.points:
                chunk_id = hit.payload.get("chunk_id", "")
                if chunk_id:
                    scores[chunk_id] = float(hit.score)
            return scores
        except Exception as e:
            logger.warning("Vector search failed: %s", e)
            return {}

    def search(
        self,
        query: str,
        kb_id: str,
        top_k: int | None = None,
    ) -> tuple[list[dict[str, Any]], bool]:
        """Execute hybrid search and return ranked results.

        Returns:
            Tuple of (results_list, degraded_flag).
        """
        top_k = top_k or self.default_top_k
        degraded = False

        # BM25 search
        bm25_scores = self._search_bm25(query, kb_id, top_k)

        # Vector search
        vector_scores = self._search_vector(query, kb_id, top_k)
        if not vector_scores:
            degraded = True

        # RRF fusion
        if bm25_scores and vector_scores:
            fused = reciprocal_rank_fusion(bm25_scores, vector_scores, self.rrf_k)
        elif bm25_scores:
            fused = bm25_scores
        elif vector_scores:
            fused = vector_scores
        else:
            return [], degraded

        # Build results with metadata
        chunks = self._fetch_chunks(kb_id)
        chunk_map = {c["id"]: c for c in chunks}

        results: list[dict[str, Any]] = []
        for chunk_id, score in sorted(fused.items(), key=lambda x: x[1], reverse=True):
            if len(results) >= top_k:
                break
            chunk = chunk_map.get(chunk_id, {})
            has_bm25 = chunk_id in bm25_scores
            has_vec = chunk_id in vector_scores
            if has_bm25 and has_vec:
                source = "fused"
            elif has_bm25:
                source = "bm25"
            else:
                source = "vector"

            results.append(
                {
                    "chunk_id": chunk_id,
                    "doc_id": chunk.get("doc_id", ""),
                    "doc_title": "",
                    "content": chunk.get("content", ""),
                    "score": score,
                    "source": source,
                }
            )

        return results, degraded

    def invalidate_cache(self, kb_id: str) -> None:
        """Clear cached BM25 index for a KB (call after document changes)."""
        self._bm25_cache.pop(kb_id, None)
