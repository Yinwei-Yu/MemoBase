"""Tests for each graph node function in isolation."""

from unittest.mock import MagicMock


from agent.nodes import (
    _extract_token_usage,
    _parse_json_output,
    decide_node,
    generate_node,
    grade_node,
    rewrite_node,
)

# ── Helpers ──────────────────────────────────────────────────────────────────


def _make_chain(content: str = "", metadata: dict | None = None, side_effect=None):
    """Build a mock RunnableSequence (chain) for testing."""
    chain = MagicMock()
    if side_effect:
        chain.invoke.side_effect = side_effect
    else:
        mock_response = MagicMock()
        mock_response.content = content
        mock_response.response_metadata = metadata or {
            "prompt_eval_count": 10,
            "eval_count": 5,
        }
        chain.invoke.return_value = mock_response
    return chain


# ── Helpers ──────────────────────────────────────────────────────────────────


class TestParseJsonOutput:
    def test_plain_json(self):
        parsed = _parse_json_output('{"decision": "search", "reasoning": "ok"}')
        assert parsed == {"decision": "search", "reasoning": "ok"}

    def test_json_in_markdown_fence(self):
        raw = '```json\n{"decision": "respond"}\n```'
        parsed = _parse_json_output(raw)
        assert parsed == {"decision": "respond"}

    def test_json_with_surrounding_text(self):
        raw = 'Some text before {"decision": "search", "reasoning": "Because..."} and after'
        parsed = _parse_json_output(raw)
        assert parsed["decision"] == "search"

    def test_invalid_json_returns_empty(self):
        parsed = _parse_json_output("not json at all")
        assert parsed == {}

    def test_empty_string(self):
        assert _parse_json_output("") == {}


class TestExtractTokenUsage:
    def test_normal_metadata(self):
        meta = {
            "prompt_eval_count": 100,
            "eval_count": 50,
            "total_duration": 1_000_000_000,
        }
        usage = _extract_token_usage(meta)
        assert usage == {
            "prompt_tokens": 100,
            "completion_tokens": 50,
            "total_tokens": 150,
        }

    def test_empty_metadata(self):
        usage = _extract_token_usage({})
        assert usage == {"prompt_tokens": 0, "completion_tokens": 0, "total_tokens": 0}


# ── decide_node ──────────────────────────────────────────────────────────────


class TestDecideNode:
    def test_decides_search(self, sample_state):
        chain = _make_chain(
            content='{"decision": "search", "reasoning": "Need to look up facts."}',
            metadata={"prompt_eval_count": 15, "eval_count": 5},
        )
        result = decide_node(sample_state, chain=chain)
        assert result["decision"] == "search"
        assert result.get("degraded") is not True
        assert "trace_steps" in result

    def test_decides_respond(self, sample_state):
        chain = _make_chain(
            content='{"decision": "respond", "reasoning": "Just a greeting."}',
            metadata={"prompt_eval_count": 10, "eval_count": 3},
        )
        result = decide_node(sample_state, chain=chain)
        assert result["decision"] == "respond"

    def test_invalid_decision_defaults_to_search(self, sample_state):
        chain = _make_chain(
            content='{"decision": "maybe", "reasoning": "..."}',
            metadata={"prompt_eval_count": 10, "eval_count": 3},
        )
        result = decide_node(sample_state, chain=chain)
        assert result["decision"] == "search"

    def test_malformed_json_defaults_to_search(self, sample_state):
        chain = _make_chain(
            content="not valid json at all",
            metadata={"prompt_eval_count": 5, "eval_count": 2},
        )
        result = decide_node(sample_state, chain=chain)
        assert result["decision"] == "search"

    def test_llm_error_returns_search_degraded(self, sample_state):
        chain = _make_chain(side_effect=RuntimeError("Ollama offline"))
        result = decide_node(sample_state, chain=chain)
        assert result["decision"] == "search"
        assert result.get("degraded") is True

    def test_accumulates_token_usage(self, sample_state):
        chain = _make_chain(
            content='{"decision": "search", "reasoning": "ok"}',
            metadata={"prompt_eval_count": 100, "eval_count": 50},
        )
        result = decide_node(sample_state, chain=chain)
        tu = result["token_usage"]
        assert tu["prompt_tokens"] == 100
        assert tu["completion_tokens"] == 50
        assert tu["total_tokens"] == 150


# ── grade_node ───────────────────────────────────────────────────────────────


