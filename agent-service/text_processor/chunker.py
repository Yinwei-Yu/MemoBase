"""Markdown-aware document chunker with sentence boundary detection."""

import re

# Split on markdown headings (any level)
_HEADING_RE = re.compile(r"(?m)^#{1,6}\s+.*$")

# Split on code fences
_CODE_FENCE_RE = re.compile(r"(?m)^```[^\n]*\n[\s\S]*?^```")

# Sentence boundary: Chinese period/question/exclamation, or Latin punctuation
_SENTENCE_END_RE = re.compile(r"(?<=[。！？.!?])\s*")


def chunk_document(
    content: str,
    max_chunk_size: int = 500,
    overlap: int = 100,
    markdown_aware: bool = True,
) -> list[str]:
    """Split document into chunks respecting markdown structure and sentence boundaries.

    Strategy:
    1. If markdown_aware, split on headings and code fences first (structural chunks).
    2. Within each structural section, split into sentence groups up to max_chunk_size.
    3. Apply overlap by including trailing sentences from previous chunk.
    """
    if max_chunk_size <= 0:
        max_chunk_size = 500
    if overlap < 0:
        overlap = 0
    if overlap >= max_chunk_size:
        overlap = max_chunk_size // 5

    content = content.strip()
    if not content:
        return []

    if markdown_aware:
        sections = _split_markdown_sections(content)
    else:
        sections = [content]

    chunks: list[str] = []
    for section in sections:
        section = section.strip()
        if not section:
            continue
        if len(section) <= max_chunk_size:
            chunks.append(section)
        else:
            chunks.extend(_split_by_sentences(section, max_chunk_size, overlap))

    return chunks if chunks else []


def _split_markdown_sections(content: str) -> list[str]:
    """Split content into sections by headings and code fences."""
    # Find all split points
    splits: list[int] = [0]

    for m in _HEADING_RE.finditer(content):
        if m.start() > 0:
            splits.append(m.start())

    for m in _CODE_FENCE_RE.finditer(content):
        if m.start() > 0:
            splits.append(m.start())
        splits.append(m.end())

    splits.append(len(content))
    splits = sorted(set(splits))

    sections: list[str] = []
    for i in range(len(splits) - 1):
        chunk = content[splits[i] : splits[i + 1]].strip()
        if chunk:
            sections.append(chunk)

    return sections


def _split_by_sentences(text: str, max_size: int, overlap: int) -> list[str]:
    """Split text into chunks at sentence boundaries with overlap."""
    sentences = _SENTENCE_END_RE.split(text)
    sentences = [s.strip() for s in sentences if s.strip()]

    if not sentences:
        return [text] if text.strip() else []

    chunks: list[str] = []
    current: list[str] = []
    current_len = 0

    for sent in sentences:
        sent_len = len(sent)
        if current_len + sent_len > max_size and current:
            chunks.append("".join(current))
            # Apply overlap: keep trailing sentences that fit within overlap
            overlap_buf: list[str] = []
            overlap_len = 0
            for s in reversed(current):
                if overlap_len + len(s) > overlap:
                    break
                overlap_buf.insert(0, s)
                overlap_len += len(s)
            current = overlap_buf
            current_len = overlap_len

        current.append(sent)
        current_len += sent_len

    if current:
        chunks.append("".join(current))

    return chunks
