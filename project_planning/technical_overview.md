# Technical Overview: Autonomous AI CI/CD Pipeline

## 1. Executive Summary
The **Self-Healing AI DevOps Agent** is an automated, closed-loop infrastructure system designed to detect, diagnose, and surface human-reviewed fix suggestions for software delivery and testing pipeline failures. By integrating real-time repository orchestration with a Retrieval-Augmented Generation (RAG) artificial intelligence loop, the system transforms standard Continuous Integration/Continuous Deployment (CI/CD) workflows from static feedback loops into proactive, self-diagnosing mechanisms that isolate failures, author targeted tests to characterize them, and hand developers a clear, actionable report — rather than applying changes to application code on its own.

---

## 2. The High-Level Architecture (In Simple Terms)
To understand how this system works, imagine an automated software engineering team split into three primary roles:

*   **The Project Manager & Builder (Repository & Orchestration Zone):** Written in highly performant Go, this side of the system acts as the "nervous system." It watches GitHub for new code contributions, spins up isolated container boxes (Docker containers) to test the code, and tracks whether those tests pass or fail.
*   **The Direct Line (Go↔Python HTTP + the Analyst's Private To-Do List):** Rather than routing every hand-off through a shared post office, the manager (Go) and the analyst (Python) talk to each other directly over HTTP — the manager hands off a request and gets an instant "got it" back, then keeps working while the analyst investigates in the background. Internally, the analyst keeps its own private to-do list (built using Redis and Celery) to manage that background work; this list isn't shared infrastructure with the manager. When the analyst finishes, it calls the manager back directly with the result.
*   **The AI Diagnostic Engineer (Data & AI Processing Zone):** Written in Python and FastAPI, this acts as the system's "brain." If a test job fails, this component investigates the error logs, searches a historical database (PostgreSQL with `pgvector`) to find similar problems or related code, and writes new test code — additional or refined tests aimed at reproducing and isolating the failure — without ever touching the existing application source. It then compiles a summary report describing what it investigated, the tests it authored and ran, their pass/fail results, and recommended fixes for a human developer to review and apply themselves.

---

## 3. Repository Structure (Monorepo)
Despite being split into two runtime "roles" above, the system is built and 
shipped from a **single repository**. The Go orchestrator and the Python 
AI/Data service are separate *processes* that happen to talk over HTTP — they 
are not separate *projects*, and neither is designed, documented, or intended 
to be built, deployed, or versioned on its own.

```
repo-root/
├── orchestrator/          # Go Backend Orchestrator
├── ai-service/            # Python + FastAPI AI/Data Processing service
├── docker-compose.yml     # Local dev entrypoint — brings up orchestrator,
│                          #   ai-service, PostgreSQL+pgvector, and the
│                          #   ai-service-internal Redis on one network
├── docs/
└── .github/workflows/     # A single CI pipeline covering both services
```

Practical implications of this structure:
* **One version, one release cadence.** A release/tag covers the whole 
  system; there are no independent version numbers for the Go and Python 
  sides.
* **Coordinated changes.** A single pull request can touch both 
  `orchestrator/` and `ai-service/` when a change to their shared contract 
  (the job payload, the workflow ID, the `/ai-callback` shape) requires it, 
  and CI validates both together.
* **Internal, not external, interface.** The HTTP contract between the two 
  services is an internal interface owned by this repo, not a published/ 
  versioned public API — it can change freely as long as both sides are 
  updated together.
* **One way to run it.** The root `docker-compose.yml` is the reference 
  environment for local development; the individual services aren't expected 
  to be stood up or documented as standalone deployables.

---

## 4. Detailed Component Deep Dive

### 4.1 Repository & Orchestration (Go Backend)
Constructed for concurrency, speed, and safety, the Go backend orchestrates external triggers and runtime compute infrastructure.
*   **Backend Orchestrator (Go):** Acts as the foundational web service listening to GitHub Webhooks. Upon a `PR Create/Update` event, it initializes the internal state machine.
*   **Job Scheduler & Pipeline Tracker:** Decoupled processes within the Go binary that determine job order (e.g., Linting $\rightarrow$ Building $\rightarrow$ Testing), manage dependencies, and track real-time execution states.
*   **Docker Engine API Client:** Utilizing the official Docker Engine SDK, this module dynamically spins up isolated, reproducible execution containers. By avoiding reliance on bulky virtual machines, it optimizes system throughput and resource isolation.

### 4.2 Go↔Python Communication & Python's Internal Task Queue
A direct HTTP link connects the real-time orchestration tier to the computational AI tier, with the AI tier managing its own asynchronous execution internally rather than through a broker shared with Go.
*   **Technologies:** Plain HTTP request/response for the Go↔Python leg (job dispatch and callback). Internally, the Python service uses Celery with Redis as its broker to run RAG/LLM analysis asynchronously; this Redis instance is scoped to the Python service only.
*   **Purpose:** Decouples the low-latency Go backend from the high-latency LLM execution routines without requiring a shared message broker between services. Go sends the job payload — tagged with a workflow ID generated when the originating webhook was first received — and immediately gets back a `200 OK`, then continues processing other linting and tracking jobs concurrently. Once Celery finishes its investigation and test authoring, Python POSTs a callback to a dedicated Go endpoint (`/ai-callback`), using the same workflow ID so Go can correlate the result with the correct pipeline run. Because plain HTTP offers no delivery guarantee if Go is briefly unreachable (e.g. mid-restart), the callback includes retry-with-backoff logic on the Python side.

**Design note — why not a shared queue?** An alternative design routed the Go↔Python leg through Redis as well, with Go pushing jobs onto a shared queue instead of calling Python directly. That would add durability (jobs surviving a Go/Python restart) and backlog visibility (queue-depth inspection), but was judged not worth the added infrastructure and cross-service coordination (a shared broker address, queue naming, etc.) given the team's current experience level and the low real-world risk of the callback failure mode during active development. This is a contained, revisitable decision: migrating the Go↔Python leg to a queue later would only mean swapping the transport layer, not restructuring either service.

### 4.3 Data & AI Processing (Python + FastAPI)
The logical and cognitive tier running specialized analytical modules.
*   **FastAPI Wrapper:** Exposes ultra-fast, asynchronous REST endpoints for receiving context requests from the Event Bus.
*   **RAG (Retrieval-Augmented Generation) Pipeline:** Rather than sending empty queries to an LLM, the RAG pipeline gathers granular context—failing code snippets, targeted dependencies, and historical documentation—and builds a heavily enriched semantic prompt.
*   **Test Author:** The algorithmic execution block that interfaces with LLMs to author new, syntactically valid test code aimed at reproducing and isolating the explicit failure pattern. This component never modifies existing application source files; it may iterate on and refine the tests it has itself authored. Its output — the tests, their run results, and recommended fixes — feeds into a summary report rather than a patch applied to production code.

### 4.4 Intelligence & Persistence Layer
*   **PostgreSQL + pgvector:** A dual-purpose relational and vector database. Standard tables hold application configuration, logs, and pipeline statuses, while the `pgvector` extension stores high-dimensional mathematical embeddings of your codebase, documentation, and historical error logs.
*   **Log Analysis Module:** Extracts diagnostic tokens, stack traces, and exit signals out of raw text streams, transforming unstructured logs into structured relational records and search vectors.

---

## 5. Low-Level Component Interactions (Step-by-Step Workflow)

The full operational capabilities of the agent can be mapped across three distinct conversation loops:

### Loop A: Event Ingestion & Job Execution
1.  **Trigger:** A developer creates or updates a Pull Request on GitHub. GitHub fires a Webhook POST request containing full event metadata.
2.  **Ingestion:** The **Backend Orchestrator (Go)** ingests the payload, evaluates the target branch, and passes the execution recipe to the **Job Scheduler**.
3.  **Containers Provisioned:** The **Docker Engine API Client** communicates with the **Docker Engine API (SDK)** to programmatically provision separate containers for the **Linting Job**, **Build Job**, and **Test Job**.
4.  **Telemetry Stream:** Execution streams (`STDOUT`/`STDERR`) and termination codes flow directly back to the **Backend Orchestrator**, which updates its own internal pipeline state. If the Test Job fails, Go immediately triggers Loop B by calling the AI/Data Processing service directly over HTTP.

### Loop B: Failure Analysis & The RAG Pipeline
1.  **Interception:** If the **Test Job** returns a non-zero exit code (failure), the **Backend Orchestrator** sends an HTTP request directly to the **AI/Data Processing (Python + FastAPI)** layer, containing the job payload and the workflow ID generated when the originating webhook was first received. FastAPI immediately responds with a `200 OK`, then hands the task to Celery for asynchronous processing.
2.  **Log Diagnostics:** The **Log Analysis** module scans the raw terminal streams, separating ambient boilerplate logs from explicit error exceptions and stack traces.
3.  **Vector Store Query:** The system transforms the error signatures into vector embeddings and queries the **PostgreSQL DB + pgvector**. It checks for:
    *   Historically resolved errors with similar semantic profiles.
    *   The structural definition of the exact files and lines of code mentioned in the stack trace.
4.  **Context Synthesis:** The database returns a curated corpus of contextual analytics, past patches, and raw code snippets, assembling them within the **RAG Pipeline** to provide complete situational awareness.

### Loop C: Self-Diagnosis & Test Authoring
1.  **AI Orchestration:** The **RAG Pipeline** sends the context-rich prompt matrix directly into the **Test Author**.
2.  **Test Synthesis:** The underlying AI model evaluates the failure, forms a hypothesis about the underlying defect, and authors new test code — or refines tests it previously wrote — designed to reproduce, isolate, and characterize the failing behavior. It does not touch, diff, or otherwise modify the application's existing source files.
3.  **Report Delivery:** Once Celery finishes running the authored tests and compiling results, the Python service POSTs a structured summary report directly to a dedicated Go endpoint (**`/ai-callback`**), including the same workflow ID so Go can correlate the report with the correct pipeline run. The report covers what was investigated, the tests written, which passed/failed, and recommended fixes framed as suggestions for a human to review — not as an applied change. Because a plain HTTP request has no delivery guarantee if Go is briefly unreachable (e.g. mid-restart), this callback includes retry-with-backoff logic on the Python side.
4.  **Report Surfacing:** The Go Orchestrator receives the summary report and makes it available for human review — e.g., posting it as a PR comment, or storing it for retrieval elsewhere. The Go orchestrator does **not** run `git apply`, commit, or push any AI-authored change to source code. The pipeline only re-runs when a human pushes their own fix.

---

## 6. Summary of System Benefits
*   **Fast, Continuous Diagnosis:** Investigation and reproduction of test failures — via newly authored, targeted tests — happens within minutes of a failure, so a human developer starts debugging with a head start instead of from scratch.
*   **Reduced Engineering Cognitive Load:** Developers are spared the initial legwork of reproducing and isolating trivial, repetitive failures, allowing focus on deciding and implementing the actual fix.
*   **Institutional Memory:** By logging every error, vectorizing its context, and capturing successful human fixes alongside the AI's own diagnostic tests inside `pgvector`, the system gets progressively better at pinpointing failures with every single build cycle.
*   **Simplified Coordination via Monorepo:** Because the orchestrator and AI Engine ship from one repository with one CI pipeline and one release cadence, changes to their shared contract (job payload, workflow ID, callback shape) are reviewed and merged atomically instead of coordinated across separate repos and release schedules.