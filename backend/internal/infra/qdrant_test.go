package infra

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type qdrantRoundTripFunc func(*http.Request) (*http.Response, error)

func (f qdrantRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func qdrantResp(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func newQdrantTestClient(fn qdrantRoundTripFunc) *QdrantClient {
	client := NewQdrantClient("http://qdrant.local")
	client.Client = &http.Client{Transport: fn}
	return client
}

func TestEnsureCollectionUsesCache(t *testing.T) {
	t.Parallel()

	getCount := 0
	client := newQdrantTestClient(func(r *http.Request) (*http.Response, error) {
		if r.Method == http.MethodGet && r.URL.Path == "/collections/kb_chunks" {
			getCount++
			return qdrantResp(http.StatusOK, `{"result":{"config":{"params":{"vectors":{"size":4}}}}}`), nil
		}
		t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		return nil, nil
	})

	if err := client.EnsureCollection(context.Background(), "kb_chunks", 4); err != nil {
		t.Fatalf("EnsureCollection() error = %v", err)
	}
	if err := client.EnsureCollection(context.Background(), "kb_chunks", 4); err != nil {
		t.Fatalf("EnsureCollection() second call error = %v", err)
	}
	if getCount != 1 {
		t.Fatalf("GET /collections called %d times; want 1 (cached)", getCount)
	}
}

func TestEnsureCollectionCreateOnNotFound(t *testing.T) {
	t.Parallel()

	getCount := 0
	putCount := 0
	client := newQdrantTestClient(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/collections/new_collection":
			getCount++
			return qdrantResp(http.StatusNotFound, `{}`), nil
		case r.Method == http.MethodPut && r.URL.Path == "/collections/new_collection":
			putCount++
			return qdrantResp(http.StatusOK, `{}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
			return nil, nil
		}
	})

	if err := client.EnsureCollection(context.Background(), "new_collection", 8); err != nil {
		t.Fatalf("EnsureCollection() error = %v", err)
	}
	if getCount != 1 || putCount != 1 {
		t.Fatalf("calls = (get:%d, put:%d); want (1,1)", getCount, putCount)
	}
}

func TestEnsureCollectionMismatch(t *testing.T) {
	t.Parallel()

	client := newQdrantTestClient(func(r *http.Request) (*http.Response, error) {
		return qdrantResp(http.StatusOK, `{"result":{"config":{"params":{"vectors":{"size":4}}}}}`), nil
	})

	err := client.EnsureCollection(context.Background(), "kb_chunks", 8)
	if err == nil || !strings.Contains(err.Error(), "vector size mismatch") {
		t.Fatalf("EnsureCollection() error = %v; want vector size mismatch", err)
	}
}

func TestQdrantCRUDAndSearch(t *testing.T) {
	t.Parallel()

	client := newQdrantTestClient(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/collections/c1/points":
			return qdrantResp(http.StatusOK, `{}`), nil
		case r.Method == http.MethodPost && r.URL.Path == "/collections/c1/points/delete":
			return qdrantResp(http.StatusOK, `{}`), nil
		case r.Method == http.MethodDelete && r.URL.Path == "/collections/c1":
			return qdrantResp(http.StatusOK, `{}`), nil
		case r.Method == http.MethodPost && r.URL.Path == "/collections/c1/points/search":
			return qdrantResp(http.StatusOK, `{"result":[{"score":0.9,"payload":{"chunk_id":"ck_1"}},{"score":0.4,"payload":{}}]}`), nil
		case r.Method == http.MethodGet && r.URL.Path == "/collections":
			return qdrantResp(http.StatusOK, `{}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
			return nil, nil
		}
	})

	if err := client.Upsert(context.Background(), "c1", []QdrantPoint{{ID: "1", Vector: []float64{0.1}}}); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}
	if err := client.DeleteByDoc(context.Background(), "c1", "doc1"); err != nil {
		t.Fatalf("DeleteByDoc() error = %v", err)
	}
	scores, err := client.Search(context.Background(), "c1", []float64{0.2}, 10)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if scores["ck_1"] != 0.9 {
		t.Fatalf("scores[ck_1] = %f; want 0.9", scores["ck_1"])
	}
	if err := client.DeleteCollection(context.Background(), "c1"); err != nil {
		t.Fatalf("DeleteCollection() error = %v", err)
	}
	if err := client.Ready(context.Background()); err != nil {
		t.Fatalf("Ready() error = %v", err)
	}
}

func TestQdrantSearch404ReturnsEmpty(t *testing.T) {
	t.Parallel()

	client := newQdrantTestClient(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/collections/c1/points/search" {
			return qdrantResp(http.StatusNotFound, `{}`), nil
		}
		t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		return nil, nil
	})

	scores, err := client.Search(context.Background(), "c1", []float64{0.2}, 10)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(scores) != 0 {
		t.Fatalf("len(scores) = %d; want 0", len(scores))
	}
}

func TestParseCollectionVectorSize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		json    string
		want    int
		wantErr bool
	}{
		{
			name: "single vector config",
			json: `{"result":{"config":{"params":{"vectors":{"size":768}}}}}`,
			want: 768,
		},
		{
			name: "named vector config",
			json: `{"result":{"config":{"params":{"vectors":{"text":{"size":1024}}}}}}`,
			want: 1024,
		},
		{
			name:    "invalid json",
			json:    `{`,
			want:    0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseCollectionVectorSize(strings.NewReader(tt.json))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseCollectionVectorSize() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("parseCollectionVectorSize() = %d; want %d", got, tt.want)
			}
		})
	}
}

func TestQdrantHTTPError(t *testing.T) {
	t.Parallel()

	errWithBody := qdrantHTTPError("search", &http.Response{
		StatusCode: http.StatusBadRequest,
		Body:       io.NopCloser(strings.NewReader("bad request")),
	})
	if errWithBody == nil || !strings.Contains(errWithBody.Error(), "bad request") {
		t.Fatalf("error = %v; want body content", errWithBody)
	}

	errNoBody := qdrantHTTPError("search", &http.Response{
		StatusCode: http.StatusBadGateway,
		Body:       io.NopCloser(strings.NewReader("")),
	})
	if errNoBody == nil || !strings.Contains(errNoBody.Error(), "status: 502") {
		t.Fatalf("error = %v; want status only", errNoBody)
	}
}
