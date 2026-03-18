package core

type Pagination struct {
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
	Total    int `json:"total"`
}

type Citation struct {
	DocID           string  `json:"doc_id"`
	DocTitle        string  `json:"doc_title"`
	ChunkID         string  `json:"chunk_id"`
	Snippet         string  `json:"snippet"`
	Score           float64 `json:"score"`
	RetrievalSource string  `json:"retrieval_source"`
}

type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}
