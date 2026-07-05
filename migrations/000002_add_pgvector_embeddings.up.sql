-- Migration 000002: pgvector extension + code embeddings table
--
-- Enables vector similarity search so the RAG pipeline can find historically
-- similar errors and related code snippets for a given failure signature.
--
-- IMPORTANT: This migration depends on 000001 (pipeline_runs must exist first).
-- The pgvector/pgvector Docker image ships with the extension pre-built;
-- CREATE EXTENSION just activates it for this database.

CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS code_embeddings (
    id              SERIAL PRIMARY KEY,
    pipeline_run_id INTEGER     REFERENCES pipeline_runs (id) ON DELETE CASCADE,
    --  source_type distinguishes what kind of content was embedded:
    --    'error_log'   — a parsed error signature from a failing job
    --    'source_file' — a snippet of repository source code
    --    'patch'       — a previously generated healing patch (for learning)
    source_type     TEXT        NOT NULL,
    content_text    TEXT        NOT NULL,   -- the original text that was embedded
    --  384-dimensional vector produced by all-MiniLM-L6-v2.
    --  If you change the embedding model, update this dimension to match.
    embedding       vector(384),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Exact-match lookup by pipeline run (e.g. "fetch all embeddings for run #42")
CREATE INDEX IF NOT EXISTS idx_code_embeddings_run
    ON code_embeddings (pipeline_run_id);

-- HNSW approximate nearest-neighbour index for fast cosine similarity search.
-- vector_cosine_ops = use cosine distance (<->).
-- This index is what makes RAG retrieval fast at scale.
CREATE INDEX IF NOT EXISTS idx_code_embeddings_hnsw
    ON code_embeddings USING hnsw (embedding vector_cosine_ops);
