package ci

import (
	_ "embed"
	_ "encoding/json"
	"testing"

	"github.com/CrimsonBlade7/Autonomous-Self-Healing-AI-CI-CD-Platform/orchestrator/internal/types"
)

//go:embed testdata/pr_opened.json
var prOpenedPayload []byte

//go:embed testdata/fake_pr.json
var missingFieldsPayload []byte

func TestPullRequest_unmarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		wantErr  bool
		expected types.PullRequest
	}{
		{
			name:    "Standard opened PR event",
			data:    prOpenedPayload,
			wantErr: false,
			expected: types.PullRequest{
				Name:    "hello-world",
				Action:  "opened",
				Url:     "https://github.com/octocat/Hello-World.git",
				Branch:  "feature/retry-logic",
				Title:   "Add retry logic to fetch client",
				Body:    "This adds exponential backoff to the fetch client.",
				HeadSHA: "6dcb09b5b57875f334f61aebed695e2e4193db5",
				BaseSHA: "e5bd3914e2e596debea16f433f57875b5b90bcd",
			},
		},
		{
			name:    "Empty fields",
			data:    missingFieldsPayload,
			wantErr: true,
			expected: types.PullRequest{},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var pr types.PullRequest
			gotErr := pr.UnmarshalJSON(test.data)
			if test.wantErr {
				if gotErr == nil {
					t.Errorf("%s: Expected an error, but no errors occured", test.name)
				} else {
					// pass
				}
			} else {
				if gotErr != nil {
					t.Errorf("%s: Unexpected error: %v", test.name, gotErr)
				} else {
					if pr == test.expected {
						// pass
					} else {
						t.Errorf("%s: Result does not match expected value", test.name)
					}
				}
			}
		})
	}
}
