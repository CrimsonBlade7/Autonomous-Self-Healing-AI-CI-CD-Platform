-- Migration 000001: Initial relational schema
--
-- Creates the pipeline_runs table that the Go orchestrator and Python service
-- both use to track the state of each CI pipeline triggered by a GitHub PR.
--
-- This table is purely relational — no vectors here.
-- Vector embeddings live in code_embeddings (migration 000002).

CREATE TABLE IF NOT EXISTS pipeline_runs (
    id          SERIAL PRIMARY KEY,
    repo        TEXT        NOT NULL,           -- e.g. "org/repo-name"
    pr_number   INTEGER     NOT NULL,
    commit_sha  TEXT        NOT NULL,           -- HEAD commit at trigger time
    status      TEXT        NOT NULL DEFAULT 'pending',
    --  Allowed values: pending | running | failed | healed | succeeded
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Fast lookup by status (used by the scheduler to find in-progress pipelines)
CREATE INDEX IF NOT EXISTS idx_pipeline_runs_status
    ON pipeline_runs (status);

-- Fast lookup by repo + PR number (used when a new webhook arrives)
CREATE INDEX IF NOT EXISTS idx_pipeline_runs_repo_pr
    ON pipeline_runs (repo, pr_number);