class TestGradeNode:
    def _state_with_docs(self, sample_state, docs):
        s = dict(sample_state)
        s["retrieved_docs"] = docs
        return s

    def test_all_relevant(self, sample_state):
        docs = [
            {"chunk_id": "c1", "content": "Relevant text about architecture."},
            {"chunk_id": "c2", "content": "More relevant text."},
        ]
        state = self._state_with_docs(sample_state, docs)

        chain = _make_chain(
            content='{"relevant": "yes", "reasoning": "Matches."}',
            metadata={"prompt_eval_count": 10, "eval_count": 3},
        )
        result = grade_node(state, chain=chain)
        grades = result["doc_grades"]
        assert len(grades) == 2
        assert all(g["relevant"] == "yes" for g in grades)

    def test_all_irrelevant(self, sample_state):
        docs = [{"chunk_id": "c1", "content": "Irrelevant."}]
        state = self._state_with_docs(sample_state, docs)

        chain = _make_chain(
            content='{"relevant": "no", "reasoning": "No match."}',
            metadata={"prompt_eval_count": 5, "eval_count": 2},
        )
        result = grade_node(state, chain=chain)
        assert result["doc_grades"][0]["relevant"] == "no"

    def test_no_docs_returns_empty_grades(self, sample_state):
        chain = _make_chain()
        result = grade_node(sample_state, chain=chain)
        assert result["doc_grades"] == []

    def test_grading_error_marks_doc_irrelevant(self, sample_state):
        docs = [{"chunk_id": "c1", "content": "text"}]
        state = self._state_with_docs(sample_state, docs)

        chain = _make_chain(side_effect=RuntimeError("LLM error"))
        result = grade_node(state, chain=chain)
        assert result["doc_grades"][0]["relevant"] == "no"
        assert result.get("degraded") is True

    def test_truncates_long_docs(self, sample_state):
        docs = [{"chunk_id": "c1", "content": "x" * 3000}]
        state = self._state_with_docs(sample_state, docs)

        chain = _make_chain(
            content='{"relevant": "yes", "reasoning": "ok"}',
            metadata={"prompt_eval_count": 5, "eval_count": 2},
        )
        result = grade_node(state, chain=chain)
        assert len(result["doc_grades"]) == 1


# ── rewrite_node ─────────────────────────────────────────────────────────────


class TestRewriteNode:
    def test_rewrites_query(self, sample_state):
        chain = _make_chain(
            content="improved query for retrieval",
            metadata={"prompt_eval_count": 12, "eval_count": 4},
        )
        result = rewrite_node(sample_state, chain=chain)
        assert result["question"] == "improved query for retrieval"
        assert result["rewrite_count"] == 1
        assert result["retrieved_docs"] == []
        assert result["doc_grades"] == []

    def test_handles_empty_response(self, sample_state):
        chain = _make_chain(
            content="",
            metadata={"prompt_eval_count": 5, "eval_count": 0},
        )
        result = rewrite_node(sample_state, chain=chain)
        assert result["question"] == sample_state["question"]

    def test_handles_short_response(self, sample_state):
        chain = _make_chain(
            content="ab",
            metadata={"prompt_eval_count": 5, "eval_count": 1},
        )
        result = rewrite_node(sample_state, chain=chain)
        assert result["question"] == sample_state["question"]

    def test_increments_rewrite_count(self, sample_state):
        state = dict(sample_state)
        state["rewrite_count"] = 1
        chain = _make_chain(
            content="another rewrite",
            metadata={"prompt_eval_count": 10, "eval_count": 3},
        )
        result = rewrite_node(state, chain=chain)
        assert result["rewrite_count"] == 2

    def test_llm_error_sets_degraded(self, sample_state):
        chain = _make_chain(side_effect=RuntimeError("LLM down"))
        result = rewrite_node(sample_state, chain=chain)
        assert result.get("degraded") is True


# ── generate_node ────────────────────────────────────────────────────────────


class TestGenerateNode:
    def test_generates_with_docs(self, sample_state):
        state = dict(sample_state)
        state["retrieved_docs"] = [
            {
                "chunk_id": "c1",
                "doc_id": "d1",
                "doc_title": "Architecture Guide",
                "content": "The system uses microservices.",
                "score": 0.95,
                "source": "fused",
            },
        ]

        chain = _make_chain(
            content="The system uses a microservices architecture [1].",
            metadata={"prompt_eval_count": 20, "eval_count": 10},
        )
        result = generate_node(state, chain=chain)
        assert "microservices" in result["final_answer"]
        assert len(result["citations"]) == 1
        assert result["citations"][0]["chunk_id"] == "c1"

    def test_generates_without_docs(self, sample_state):
        chain = _make_chain(
            content="I don't have enough information to answer that.",
            metadata={"prompt_eval_count": 10, "eval_count": 5},
        )
        result = generate_node(sample_state, chain=chain)
        assert len(result["final_answer"]) > 0
        assert result["citations"] == []

    def test_llm_error_returns_graceful_message(self, sample_state):
        chain = _make_chain(side_effect=RuntimeError("LLM error"))
        result = generate_node(sample_state, chain=chain)
        assert "sorry" in result["final_answer"].lower()
        assert result.get("degraded") is True
