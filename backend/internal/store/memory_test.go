package store

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestMemoryStructJSON_FullFields(t *testing.T) {
	t.Parallel()

	sid := "sess_123"
	uid := "u_456"
	eid := "qdrant_point_789"
	lastAccess := time.Date(2026, 5, 14, 11, 0, 0, 0, time.UTC)

	mem := Memory{
		ID:               "mem_test_001",
		SessionID:        &sid,
		UserID:           &uid,
		Type:             "long_term",
		Summary:          "用户熟悉Go语言",
		Importance:       0.85,
		AccessCount:      12,
		LastAccessedAt:   &lastAccess,
		EmbeddingID:      &eid,
		SourceSessionIDs: []string{"sess_001", "sess_002", "sess_003"},
		ExpiresAt:        nil,
		CreatedAt:        time.Date(2026, 5, 14, 10, 0, 0, 0, time.UTC),
	}

	raw, err := json.Marshal(mem)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	assertStr := func(key, expected string) {
		t.Helper()
		v, ok := data[key]
		if !ok {
			t.Errorf("missing key %q", key)
			return
		}
		if v != expected {
			t.Errorf("%s = %v; want %s", key, v, expected)
		}
	}

	assertStr("memory_id", "mem_test_001")
	assertStr("session_id", "sess_123")
	assertStr("user_id", "u_456")
	assertStr("type", "long_term")
	assertStr("summary", "用户熟悉Go语言")
	assertStr("embedding_id", "qdrant_point_789")

	if v, ok := data["importance"]; !ok || v.(float64) != 0.85 {
		t.Errorf("importance = %v; want 0.85", data["importance"])
	}
	if v, ok := data["access_count"]; !ok || v.(float64) != 12 {
		t.Errorf("access_count = %v; want 12", data["access_count"])
	}
}

func TestMemoryStructJSON_UserLevelNoSession(t *testing.T) {
	t.Parallel()

	uid := "u_456"
	mem := Memory{
		ID:         "mem_test_002",
		UserID:     &uid,
		Type:       "fact",
		Summary:    "用户公司用K8s部署",
		Importance: 0.9,
		CreatedAt:  time.Now(),
	}

	raw, err := json.Marshal(mem)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var data map[string]interface{}
	json.Unmarshal(raw, &data)

	if data["session_id"] != nil {
		t.Errorf("session_id should be nil for user-level memory, got %v", data["session_id"])
	}
	if data["user_id"] != "u_456" {
		t.Errorf("user_id = %v; want u_456", data["user_id"])
	}
}

func TestMemoryStructJSON_SessionLevelNoUser(t *testing.T) {
	t.Parallel()

	sid := "sess_789"
	mem := Memory{
		ID:         "mem_test_003",
		SessionID:  &sid,
		Type:       "short_term",
		Summary:    "Q: 你好 | A: 你好！",
		Importance: 0.5,
		CreatedAt:  time.Now(),
	}

	raw, err := json.Marshal(mem)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var data map[string]interface{}
	json.Unmarshal(raw, &data)

	if data["session_id"] != "sess_789" {
		t.Errorf("session_id = %v; want sess_789", data["session_id"])
	}
	if data["user_id"] != nil {
		t.Errorf("user_id should be nil, got %v", data["user_id"])
	}
}

func TestMemoryStructJSON_EmptySourceSessionIDs(t *testing.T) {
	t.Parallel()

	mem := Memory{
		ID:               "mem_test_004",
		Type:             "compressed",
		Summary:          "compressed summary",
		SourceSessionIDs: []string{},
		CreatedAt:        time.Now(),
	}

	raw, err := json.Marshal(mem)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var data map[string]interface{}
	json.Unmarshal(raw, &data)

	// source_session_ids should be empty array, not nil
	if _, ok := data["source_session_ids"]; !ok {
		t.Error("source_session_ids missing from JSON")
	}
}

func TestStringSliceToPgArray(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input []string
		want  string
	}{
		{nil, "{}"},
		{[]string{}, "{}"},
		{[]string{"a"}, `{"a"}`},
		{[]string{"a", "b", "c"}, `{"a","b","c"}`},
		{[]string{"sess_001", "sess_002"}, `{"sess_001","sess_002"}`},
	}
	for _, tt := range tests {
		got := stringSliceToPgArray(tt.input)
		if got != tt.want {
			t.Errorf("stringSliceToPgArray(%v) = %q; want %q", tt.input, got, tt.want)
		}
	}
}

// ── Integration tests (require running Postgres) ─────────────────────────────

