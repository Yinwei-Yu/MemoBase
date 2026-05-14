-- Migration 004: Add embedding_model column to model_providers
-- This allows configuring a separate embedding model per provider.

ALTER TABLE model_providers ADD COLUMN IF NOT EXISTS embedding_model TEXT NOT NULL DEFAULT '';
