"""Tests for the hybrid retriever module."""


from retriever.hybrid import (
    _scores_to_ranks,
    reciprocal_rank_fusion,
)
from text_processor.tokenizer import tokenize


class TestTokenize:
    def test_simple_english(self):
        tokens = tokenize("Hello world, this is a test!")
        assert "hello" in tokens
        assert "world" in tokens
        assert "this" in tokens

    def test_chinese_text(self):
        tokens = tokenize("机器学习是人工智能的一个分支")
        assert len(tokens) > 0

    def test_empty_string(self):
        tokens = tokenize("")
        assert tokens == []

    def test_numbers_and_symbols(self):
        tokens = tokenize("test 123 abc !@#")
        assert "test" in tokens
        assert "123" in tokens
        assert "abc" in tokens


class TestScoreToRanks:
    def test_basic_ranking(self):
        scores = {"a": 0.9, "b": 0.8, "c": 0.7}
        ranks = _scores_to_ranks(scores)
        assert ranks["a"] == 1.0
        assert ranks["b"] == 2.0
        assert ranks["c"] == 3.0

    def test_ties_same_rank(self):
        scores = {"a": 0.9, "b": 0.9, "c": 0.7}
        ranks = _scores_to_ranks(scores)
        assert ranks["a"] == 1.0
        assert ranks["b"] == 1.0  # Same rank as 'a' due to tie
        assert ranks["c"] == 3.0

    def test_empty_scores(self):
        ranks = _scores_to_ranks({})
        assert ranks == {}

    def test_single_score(self):
        ranks = _scores_to_ranks({"a": 0.5})
        assert ranks["a"] == 1.0


class TestRRF:
    def test_basic_fusion(self):
        bm25 = {"a": 0.9, "b": 0.5}
        vector = {"a": 0.8, "c": 0.7}
        fused = reciprocal_rank_fusion(bm25, vector, k=60)
        assert "a" in fused  # Present in both
        assert "b" in fused  # Only in BM25
        assert "c" in fused  # Only in vector

    def test_empty_inputs(self):
        fused = reciprocal_rank_fusion({}, {}, k=60)
        assert fused == {}

    def test_bm25_only(self):
        bm25 = {"a": 0.9, "b": 0.5}
        fused = reciprocal_rank_fusion(bm25, {}, k=60)
        assert "a" in fused
        assert "b" in fused

    def test_vector_only(self):
        vector = {"a": 0.8, "c": 0.7}
        fused = reciprocal_rank_fusion({}, vector, k=60)
        assert "a" in fused
        assert "c" in fused

    def test_higher_rank_wins(self):
        """Document ranked #1 in both should score higher than #2."""
        bm25 = {"a": 0.9, "b": 0.5}
        vector = {"a": 0.8, "b": 0.6}
        fused = reciprocal_rank_fusion(bm25, vector, k=60)
        assert fused["a"] > fused["b"]
