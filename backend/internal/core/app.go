package core

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"

	"memobase/backend/internal/config"
	"memobase/backend/internal/infra"
	"memobase/backend/internal/store"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"golang.org/x/crypto/bcrypt"
)

type App struct {
	Config config.Config
	Logger *slog.Logger
	DB     *sqlx.DB
	Store  *store.Store
	Qdrant *infra.QdrantClient
	Ollama *infra.OllamaClient
}

func New(cfg config.Config, logger *slog.Logger) (*App, error) {
	db, err := infra.NewDB(cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := infra.InitSchema(ctx, db); err != nil {
		return nil, err
	}
	st := store.New(db)
	passwordHash, _ := bcrypt.GenerateFromPassword([]byte("demo123"), bcrypt.DefaultCost)
	if err := st.EnsureDemoUser(ctx, "demo", string(passwordHash), "Demo User"); err != nil {
		return nil, err
	}
	app := &App{
		Config: cfg,
		Logger: logger,
		DB:     db,
		Store:  st,
		Qdrant: infra.NewQdrantClient(cfg.QdrantURL),
		Ollama: infra.NewOllamaClient(cfg.OllamaURL, cfg.OllamaTimeout),
	}
	return app, nil
}

func (a *App) Close() error {
	return a.DB.Close()
}

func (a *App) VerifyUser(ctx context.Context, username, password string) (store.User, error) {
	user, err := a.Store.GetUserByUsername(ctx, username)
	if err != nil {
		return user, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return user, fmt.Errorf("invalid password")
	}
	return user, nil
}

var qdrantCollectionPartRe = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

func sanitizeQdrantCollectionPart(part string) string {
	part = strings.TrimSpace(part)
	part = qdrantCollectionPartRe.ReplaceAllString(part, "_")
	part = strings.Trim(part, "_")
	if part == "" {
		return "default"
	}
	return part
}

func (a *App) QdrantCollectionForKB(kbID string) string {
	prefix := sanitizeQdrantCollectionPart(a.Config.QdrantCollection)
	kbPart := sanitizeQdrantCollectionPart(kbID)
	return prefix + "__" + kbPart
}

func splitIntoChunks(text string, chunkSize, overlap int) []string {
	if chunkSize <= 0 {
		chunkSize = 500
	}
	if overlap < 0 {
		overlap = 0
	}
	if overlap >= chunkSize {
		overlap = chunkSize / 5
	}
	runes := []rune(text)
	if len(runes) == 0 {
		return []string{}
	}
	chunks := make([]string, 0)
	step := chunkSize - overlap
	for start := 0; start < len(runes); start += step {
		end := start + chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		chunk := strings.TrimSpace(string(runes[start:end]))
		if chunk != "" {
			chunks = append(chunks, chunk)
		}
		if end == len(runes) {
			break
		}
	}
	return chunks
}

var tokenRe = regexp.MustCompile(`[\p{L}\p{N}]+`)

func tokenize(text string) []string {
	lower := strings.ToLower(text)
	parts := tokenRe.FindAllString(lower, -1)
	if len(parts) == 0 {
		return []string{}
	}
	return parts
}

func bm25LikeScore(query, doc string) float64 {
	qTokens := tokenize(query)
	if len(qTokens) == 0 {
		return 0
	}
	docTokens := tokenize(doc)
	if len(docTokens) == 0 {
		return 0
	}
	freq := map[string]int{}
	for _, t := range docTokens {
		freq[t]++
	}
	score := 0.0
	for _, qt := range qTokens {
		if n, ok := freq[qt]; ok {
			score += math.Log(1 + float64(n))
		}
	}
	return score / math.Sqrt(float64(len(docTokens))+1)
}

func normalizeScores(scores map[string]float64) map[string]float64 {
	if len(scores) == 0 {
		return map[string]float64{}
	}
	minV, maxV := math.MaxFloat64, -math.MaxFloat64
	for _, v := range scores {
		if v < minV {
			minV = v
		}
		if v > maxV {
			maxV = v
		}
	}
	out := make(map[string]float64, len(scores))
	if math.Abs(maxV-minV) < 1e-9 {
		for k := range scores {
			out[k] = 1
		}
		return out
	}
	for k, v := range scores {
		out[k] = (v - minV) / (maxV - minV)
	}
	return out
}

type RetrievedChunk struct {
	Chunk store.Chunk
	Score float64
	Src   string
}

func (a *App) RetrieveChunks(ctx context.Context, kbID, query string, topK int) ([]RetrievedChunk, bool, error) {
	if topK <= 0 {
		topK = 6
	}
	limit := a.Config.RetrieveLimit
	if limit <= 0 {
		limit = 500
	}
	chunks, err := a.Store.GetChunksByKB(ctx, kbID, limit)
	if err != nil {
		return nil, false, err
	}
	bm25Raw := map[string]float64{}
	for _, c := range chunks {
		bm25Raw[c.ID] = bm25LikeScore(query, c.Content)
	}
	bm25Norm := normalizeScores(bm25Raw)
	vectorNorm := map[string]float64{}
	degraded := false
	if emb, err := a.Ollama.Embed(ctx, a.Config.OllamaEmbedModel, query); err == nil {
		collection := a.QdrantCollectionForKB(kbID)
		if vectorRaw, err := a.Qdrant.Search(ctx, collection, emb, topK*3); err == nil {
			vectorNorm = normalizeScores(vectorRaw)
		} else {
			degraded = true
		}
	} else {
		degraded = true
	}
	type scored struct {
		chunk store.Chunk
		score float64
		src   string
	}
	all := make([]scored, 0, len(chunks))
	for _, c := range chunks {
		bs := bm25Norm[c.ID]
		vs := vectorNorm[c.ID]
		fs := a.Config.BM25Weight*bs + a.Config.VectorWeight*vs
		src := "fused"
		if vs == 0 && bs > 0 {
			src = "bm25"
		}
		if bs == 0 && vs > 0 {
			src = "vector"
		}
		if fs > 0 {
			all = append(all, scored{chunk: c, score: fs, src: src})
		}
	}
	sort.Slice(all, func(i, j int) bool { return all[i].score > all[j].score })
	if len(all) > topK {
		all = all[:topK]
	}
	result := make([]RetrievedChunk, 0, len(all))
	for _, x := range all {
		result = append(result, RetrievedChunk{Chunk: x.chunk, Score: x.score, Src: x.src})
	}
	return result, degraded, nil
}

func (a *App) ProcessDocument(taskID, kbID, docID, filePath string, chunkSize, overlap int) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	_ = a.Store.UpdateTask(ctx, taskID, "processing", 10, nil, nil)
	_ = a.Store.UpdateDocumentStatus(ctx, docID, "processing")
	content, err := readTextFile(filePath)
	if err != nil {
		a.failTask(ctx, taskID, docID, "READ_FILE_FAILED", err.Error())
		return
	}
	content = strings.TrimSpace(content)
	if content == "" {
		a.failTask(ctx, taskID, docID, "EMPTY_DOCUMENT", "document content is empty")
		return
	}
	_ = a.Store.UpdateDocumentContent(ctx, docID, content)
	chunks := splitIntoChunks(content, chunkSize, overlap)
	dbChunks := make([]store.Chunk, 0, len(chunks))
	for i, chunk := range chunks {
		dbChunks = append(dbChunks, store.Chunk{
			ID:         "ck_" + uuid.NewString(),
			DocID:      docID,
			KBID:       kbID,
			ChunkIndex: i,
			Content:    chunk,
		})
	}
	if err := a.Store.ReplaceChunks(ctx, docID, dbChunks); err != nil {
		a.failTask(ctx, taskID, docID, "DB_CHUNK_WRITE_FAILED", err.Error())
		return
	}
	_ = a.Store.UpdateTask(ctx, taskID, "processing", 60, nil, nil)
	points := make([]infra.QdrantPoint, 0, len(dbChunks))
	collection := a.QdrantCollectionForKB(kbID)
	collectionEnsured := false
	expectedDim := 0
	for _, c := range dbChunks {
		emb, err := a.Ollama.Embed(ctx, a.Config.OllamaEmbedModel, c.Content)
		if err != nil {
			a.failTask(ctx, taskID, docID, "OLLAMA_EMBED_FAILED", err.Error())
			return
		}
		if !collectionEnsured {
			if err := a.Qdrant.EnsureCollection(ctx, collection, len(emb)); err != nil {
				a.failTask(ctx, taskID, docID, "QDRANT_COLLECTION_FAILED", err.Error())
				return
			}
			collectionEnsured = true
			expectedDim = len(emb)
		}
		if len(emb) != expectedDim {
			a.failTask(ctx, taskID, docID, "EMBEDDING_DIMENSION_MISMATCH", fmt.Sprintf("embedding dim changed from %d to %d", expectedDim, len(emb)))
			return
		}
		points = append(points, infra.QdrantPoint{
			ID:     qdrantPointID(c.ID),
			Vector: emb,
			Payload: map[string]interface{}{
				"kb_id":       kbID,
				"doc_id":      docID,
				"chunk_id":    c.ID,
				"chunk_index": c.ChunkIndex,
				"source":      "document",
				"created_at":  time.Now().UTC().Format(time.RFC3339),
			},
		})
	}
	if err := a.Qdrant.DeleteByDoc(ctx, collection, docID); err != nil {
		a.Logger.Warn("qdrant_delete_old_points_failed",
			slog.String("doc_id", docID),
			slog.String("error", err.Error()),
		)
	}
	if err := a.Qdrant.Upsert(ctx, collection, points); err != nil {
		a.failTask(ctx, taskID, docID, "QDRANT_UPSERT_FAILED", err.Error())
		return
	}
	_ = a.Store.UpdateDocumentStatus(ctx, docID, "indexed")
	_ = a.Store.UpdateTask(ctx, taskID, "succeeded", 100, nil, nil)
}

