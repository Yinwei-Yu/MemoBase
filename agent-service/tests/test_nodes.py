"""Tests for agent graph nodes."""

from unittest.mock import MagicMock, patch

import pytest

from agent.nodes import (
    _format_chat_history,
    _format_docs,
    _format_memories,
    _parse_json_response,
    decide_node,
    generate_node,
    grade_node,
    retrieve_node,
    rewrite_node,
)
from agent.state import initial_state


class TestParseJSON:
    def test_valid_json(self):
        result = _parse_json_response('{"decision": "search"}')
        assert result["decision"] == "search"

    def test_json_with_markdown_fence(self):
        result = _parse_json_response('```json\n{"decision": "respond"}\n```')
        assert result["decision"] == "respond"

    def test_json_with_extra_text(self):
        result = _parse_json_response('Here is the result: {"decision": "search"} end')
        assert result["decision"] == "search"

    def test_array_json(self):
        result = _parse_json_response('[{"relevant": true}, {"relevant": false}]')
        assert "items" in result
        assert len(result["items"]) == 2

    def test_invalid_json(self):
        result = _parse_json_response("not json at all")
        assert result == {}


class TestFormatDocs:
    def test_format_multiple_docs(self):
        docs = [
            {"chunk_id": "c1", "content": "First doc content."},
            {"chunk_id": "c2", "content": "Second doc content."},
        ]
        formatted = _format_docs(docs)
        assert "c1" in formatted
        assert "c2" in formatted
        assert "First doc content" in formatted

    def test_format_empty(self):
        formatted = _format_docs([])
        assert "无检索结果" in formatted


class TestFormatChatHistory:
    def test_empty_history(self):
        state = initial_state(question="test", kb_id="kb1")
        result = _format_chat_history(state)
        assert "无对话历史" in result

    def test_with_history(self):
        from langchain_core.messages import AIMessage, HumanMessage

        state = initial_state(
            question="test",
            kb_id="kb1",
            chat_history=[
                HumanMessage(content="Hello"),
                AIMessage(content="Hi there"),
            ],
        )
        result = _format_chat_history(state)
        assert "Hello" in result
        assert "Hi there" in result


class TestFormatMemories:
    def test_empty_memories(self):
        result = _format_memories([])
        assert "无历史记忆" in result

    def test_none_memories(self):
        result = _format_memories(None)
        assert "无历史记忆" in result

    def test_single_memory(self):
        memories = [{"type": "short_term", "summary": "用户问了Go问题"}]
        result = _format_memories(memories)
        assert "short_term" in result
        assert "用户问了Go问题" in result

    def test_multiple_memories(self):
        memories = [
            {"type": "short_term", "summary": "Q: 什么是AI"},
            {"type": "long_term", "summary": "用户熟悉Go语言"},
            {"type": "fact", "summary": "用户公司用K8s"},
        ]
        result = _format_memories(memories)
        assert "short_term" in result
        assert "long_term" in result
        assert "fact" in result
        assert "用户熟悉Go语言" in result

    def test_memory_missing_fields(self):
        memories = [{"type": "long_term"}, {"summary": "some fact"}]
        result = _format_memories(memories)
        assert "long_term" in result
        assert "some fact" in result


class TestDecideNode:
    @pytest.mark.asyncio
    async def test_decide_search(self, mock_llm):
        mock_llm.ainvoke.return_value = MagicMock(
            content='{"decision": "search", "reasoning": "Need facts."}'
        )
        state = initial_state(question="What is AI?", kb_id="kb1")
        result = await decide_node(state)
        assert result["decision"] == "search"
        assert result["decision_reasoning"] == "Need facts."

    @pytest.mark.asyncio
    async def test_decide_respond(self, mock_llm):
        mock_llm.ainvoke.return_value = MagicMock(
            content='{"decision": "respond", "reasoning": "Greeting."}'
        )
        state = initial_state(question="Hello", kb_id="kb1")
        result = await decide_node(state)
        assert result["decision"] == "respond"

    @pytest.mark.asyncio
    async def test_decide_default_search_on_error(self, mock_llm):
        mock_llm.ainvoke.side_effect = Exception("LLM unavailable")
        state = initial_state(question="What is AI?", kb_id="kb1")
        result = await decide_node(state)
        assert result["decision"] == "respond"


