-- Migration 003: Memory enhancement
-- Makes session_id nullable, adds user-level memory fields

-- Make session_id nullable (user-level memories don't belong to a session)
ALTER TABLE memories ALTER COLUMN session_id DROP NOT NULL;

-- Add new columns
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='memories' AND column_name='user_id') THEN
        ALTER TABLE memories ADD COLUMN user_id TEXT REFERENCES users(id);
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='memories' AND column_name='importance') THEN
        ALTER TABLE memories ADD COLUMN importance REAL NOT NULL DEFAULT 0.5;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='memories' AND column_name='access_count') THEN
        ALTER TABLE memories ADD COLUMN access_count INT NOT NULL DEFAULT 0;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='memories' AND column_name='last_accessed_at') THEN
        ALTER TABLE memories ADD COLUMN last_accessed_at TIMESTAMPTZ;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='memories' AND column_name='embedding_id') THEN
        ALTER TABLE memories ADD COLUMN embedding_id TEXT;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='memories' AND column_name='source_session_ids') THEN
        ALTER TABLE memories ADD COLUMN source_session_ids TEXT[] DEFAULT '{}';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='memories' AND column_name='expires_at') THEN
        ALTER TABLE memories ADD COLUMN expires_at TIMESTAMPTZ;
    END IF;
END $$;

-- Backfill user_id from session → kb → user
UPDATE memories m SET user_id = (
    SELECT kb.user_id FROM sessions s
    INNER JOIN knowledge_bases kb ON kb.id = s.kb_id
    WHERE s.id = m.session_id
) WHERE m.user_id IS NULL AND m.session_id IS NOT NULL;

-- Indexes
CREATE INDEX IF NOT EXISTS idx_memories_user ON memories(user_id, type, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_memories_importance ON memories(user_id, importance DESC);
CREATE INDEX IF NOT EXISTS idx_memories_expires ON memories(expires_at) WHERE expires_at IS NOT NULL;

-- Access log table
CREATE TABLE IF NOT EXISTS memory_access_log (
    id TEXT PRIMARY KEY,
    memory_id TEXT NOT NULL REFERENCES memories(id) ON DELETE CASCADE,
    accessed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    context TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_mem_access ON memory_access_log(memory_id, accessed_at DESC);
