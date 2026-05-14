package infra

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ProviderClient is a generic OpenAI-compatible chat completions client.
type ProviderClient struct {
	Client *http.Client
}

func NewProviderClient() *ProviderClient {
	return &ProviderClient{Client: &http.Client{Timeout: 60 * time.Second}}
}

type providerChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type providerChatRequest struct {
	Model    string                 `json:"model"`
	Messages []providerChatMessage  `json:"messages"`
	Stream   bool                   `json:"stream"`
}

type providerChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

// Chat sends a non-streaming chat request to an OpenAI-compatible endpoint.
// Returns (answer, promptTokens, completionTokens, error).
func (p *ProviderClient) Chat(ctx context.Context, apiBaseURL, apiKey, model, prompt string) (string, int, int, error) {
	startedAt := time.Now()

	payload, _ := json.Marshal(providerChatRequest{
		Model:    model,
		Messages: []providerChatMessage{{Role: "user", Content: prompt}},
		Stream:   false,
	})

	url := buildChatURL(apiBaseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return "", 0, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := p.Client.Do(req)
	if err != nil {
		return "", 0, 0, fmt.Errorf("provider request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, 0, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return "", 0, 0, fmt.Errorf("provider returned status %d: %s", resp.StatusCode, string(body))
	}

	var out providerChatResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return "", 0, 0, fmt.Errorf("failed to parse response: %w", err)
	}
	if out.Error != nil {
		return "", 0, 0, fmt.Errorf("provider error: %s", out.Error.Message)
	}
	if len(out.Choices) == 0 {
		return "", 0, 0, fmt.Errorf("provider returned no choices")
	}

	promptT, completionT := 0, 0
	if out.Usage != nil {
		promptT = out.Usage.PromptTokens
		completionT = out.Usage.CompletionTokens
	}

	_ = startedAt // available for logging if needed
	return out.Choices[0].Message.Content, promptT, completionT, nil
}

// TestConnection sends a simple test prompt to verify the provider works.
func (p *ProviderClient) TestConnection(ctx context.Context, apiBaseURL, apiKey, model string) (string, time.Duration, error) {
	start := time.Now()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	answer, _, _, err := p.Chat(ctx, apiBaseURL, apiKey, model, "Say hello in one word.")
	if err != nil {
		return "", time.Since(start), err
	}
	return answer, time.Since(start), nil
}

// buildChatURL ensures the URL ends with /v1/chat/completions.
func buildChatURL(baseURL string) string {
	baseURL = strings.TrimRight(baseURL, "/")
	if strings.HasSuffix(baseURL, "/v1/chat/completions") {
		return baseURL
	}
	if strings.HasSuffix(baseURL, "/v1") {
		return baseURL + "/chat/completions"
	}
	return baseURL + "/v1/chat/completions"
}
