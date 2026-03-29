package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type Store struct {
	DB *sqlx.DB
}

func New(db *sqlx.DB) *Store {
	return &Store{DB: db}
}

type User struct {
	ID           string    `db:"id" json:"user_id"`
	Username     string    `db:"username" json:"username"`
	PasswordHash string    `db:"password_hash" json:"-"`
	DisplayName  string    `db:"display_name" json:"display_name"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
}

type KnowledgeBase struct {
	ID          string    `db:"id" json:"kb_id"`
	UserID      string    `db:"user_id" json:"user_id"`
	Name        string    `db:"name" json:"name"`
	Description string    `db:"description" json:"description"`
	TagsRaw     []byte    `db:"tags" json:"-"`
	DocCount    int       `db:"doc_count" json:"doc_count"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
	Tags        []string  `json:"tags"`
}

type Document struct {
	ID        string    `db:"id" json:"doc_id"`
	KBID      string    `db:"kb_id" json:"kb_id"`
	Title     string    `db:"title" json:"title"`
	FileName  string    `db:"file_name" json:"file_name"`
	Status    string    `db:"status" json:"status"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

type DocumentContent struct {
	ID          string    `db:"id" json:"doc_id"`
	KBID        string    `db:"kb_id" json:"kb_id"`
	Title       string    `db:"title" json:"title"`
	FileName    string    `db:"file_name" json:"file_name"`
	Status      string    `db:"status" json:"status"`
	ContentText string    `db:"content_text" json:"content_text"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
}

type Chunk struct {
	ID         string    `db:"id" json:"chunk_id"`
	DocID      string    `db:"doc_id" json:"doc_id"`
	KBID       string    `db:"kb_id" json:"kb_id"`
	ChunkIndex int       `db:"chunk_index" json:"chunk_index"`
	Content    string    `db:"content" json:"content"`
	CreatedAt  time.Time `db:"created_at" json:"created_at"`
}

