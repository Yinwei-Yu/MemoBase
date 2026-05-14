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

	pb "memobase/backend/proto"

	"github.com/jmoiron/sqlx"
	"golang.org/x/crypto/bcrypt"
)

type App struct {
	Config   config.Config
	Logger   *slog.Logger
	DB       *sqlx.DB
	Store    *store.Store
	Agent    *infra.AgentClient
	Provider *infra.ProviderClient

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
		Config:   cfg,
		Logger:   logger,
		DB:       db,
		Store:    st,
		Agent:    agentClient,
		Provider: infra.NewProviderClient(),
		ctx:      lifecycleCtx,
		cancel:   lifecycleCancel,
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

	if a.Agent == nil {
		a.failTask(ctx, taskID, docID, "AGENT_UNAVAILABLE", "agent service required for indexing")
		return
	}

	collection := a.QdrantCollectionForKB(kbID)
	_ = a.Store.UpdateTask(ctx, taskID, "processing", 30, nil, nil)

	resp, err := a.Agent.IndexDocument(ctx, &pb.IndexDocumentRequest{
		KbId:           kbID,
		DocId:          docID,
		Content:        content,
		ChunkSize:      int32(chunkSize),
		Overlap:        int32(overlap),
		CollectionName: collection,
	})
	if err != nil {
		a.failTask(ctx, taskID, docID, "INDEX_DOCUMENT_FAILED", err.Error())
		return
	}
	if !resp.Success {
		a.failTask(ctx, taskID, docID, "INDEX_DOCUMENT_FAILED", resp.ErrorMessage)
		return
	}

	// Store chunk records to PostgreSQL
	dbChunks := make([]store.Chunk, 0, resp.ChunkCount)
	for i, cid := range resp.ChunkIds {
		dbChunks = append(dbChunks, store.Chunk{
			ID:         cid,
			DocID:      docID,
			KBID:       kbID,
			ChunkIndex: i,
		})
	}
	if err := a.Store.ReplaceChunks(ctx, docID, dbChunks); err != nil {
		a.failTask(ctx, taskID, docID, "DB_CHUNK_WRITE_FAILED", err.Error())
		return
	}

	_ = a.Store.UpdateDocumentStatus(ctx, docID, "indexed")
	_ = a.Store.UpdateTask(ctx, taskID, "succeeded", 100, nil, nil)
	a.InvalidateBM25Index(kbID)
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
