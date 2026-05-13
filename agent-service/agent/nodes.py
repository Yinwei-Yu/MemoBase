"""Graph node implementations for the agent workflow.

Five nodes: decide, retrieve, grade, rewrite, generate.
Each node takes AgentState and returns a partial state update.
"""

from __future__ import annotations

import json
import logging
import time
from typing import Any

from agent.llm import get_chat_llm
from agent.state import AgentState
from agent.tools import search_knowledge_base

logger = logging.getLogger("agent-service.nodes")

# ── Helpers ──────────────────────────────────────────────────────────────────


def _add_trace(
    state: AgentState,
    node: str,
    action: str,
    duration_ms: int,
    data: dict[str, str] | None = None,
) -> None:
    """Append a trace step to the state (mutates in place for operator.add)."""
    step = {
        "node": node,
        "action": action,
        "duration_ms": duration_ms,
        "data": data or {},
    }
    # operator.add for trace_steps means we append
    state.setdefault("trace_steps", []).append(step)


def _parse_json_response(text: str) -> dict[str, Any]:
    """Try to parse a JSON object from LLM response text."""
    text = text.strip()
    # Try direct parse
    try:
        return json.loads(text)
    except json.JSONDecodeError:
        pass
    # Try to extract JSON from code blocks or surrounding text
    import re

    match = re.search(r"\{[^{}]*\}", text)
    if match:
        try:
            return json.loads(match.group())
        except json.JSONDecodeError:
            pass
    match = re.search(r"\[.*\]", text, re.DOTALL)
    if match:
        try:
            parsed = json.loads(match.group())
            if isinstance(parsed, list):
                return {"items": parsed}
        except json.JSONDecodeError:
            pass
    return {}


def _format_docs(docs: list[dict[str, Any]]) -> str:
    """Format retrieved docs for prompts."""
    parts = []
    for i, doc in enumerate(docs):
        parts.append(
            f"[{i + 1}] chunk_id: {doc.get('chunk_id', '?')}\n"
            f"    content: {doc.get('content', '')[:300]}"
        )
    return "\n".join(parts) if parts else "(无检索结果)"


def _format_chat_history(state: AgentState) -> str:
    """Format chat history as a string."""
    history = state.get("chat_history", [])
    if not history:
        return "(无对话历史)"
    parts = []
    for msg in history[-6:]:  # last 6 messages
        role = getattr(msg, "type", "unknown")
        content = getattr(msg, "content", str(msg))
        parts.append(f"[{role}]: {content[:200]}")
    return "\n".join(parts)


# ── Node: Decide ─────────────────────────────────────────────────────────────


async def decide_node(state: AgentState) -> dict[str, Any]:
    """Decide whether to search knowledge base or respond directly.

    Uses prompt-based routing since qwen2.5:3b may not support tool calling.
    """
    from agent.prompts import DECIDE_PROMPT

    started = time.monotonic()
    logger.info("decide_node: question=%.60s", state["question"])

    try:
        llm = get_chat_llm(state.get("model"))
        prompt = DECIDE_PROMPT.format(
            question=state["question"],
            chat_history=_format_chat_history(state),
        )
        response = await llm.ainvoke(prompt)
        content = response.content if hasattr(response, "content") else str(response)
        logger.debug("decide_node response: %s", content[:200])

        result = _parse_json_response(content)
        decision = result.get("decision", "search")
        reasoning = result.get("reasoning", "")

        # Default to search if parsing failed
        if decision not in ("respond", "search"):
            decision = "search"
            reasoning = "failed to parse decision, defaulting to search"

    except Exception as e:
        logger.warning("decide_node failed: %s, defaulting to respond", e)
        decision = "respond"
        reasoning = f"error: {e}"

    duration_ms = int((time.monotonic() - started) * 1000)
    _add_trace(state, "decide", decision, duration_ms, {"reasoning": reasoning})

    return {
        "decision": decision,
        "decision_reasoning": reasoning,
    }


# ── Node: Retrieve ───────────────────────────────────────────────────────────


async def retrieve_node(state: AgentState) -> dict[str, Any]:
    """Execute hybrid retrieval (BM25 + vector)."""
    started = time.monotonic()

    query = state["question"]
    kb_id = state["kb_id"]
    top_k = state.get("top_k", 6)

    # If the query was rewritten, use the rewritten version
    if state.get("rewrite_count", 0) > 0 and state.get("decision_reasoning"):
        pass  # We use original question + context; rewriting is handled by rewrite_node

    logger.info("retrieve_node: kb_id=%s top_k=%d", kb_id, top_k)

    try:
        results, degraded = await search_knowledge_base(
            query=query,
            kb_id=kb_id,
            top_k=top_k,
        )
    except Exception as e:
        logger.error("retrieve_node failed: %s", e)
        results, degraded = [], True

    duration_ms = int((time.monotonic() - started) * 1000)
    _add_trace(
        state,
        "retrieve",
        f"found_{len(results)}_docs",
        duration_ms,
        {"result_count": str(len(results)), "degraded": str(degraded)},
    )

    return {
        "retrieved_docs": results,
        "degraded": state.get("degraded", False) or degraded,
    }


# ── Node: Grade ──────────────────────────────────────────────────────────────


