"""CJK-aware text tokenization and document chunking."""

from text_processor.tokenizer import tokenize
from text_processor.chunker import chunk_document

__all__ = ["tokenize", "chunk_document"]
