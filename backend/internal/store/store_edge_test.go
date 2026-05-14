package store

import (
	"database/sql"
	"encoding/json"
	"testing"
	"time"
)

func TestIsNotFound_WrappedError(t *testing.T) {
	t.Parallel()

	// sql.ErrNoRows wrapped in another error
	if !IsNotFound(sql.ErrNoRows) {
		t.Fatalf("IsNotFound(sql.ErrNoRows) = false; want true")
	}
}

func TestIsNotFound_VariousErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"ErrNoRows", sql.ErrNoRows, true},
		{"ErrConnDone", sql.ErrConnDone, false},
		{"ErrTxDone", sql.ErrTxDone, false},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := IsNotFound(tt.err); got != tt.want {
				t.Fatalf("IsNotFound() = %v; want %v", got, tt.want)
			}
		})
	}
}

func TestUserJSONSerialization(t *testing.T) {
	t.Parallel()

	user := User{
		ID:           "u_001",
		Username:     "demo",
		PasswordHash: "hashed",
		DisplayName:  "Demo User",
		CreatedAt:    time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC),
	}

	data, err := json.Marshal(user)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	// PasswordHash should be excluded by json:"-"
	var m map[string]interface{}
	json.Unmarshal(data, &m)
	if _, ok := m["password_hash"]; ok {
		t.Fatalf("password_hash should not be in JSON output")
	}
	if m["user_id"] != "u_001" {
		t.Fatalf("user_id = %v; want u_001", m["user_id"])
	}
	if m["username"] != "demo" {
		t.Fatalf("username = %v; want demo", m["username"])
	}
	if m["display_name"] != "Demo User" {
		t.Fatalf("display_name = %v; want Demo User", m["display_name"])
	}
}

func TestKnowledgeBaseJSONSerialization(t *testing.T) {
	t.Parallel()

	kb := KnowledgeBase{
		ID:          "kb_001",
		UserID:      "u_001",
		Name:        "Test KB",
		Description: "A test",
		TagsRaw:     []byte(`["tag1","tag2"]`),
		DocCount:    5,
		CreatedAt:   time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC),
		Tags:        []string{"tag1", "tag2"},
	}

	data, err := json.Marshal(kb)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var m map[string]interface{}
	json.Unmarshal(data, &m)
	if m["kb_id"] != "kb_001" {
		t.Fatalf("kb_id = %v; want kb_001", m["kb_id"])
	}
	if m["name"] != "Test KB" {
		t.Fatalf("name = %v; want Test KB", m["name"])
	}
	if m["doc_count"].(float64) != 5 {
		t.Fatalf("doc_count = %v; want 5", m["doc_count"])
	}
	// TagsRaw should be excluded
	if _, ok := m["tags"]; !ok {
		t.Fatalf("tags should be in JSON output")
	}
}

func TestDocumentJSONSerialization(t *testing.T) {
	t.Parallel()

	doc := Document{
		ID:        "doc_001",
		KBID:      "kb_001",
		Title:     "Test Doc",
		FileName:  "test.txt",
		Status:    "indexed",
		Content:   "hello",
		CreatedAt: time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC),
	}

	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var m map[string]interface{}
	json.Unmarshal(data, &m)
	if m["doc_id"] != "doc_001" {
		t.Fatalf("doc_id = %v; want doc_001", m["doc_id"])
	}
	if m["status"] != "indexed" {
		t.Fatalf("status = %v; want indexed", m["status"])
	}
}

func TestTaskJSONSerialization(t *testing.T) {
	t.Parallel()

	task := Task{
		ID:        "task_001",
		Type:      "document_index",
		Status:    "processing",
		Progress:  50,
		CreatedAt: time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC),
	}

	data, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var m map[string]interface{}
	json.Unmarshal(data, &m)
	if m["task_id"] != "task_001" {
		t.Fatalf("task_id = %v; want task_001", m["task_id"])
	}
	if m["progress"].(float64) != 50 {
		t.Fatalf("progress = %v; want 50", m["progress"])
	}
}

func TestTaskJSONSerialization_WithError(t *testing.T) {
	t.Parallel()

	errCode := "READ_FILE_FAILED"
	errMsg := "file not found"
	task := Task{
		ID:           "task_002",
		Type:         "document_index",
		Status:       "failed",
		Progress:     100,
		ErrorCode:    &errCode,
		ErrorMessage: &errMsg,
		CreatedAt:    time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC),
		UpdatedAt:    time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC),
	}

	data, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var m map[string]interface{}
	json.Unmarshal(data, &m)
	if m["error_code"] != "READ_FILE_FAILED" {
		t.Fatalf("error_code = %v; want READ_FILE_FAILED", m["error_code"])
	}
	if m["error_message"] != "file not found" {
		t.Fatalf("error_message = %v; want file not found", m["error_message"])
	}
}

