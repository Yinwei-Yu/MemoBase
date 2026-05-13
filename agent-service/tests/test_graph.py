"""Tests for the agent graph routing logic."""


from agent.graph import _route_after_decide, _route_after_grade, build_graph
from agent.state import initial_state


class TestRouteAfterDecide:
    def test_route_to_retrieve_on_search(self):
        state = initial_state(question="What is ML?", kb_id="kb1")
        state["decision"] = "search"
        result = _route_after_decide(state)
        assert result == "retrieve"

    def test_route_to_generate_on_respond(self):
        state = initial_state(question="Hello", kb_id="kb1")
        state["decision"] = "respond"
        result = _route_after_decide(state)
        assert result == "generate"


class TestRouteAfterGrade:
    def test_route_to_generate_when_relevant(self):
        state = initial_state(question="test", kb_id="kb1")
        state["doc_grades"] = [
            {"chunk_id": "c1", "relevant": True, "reasoning": "Matches"}
        ]
        result = _route_after_grade(state)
        assert result == "generate"

    def test_route_to_rewrite_when_none_relevant(self):
        state = initial_state(question="test", kb_id="kb1")
        state["doc_grades"] = [
            {"chunk_id": "c1", "relevant": False, "reasoning": "No match"}
        ]
        state["rewrite_count"] = 0
        result = _route_after_grade(state)
        assert result == "rewrite"

    def test_route_to_generate_when_max_rewrites(self):
        state = initial_state(question="test", kb_id="kb1")
        state["doc_grades"] = [
            {"chunk_id": "c1", "relevant": False, "reasoning": "No match"}
        ]
        state["rewrite_count"] = 2  # MAX_REWRITE_ATTEMPTS
        result = _route_after_grade(state)
        assert result == "generate"

    def test_route_to_generate_when_no_grades(self):
        state = initial_state(question="test", kb_id="kb1")
        state["doc_grades"] = []
        result = _route_after_grade(state)
        assert result == "generate"

    def test_route_partial_relevance(self):
        """If at least one doc is relevant, go to generate."""
        state = initial_state(question="test", kb_id="kb1")
        state["doc_grades"] = [
            {"chunk_id": "c1", "relevant": False, "reasoning": "No"},
            {"chunk_id": "c2", "relevant": True, "reasoning": "Yes"},
        ]
        result = _route_after_grade(state)
        assert result == "generate"


class TestGraphBuild:
    def test_graph_compiles(self):
        """Verify the graph can be built without errors."""
        graph = build_graph()
        assert graph is not None
        # Graph should have the expected nodes
        nodes = graph.get_graph().nodes
        assert "decide" in nodes
        assert "retrieve" in nodes
        assert "grade" in nodes
        assert "rewrite" in nodes
        assert "generate" in nodes
