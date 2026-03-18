package infra

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

type QdrantClient struct {
	BaseURL    string
	Collection string
	Client     *http.Client

	ensureMu   sync.Mutex
	ensured    bool
	ensureSize int
}

type QdrantPoint struct {
	ID      string                 `json:"id"`
	Vector  []float64              `json:"vector"`
	Payload map[string]interface{} `json:"payload"`
}

type qdrantSearchResult struct {
	ID      interface{}            `json:"id"`
	Score   float64                `json:"score"`
	Payload map[string]interface{} `json:"payload"`
}

func NewQdrantClient(baseURL, collection string) *QdrantClient {
	return &QdrantClient{
		BaseURL:    baseURL,
		Collection: collection,
		Client:     &http.Client{Timeout: 15 * time.Second},
	}
}

func (q *QdrantClient) EnsureCollection(ctx context.Context, vectorSize int) error {
	q.ensureMu.Lock()
	defer q.ensureMu.Unlock()
	if q.ensured {
		if q.ensureSize != vectorSize {
			return fmt.Errorf("qdrant collection vector size mismatch: ensured=%d requested=%d", q.ensureSize, vectorSize)
		}
		return nil
	}

	getURL := fmt.Sprintf("%s/collections/%s", q.BaseURL, q.Collection)
	getReq, err := http.NewRequestWithContext(ctx, http.MethodGet, getURL, nil)
	if err != nil {
		return err
	}
	getResp, err := q.Client.Do(getReq)
	if err != nil {
		return err
	}
	defer getResp.Body.Close()
	if getResp.StatusCode == http.StatusOK {
		size, err := parseCollectionVectorSize(getResp.Body)
		if err != nil {
			return err
		}
		if size > 0 && size != vectorSize {
			return fmt.Errorf("qdrant collection vector size mismatch: existing=%d requested=%d", size, vectorSize)
		}
		q.ensured = true
		q.ensureSize = vectorSize
		return nil
	}
	if getResp.StatusCode != http.StatusNotFound {
		return qdrantHTTPError("get collection", getResp)
	}

	payload := map[string]interface{}{
		"vectors": map[string]interface{}{
			"size":     vectorSize,
			"distance": "Cosine",
		},
	}
	body, _ := json.Marshal(payload)
	url := fmt.Sprintf("%s/collections/%s", q.BaseURL, q.Collection)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := q.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return qdrantHTTPError("ensure collection", resp)
	}
	q.ensured = true
	q.ensureSize = vectorSize
	return nil
}

func (q *QdrantClient) Upsert(ctx context.Context, points []QdrantPoint) error {
	payload := map[string]interface{}{"points": points}
	body, _ := json.Marshal(payload)
	url := fmt.Sprintf("%s/collections/%s/points?wait=true", q.BaseURL, q.Collection)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := q.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return qdrantHTTPError("upsert", resp)
	}
	return nil
}

func (q *QdrantClient) DeleteByDoc(ctx context.Context, docID string) error {
	payload := map[string]interface{}{
		"filter": map[string]interface{}{
			"must": []map[string]interface{}{
				{"key": "doc_id", "match": map[string]interface{}{"value": docID}},
			},
		},
	}
	body, _ := json.Marshal(payload)
	url := fmt.Sprintf("%s/collections/%s/points/delete?wait=true", q.BaseURL, q.Collection)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := q.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return qdrantHTTPError("delete", resp)
	}
	return nil
}

func (q *QdrantClient) SearchByKB(ctx context.Context, vector []float64, kbID string, limit int) (map[string]float64, error) {
	payload := map[string]interface{}{
		"vector":       vector,
		"limit":        limit,
		"with_payload": true,
		"filter": map[string]interface{}{
			"must": []map[string]interface{}{
				{"key": "kb_id", "match": map[string]interface{}{"value": kbID}},
			},
		},
	}
	body, _ := json.Marshal(payload)
	url := fmt.Sprintf("%s/collections/%s/points/search", q.BaseURL, q.Collection)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := q.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, qdrantHTTPError("search", resp)
	}
	var out struct {
		Result []qdrantSearchResult `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	scores := make(map[string]float64, len(out.Result))
	for _, item := range out.Result {
		chunkID, _ := item.Payload["chunk_id"].(string)
		if chunkID != "" {
			scores[chunkID] = item.Score
		}
	}
	return scores, nil
}

func (q *QdrantClient) Ready(ctx context.Context) error {
	url := fmt.Sprintf("%s/collections", q.BaseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := q.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return qdrantHTTPError("ready", resp)
	}
	return nil
}

func qdrantHTTPError(op string, resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	msg := strings.TrimSpace(string(body))
	if msg == "" {
		return fmt.Errorf("qdrant %s status: %d", op, resp.StatusCode)
	}
	return fmt.Errorf("qdrant %s status: %d, body: %s", op, resp.StatusCode, msg)
}

func parseCollectionVectorSize(r io.Reader) (int, error) {
	var out struct {
		Result struct {
			Config struct {
				Params struct {
					Vectors json.RawMessage `json:"vectors"`
				} `json:"params"`
			} `json:"config"`
		} `json:"result"`
	}
	if err := json.NewDecoder(r).Decode(&out); err != nil {
		return 0, fmt.Errorf("decode qdrant collection info failed: %w", err)
	}
	if len(out.Result.Config.Params.Vectors) == 0 {
		return 0, nil
	}

	var single struct {
		Size int `json:"size"`
	}
	if err := json.Unmarshal(out.Result.Config.Params.Vectors, &single); err == nil && single.Size > 0 {
		return single.Size, nil
	}

	var named map[string]struct {
		Size int `json:"size"`
	}
	if err := json.Unmarshal(out.Result.Config.Params.Vectors, &named); err == nil {
		for _, v := range named {
			if v.Size > 0 {
				return v.Size, nil
			}
		}
	}
	return 0, nil
}
