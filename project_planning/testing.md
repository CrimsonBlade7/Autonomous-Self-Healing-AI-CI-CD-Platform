I can’t create files directly in the current read-only mode, so I’ll give you the full markdown document content here in a clean format you can drop into a new notes file.

# Test Generation Loop Plan (Logs -> Tests -> Orchestrator -> Logs)

Date: 2026-08-04

## 1. Goal
Design and implement a closed-loop workflow where:
1. We ingest failing logs.
2. We generate tests and a human-readable test description from those logs.
3. We send generated tests to the orchestrator.
4. The orchestrator runs those tests and returns execution logs.
5. We use returned logs for next-step analysis (and later code-fix generation).

## 2. Current State (What Exists Today)

### 2.1 Log Parsing and User-Facing Result API
Implemented:
1. Structured log parsing is available.
2. User-facing parse results are returned through analyze/result endpoints.
3. Response includes fields like error type, failing test, stack trace lines, signature, and metadata.
4. Redaction logic exists for sensitive/high-variance tokens.

### 2.2 Test Creator (Current Meaning)
Implemented:
1. A test fixture creator process exists for parser validation.
2. This currently creates parser test fixtures (input log + expected parse output), not runtime regression tests for user repositories.

Important distinction:
1. Existing test creator validates parser quality.
2. It does not yet generate executable repo test files from failures in live runtime flow.

### 2.3 RAG and Patch Components
Current status:
1. RAG pipeline is still a planned placeholder.
2. Patch generation is still a planned placeholder.
3. Worker chaining is planned, not fully wired end-to-end.

### 2.4 Orchestrator Integration Point
Current status:
1. Orchestrator pipeline has a TODO location where AI-generated tests can be inserted before build/test execution.
2. Webhook ingestion and job start flow are already present.

## 3. Clarified Target Workflow

## 3.1 End-to-End Loop
1. Orchestrator runs CI job and captures logs.
2. Logs are sent to AI engine.
3. AI engine parses logs into structured failure context.
4. AI engine generates:
   - regression test patch
   - test description and rationale
5. AI engine returns generated test package to orchestrator.
6. Orchestrator applies test patch in workspace/container.
7. Orchestrator runs tests.
8. Orchestrator returns new logs and result status.
9. AI engine decides next step:
   - refine tests
   - generate code fix
   - or stop if validated

## 3.2 Why This Is Better
1. Guarantees failure is reproducible as a test first.
2. Separates “test synthesis” from “code repair” for safer automation.
3. Provides clearer user visibility into what the AI is asserting.

## 4. API Contracts Needed for Test Generation

### 4.1 AI Engine Input Contract (from Orchestrator)
Required fields:
1. job_id
2. repository info (name/url/branch/head SHA/base SHA)
3. raw_log
4. optional parsed_error override
5. optional repo_context snippet

### 4.2 AI Engine Output Contract (to Orchestrator)
Required fields:
1. job_id
2. test_patch (unified diff)
3. test_description (human-readable summary)
4. test_targets (files/frameworks)
5. confidence
6. fallback_reason (if low confidence)
7. parser metadata used to generate tests

### 4.3 Orchestrator Execution Report Contract (back to AI Engine)
Required fields:
1. job_id
2. apply_status
3. test_run_status
4. exit_code
5. stdout
6. stderr
7. changed_files_applied

## 5. Component Responsibilities

### 5.1 AI Engine
1. Parse logs.
2. Build failure signature + context.
3. Generate candidate tests.
4. Emit patch + explanation.
5. Analyze returned run logs.

### 5.2 Orchestrator
1. Accept test patch payload.
2. Apply patch safely in isolated workspace.
3. Run test command(s).
4. Collect and return logs + status.

## 6. Safety and Quality Requirements

1. Never overwrite existing tests without explicit targeting rules.
2. Keep generated tests minimal and failure-focused.
3. Redact secrets/paths/tokens in logs shown to users.
4. Include confidence and fallback reason in every generation response.
5. Hard-limit patch size and number of generated files per cycle.

## 7. Execution Plan (Step-by-Step)

### Step 1: Define Contracts
1. Add request/response models for test generation and execution reports.
2. Version the contract (for example v1).

### Step 2: Build Test Generation Service
1. Add service to transform parsed errors into test prompts.
2. Generate unified diff test patches.
3. Generate test descriptions.

### Step 3: Add Orchestrator Hook
1. Implement TODO insertion point to request generated tests from AI engine.
2. Apply returned patch.
3. Execute tests and collect logs.

### Step 4: Close the Loop
1. Post execution report/logs back to AI engine.
2. Decide next phase (retry/refine/fix code).

### Step 5: Add End-to-End Tests
1. Simulate failed logs -> generated tests -> test execution -> returned logs.
2. Validate deterministic behavior and graceful fallbacks.

## 8. Success Criteria

1. Generated tests reproduce failure in isolated run.
2. Orchestrator can apply generated test patch reliably.
3. Returned logs are parsed and actionable.
4. User receives clear test description plus outcome.
5. Low-confidence cases are explicitly flagged, not silently auto-applied.

## 9. What We Have vs What Is Missing

Implemented now:
1. Log parse + user-facing structured response.
2. Parser fixture-based validation workflow.
3. API response documentation and redaction behavior.

Missing for the desired feature:
1. Runtime test-generation engine.
2. Orchestrator endpoint integration for generated tests.
3. Execution report callback contract.
4. End-to-end closed-loop runtime tests.

## 10. Final Clarification
Your understanding is correct for the desired architecture:
1. Use logs to generate tests and test descriptions.
2. Send tests to orchestrator.
3. Orchestrator runs tests and returns logs.
4. Use returned logs for next iteration and eventual code-fix stage.

The only correction is that this is the target flow, not the fully implemented flow yet.