type Task struct {
	ID           string                 `db:"id" json:"task_id"`
	Type         string                 `db:"type" json:"type"`
	Status       string                 `db:"status" json:"status"`
	Progress     int                    `db:"progress" json:"progress"`
	ErrorCode    *string                `db:"error_code" json:"error_code"`
	ErrorMessage *string                `db:"error_message" json:"error_message"`
	MetaRaw      []byte                 `db:"meta" json:"-"`
	CreatedAt    time.Time              `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time              `db:"updated_at" json:"updated_at"`
	Meta         map[string]interface{} `json:"meta,omitempty"`
}

type Session struct {
	ID        string    `db:"id" json:"session_id"`
	KBID      string    `db:"kb_id" json:"kb_id"`
	Title     string    `db:"title" json:"title"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

type Message struct {
	ID        string    `db:"id" json:"message_id"`
	SessionID string    `db:"session_id" json:"session_id"`
	Role      string    `db:"role" json:"role"`
	Content   string    `db:"content" json:"content"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

type Memory struct {
	ID        string    `db:"id" json:"memory_id"`
	SessionID string    `db:"session_id" json:"session_id"`
	Type      string    `db:"type" json:"type"`
	Summary   string    `db:"summary" json:"summary"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

type Trace struct {
	ID        string                   `db:"id" json:"trace_id"`
	SessionID string                   `db:"session_id" json:"session_id"`
	StepsRaw  []byte                   `db:"steps" json:"-"`
	CreatedAt time.Time                `db:"created_at" json:"created_at"`
	Steps     []map[string]interface{} `json:"steps"`
}

func (s *Store) EnsureDemoUser(ctx context.Context, username, passwordHash, displayName string) error {
	var count int
	if err := s.DB.GetContext(ctx, &count, `SELECT COUNT(1) FROM users WHERE username=$1`, username); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO users (id, username, password_hash, display_name)
		VALUES ($1,$2,$3,$4)
	`, "u_demo", username, passwordHash, displayName)
	return err
}

func (s *Store) GetUserByUsername(ctx context.Context, username string) (User, error) {
	var user User
	err := s.DB.GetContext(ctx, &user, `SELECT id, username, password_hash, display_name, created_at FROM users WHERE username=$1`, username)
	return user, err
}

func (s *Store) GetUserByID(ctx context.Context, id string) (User, error) {
	var user User
	err := s.DB.GetContext(ctx, &user, `SELECT id, username, password_hash, display_name, created_at FROM users WHERE id=$1`, id)
	return user, err
}

func (s *Store) CreateKB(ctx context.Context, userID, name, description string, tags []string) (KnowledgeBase, error) {
	tagsRaw, _ := json.Marshal(tags)
	kb := KnowledgeBase{ID: "kb_" + uuid.NewString(), UserID: userID, Name: name, Description: description, Tags: tags}
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO knowledge_bases (id, user_id, name, description, tags)
		VALUES ($1,$2,$3,$4,$5)
	`, kb.ID, kb.UserID, kb.Name, kb.Description, tagsRaw)
	if err != nil {
		return kb, err
	}
	return s.GetKB(ctx, kb.ID)
}

func (s *Store) ListKB(ctx context.Context, userID string, limit, offset int, keyword string) ([]KnowledgeBase, int, error) {
	rows := []KnowledgeBase{}
	like := "%" + keyword + "%"
	if err := s.DB.SelectContext(ctx, &rows, `
		SELECT kb.id, kb.user_id, kb.name, kb.description, kb.tags,
			COALESCE(d.doc_count,0) AS doc_count, kb.created_at, kb.updated_at
		FROM knowledge_bases kb
		LEFT JOIN (
			SELECT kb_id, COUNT(1) AS doc_count FROM documents WHERE deleted_at IS NULL GROUP BY kb_id
		) d ON d.kb_id = kb.id
		WHERE kb.user_id=$1 AND kb.deleted_at IS NULL AND (kb.name ILIKE $2 OR kb.description ILIKE $2)
		ORDER BY kb.created_at DESC
		LIMIT $3 OFFSET $4
	`, userID, like, limit, offset); err != nil {
		return nil, 0, err
	}
	var total int
	if err := s.DB.GetContext(ctx, &total, `
		SELECT COUNT(1) FROM knowledge_bases
		WHERE user_id=$1 AND deleted_at IS NULL AND (name ILIKE $2 OR description ILIKE $2)
	`, userID, like); err != nil {
		return nil, 0, err
	}
	for i := range rows {
		_ = json.Unmarshal(rows[i].TagsRaw, &rows[i].Tags)
	}
	return rows, total, nil
}

func (s *Store) GetKB(ctx context.Context, kbID string) (KnowledgeBase, error) {
	var kb KnowledgeBase
	err := s.DB.GetContext(ctx, &kb, `
		SELECT kb.id, kb.user_id, kb.name, kb.description, kb.tags,
			COALESCE(d.doc_count,0) AS doc_count, kb.created_at, kb.updated_at
		FROM knowledge_bases kb
		LEFT JOIN (
			SELECT kb_id, COUNT(1) AS doc_count FROM documents WHERE deleted_at IS NULL GROUP BY kb_id
		) d ON d.kb_id = kb.id
		WHERE kb.id=$1 AND kb.deleted_at IS NULL
	`, kbID)
	if err != nil {
		return kb, err
	}
	_ = json.Unmarshal(kb.TagsRaw, &kb.Tags)
	return kb, nil
}

func (s *Store) PatchKB(ctx context.Context, kbID string, name, description *string, tags *[]string) (KnowledgeBase, error) {
	sets := make([]string, 0, 4)
	args := make([]interface{}, 0, 4)
	args = append(args, kbID)
	argPos := 2

	if name != nil {
		sets = append(sets, fmt.Sprintf("name=$%d", argPos))
		args = append(args, *name)
		argPos++
	}
	if description != nil {
		sets = append(sets, fmt.Sprintf("description=$%d", argPos))
		args = append(args, *description)
		argPos++
	}
	if tags != nil {
		tagsRaw, _ := json.Marshal(*tags)
		sets = append(sets, fmt.Sprintf("tags=$%d", argPos))
		args = append(args, tagsRaw)
		argPos++
	}
	if len(sets) == 0 {
		return s.GetKB(ctx, kbID)
	}
	sets = append(sets, "updated_at=NOW()")

	query := fmt.Sprintf(`
		UPDATE knowledge_bases
		SET %s
		WHERE id=$1 AND deleted_at IS NULL
	`, strings.Join(sets, ", "))
	res, err := s.DB.ExecContext(ctx, query, args...)
	if err != nil {
		return KnowledgeBase{}, err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return KnowledgeBase{}, sql.ErrNoRows
	}
	return s.GetKB(ctx, kbID)
}

func (s *Store) DeleteKB(ctx context.Context, kbID string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE knowledge_bases SET deleted_at=NOW() WHERE id=$1`, kbID)
	return err
}

func (s *Store) CreateDocument(ctx context.Context, kbID, title, fileName string) (Document, error) {
	doc := Document{ID: "doc_" + uuid.NewString(), KBID: kbID, Title: title, FileName: fileName, Status: "pending"}
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO documents (id, kb_id, title, file_name, status)
		VALUES ($1,$2,$3,$4,$5)
	`, doc.ID, doc.KBID, doc.Title, doc.FileName, doc.Status)
	if err != nil {
		return doc, err
	}
	return s.GetDocument(ctx, kbID, doc.ID)
}

