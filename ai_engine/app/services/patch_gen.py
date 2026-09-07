# Phase 4: Patch Generator
#
# Sends the RAG context window to Google Gemini and extracts the resulting
# unified diff patch.
#
# Input:
#   context_window: str  — assembled by rag_pipeline (error + historical context)
#   failing_code:   str  — the source file(s) containing the failing logic
#   error_logs:     str  — raw log output for direct reference in the prompt
#
# Output:
#   patch: str  — a unified diff (--- a/file / +++ b/file format) ready to be
#                 applied by the Go orchestrator via `git apply`.
#
# The LLM is instructed to:
#   - Fix only the failing assertion/logic, not rewrite unrelated code.
#   - Output ONLY the diff block with no surrounding prose.
#   - Avoid introducing new imports unless strictly necessary.
#
# Implementation arrives in Phase 4.
