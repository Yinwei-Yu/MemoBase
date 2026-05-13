"""CJK-aware tokenizer using jieba search mode."""

import unicodedata

import jieba

from text_processor.stopwords import is_stopword


def _is_cjk(s: str) -> bool:
    for ch in s:
        if "一" <= ch <= "鿿" or "぀" <= ch <= "ヿ":
            return True
    return False


def _is_punct_or_space(s: str) -> bool:
    for ch in s:
        cat = unicodedata.category(ch)
        if not (cat.startswith("P") or cat.startswith("Z") or cat.startswith("C")):
            return False
    return True


def tokenize(text: str) -> list[str]:
    """Split text into tokens for BM25 indexing.

    CJK: jieba search mode segmentation + unigram fallback, stopwords filtered.
    Latin: word-boundary splitting, lowercased.
    """
    segments = jieba.cut_for_search(text)

    tokens: list[str] = []
    seen: set[str] = set()

    for seg in segments:
        seg = seg.strip()
        if not seg:
            continue
        if _is_punct_or_space(seg):
            continue

        if _is_cjk(seg):
            # Add segmented token (filtered by stopwords)
            if not is_stopword(seg) and seg not in seen:
                tokens.append(seg)
                seen.add(seg)
            # Unigram fallback for tokens > 1 character
            if len(seg) > 1:
                for ch in seg:
                    if not is_stopword(ch) and ch not in seen:
                        tokens.append(ch)
                        seen.add(ch)
        else:
            # Latin/numeric: lowercase
            lower = seg.lower()
            if lower not in seen:
                tokens.append(lower)
                seen.add(lower)

    return tokens if tokens else []