func (s *Store) UpdateDocumentStatus(ctx context.Context, docID, status string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE documents SET status=$2, updated_at=NOW() WHERE id=$1`, docID, status)
	return err
}

func (s *Store) UpdateDocumentContent(ctx context.Context, docID, content string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE documents SET content_text=$2, updated_at=NOW() WHERE id=$1`, docID, content)
	return err
}

func (s *Store) GetDocument(ctx context.Context, kbID, docID string) (Document, error) {
	var doc Document
	err := s.DB.GetContext(ctx, &doc, `
		SELECT id, kb_id, title, file_name, status, created_at, updated_at
		FROM documents WHERE id=$1 AND kb_id=$2 AND deleted_at IS NULL
	`, docID, kbID)
	return doc, err
}

func (s *Store) GetDocumentContent(ctx context.Context, kbID, docID string) (DocumentContent, error) {
	var doc DocumentContent
	err := s.DB.GetContext(ctx, &doc, `
		SELECT id, kb_id, title, file_name, status, content_text, created_at, updated_at
		FROM documents WHERE id=$1 AND kb_id=$2 AND deleted_at IS NULL
	`, docID, kbID)
	return doc, err
}

func (s *Store) ListDocuments(ctx context.Context, kbID, status string, limit, offset int) ([]Document, int, error) {
	docs := []Document{}
	if status == "" {
		if err := s.DB.SelectContext(ctx, &docs, `
			SELECT id, kb_id, title, file_name, status, created_at, updated_at
			FROM documents WHERE kb_id=$1 AND deleted_at IS NULL
			ORDER BY created_at DESC LIMIT $2 OFFSET $3
		`, kbID, limit, offset); err != nil {
			return nil, 0, err
		}
	} else {
		if err := s.DB.SelectContext(ctx, &docs, `
			SELECT id, kb_id, title, file_name, status, created_at, updated_at
			FROM documents WHERE kb_id=$1 AND status=$2 AND deleted_at IS NULL
			ORDER BY created_at DESC LIMIT $3 OFFSET $4
		`, kbID, status, limit, offset); err != nil {
			return nil, 0, err
		}
	}
	var total int
	if status == "" {
		err := s.DB.GetContext(ctx, &total, `SELECT COUNT(1) FROM documents WHERE kb_id=$1 AND deleted_at IS NULL`, kbID)
		if err != nil {
			return nil, 0, err
		}
	} else {
		err := s.DB.GetContext(ctx, &total, `SELECT COUNT(1) FROM documents WHERE kb_id=$1 AND status=$2 AND deleted_at IS NULL`, kbID, status)
		if err != nil {
			return nil, 0, err
		}
	}
	return docs, total, nil
}

