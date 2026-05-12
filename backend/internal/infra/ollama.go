package infra

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

type OllamaClient struct {
	BaseURL string
	Client  *http.Client
	Logger  *slog.Logger
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
}

type chatResponse struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
	PromptEvalCount    int `json:"prompt_eval_count"`
	EvalCount          int `json:"eval_count"`
	TotalDuration      int `json:"total_duration"`
	PromptEvalDuration int `json:"prompt_eval_duration"`
	EvalDuration       int `json:"eval_duration"`
}

type embedRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type embedResponse struct {
	Embedding []float64 `json:"embedding"`
}

func NewOllamaClient(baseURL string, timeout time.Duration, logger *slog.Logger) *OllamaClient {
	return &OllamaClient{BaseURL: baseURL, Client: &http.Client{Timeout: timeout}, Logger: logger}
}

func (o *OllamaClient) Chat(ctx context.Context, model, prompt string) (string, int, int, error) {
	startedAt := time.Now()
	payload, _ := json.Marshal(chatRequest{
		Model:    model,
		Messages: []chatMessage{{Role: "user", Content: prompt}},
		Stream:   false,
	})
	url := fmt.Sprintf("%s/api/chat", o.BaseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return "", 0, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := o.Client.Do(req)
	if err != nil {
		o.Logger.Error("ollama_chat_request_failed",
			slog.String("model", model),
			slog.String("error", err.Error()),
		)
		return "", 0, 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		o.Logger.Error("ollama_chat_status_error",
			slog.String("model", model),
			slog.Int("status", resp.StatusCode),
		)
		return "", 0, 0, fmt.Errorf("ollama chat status: %d", resp.StatusCode)
	}
	var out chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", 0, 0, err
	}
	o.Logger.Info("ollama_chat_completed",
		slog.String("model", model),
		slog.Int("prompt_tokens", out.PromptEvalCount),
		slog.Int("completion_tokens", out.EvalCount),
		slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
	)
	return out.Message.Content, out.PromptEvalCount, out.EvalCount, nil
}

func (o *OllamaClient) Embed(ctx context.Context, model, text string) ([]float64, error) {
	startedAt := time.Now()
	payload, _ := json.Marshal(embedRequest{Model: model, Prompt: text})
	url := fmt.Sprintf("%s/api/embeddings", o.BaseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := o.Client.Do(req)
	if err != nil {
		o.Logger.Error("ollama_embed_request_failed",
			slog.String("model", model),
			slog.String("error", err.Error()),
		)
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		o.Logger.Error("ollama_embed_status_error",
			slog.String("model", model),
			slog.Int("status", resp.StatusCode),
		)
		return nil, fmt.Errorf("ollama embed status: %d", resp.StatusCode)
	}
	var out embedResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if len(out.Embedding) == 0 {
		o.Logger.Warn("ollama_embed_empty",
			slog.String("model", model),
		)
		return nil, fmt.Errorf("empty embedding")
	}
	o.Logger.Debug("ollama_embed_completed",
		slog.String("model", model),
		slog.Int("embedding_dim", len(out.Embedding)),
		slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
	)
	return out.Embedding, nil
}

func (o *OllamaClient) Ready(ctx context.Context) error {
	url := fmt.Sprintf("%s/api/tags", o.BaseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := o.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("ollama status: %d", resp.StatusCode)
	}
	return nil
}
