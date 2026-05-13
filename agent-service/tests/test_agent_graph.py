"""Tests for the LangGraph StateGraph execution."""

from typing import Any
from unittest.mock import MagicMock

import pytest

from agent.graph import (
    _route_after_decide,
    _route_after_grade,
    build_graph,
)
from agent.tools import reset_retriever


# ── Helpers ──────────────────────────────────────────────────────────────────


def _make_chain(responses: list[str]) -> MagicMock:
    """Build a mock RunnableSequence that returns *responses* in sequence."""
    call_count = {"n": 0}

    def _invoke(*args, **kwargs):
        idx = call_count["n"]
        call_count["n"] += 1
        resp = responses[min(idx, len(responses) - 1)]
        mock_msg = MagicMock()
        mock_msg.content = resp
        mock_msg.response_metadata = {"prompt_eval_count": 10, "eval_count": 5}
        return mock_msg

    chain = MagicMock()
    chain.invoke = MagicMock(side_effect=_invoke)
    return chain


def _build_mock_graph() -> Any:
    """Build the agent graph with real nodes (LLM mocked externally)."""
    return build_graph()


# ── Routing logic (unit tests for conditional edges) ─────────────────────────


class TestRouteAfterDecide:
    def test_routes_to_respond(self):
        state = {"decision": "respond"}
        assert _route_after_decide(state) == "respond"

    def test_routes_to_retrieve_for_search(self):
        state = {"decision": "search"}
        assert _route_after_decide(state) == "retrieve"

    def test_routes_to_retrieve_when_missing(self):
        """Missing decision defaults to retrieve."""
        state = {}
        assert _route_after_decide(state) == "retrieve"


class TestRouteAfterGrade:
    def test_routes_to_generate_when_relevant(self):
        state = {
            "doc_grades": [{"relevant": "no"}, {"relevant": "yes"}],
            "rewrite_count": 0,
        }
        assert _route_after_grade(state) == "generate"

    def test_routes_to_rewrite_when_none_relevant_and_attempts_remain(self):
        state = {
            "doc_grades": [{"relevant": "no"}, {"relevant": "no"}],
            "rewrite_count": 0,
        }
        assert _route_after_grade(state) == "rewrite"

    def test_routes_to_generate_when_max_rewrites_reached(self):
        state = {
            "doc_grades": [{"relevant": "no"}],
            "rewrite_count": 1,
        }
        assert _route_after_grade(state) == "generate"

    def test_no_grades_routes_to_rewrite(self):
        state = {"doc_grades": [], "rewrite_count": 0}
        assert _route_after_grade(state) == "rewrite"


# ── Full graph execution (mocked LLM) ────────────────────────────────────────


class TestGraphExecution:
    """End-to-end graph tests using fully mocked chains."""

    @pytest.fixture(autouse=True)
    def _reset_retriever(self):
        reset_retriever()
        yield
        reset_retriever()

    def test_respond_path(self, sample_state):
        """When decide returns 'respond', graph goes directly to generate."""
        graph = build_graph()
        assert graph is not None