func (s *Store) DeleteDocument(ctx context.Context, kbID, docID string) error {
	tx, err := s.DB.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM document_chunks WHERE doc_id=$1`, docID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE documents SET deleted_at=NOW(), status='deleted', updated_at=NOW() WHERE id=$1 AND kb_id=$2`, docID, kbID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) ReplaceChunks(ctx context.Context, docID string, chunks []Chunk) error {
	tx, err := s.DB.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM document_chunks WHERE doc_id=$1`, docID); err != nil {
		return err
	}
	for _, c := range chunks {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO document_chunks (id, doc_id, kb_id, chunk_index, content)
			VALUES ($1,$2,$3,$4,$5)
		`, c.ID, c.DocID, c.KBID, c.ChunkIndex, c.Content)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) DeleteChunksByDoc(ctx context.Context, docID string) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM document_chunks WHERE doc_id=$1`, docID)
	return err
}

func (s *Store) GetChunksByKB(ctx context.Context, kbID string, limit int) ([]Chunk, error) {
	chunks := []Chunk{}
	err := s.DB.SelectContext(ctx, &chunks, `
		SELECT c.id, c.doc_id, c.kb_id, c.chunk_index, c.content, c.created_at
		FROM document_chunks c
		INNER JOIN documents d ON d.id = c.doc_id
		WHERE c.kb_id=$1 AND d.status='indexed' AND d.deleted_at IS NULL
		ORDER BY c.created_at DESC
		LIMIT $2
	`, kbID, limit)
	return chunks, err
}

func (s *Store) CreateTask(ctx context.Context, typ string, meta map[string]interface{}) (Task, error) {
	metaRaw, _ := json.Marshal(meta)
	task := Task{ID: "task_" + uuid.NewString(), Type: typ, Status: "pending", Progress: 0, Meta: meta}
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO tasks (id, type, status, progress, meta)
		VALUES ($1,$2,$3,$4,$5)
	`, task.ID, task.Type, task.Status, task.Progress, metaRaw)
	if err != nil {
		return task, err
	}
	return s.GetTask(ctx, task.ID)
}

func (s *Store) UpdateTask(ctx context.Context, taskID, status string, progress int, errorCode, errorMessage *string) error {
	_, err := s.DB.ExecContext(ctx, `
		UPDATE tasks
		SET status=$2, progress=$3, error_code=$4, error_message=$5, updated_at=NOW()
		WHERE id=$1
	`, taskID, status, progress, errorCode, errorMessage)
	return err
}

func (s *Store) GetTask(ctx context.Context, taskID string) (Task, error) {
	var task Task
	err := s.DB.GetContext(ctx, &task, `
		SELECT id, type, status, progress, error_code, error_message, meta, created_at, updated_at
		FROM tasks WHERE id=$1
	`, taskID)
	if err != nil {
		return task, err
	}
	if len(task.MetaRaw) > 0 {
		_ = json.Unmarshal(task.MetaRaw, &task.Meta)
	}
	return task, nil
}

func (s *Store) CreateSession(ctx context.Context, kbID, title string) (Session, error) {
	sess := Session{ID: "sess_" + uuid.NewString(), KBID: kbID, Title: title}
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO sessions (id, kb_id, title)
		VALUES ($1,$2,$3)
	`, sess.ID, sess.KBID, sess.Title)
	if err != nil {
		return sess, err
	}
	return s.GetSession(ctx, sess.ID)
}

func (s *Store) GetSession(ctx context.Context, sessionID string) (Session, error) {
	var sess Session
	err := s.DB.GetContext(ctx, &sess, `
		SELECT id, kb_id, title, created_at, updated_at
		FROM sessions WHERE id=$1 AND deleted_at IS NULL
	`, sessionID)
	return sess, err
}

func (s *Store) ListSessions(ctx context.Context, kbID string, limit, offset int) ([]Session, int, error) {
	sessions := []Session{}
	if kbID == "" {
		if err := s.DB.SelectContext(ctx, &sessions, `
			SELECT id, kb_id, title, created_at, updated_at FROM sessions
			WHERE deleted_at IS NULL ORDER BY updated_at DESC LIMIT $1 OFFSET $2
		`, limit, offset); err != nil {
			return nil, 0, err
		}
	} else {
		if err := s.DB.SelectContext(ctx, &sessions, `
			SELECT id, kb_id, title, created_at, updated_at FROM sessions
			WHERE kb_id=$1 AND deleted_at IS NULL ORDER BY updated_at DESC LIMIT $2 OFFSET $3
		`, kbID, limit, offset); err != nil {
			return nil, 0, err
		}
	}
	var total int
	if kbID == "" {
		err := s.DB.GetContext(ctx, &total, `SELECT COUNT(1) FROM sessions WHERE deleted_at IS NULL`)
		if err != nil {
			return nil, 0, err
		}
	} else {
		err := s.DB.GetContext(ctx, &total, `SELECT COUNT(1) FROM sessions WHERE kb_id=$1 AND deleted_at IS NULL`, kbID)
		if err != nil {
			return nil, 0, err
		}
	}
	return sessions, total, nil
}

func (s *Store) DeleteSession(ctx context.Context, sessionID string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE sessions SET deleted_at=NOW(), updated_at=NOW() WHERE id=$1`, sessionID)
	return err
}

