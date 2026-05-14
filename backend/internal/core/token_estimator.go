package core

import "unicode"

// MessageTokenInput is a lightweight struct for token estimation without store dependency.
type MessageTokenInput struct {
	Role    string
	Content string
}

// EstimateTokens estimates token count for text.
// Works for CJK + English mixed text, ±20% accuracy.
func EstimateTokens(text string) int {
	if text == "" {
		return 0
	}

	runes := []rune(text)
	tokens := 0
	i := 0

	for i < len(runes) {
		r := runes[i]

		switch {
		case isCJK(r):
			// CJK: ~1.5 token/char, use 3/2 integer math
			tokens += 3
			i++

		case unicode.IsLetter(r) || r == '\'':
			// English word: consume entire word
			wordLen := 0
			for i < len(runes) && (unicode.IsLetter(runes[i]) || runes[i] == '\'') {
				wordLen++
				i++
			}
			// avg 1 token/word, long words ~1 token/4 chars
			if wordLen <= 4 {
				tokens += 2 // 1 token * 2 for integer math
			} else {
				tokens += wordLen // ~1 token per char
			}

		case unicode.IsDigit(r):
			// digit sequence
			for i < len(runes) && unicode.IsDigit(runes[i]) {
				i++
			}
			tokens += 2

		default:
			// punctuation, space, special chars
			tokens += 1
			i++
		}
	}

	// divide by 2 (we used *2 for integer math)
	return tokens / 2
}

func isCJK(r rune) bool {
	return (r >= 0x4E00 && r <= 0x9FFF) || // CJK Unified Ideographs
		(r >= 0x3400 && r <= 0x4DBF) || // CJK Extension A
		(r >= 0xF900 && r <= 0xFAFF) || // CJK Compatibility
		(r >= 0x20000 && r <= 0x2A6DF) // CJK Extension B
}

// EstimateMessagesTokens estimates total tokens for a message list.
func EstimateMessagesTokens(messages []MessageTokenInput) int {
	total := 0
	for _, m := range messages {
		total += 4 // role overhead (~2-4 tokens)
		total += EstimateTokens(m.Content)
	}
	return total
}