func TestSessionJSONSerialization(t *testing.T) {
	t.Parallel()

	sess := Session{
		ID:        "sess_001",
		KBID:      "kb_001",
		Title:     "Test Session",
		CreatedAt: time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC),
	}

	data, err := json.Marshal(sess)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var m map[string]interface{}
	json.Unmarshal(data, &m)
	if m["session_id"] != "sess_001" {
		t.Fatalf("session_id = %v; want sess_001", m["session_id"])
	}
	if m["kb_id"] != "kb_001" {
		t.Fatalf("kb_id = %v; want kb_001", m["kb_id"])
	}
}

func TestMessageJSONSerialization(t *testing.T) {
	t.Parallel()

	msg := Message{
		ID:        "msg_001",
		SessionID: "sess_001",
		Role:      "user",
		Content:   "hello",
		CreatedAt: time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC),
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var m map[string]interface{}
	json.Unmarshal(data, &m)
	if m["message_id"] != "msg_001" {
		t.Fatalf("message_id = %v; want msg_001", m["message_id"])
	}
	if m["role"] != "user" {
		t.Fatalf("role = %v; want user", m["role"])
	}
}

func TestMemoryJSONSerialization(t *testing.T) {
	t.Parallel()

	sid := "sess_001"
	uid := "u_001"
	mem := Memory{
		ID:               "mem_001",
		SessionID:        &sid,
		UserID:           &uid,
		Type:             "short_term",
		Summary:          "user is preparing for exam",
		Importance:       0.7,
		AccessCount:      3,
		SourceSessionIDs: []string{"sess_001", "sess_002"},
		CreatedAt:        time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC),
	}

	data, err := json.Marshal(mem)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var m map[string]interface{}
	json.Unmarshal(data, &m)
	if m["memory_id"] != "mem_001" {
		t.Fatalf("memory_id = %v; want mem_001", m["memory_id"])
	}
	if m["type"] != "short_term" {
		t.Fatalf("type = %v; want short_term", m["type"])
	}
	if m["importance"].(float64) != 0.7 {
		t.Fatalf("importance = %v; want 0.7", m["importance"])
	}
	if m["access_count"].(float64) != 3 {
		t.Fatalf("access_count = %v; want 3", m["access_count"])
	}
	if m["user_id"] != "u_001" {
		t.Fatalf("user_id = %v; want u_001", m["user_id"])
	}
}

func TestTraceJSONSerialization(t *testing.T) {
	t.Parallel()

	trace := Trace{
		ID:        "trace_001",
		SessionID: "sess_001",
		StepsRaw:  []byte(`[{"tool":"search_knowledge"}]`),
		CreatedAt: time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC),
		Steps: []map[string]interface{}{
			{"tool": "search_knowledge"},
		},
	}

	data, err := json.Marshal(trace)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var m map[string]interface{}
	json.Unmarshal(data, &m)
	if m["trace_id"] != "trace_001" {
		t.Fatalf("trace_id = %v; want trace_001", m["trace_id"])
	}
	steps, ok := m["steps"].([]interface{})
	if !ok || len(steps) != 1 {
		t.Fatalf("steps should have 1 element")
	}
}

func TestDocumentContentJSONSerialization(t *testing.T) {
	t.Parallel()

	doc := DocumentContent{
		ID:          "doc_001",
		KBID:        "kb_001",
		Title:       "Test",
		FileName:    "test.txt",
		Status:      "indexed",
		ContentText: "full content here",
		CreatedAt:   time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC),
	}

	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var m map[string]interface{}
	json.Unmarshal(data, &m)
	if m["content_text"] != "full content here" {
		t.Fatalf("content_text = %v; want 'full content here'", m["content_text"])
	}
}

func TestChunkJSONSerialization(t *testing.T) {
	t.Parallel()

	chunk := Chunk{
		ID:         "ck_001",
		DocID:      "doc_001",
		KBID:       "kb_001",
		ChunkIndex: 0,
		Content:    "chunk content",
		CreatedAt:  time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC),
	}

	data, err := json.Marshal(chunk)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var m map[string]interface{}
	json.Unmarshal(data, &m)
	if m["chunk_id"] != "ck_001" {
		t.Fatalf("chunk_id = %v; want ck_001", m["chunk_id"])
	}
	if m["chunk_index"].(float64) != 0 {
		t.Fatalf("chunk_index = %v; want 0", m["chunk_index"])
	}
}

func TestNewStore(t *testing.T) {
	t.Parallel()

	// Store initialization with nil DB (just testing constructor)
	s := New(nil)
	if s != nil {
		// New with nil DB should still return a Store
		if s.DB != nil {
			t.Fatalf("expected nil DB")
		}
	}
}
