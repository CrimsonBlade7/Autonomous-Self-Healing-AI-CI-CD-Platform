# Project Summary: Autonomous Self-Healing AI CI/CD Platform

## What It Does

This system is an automated CI/CD agent that watches a GitHub repository for pull requests, runs the PR's test suite in an isolated Docker container, and — if the tests fail — uses an AI/RAG pipeline to investigate the failure, author diagnostic tests, and produce a structured report with recommended fixes for a human developer to review. It never applies changes to application source code on its own.

---

## Repository Layout (Monorepo)

```
repo-root/
├── orchestrator/          # Go backend — event ingestion, Docker control, pipeline state
├── ai_engine/             # Python + FastAPI — log analysis, RAG pipeline, test generation
├── migrations/            # PostgreSQL schema (relational + pgvector)
├── compose.yml            # Local dev entrypoint (work-in-progress)
├── Dockerfile             # User-supplied; run inside CI containers
└── project_planning/      # Architecture docs, diagrams, planning notes
```

Both services ship from one repository, share one version/release cadence, and are not designed to run independently.

---

## Tech Stack

| Layer | Technology |
|---|---|
| Orchestrator | Go (`net/http`, Moby/Docker SDK, `log/slog`) |
| AI Engine | Python 3.x, FastAPI, Celery, Pydantic |
| Internal task queue | Redis (scoped to the Python service only) |
| Database | PostgreSQL + `pgvector` extension |
| Embedding model | `all-MiniLM-L6-v2` (384-dim vectors) |
| Containerisation | Docker (containers provisioned at runtime via SDK) |

---

## Components

### Go Orchestrator (`orchestrator/`)

| File/Package | Role |
|---|---|
| `cmd/main.go` | Entry point — initialises config, Docker client, task channel, workflow manager, HTTP server |
| `internal/servertools/server.go` | HTTP server — validates GitHub webhook HMAC signatures, routes `pull_request` events and AI Engine callbacks |
| `internal/pipelines/workflow.go` | Per-PR state machine — processes jobs (`open`, `edit`, `sync`, `run_tests`, `commit_push`) sequentially |
| `internal/pipelines/workflow_manager.go` | Fan-out — routes incoming `Task` values to the correct `Workflow` |
| `internal/dockertools/docker.go` | Docker SDK wrapper — provisions/tears down containers, streams logs |
| `internal/wstools/workspace_manager.go` | Git workspace lifecycle — clones repos, manages local working copies |
| `internal/wstools/github_client.go` | GitHub REST calls (PR comments, status updates) |
| `internal/config/config.go` | Loads env vars (webhook secret, AI Engine URL, etc.) |
| `internal/types/` | Shared types: `PullRequest`, `Task`, `AIEngineRequest`, `PushedCommits` |

### Python AI Engine (`ai_engine/`)

| File/Package | Role |
|---|---|
| `app/main.py` | FastAPI app — exposes `/job` (receives work from Go), `/analyze`, `/generate-tests`; sanitises and redacts log output before returning it to callers |
| `app/services/log_analyzer.py` | Parses raw CI log text into structured records (error type, failing test, stack trace, error signature) |
| `app/services/rag_pipeline.py` | Builds enriched LLM prompts from parsed logs + vector-store context |
| `app/services/test_gen.py` | Calls an LLM to author diagnostic tests that reproduce a failure |
| `app/services/patch_gen.py` | Generates fix suggestions (surfaced in the report; never auto-applied) |
| `app/services/orchestrator_client.py` | POSTs the completed report back to Go's `/ai-callback` with retry-with-backoff |
| `app/tasks/workers.py` | Celery task definitions — runs RAG + test authoring asynchronously |
| `app/database/vector_store.py` | pgvector query helpers — semantic similarity search over `code_embeddings` |
| `app/core/config.py` | Pydantic settings — validates all required env vars on startup |

### Database (`migrations/`)

| Migration | Creates |
|---|---|
| `000001_init_schema.up.sql` | `pipeline_runs` — tracks every PR trigger with status (`pending`, `running`, `failed`, `healed`, `succeeded`) and indexed by `(repo, pr_number)` and `status` |
| `000002_add_pgvector_embeddings.up.sql` | `code_embeddings` — stores 384-dim vectors for error logs, source snippets, and historical patches, linked to a `pipeline_run` |

---

## End-to-End Data Flow

```
GitHub PR opened/updated
        │
        ▼
Go Orchestrator (HMAC-verified webhook)
        │
        ├─► Clone repo → provision Docker containers
        │         │
        │    Linting → Build → Tests
        │         │
        │    Tests PASS ──────────────────────────────► Post success to PR
        │         │
        │    Tests FAIL
        │         │
        ▼         ▼
Go HTTP POST ──► Python FastAPI /job  (200 OK immediately)
                        │
                   Celery worker (async)
                        │
                   Log Analysis
                   (parse stack trace, extract error signature)
                        │
                   pgvector similarity search
                   (find similar historical errors + related source)
                        │
                   RAG Pipeline
                   (build context-enriched LLM prompt)
                        │
                   Test Author
                   (LLM writes diagnostic tests; never touches existing source)
                        │
                   Run authored tests → collect results
                        │
        Python POST ──► Go /ai-callback  (retry-with-backoff)
                                │
                        Go surfaces report
                        (PR comment / stored artifact)
                        Human reviews + decides on fix
```

---

## Communication Contract

- **Go → Python:** `POST /job` with an `AIEngineRequest` JSON body containing `Wfid`, `PullRequest`, stdout/stderr logs, timing, exit code, and OOM flag. An `X-Job-Type` header (`open | close | edit | sync | logs`) tells the AI Engine what kind of event triggered the call.
- **Python → Go:** `POST /ai-callback` with the workflow ID and a structured `ParsedLogResponse` (error type, failing test, stack trace, language, framework, confidence, root cause, recommended fix). Includes retry-with-backoff because HTTP offers no delivery guarantee if Go restarts.
- Both legs are HMAC-SHA-256 signed.

---

## Security Highlights

- GitHub webhook payloads are verified with HMAC-SHA-256 before any processing.
- Log text returned to callers is redacted: absolute paths → `<path>`, UUIDs → `<uuid>`, long hex hashes → `<hash>`, timestamps → `<time>`, credential-like key-value pairs → `<secret>`.
- The AI Engine never writes to or commits application source files.

---

## Current State & Known Gaps

- `compose.yml` is a work-in-progress placeholder; the full `docker/docker-compose.yml` contains the active service definitions.
- Report delivery mechanism (PR comment vs. dashboard vs. artifact store) is not yet finalised.
- No retry logic for failed/hanging workflows — a dropped workflow is currently discarded with no retry.
- Docker volumes for workspace and log persistence are not yet implemented.
- Test infrastructure is built out for the AI Engine (`ai_engine/tests/`); orchestrator tests are listed as a TODO.
