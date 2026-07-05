=========================================
SELF-HEALING AI DEVOPS AGENT: SYSTEM ARCHITECTURE
=========================================

1. HIGH-LEVEL SYSTEM OVERVIEW
-----------------------------
The system is divided into two major functional zones mediated by a central 
communication hub.

* On the Left: The Repository & Orchestration zone acts as the system's "central 
  nervous system." It is reactive, listening to outside events (like GitHub PRs) 
  and commanding local infrastructure (Docker containers) to execute tasks. The 
  Backend Orchestrator (written in Go) owns the pipeline execution logic, state, 
  and container lifecycles.

* On the Right: The Data & AI Processing zone serves as the system's "brain." 
  It is computational and analytical. The AI/Data Processing service (Python + 
  FastAPI) manages complex data analysis, context retrieval, and decision-making 
  using the RAG Pipeline and Code Generator. These components query a centralized 
  intelligence store--the PostgreSQL database with pgvector--to understand code 
  and logs before generating responses.

* In the Middle: The Event Bus decouples these two zones. This ensures that the 
  heavy lifting of code generation or log analysis does not block the real-time 
  scheduling of simple, lightweight linting jobs.


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

* Backend Orchestrator (Go) -> Pipeline Updates -> Event Bus
  The Go service updates the pipeline status (e.g., "Build Failed") and 
  publishes an event message to the Event Bus (Redis/Celery or internal Go 
  channels).


LOOP B: Failure Analysis (RAG Pipeline)
---------------------------------------
* Event Bus -> Context/Request -> AI/Data Processing (Python + FastAPI)
  The Event Bus notifies the Python service that a Test Job failed for a 
  specific PR. The Python service accepts this event as a task via its FastAPI 
  interface.

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

* AI/Data Processing -> Patch/Code Patch -> Event Bus
  The Python service encapsulates the fix as a standard unified diff format 
  (a patch) and publishes it back to the Event Bus.

* Event Bus -> Patch -> Backend Orchestrator (Go)
  The Go Orchestrator consumes the patch event.

* Backend Orchestrator (Go) -> Repository Update
  The Go Orchestrator uses the Docker Engine SDK to launch a specialized 'Git' 
  container that applies the patch, commits the changes, and pushes the fix 
  back to the GitHub PR. This allows the pipeline to re-run and confirm the 
  automated fix.