func setupTestStore(t *testing.T) (*Store, func()) {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://memo:memo@localhost:5432/memo?sslmode=disable"
	}
	db, err := sqlx.Connect("pgx", dsn)
	if err != nil {
		t.Skipf("DB not available: %v", err)
	}
	st := &Store{DB: db}

	// Ensure test user exists
	_, _ = db.ExecContext(context.Background(), `
		INSERT INTO users (id, username, password_hash, display_name)
		VALUES ('u_test_mem', 'test_mem_user', 'hash', 'Test Memory User')
		ON CONFLICT (id) DO NOTHING
	`)

	cleanup := func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM memories WHERE user_id = 'u_test_mem' OR session_id IN (SELECT id FROM sessions WHERE kb_id IN (SELECT id FROM knowledge_bases WHERE user_id = 'u_test_mem'))`)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM knowledge_bases WHERE user_id = 'u_test_mem'`)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM users WHERE id = 'u_test_mem'`)
		db.Close()
	}
	return st, cleanup
}

func setupTestKBAndSession(t *testing.T, st *Store) (kbID, sessionID string) {
	t.Helper()
	ctx := context.Background()

	kb, err := st.CreateKB(ctx, "u_test_mem", "Test KB", "for memory tests", nil)
	if err != nil {
		t.Fatalf("CreateKB: %v", err)
	}
	kbID = kb.ID

	sess, err := st.CreateSession(ctx, "u_test_mem", kbID, "Test Session")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	return kbID, sess.ID
}

func TestCreateMemory_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	st, cleanup := setupTestStore(t)
	defer cleanup()
	_, sessionID := setupTestKBAndSession(t, st)
	ctx := context.Background()

	mem, err := st.CreateMemory(ctx, sessionID, "short_term", "用户问了关于Go的问题")
	if err != nil {
		t.Fatalf("CreateMemory: %v", err)
	}
	if mem.ID == "" {
		t.Error("expected non-empty ID")
	}
	if mem.SessionID == nil || *mem.SessionID != sessionID {
		t.Errorf("session_id = %v; want %s", mem.SessionID, sessionID)
	}
	if mem.Type != "short_term" {
		t.Errorf("type = %s; want short_term", mem.Type)
	}
	if mem.Importance != 0.5 {
		t.Errorf("importance = %f; want 0.5", mem.Importance)
	}
}

func TestCreateUserMemory_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	st, cleanup := setupTestStore(t)
	defer cleanup()
	ctx := context.Background()

	mem, err := st.CreateUserMemory(ctx, "u_test_mem", "long_term", "用户熟悉Go语言", 0.8)
	if err != nil {
		t.Fatalf("CreateUserMemory: %v", err)
	}
	if mem.UserID == nil || *mem.UserID != "u_test_mem" {
		t.Errorf("user_id = %v; want u_test_mem", mem.UserID)
	}
	if mem.SessionID != nil {
		t.Errorf("session_id should be nil for user memory, got %v", mem.SessionID)
	}
	if mem.Importance != 0.8 {
		t.Errorf("importance = %f; want 0.8", mem.Importance)
	}
}

func TestListUserMemories_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	st, cleanup := setupTestStore(t)
	defer cleanup()
	ctx := context.Background()

	st.CreateUserMemory(ctx, "u_test_mem", "long_term", "熟悉Go", 0.8)
	st.CreateUserMemory(ctx, "u_test_mem", "fact", "公司用K8s", 0.9)
	st.CreateUserMemory(ctx, "u_test_mem", "preference", "偏好简洁回答", 0.6)

	all, err := st.ListUserMemories(ctx, "u_test_mem", "", 10)
	if err != nil {
		t.Fatalf("ListUserMemories: %v", err)
	}
	if len(all) < 3 {
		t.Errorf("expected >= 3 memories, got %d", len(all))
	}

	facts, err := st.ListUserMemories(ctx, "u_test_mem", "fact", 10)
	if err != nil {
		t.Fatalf("ListUserMemories(fact): %v", err)
	}
	for _, f := range facts {
		if f.Type != "fact" {
			t.Errorf("expected type=fact, got %s", f.Type)
		}
	}
}

func TestGetMemory_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	st, cleanup := setupTestStore(t)
	defer cleanup()
	ctx := context.Background()

	created, _ := st.CreateUserMemory(ctx, "u_test_mem", "fact", "测试获取", 0.7)
	fetched, err := st.GetMemory(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetMemory: %v", err)
	}
	if fetched.ID != created.ID {
		t.Errorf("id mismatch: %s vs %s", fetched.ID, created.ID)
	}
	if fetched.Summary != "测试获取" {
		t.Errorf("summary = %s; want 测试获取", fetched.Summary)
	}
}

func TestUpdateMemoryImportance_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	st, cleanup := setupTestStore(t)
	defer cleanup()
	ctx := context.Background()

	created, _ := st.CreateUserMemory(ctx, "u_test_mem", "long_term", "测试重要性", 0.5)
	if err := st.UpdateMemoryImportance(ctx, created.ID, 0.95); err != nil {
		t.Fatalf("UpdateMemoryImportance: %v", err)
	}
	fetched, _ := st.GetMemory(ctx, created.ID)
	if fetched.Importance != 0.95 {
		t.Errorf("importance = %f; want 0.95", fetched.Importance)
	}
}

func TestTouchMemoryAccess_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	st, cleanup := setupTestStore(t)
	defer cleanup()
	ctx := context.Background()

	created, _ := st.CreateUserMemory(ctx, "u_test_mem", "long_term", "测试访问", 0.5)
	if created.AccessCount != 0 {
		t.Errorf("initial access_count = %d; want 0", created.AccessCount)
	}

	if err := st.TouchMemoryAccess(ctx, created.ID); err != nil {
		t.Fatalf("TouchMemoryAccess: %v", err)
	}
	fetched, _ := st.GetMemory(ctx, created.ID)
	if fetched.AccessCount != 1 {
		t.Errorf("access_count = %d; want 1", fetched.AccessCount)
	}
	if fetched.LastAccessedAt == nil {
		t.Error("last_accessed_at should be set after touch")
	}
}

func TestDeleteExpiredMemories_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	st, cleanup := setupTestStore(t)
	defer cleanup()
	ctx := context.Background()

	past := time.Now().Add(-1 * time.Hour)
	mem, _ := st.CreateUserMemory(ctx, "u_test_mem", "short_term", "即将过期", 0.3)
	_, _ = st.DB.ExecContext(ctx, `UPDATE memories SET expires_at=$2 WHERE id=$1`, mem.ID, past)

	affected, err := st.DeleteExpiredMemories(ctx)
	if err != nil {
		t.Fatalf("DeleteExpiredMemories: %v", err)
	}
	if affected < 1 {
		t.Errorf("expected >= 1 deleted, got %d", affected)
	}

	_, err = st.GetMemory(ctx, mem.ID)
	if err == nil {
		t.Error("expected error fetching expired memory")
	}
}

func TestMergeMemories_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	st, cleanup := setupTestStore(t)
	defer cleanup()
	ctx := context.Background()

	keep, _ := st.CreateUserMemory(ctx, "u_test_mem", "long_term", "保留这条", 0.8)
	m1, _ := st.CreateUserMemory(ctx, "u_test_mem", "long_term", "合并1", 0.4)
	m2, _ := st.CreateUserMemory(ctx, "u_test_mem", "long_term", "合并2", 0.3)

	if err := st.MergeMemories(ctx, keep.ID, []string{m1.ID, m2.ID}); err != nil {
		t.Fatalf("MergeMemories: %v", err)
	}

	_, err := st.GetMemory(ctx, m1.ID)
	if err == nil {
		t.Error("merged memory m1 should be deleted")
	}
	_, err = st.GetMemory(ctx, m2.ID)
	if err == nil {
		t.Error("merged memory m2 should be deleted")
	}

	surviving, err := st.GetMemory(ctx, keep.ID)
	if err != nil {
		t.Fatalf("kept memory should exist: %v", err)
	}
	if surviving.Summary != "保留这条" {
		t.Errorf("summary = %s; want 保留这条", surviving.Summary)
	}
}

func TestListSessionMemories_WithNewFields_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	st, cleanup := setupTestStore(t)
	defer cleanup()
	_, sessionID := setupTestKBAndSession(t, st)
	ctx := context.Background()

	st.CreateMemory(ctx, sessionID, "short_term", "对话1")
	st.CreateMemory(ctx, sessionID, "short_term", "对话2")

	mems, err := st.ListSessionMemories(ctx, sessionID, 10)
	if err != nil {
		t.Fatalf("ListSessionMemories: %v", err)
	}
	if len(mems) < 2 {
		t.Errorf("expected >= 2, got %d", len(mems))
	}
	for _, m := range mems {
		if m.Importance == 0 && m.Summary != "" {
			t.Errorf("importance should have default value for memory %s", m.ID)
		}
	}
}