class TestRetrieveNode:
    @pytest.mark.asyncio
    async def test_retrieve_success(self):
        with patch("agent.nodes.search_knowledge_base") as mock_search:
            mock_search.return_value = (
                [
                    {
                        "chunk_id": "c1",
                        "doc_id": "d1",
                        "content": "test",
                        "score": 0.9,
                        "source": "fused",
                    }
                ],
                False,
            )
            state = initial_state(question="test", kb_id="kb1")
            result = await retrieve_node(state)
            assert len(result["retrieved_docs"]) == 1
            assert not result["degraded"]

    @pytest.mark.asyncio
    async def test_retrieve_degraded(self):
        with patch("agent.nodes.search_knowledge_base") as mock_search:
            mock_search.side_effect = Exception("Qdrant unavailable")
            state = initial_state(question="test", kb_id="kb1")
            result = await retrieve_node(state)
            assert result["retrieved_docs"] == []
            assert result["degraded"]


class TestGradeNode:
    @pytest.mark.asyncio
    async def test_grade_no_docs(self):
        state = initial_state(question="test", kb_id="kb1")
        state["retrieved_docs"] = []
        result = await grade_node(state)
        assert result["doc_grades"] == []

    @pytest.mark.asyncio
    async def test_grade_fallback_on_error(self, mock_llm):
        mock_llm.ainvoke.side_effect = Exception("LLM error")
        state = initial_state(question="test", kb_id="kb1")
        state["retrieved_docs"] = [{"chunk_id": "c1", "content": "test", "score": 0.9}]
        result = await grade_node(state)
        assert len(result["doc_grades"]) > 0
        assert result["doc_grades"][0]["relevant"] is True


class TestRewriteNode:
    @pytest.mark.asyncio
    async def test_rewrite_increments_count(self, mock_llm):
        mock_llm.ainvoke.return_value = MagicMock(content="rewritten query")
        state = initial_state(question="original query", kb_id="kb1")
        result = await rewrite_node(state)
        assert result["rewrite_count"] == 1
        assert result["question"] == "rewritten query"

    @pytest.mark.asyncio
    async def test_rewrite_clears_docs(self, mock_llm):
        mock_llm.ainvoke.return_value = MagicMock(content="new query")
        state = initial_state(question="original", kb_id="kb1")
        state["retrieved_docs"] = [{"chunk_id": "c1"}]
        state["doc_grades"] = [{"chunk_id": "c1", "relevant": False}]
        result = await rewrite_node(state)
        assert result["retrieved_docs"] == []
        assert result["doc_grades"] == []

    @pytest.mark.asyncio
    async def test_rewrite_fallback_on_error(self, mock_llm):
        mock_llm.ainvoke.side_effect = Exception("LLM error")
        state = initial_state(question="original query", kb_id="kb1")
        result = await rewrite_node(state)
        assert result["question"] == "original query"


class TestGenerateNode:
    @pytest.mark.asyncio
    async def test_generate_with_docs(self, mock_llm):
        mock_llm.ainvoke.return_value = MagicMock(
            content="Based on the documents, AI is artificial intelligence.",
            response_metadata={"prompt_eval_count": 20, "eval_count": 10},
        )
        state = initial_state(question="What is AI?", kb_id="kb1")
        state["retrieved_docs"] = [
            {
                "chunk_id": "c1",
                "content": "AI definition",
                "score": 0.9,
                "source": "fused",
            }
        ]
        state["doc_grades"] = [{"chunk_id": "c1", "relevant": True}]
        result = await generate_node(state)
        assert len(result["final_answer"]) > 0
        assert len(result["citations"]) > 0
        assert result["token_usage"]["prompt_tokens"] == 20

    @pytest.mark.asyncio
    async def test_generate_without_docs(self, mock_llm):
        mock_llm.ainvoke.return_value = MagicMock(
            content="I cannot answer without context.",
            response_metadata={"prompt_eval_count": 5, "eval_count": 5},
        )
        state = initial_state(question="What is AI?", kb_id="kb1")
        result = await generate_node(state)
        assert len(result["final_answer"]) > 0

    @pytest.mark.asyncio
    async def test_generate_error_fallback(self, mock_llm):
        mock_llm.ainvoke.side_effect = Exception("LLM error")
        state = initial_state(question="test", kb_id="kb1")
        result = await generate_node(state)
        assert "出错" in result["final_answer"]
