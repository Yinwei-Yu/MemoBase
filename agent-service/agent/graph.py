"""LangGraph agent graph definition.

Builds the agent workflow:
START → decide → (respond → generate | search → retrieve → grade)
  grade → (all relevant → generate | any irrelevant → rewrite → retrieve)
  rewrite loop limited to max_retrieval_attempts (default 2).
"""

from __future__ import annotations

import logging
from typing import Any, Literal

from langgraph.graph import END, StateGraph

from agent.nodes import (
    decide_node,
    generate_node,
    grade_node,
    retrieve_node,
    rewrite_node,
)
from agent.state import AgentState

logger = logging.getLogger("agent-service.graph")

MAX_REWRITE_ATTEMPTS = 2


def _route_after_decide(state: AgentState) -> Literal["retrieve", "generate"]:
    """Route based on the decide node's decision."""
    decision = state.get("decision", "search")
    if decision == "respond":
        logger.info("Routing: decide → generate (direct response)")
        return "generate"
    logger.info("Routing: decide → retrieve (need to search)")
    return "retrieve"


def _route_after_grade(state: AgentState) -> Literal["generate", "rewrite"]:
    """Route after grading: generate if docs are relevant, else rewrite.

    If no docs were retrieved, go straight to generate.
    If rewrite count exceeds max, generate anyway.
    """
    grades = state.get("doc_grades", [])
    rewrite_count = state.get("rewrite_count", 0)

    if not grades:
        logger.info("Routing: grade → generate (no docs to grade)")
        return "generate"

    # Check if any docs are relevant
    has_relevant = any(g.get("relevant") for g in grades)
    if has_relevant:
        logger.info("Routing: grade → generate (found relevant docs)")
        return "generate"

    # No relevant docs - try rewrite if under limit
    if rewrite_count < MAX_REWRITE_ATTEMPTS:
        logger.info(
            "Routing: grade → rewrite (no relevant docs, attempt %d/%d)",
            rewrite_count + 1,
            MAX_REWRITE_ATTEMPTS,
        )
        return "rewrite"

    logger.info(
        "Routing: grade → generate (max rewrite attempts %d reached)",
        MAX_REWRITE_ATTEMPTS,
    )
    return "generate"


def build_graph() -> StateGraph:
    """Build and compile the agent StateGraph.

    Graph structure:
        START → decide
        decide → retrieve (if search) or generate (if respond)
        retrieve → grade
        grade → generate (if relevant) or rewrite (if not)
        rewrite → retrieve (loop back)
        generate → END
    """
    graph = StateGraph(AgentState)

    # Add nodes
    graph.add_node("decide", decide_node)
    graph.add_node("retrieve", retrieve_node)
    graph.add_node("grade", grade_node)
    graph.add_node("rewrite", rewrite_node)
    graph.add_node("generate", generate_node)

    # Edges
    graph.set_entry_point("decide")

    # Conditional edge from decide
    graph.add_conditional_edges(
        "decide",
        _route_after_decide,
        {
            "retrieve": "retrieve",
            "generate": "generate",
        },
    )

    # retrieve → grade
    graph.add_edge("retrieve", "grade")

    # Conditional edge from grade
    graph.add_conditional_edges(
        "grade",
        _route_after_grade,
        {
            "generate": "generate",
            "rewrite": "rewrite",
        },
    )

    # rewrite → retrieve (loop back)
    graph.add_edge("rewrite", "retrieve")

    # generate → END
    graph.add_edge("generate", END)

    compiled = graph.compile()
    logger.info("Agent graph compiled successfully")
    return compiled


# Module-level compiled graph (lazy)
_compiled_graph: Any | None = None


def get_graph() -> Any:
    """Get or compile the agent graph."""
    global _compiled_graph
    if _compiled_graph is None:
        _compiled_graph = build_graph()
    return _compiled_graph
