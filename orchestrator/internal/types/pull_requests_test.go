package types

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUnmarshalPullRequest_OpenedFixture(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile(filepath.Join("test_prs", "pr_opened.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	var pr PullRequest
	if err := pr.UnmarshalPullRequest(data); err != nil {
		t.Fatalf("UnmarshalPullRequest: %v", err)
	}

	if pr.Number != 42 {
		t.Errorf("Number = %d, want 42", pr.Number)
	}
	if pr.Action != "opened" {
		t.Errorf("Action = %q, want opened", pr.Action)
	}
	if pr.Branch != "feature/retry-logic" {
		t.Errorf("Branch = %q, want feature/retry-logic", pr.Branch)
	}
	if pr.Title != "Add retry logic to fetch client" {
		t.Errorf("Title = %q", pr.Title)
	}
	if pr.HeadSHA != "6dcb09b5b57875f334f61aebed695e2e4193db5" {
		t.Errorf("HeadSHA = %q", pr.HeadSHA)
	}
	if pr.BaseSHA != "e5bd3914e2e596debea16f433f57875b5b90bcd" {
		t.Errorf("BaseSHA = %q", pr.BaseSHA)
	}
	if pr.Merged {
		t.Error("Merged = true, want false")
	}
}

func TestUnmarshalPullRequest_InvalidJSON(t *testing.T) {
	t.Parallel()

	var pr PullRequest
	if err := pr.UnmarshalPullRequest([]byte("{not-json")); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestUnmarshalPullRequest_FakePayloadLeavesZeroValue(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile(filepath.Join("test_prs", "fake_pr.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	var pr PullRequest
	if err := pr.UnmarshalPullRequest(data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pr != (PullRequest{}) {
		t.Errorf("got %+v, want zero value", pr)
	}
}
