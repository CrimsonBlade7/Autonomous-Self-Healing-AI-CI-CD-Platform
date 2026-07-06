.
├── .github/                  # GitHub workflow definitions (if bootstrapping itself)
├── docker/                   # Shared Docker & deployment configurations
│   ├── docker-compose.yml    # Local multi-service orchestrator (Go, Python, DB, Redis)
│   ├── orchestrator.Dockerfile
│   └── ai-engine.Dockerfile
│
├── migrations/               # PostgreSQL & pgvector schema migrations
│   ├── 000001_init_schema.up.sql
│   └── 000002_add_pgvector_embeddings.up.sql
│
├── orchestrator/             # --- BACKEND ORCHESTRATOR (Go) ---
│   ├── cmd/
│   │   └── main.go           # Application entry point
│   ├── internal/             # Private application code
│   │   ├── config/           # Environment and system config parsing
│   │   ├── docker/           # Docker Engine SDK wrapper (Job lifecycle management)
│   │   │   ├── client.go
│   │   │   └── jobs.go       # Lint, Build, Test run logic
│   │   ├── eventbus/         # Redis/Celery/Channel interaction layer
│   │   ├── server/           # HTTP Webhook listener for GitHub
│   │   │   ├── github.go     # PR payload validation & signature checking
│   │   │   └── handlers.go
│   │   └── storage/          # Database interaction (relational pipeline tracking)
│   ├── go.mod
│   └── go.sum
│
└── ai_engine/                # --- AI/DATA PROCESSING (Python + FastAPI) ---
    ├── app/
    │   ├── __init__.py
    │   ├── main.py           # FastAPI entry point
    │   ├── core/             # Core configurations and security settings
    │   │   └── config.py
    │   ├── database/         # SQLAlchemy / pgvector interaction layers
    │   │   ├── session.py
    │   │   └── vector_store.py
    │   ├── services/         # Core business logic
    │   │   ├── log_analyzer.py # Parses test failures and stack traces
    │   │   ├── patch_gen.py  # Generates unified diffs / healing code patches
    │   │   └── rag_pipeline.py # Context retrieval, embedding creation via pgvector
    │   └── tasks/            # Celery worker tasks for async evaluation
    │       └── workers.py
    ├── pyproject.toml        # Poetry / dependency management
    └── requirements.txt

### Architectural Highlights

- orchestrator/internal/docker: This encapsulates the Docker Engine API SDK logic. It keeps the core Go application clean from messy low-level container bindings. When a job triggers, this package handles downloading images, injecting volumes, running the test command, and streaming logs back.

- ai_engine/app/services/rag_pipeline.py: This handles the generation of vector embeddings (e.g., using OpenAI or HuggingFace local models) and interfaces with vector_store.py to execute similarity searches across historical failures and codebase fragments.

- migrations/: Kept at the root level because both services rely on the central PostgreSQL database. The Go orchestrator queries structural data (pipeline tracking), while the Python AI engine utilizes both structural data (logs) and unstructured vector embeddings (pgvector).

- docker/: Consolidates the system orchestration. Running docker-compose up inside this folder instantly brings up the local Go api, Python worker nodes, Redis event queue, and a pre-configured PostgreSQL instance.