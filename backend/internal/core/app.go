package core

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
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
	Agent  *infra.AgentClient

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
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
	if cfg.EnableDemoUser {
		passwordHash, err := bcrypt.GenerateFromPassword([]byte("demo123"), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
		if err := st.EnsureDemoUser(ctx, "demo", string(passwordHash), "Demo User"); err != nil {
			return nil, err
		}
	}
	agentClient, err := infra.NewAgentClient(cfg.AgentServiceURL)
	if err != nil {
		logger.Warn("agent_service_unavailable", slog.String("error", err.Error()))
		agentClient = nil // retrieval and chunking will fail without agent service
	}
	lifecycleCtx, lifecycleCancel := context.WithCancel(context.Background())
	app := &App{
		Config: cfg,
		Logger: logger,
		DB:     db,
		Store:  st,
		Qdrant: infra.NewQdrantClient(cfg.QdrantURL),
		Ollama: infra.NewOllamaClient(cfg.OllamaURL, cfg.OllamaTimeout),
		Agent:  agentClient,
		ctx:    lifecycleCtx,
		cancel: lifecycleCancel,
	}
	return app, nil
}

func (a *App) Close() error {
	a.cancel()

	done := make(chan struct{})
	go func() {
		a.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		a.Logger.Warn("timed out waiting for document processing to finish")
	}

	if a.Agent != nil {
		_ = a.Agent.Close()
	}
	return a.DB.Close()
}

// chunkDocument delegates to Python text processor via gRPC.
func (a *App) chunkDocument(ctx context.Context, content string, chunkSize, overlap int) []string {
	if a.Agent == nil {
		a.Logger.Error("agent_service_required_for_chunking")
		return nil
	}
	chunks, err := a.Agent.ChunkDocument(ctx, content, chunkSize, overlap, true)
	if err != nil {
		a.Logger.Error("grpc_chunk_failed", slog.String("error", err.Error()))
		return nil
	}
	return chunks
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

// RetrievedChunk holds a single retrieval result from Python agent service.
type RetrievedChunk struct {
	ChunkID string
	DocID   string
	Content string
	Score   float64
	Src     string // "bm25", "vector", "fused"
}

// RetrieveChunks delegates hybrid retrieval to the Python text processor service.
func (a *App) RetrieveChunks(ctx context.Context, kbID, query string, topK int) ([]RetrievedChunk, bool, error) {
	if a.Agent == nil {
		return nil, false, fmt.Errorf("agent service not available")
	}
	if topK <= 0 {
		topK = 6
	}
	chunks, degraded, err := a.Agent.RetrieveChunks(ctx, kbID, query, topK)
	if err != nil {
		return nil, false, err
	}
	result := make([]RetrievedChunk, 0, len(chunks))
	for _, c := range chunks {
		result = append(result, RetrievedChunk{
			ChunkID: c.ChunkId,
			DocID:   c.DocId,
			Content: c.Content,
			Score:   c.Score,
			Src:     c.Source,
		})
	}
	return result, degraded, nil
}

// InvalidateBM25Index clears the BM25 cache for a KB via the Python text processor service.
func (a *App) InvalidateBM25Index(kbID string) {
	if a.Agent == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := a.Agent.InvalidateCache(ctx, kbID); err != nil {
		a.Logger.Warn("grpc_invalidate_cache_failed", slog.String("kb_id", kbID), slog.String("error", err.Error()))
	}
}

func (a *App) ProcessDocument(taskID, kbID, docID, filePath string, chunkSize, overlap int) {
	a.wg.Add(1)
	defer a.wg.Done()

	ctx, cancel := context.WithTimeout(a.ctx, 15*time.Minute)
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
	chunks := a.chunkDocument(ctx, content, chunkSize, overlap)
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
	a.InvalidateBM25Index(kbID)
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
		b.WriteString(fmt.Sprintf("[%d] (%s) %s\n", i+1, c.ChunkID, c.Content))
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