func (s *Store) CreateMessage(ctx context.Context, sessionID, role, content string) (Message, error) {
	msg := Message{ID: "msg_" + uuid.NewString(), SessionID: sessionID, Role: role, Content: content}
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO messages (id, session_id, role, content)
		VALUES ($1,$2,$3,$4)
	`, msg.ID, msg.SessionID, msg.Role, msg.Content)
	if err != nil {
		return msg, err
	}
	_ = s.touchSession(ctx, sessionID)
	return msg, s.DB.GetContext(ctx, &msg, `
		SELECT id, session_id, role, content, created_at FROM messages WHERE id=$1
	`, msg.ID)
}

func (s *Store) ListMessages(ctx context.Context, sessionID string, limit, offset int) ([]Message, int, error) {
	messages := []Message{}
	if err := s.DB.SelectContext(ctx, &messages, `
		SELECT id, session_id, role, content, created_at
		FROM messages WHERE session_id=$1
		ORDER BY created_at ASC LIMIT $2 OFFSET $3
	`, sessionID, limit, offset); err != nil {
		return nil, 0, err
	}
	var total int
	if err := s.DB.GetContext(ctx, &total, `SELECT COUNT(1) FROM messages WHERE session_id=$1`, sessionID); err != nil {
		return nil, 0, err
	}
	return messages, total, nil
}

func (s *Store) CreateMemory(ctx context.Context, sessionID, typ, summary string) (Memory, error) {
	mem := Memory{ID: "mem_" + uuid.NewString(), SessionID: sessionID, Type: typ, Summary: summary}
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO memories (id, session_id, type, summary)
		VALUES ($1,$2,$3,$4)
	`, mem.ID, mem.SessionID, mem.Type, mem.Summary)
	if err != nil {
		return mem, err
	}
	return mem, s.DB.GetContext(ctx, &mem, `
		SELECT id, session_id, type, summary, created_at FROM memories WHERE id=$1
	`, mem.ID)
}

func (s *Store) ListSessionMemories(ctx context.Context, sessionID string, limit int) ([]Memory, error) {
	memories := []Memory{}
	err := s.DB.SelectContext(ctx, &memories, `
		SELECT id, session_id, type, summary, created_at
		FROM memories WHERE session_id=$1 ORDER BY created_at DESC LIMIT $2
	`, sessionID, limit)
	return memories, err
}

func (s *Store) CreateTrace(ctx context.Context, sessionID string, steps []map[string]interface{}) (Trace, error) {
	stepsRaw, _ := json.Marshal(steps)
	trace := Trace{ID: "trace_" + uuid.NewString(), SessionID: sessionID, Steps: steps}
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO traces (id, session_id, steps)
		VALUES ($1,$2,$3)
	`, trace.ID, trace.SessionID, stepsRaw)
	if err != nil {
		return trace, err
	}
	return s.GetTrace(ctx, trace.ID)
}

func (s *Store) GetTrace(ctx context.Context, traceID string) (Trace, error) {
	var trace Trace
	err := s.DB.GetContext(ctx, &trace, `
		SELECT id, session_id, steps, created_at FROM traces WHERE id=$1
	`, traceID)
	if err != nil {
		return trace, err
	}
	_ = json.Unmarshal(trace.StepsRaw, &trace.Steps)
	return trace, nil
}

func (s *Store) touchSession(ctx context.Context, sessionID string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE sessions SET updated_at=NOW() WHERE id=$1`, sessionID)
	return err
}

func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, sql.ErrNoRows)
}
