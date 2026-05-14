"""AgentState definition for the LangGraph agent."""

from __future__ import annotations

import operator
from typing import Annotated, Sequence, TypedDict

from langchain_core.messages import BaseMessage


class RetrievedDoc(TypedDict, total=False):
    """A retrieved document chunk with metadata."""

    chunk_id: str
    doc_id: str
    doc_title: str
    content: str
    score: float
    source: str  # "bm25", "vector", "fused"


class DocGrade(TypedDict, total=False):
    """Grading result for a retrieved document."""

    chunk_id: str
    relevant: bool
    reasoning: str


class Citation(TypedDict, total=False):
    """Citation for a source document."""

    doc_id: str
    doc_title: str
    chunk_id: str
    snippet: str
    score: float
    retrieval_source: str


class TraceStep(TypedDict, total=False):
    """A single trace step."""

    node: str
    action: str
    duration_ms: int
    data: dict[str, str]


class AgentState(TypedDict):
    """State for the LangGraph agent graph.

    This TypedDict defines the complete state that flows through
    the graph nodes. Annotated fields with operator.add accumulate
    across nodes instead of replacing.
    """

    # ── Input ──────────────────────────────────────────────────────
    question: str
    kb_id: str
    session_id: str
    model: str
    top_k: int
    chat_history: Annotated[Sequence[BaseMessage], operator.add]
    memories: list[dict[str, str]]

    # ── Provider routing ───────────────────────────────────────────
    provider_api_base_url: str
    provider_api_key: str
    provider_model: str

    # ── Internal routing ───────────────────────────────────────────
    decision: str  # "respond" | "search"
    decision_reasoning: str

    # ── Retrieval ──────────────────────────────────────────────────
    retrieved_docs: list[RetrievedDoc]
    doc_grades: list[DocGrade]
    rewrite_count: int

    # ── Output ─────────────────────────────────────────────────────
    final_answer: str
    citations: list[Citation]
    trace_steps: Annotated[list[TraceStep], operator.add]
    degraded: bool
    token_usage: dict[str, int]
    error: str


def initial_state(
    *,
    question: str,
    kb_id: str,
    session_id: str = "",
    model: str = "",
    top_k: int = 6,
    chat_history: Sequence[BaseMessage] | None = None,
    memories: list[dict[str, str]] | None = None,
    provider_api_base_url: str = "",
    provider_api_key: str = "",
    provider_model: str = "",
) -> AgentState:
    """Create the initial agent state from request parameters."""
    return AgentState(
        question=question,
        kb_id=kb_id,
        session_id=session_id,
        model=model,
        top_k=top_k or 6,
        chat_history=list(chat_history or []),
        memories=memories or [],
        provider_api_base_url=provider_api_base_url,
        provider_api_key=provider_api_key,
        provider_model=provider_model,
        decision="",
        decision_reasoning="",
        retrieved_docs=[],
        doc_grades=[],
        rewrite_count=0,
        final_answer="",
        citations=[],
        trace_steps=[],
        degraded=False,
        token_usage={},
        error="",
    )
