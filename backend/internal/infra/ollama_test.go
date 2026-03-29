package infra

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type ollamaRoundTripFunc func(*http.Request) (*http.Response, error)

func (f ollamaRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func jsonResp(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestOllamaChat(t *testing.T) {
	t.Parallel()

	client := NewOllamaClient("http://ollama.local", 2*time.Second)
	client.Client = &http.Client{
		Transport: ollamaRoundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path != "/api/chat" {
				t.Fatalf("path = %s; want /api/chat", r.URL.Path)
			}
			return jsonResp(http.StatusOK, `{"message":{"content":"answer"},"prompt_eval_count":12,"eval_count":34}`), nil
		}),
	}

	answer, promptT, completionT, err := client.Chat(context.Background(), "model", "hello")
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if answer != "answer" || promptT != 12 || completionT != 34 {
		t.Fatalf("Chat() = (%q,%d,%d); want (%q,%d,%d)", answer, promptT, completionT, "answer", 12, 34)
	}
}

func TestOllamaChatStatusError(t *testing.T) {
	t.Parallel()

	client := NewOllamaClient("http://ollama.local", 2*time.Second)
	client.Client = &http.Client{
		Transport: ollamaRoundTripFunc(func(r *http.Request) (*http.Response, error) {
			return jsonResp(http.StatusBadGateway, `{}`), nil
		}),
	}

	_, _, _, err := client.Chat(context.Background(), "model", "hello")
	if err == nil || !strings.Contains(err.Error(), "ollama chat status") {
		t.Fatalf("Chat() error = %v; want ollama chat status error", err)
	}
}

func TestOllamaEmbed(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		client := NewOllamaClient("http://ollama.local", 2*time.Second)
		client.Client = &http.Client{
			Transport: ollamaRoundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.Path != "/api/embeddings" {
					t.Fatalf("path = %s; want /api/embeddings", r.URL.Path)
				}
				return jsonResp(http.StatusOK, `{"embedding":[0.1,0.2,0.3]}`), nil
			}),
		}

		emb, err := client.Embed(context.Background(), "embed-model", "hello")
		if err != nil {
			t.Fatalf("Embed() error = %v", err)
		}
		if len(emb) != 3 {
			t.Fatalf("len(embedding) = %d; want 3", len(emb))
		}
	})

	t.Run("empty embedding", func(t *testing.T) {
		t.Parallel()
		client := NewOllamaClient("http://ollama.local", 2*time.Second)
		client.Client = &http.Client{
			Transport: ollamaRoundTripFunc(func(r *http.Request) (*http.Response, error) {
				return jsonResp(http.StatusOK, `{"embedding":[]}`), nil
			}),
		}

		_, err := client.Embed(context.Background(), "embed-model", "hello")
		if err == nil || !strings.Contains(err.Error(), "empty embedding") {
			t.Fatalf("Embed() error = %v; want empty embedding", err)
		}
	})
}

func TestOllamaReady(t *testing.T) {
	t.Parallel()

	t.Run("up", func(t *testing.T) {
		t.Parallel()
		client := NewOllamaClient("http://ollama.local", 2*time.Second)
		client.Client = &http.Client{
			Transport: ollamaRoundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.Path != "/api/tags" {
					t.Fatalf("path = %s; want /api/tags", r.URL.Path)
				}
				return jsonResp(http.StatusOK, `{}`), nil
			}),
		}
		if err := client.Ready(context.Background()); err != nil {
			t.Fatalf("Ready() error = %v", err)
		}
	})

	t.Run("down", func(t *testing.T) {
		t.Parallel()
		client := NewOllamaClient("http://ollama.local", 2*time.Second)
		client.Client = &http.Client{
			Transport: ollamaRoundTripFunc(func(r *http.Request) (*http.Response, error) {
				return jsonResp(http.StatusServiceUnavailable, `{}`), nil
			}),
		}
		err := client.Ready(context.Background())
		if err == nil || !strings.Contains(err.Error(), "ollama status") {
			t.Fatalf("Ready() error = %v; want ollama status error", err)
		}
	})
}