async def grade_node(state: AgentState) -> dict[str, Any]:
    """Grade each retrieved document for relevance."""
    from agent.prompts import GRADE_PROMPT

    started = time.monotonic()
    docs = state.get("retrieved_docs", [])

    if not docs:
        _add_trace(
            state,
            "grade",
            "no_docs_to_grade",
            int((time.monotonic() - started) * 1000),
            {},
        )
        return {"doc_grades": []}

    logger.info("grade_node: grading %d documents", len(docs))

    try:
        llm = get_chat_llm(state.get("model"))
        docs_text = _format_docs(docs)
        prompt = GRADE_PROMPT.format(
            question=state["question"],
            documents=docs_text,
        )
        response = await llm.ainvoke(prompt)
        content = response.content if hasattr(response, "content") else str(response)
        logger.debug("grade_node response: %s", content[:300])

        parsed = _parse_json_response(content)
        items = parsed.get("items", parsed) if isinstance(parsed, dict) else parsed
        if isinstance(items, dict):
            items = [items]

        grades = []
        if isinstance(items, list):
            for item in items:
                if isinstance(item, dict):
                    grades.append(
                        {
                            "chunk_id": item.get("chunk_id", ""),
                            "relevant": item.get("relevant", False),
                            "reasoning": item.get("reasoning", ""),
                        }
                    )
        if not grades:
            # Default: mark first 3 as relevant
            for doc in docs[:3]:
                grades.append(
                    {
                        "chunk_id": doc.get("chunk_id", ""),
                        "relevant": True,
                        "reasoning": "default: assumed relevant",
                    }
                )
    except Exception as e:
        logger.warning("grade_node failed: %s", e)
        grades = [
            {
                "chunk_id": doc.get("chunk_id", ""),
                "relevant": True,
                "reasoning": f"error: {e}",
            }
            for doc in docs[:3]
        ]

    duration_ms = int((time.monotonic() - started) * 1000)
    relevant_count = sum(1 for g in grades if g.get("relevant"))
    _add_trace(
        state,
        "grade",
        f"{relevant_count}_relevant_of_{len(grades)}",
        duration_ms,
        {"relevant": str(relevant_count), "total": str(len(grades))},
    )

    return {"doc_grades": grades}


# ── Node: Rewrite ────────────────────────────────────────────────────────────


async def rewrite_node(state: AgentState) -> dict[str, Any]:
    """Rewrite the query for better retrieval results."""
    from agent.prompts import REWRITE_PROMPT

    started = time.monotonic()
    count = state.get("rewrite_count", 0) + 1

    logger.info("rewrite_node: attempt %d", count)

    try:
        llm = get_chat_llm(state.get("model"))
        prompt = REWRITE_PROMPT.format(question=state["question"])
        response = await llm.ainvoke(prompt)
        content = response.content if hasattr(response, "content") else str(response)
        rewritten = content.strip()
        logger.debug(
            "rewrite_node: '%s' -> '%s'", state["question"][:60], rewritten[:60]
        )
    except Exception as e:
        logger.warning("rewrite_node failed: %s", e)
        rewritten = state["question"]

    duration_ms = int((time.monotonic() - started) * 1000)
    _add_trace(
        state,
        "rewrite",
        f"attempt_{count}",
        duration_ms,
        {"original": state["question"][:100], "rewritten": rewritten[:100]},
    )

    return {
        "question": rewritten,
        "rewrite_count": count,
        "retrieved_docs": [],  # Clear previous results for re-retrieval
        "doc_grades": [],
    }


# ── Node: Generate ───────────────────────────────────────────────────────────


async def generate_node(state: AgentState) -> dict[str, Any]:
    """Generate the final answer based on retrieved context or directly."""
    from agent.prompts import GENERATE_PROMPT, NO_CONTEXT_PROMPT

    started = time.monotonic()
    docs = state.get("retrieved_docs", [])
    grades = state.get("doc_grades", [])

    # Filter to relevant docs only
    relevant_ids = {g["chunk_id"] for g in grades if g.get("relevant")}
    relevant_docs = (
        [d for d in docs if d.get("chunk_id") in relevant_ids] if relevant_ids else docs
    )

    logger.info(
        "generate_node: %d relevant docs out of %d", len(relevant_docs), len(docs)
    )

    try:
        llm = get_chat_llm(state.get("model"))

        if relevant_docs:
            context = _format_docs(relevant_docs)
            prompt = GENERATE_PROMPT.format(
                question=state["question"],
                context=context,
            )
        else:
            prompt = NO_CONTEXT_PROMPT.format(
                question=state["question"],
                chat_history=_format_chat_history(state),
            )

        response = await llm.ainvoke(prompt)
        content = response.content if hasattr(response, "content") else str(response)
        answer = content.strip()
        logger.info("generate_node: LLM returned %d chars: %.80s", len(answer), answer)

        # Extract token usage if available
        token_usage = state.get("token_usage", {})
        if hasattr(response, "response_metadata"):
            meta = response.response_metadata
            token_usage = {
                "prompt_tokens": meta.get("prompt_eval_count", 0),
                "completion_tokens": meta.get("eval_count", 0),
                "total_tokens": meta.get("prompt_eval_count", 0)
                + meta.get("eval_count", 0),
            }
    except Exception as e:
        logger.error("generate_node failed: %s", e)
        answer = f"抱歉，生成回答时出错: {e}"
        token_usage = state.get("token_usage", {})

    # Build citations
    citations = []
    for i, doc in enumerate(relevant_docs[:10]):
        citations.append(
            {
                "doc_id": doc.get("doc_id", ""),
                "doc_title": doc.get("doc_title", ""),
                "chunk_id": doc.get("chunk_id", ""),
                "snippet": (doc.get("content", "") or "")[:160],
                "score": doc.get("score", 0.0),
                "retrieval_source": doc.get("source", "unknown"),
            }
        )

    duration_ms = int((time.monotonic() - started) * 1000)
    _add_trace(
        state,
        "generate",
        "answer_generated",
        duration_ms,
        {"answer_length": str(len(answer)), "citations": str(len(citations))},
    )

    return {
        "final_answer": answer,
        "citations": citations,
        "token_usage": token_usage,
    }