func qdrantPointID(chunkID string) string {
	// Qdrant point IDs must be uint64 or UUID-like strings.
	if _, err := uuid.Parse(chunkID); err == nil {
		return chunkID
	}
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte("chunk:"+chunkID)).String()
}

func (a *App) failTask(ctx context.Context, taskID, docID, code, msg string) {
	if err := a.Store.DeleteChunksByDoc(ctx, docID); err != nil {
		a.Logger.Warn("delete_chunks_failed",
			slog.String("doc_id", docID),
			slog.String("error", err.Error()),
		)
	}
	_ = a.Store.UpdateDocumentStatus(ctx, docID, "failed")
	_ = a.Store.UpdateTask(ctx, taskID, "failed", 100, &code, &msg)
	a.Logger.Error("document_process_failed",
		slog.String("task_id", taskID),
		slog.String("doc_id", docID),
		slog.String("error_code", code),
		slog.String("error", msg),
	)
}

func (a *App) BuildChatPrompt(question string, chunks []RetrievedChunk, memories []store.Memory) string {
	var b strings.Builder
	b.WriteString("你是知识库助手。请基于给定上下文回答问题，并保持简洁。\n\n")
	b.WriteString("[上下文片段]\n")
	for i, c := range chunks {
		b.WriteString(fmt.Sprintf("[%d] (%s) %s\n", i+1, c.Chunk.ID, c.Chunk.Content))
	}
	if len(memories) > 0 {
		b.WriteString("\n[记忆]\n")
		for _, m := range memories {
			b.WriteString("- " + m.Summary + "\n")
		}
	}
	b.WriteString("\n[用户问题]\n")
	b.WriteString(question)
	return b.String()
}

func summarize(text string, maxRunes int) string {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) <= maxRunes {
		return string(runes)
	}
	if maxRunes <= 3 {
		return string(runes[:maxRunes])
	}
	return string(runes[:maxRunes-3]) + "..."
}
