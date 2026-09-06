-- +goose Up
-- Phase E: semantic retrieval. text-embedding-3-small produces 1536-dim
-- vectors; HNSW (available in pgvector >= 0.5) gives good recall/speed
-- tradeoff for approximate nearest-neighbor search at this dimensionality.
-- Cosine distance (vector_cosine_ops) matches OpenAI's recommended metric
-- for their embedding models.
CREATE EXTENSION IF NOT EXISTS vector;

ALTER TABLE jobs ADD COLUMN embedding vector(1536);
ALTER TABLE jobs ADD COLUMN embedding_model TEXT;
ALTER TABLE jobs ADD COLUMN embedded_at TIMESTAMPTZ;

CREATE INDEX jobs_embedding_hnsw_idx ON jobs USING hnsw (embedding vector_cosine_ops)
  WHERE status = 'ACTIVE' AND canonical_job_id IS NULL;

-- +goose Down
DROP INDEX jobs_embedding_hnsw_idx;
ALTER TABLE jobs DROP COLUMN embedded_at;
ALTER TABLE jobs DROP COLUMN embedding_model;
ALTER TABLE jobs DROP COLUMN embedding;
DROP EXTENSION IF EXISTS vector;
