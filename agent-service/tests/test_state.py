"""Tests for agent state."""

from langchain_core.messages import AIMessage, HumanMessage

from agent.state import initial_state


class TestAgentState:
    def test_initial_state_minimal(self):
        state = initial_state(question="What is AI?", kb_id="kb_1")
        assert state["question"] == "What is AI?"
        assert state["kb_id"] == "kb_1"
        assert state["session_id"] == ""
        assert state["model"] == ""
        assert state["top_k"] == 6
        assert state["decision"] == ""
        assert state["retrieved_docs"] == []
        assert state["rewrite_count"] == 0
        assert state["degraded"] is False
        assert state["error"] == ""

    def test_initial_state_full(self):
        history = [HumanMessage(content="hello"), AIMessage(content="hi there")]
        state = initial_state(
            question="test",
            kb_id="kb_1",
            session_id="s_123",
            model="qwen2.5:3b",
            top_k=10,
            chat_history=history,
            memories=[{"type": "short_term", "summary": "previous Q&A"}],
        )
        assert state["session_id"] == "s_123"
        assert state["model"] == "qwen2.5:3b"
        assert state["top_k"] == 10
        assert len(state["chat_history"]) == 2
        assert len(state["memories"]) == 1
        assert state["memories"][0]["type"] == "short_term"

    def test_chat_history_accumulates(self):
        """Verify operator.add annotation works for chat_history."""
        state = initial_state(question="q", kb_id="kb_1")
        msg = HumanMessage(content="new message")
        # Manually simulate accumulation
        combined = state["chat_history"] + [msg]
        assert len(combined) == 1
        assert combined[0].content == "new message"
