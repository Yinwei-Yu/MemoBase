"""Chinese stopwords — embedded Harbin Institute of Technology (哈工大) stopword list."""

from pathlib import Path

_STOPWORDS_FILE = Path(__file__).parent / "data" / "hit_stopwords.txt"

_stopwords: set[str] | None = None


def _load() -> set[str]:
    global _stopwords
    if _stopwords is not None:
        return _stopwords
    if _STOPWORDS_FILE.exists():
        _stopwords = {
            line.strip()
            for line in _STOPWORDS_FILE.read_text(encoding="utf-8").splitlines()
            if line.strip() and not line.startswith("#")
        }
    else:
        # Fallback: minimal set
        _stopwords = {
            "的", "了", "在", "是", "我", "有", "和", "就", "不", "人",
            "都", "一", "一个", "上", "也", "很", "到", "说", "要", "去",
            "你", "会", "着", "没有", "看", "好", "自己", "这", "他", "她",
            "它", "们", "那", "些", "什么", "怎么", "吗", "吧", "呢", "啊",
            "嗯", "哦", "呀", "把", "被", "让", "给", "从", "向", "对",
            "以", "因为", "所以", "但是", "而且", "或者", "如果", "虽然",
            "不过", "只是", "可以", "可能", "已经", "正在", "这个", "那个",
            "这些", "那些", "这里", "那里", "哪", "哪些", "哪里", "每",
            "各", "又", "再", "还", "才", "只", "太", "非常", "最", "更",
            "比较", "及", "与", "等", "等等",
        }
    return _stopwords


def is_stopword(token: str) -> bool:
    """Check if token is a Chinese stopword."""
    return token in _load()
