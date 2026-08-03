=========================================
SELF-HEALING AI DEVOPS AGENT: SYSTEM ARCHITECTURE
=========================================

0. REPOSITORY STRUCTURE (MONOREPO)
-----------------------------------
Although the system is described below as two "zones," this is a description 
of runtime services, not of separate codebases. Both zones live in a single 
Git repository and are versioned, reviewed, and released together. Neither 
service is designed, packaged, or intended to be run, deployed, or evolved 
independently of the other.

    repo-root/
    ├── orchestrator/          # Go Backend Orchestrator (Loop A, container control)
    ├── ai-service/            # Python + FastAPI AI/Data Processing service (Loops B & C)
    ├── docker-compose.yml     # Local dev entrypoint: brings up orchestrator, ai-service,
    │                          #   PostgreSQL+pgvector, and the ai-service-internal Redis
    │                          #   together on one Docker network
    ├── docs/                  # Architecture docs (this file and its companions)
    └── .github/workflows/     # A single CI pipeline that builds/tests both services together

Because both services share one repository:
* A pull request can touch `orchestrator/` and `ai-service/` at the same time, 
  and CI validates both together rather than as independently released 
  packages.
* There is one version/tag per release covering the whole system, not 
  independent version numbers per service.
* The HTTP boundary between Go and Python (described below) exists purely as 
  a runtime/process boundary for language and workload separation — it is not 
  a repository or ownership boundary. The two directories are expected to 
  change in lockstep, and the "Job Payload + Workflow ID" contract between 
  them should be treated as an internal interface documented and reviewed in 
  this same repo, not a versioned external API.
* `docker-compose.yml` at the repo root is the reference way to run the whole 
  system locally; the individual services are not expected to be stood up or 
  documented as standalone deployables.


1. HIGH-LEVEL SYSTEM OVERVIEW
-----------------------------
The system is divided into two major functional zones mediated by a central 
communication hub. These are runtime zones within a single monorepo, not 
independently deployable services.

* On the Left: The Repository & Orchestration zone acts as the system's "central 
  nervous system." It is reactive, listening to outside events (like GitHub PRs) 
  and commanding local infrastructure (Docker containers) to execute tasks. The 
  Backend Orchestrator (written in Go, living at `orchestrator/` in the 
  monorepo) owns the pipeline execution logic, state, and container lifecycles.

* On the Right: The Data & AI Processing zone serves as the system's "brain." 
  It is computational and analytical. The AI/Data Processing service (Python + 
  FastAPI, living at `ai-service/` in the same monorepo) manages complex data 
  analysis, context retrieval, and decision-making using the RAG Pipeline and 
  Code Generator. These components query a centralized 
  intelligence store--the PostgreSQL database with pgvector--to understand code 
  and logs before generating responses.

* In the Middle: The two zones do not share a message broker. Instead, they talk 
  directly over HTTP: the Backend Orchestrator sends a job payload (tagged with 
  a workflow ID) to the AI/Data Processing service and gets an immediate 
  acknowledgment back, so the real-time scheduling of simple, lightweight 
  linting jobs is never blocked waiting on a response. The heavy lifting of 
  code generation or log analysis happens afterward, asynchronously, inside 
  the Python service itself (via Celery/Redis, which is private to that 
  service), and the result is delivered back to Go through a dedicated 
  callback endpoint once it's ready.


2. LOW-LEVEL COMPONENT INTERACTIONS
-----------------------------------
To understand specifically how these systems collaborate, let's follow a single 
workflow through three key "conversation loops."


LOOP A: Event Ingestion & Job Execution
---------------------------------------
* GitHub -> Webhooks -> Backend Orchestrator (Go)
  A pull request is created. GitHub POSTs a JSON payload to the Go Orchestrator. 
  The orchestrator validates the request, parses the repository details, and 
  initializes a new pipeline state in memory or the database.

* Job Scheduler -> Docker Engine API Client -> Docker Engine API (SDK)
  The orchestrator's Job Scheduler determines which containerized jobs must run 
  first (e.g., Linting Job, Build Job). The Docker Engine API Client translates 
  these requirements into Docker API calls. It instructs the local Docker Engine 
  SDK to run specific images in isolated environments, mounting the relevant 
  code as a volume.

* Containers -> Logs/Results -> Backend Orchestrator (Go)
  As the Linting or Test Job runs, its standard output (STDOUT/STDERR) and exit 
  code are streamed back to the orchestrator.

* Backend Orchestrator (Go) -> Pipeline Updates -> Internal State
  The Go service updates the pipeline status (e.g., "Build Failed") in its own 
  state store. If the Test Job has failed, the orchestrator immediately kicks 
  off Loop B by issuing an HTTP request straight to the AI/Data Processing 
  service — there is no intermediate message bus on this leg.


LOOP B: Failure Analysis (RAG Pipeline)
---------------------------------------
* Backend Orchestrator (Go) -> HTTP Request (Job Payload + Workflow ID) -> AI/Data Processing (Python + FastAPI)
  The Go orchestrator sends a plain HTTP request directly to the Python 
  service's FastAPI interface, containing the job payload and the workflow ID 
  that was generated when the originating webhook was first received. FastAPI 
  immediately returns a `200 OK` acknowledging receipt, then hands the actual 
  work off internally to Celery (backed by a Redis instance scoped to the 
  Python service only) so the analysis can run asynchronously without holding 
  the HTTP connection open.

* AI/Data Processing -> Pull Context -> Log Analysis
  The Python service first directs its Log Analysis module to look at the job 
  logs. It extracts specific error messages, failing stack traces, and test case 
  identifiers.

* Log Analysis -> Code Context / Error Logs -> PostgreSQL DB + pgvector
  The analysis module queries the PostgreSQL DB (the state store). To get 
  meaningful context, it also performs a semantic vector search using pgvector. 
  It converts the error signature (e.g., "RecursionError in router.go") into a 
  vector embedding and finds similar historical errors or the specific code 
  segments related to that stack trace.

* PostgreSQL DB + pgvector -> Contextual Analytics -> RAG Pipeline
  The database returns relevant context: historical patches, related code 
  definitions, and embedded documentation. The Python RAG Pipeline assembles 
  this raw data into a structured context window for an LLM.


LOOP C: Self-Healing & Patch Generation
---------------------------------------
* RAG Pipeline -> Context/Request -> Code Generator
  The RAG pipeline hands the full contextual "package" (failing code snippet + 
  error logs + historical vector context) to the Code Generator.

* Code Generator -> (LLM processing) -> Self-Healing/Code Patch
  The generator (calling an LLM) creates a proposed modification designed to fix 
  the failing test suite without introducing regressions.

* AI/Data Processing (Python) -> Callback POST -> Backend Orchestrator (Go)
  Once Celery finishes generating the fix, the Python service encapsulates it 
  as a standard unified diff format (a patch) and POSTs it directly to a 
  dedicated Go endpoint (e.g. `/ai-callback`), including the same workflow ID 
  so Go can correlate the patch with the correct pipeline run. Because a plain 
  HTTP request offers no delivery guarantee if Go is briefly unreachable (e.g. 
  mid-restart), this callback includes retry-with-backoff logic on the Python 
  side.

* Backend Orchestrator (Go) -> Repository Update
  The Go Orchestrator uses the Docker Engine SDK to launch a specialized 'Git' 
  container that applies the patch, commits the changes, and pushes the fix 
  back to the GitHub PR. This allows the pipeline to re-run and confirm the 
  automated fix.