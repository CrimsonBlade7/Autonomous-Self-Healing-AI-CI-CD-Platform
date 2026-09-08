"""Generate a reviewable fix proposal from a failed generated test.

The proposal is deliberately text-only until the orchestrator has a source
checkout and an explicit approval/apply step.  This keeps failed-test feedback
useful without allowing an LLM to mutate a repository implicitly.
"""

from __future__ import annotations

from typing import TypedDict

from app.services.log_analyzer import ParsedError


class FixProposal(TypedDict):
	suggestions: str
	documentation: str


def generate_fix_proposal(
	parsed_error: ParsedError,
	context_window: str = "",
	pr_description: str = "",
) -> FixProposal:
	"""Turn a failed test and retrieved context into actionable user guidance."""
	location = parsed_error["failing_test"]
	error = parsed_error["error_signature"]
	historical = "Historical incidents were included in the RAG context." if context_window else "No historical incident matched this failure."
	request = pr_description.strip() or "No additional acceptance criteria were supplied."

	suggestions = (
		f"Failure: {error}\n"
		f"Likely target: {location}.\n"
		"Inspect the failing implementation and add the smallest code change "
		"that makes the generated regression test pass.\n"
		f"{historical}"
	)
	documentation = (
		"# AI Fix Review\n\n"
		"## PR Intent\n"
		f"{request}\n\n"
		"## Failure Context\n"
		f"- Test: `{location}`\n"
		f"- Signature: `{error}`\n\n"
		"## Next Fix Step\n"
		"Apply and review the minimal source change, then rerun the generated "
		"regression test. The AI engine does not apply source patches automatically.\n"
	)
	return {"suggestions": suggestions, "documentation": documentation}
