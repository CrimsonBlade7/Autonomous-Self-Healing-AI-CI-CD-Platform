package types

import (
	_ "embed"
	_ "encoding/json"
	"fmt"
	"testing"
)

//go:embed test_prs/pr_opened.json
var prOpenedPayload []byte

//go:embed test_prs/fake_pr.json
var missingFieldsPayload []byte

func TestPullRequest_unmarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		wantErr  bool
		expected PullRequest
	}{
		{
			name:    "Standard opened PR event",
			data:    prOpenedPayload,
			wantErr: false,
			expected: PullRequest{
				Name:    "Hello-World",
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
			expected: PullRequest{},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var pr PullRequest
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
						fmt.Printf("%+v", pr)
						t.Errorf("%s: Result does not match expected value\nExpected: %+v\nActual: %+v", test.name, pr, test.expected)
					}
				}
			}
		})
	}
}
