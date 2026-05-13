"""Pytest configuration and shared fixtures for agent-service tests."""

from __future__ import annotations

from unittest.mock import AsyncMock, MagicMock, patch

import pytest

from agent.state import AgentState, initial_state


@pytest.fixture
def sample_state() -> AgentState:
    """Create a sample agent state for testing."""
    return initial_state(
        question="What is machine learning?",
        kb_id="kb_test_001",
        session_id="sess_test_001",
        model="qwen2.5:3b",
        top_k=6,
        memories=[{"type": "short_term", "summary": "Previous Q: What is AI?"}],
    )


@pytest.fixture
def sample_state_with_history() -> AgentState:
    """Create a state with chat history."""
    from langchain_core.messages import AIMessage, HumanMessage

    return initial_state(
        question="Tell me more",
        kb_id="kb_test_001",
        session_id="sess_test_001",
        chat_history=[
            HumanMessage(content="What is AI?"),
            AIMessage(content="AI is artificial intelligence..."),
            HumanMessage(content="Tell me more"),
        ],
    )


@pytest.fixture
def mock_chunks():
    """Mock chunk data for testing retriever."""
    return [
        {
            "id": "ck_001",
            "doc_id": "doc_001",
            "kb_id": "kb_test_001",
            "chunk_index": 0,
            "content": "Machine learning is a subset of artificial intelligence.",
            "created_at": "2024-01-01T00:00:00",
        },
        {
            "id": "ck_002",
            "doc_id": "doc_001",
            "kb_id": "kb_test_001",
            "chunk_index": 1,
            "content": "Deep learning uses neural networks with many layers.",
            "created_at": "2024-01-01T00:00:00",
        },
        {
            "id": "ck_003",
            "doc_id": "doc_002",
            "kb_id": "kb_test_001",
            "chunk_index": 0,
            "content": "Python is a popular programming language for data science.",
            "created_at": "2024-01-01T00:00:00",
        },
    ]


@pytest.fixture
def mock_retrieved_docs():
    """Mock retrieved documents."""
    return [
        {
            "chunk_id": "ck_001",
            "doc_id": "doc_001",
            "doc_title": "ML Basics",
            "content": "Machine learning is a subset of artificial intelligence.",
            "score": 0.95,
            "source": "fused",
        },
        {
            "chunk_id": "ck_002",
            "doc_id": "doc_001",
            "doc_title": "ML Basics",
            "content": "Deep learning uses neural networks with many layers.",
            "score": 0.87,
            "source": "vector",
        },
    ]


@pytest.fixture
def mock_llm_response():
    """Create a mock LLM response."""
    mock = MagicMock()
    mock.content = '{"decision": "search", "reasoning": "Need to look up facts"}'
    mock.response_metadata = {
        "prompt_eval_count": 50,
        "eval_count": 10,
    }
    return mock


@pytest.fixture
def mock_llm():
    """Create a mock ChatOllama."""
    with patch("agent.llm.get_chat_llm") as mock:
        llm = AsyncMock()
        mock.return_value = llm
        yield llm


@pytest.fixture
def mock_embeddings():
    """Create mock embeddings."""
    with patch("agent.llm.get_embeddings") as mock:
        emb = MagicMock()
        emb.embed_query.return_value = [0.1] * 768
        mock.return_value = emb
        yield emb
