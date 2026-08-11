package types

import (
	"encoding/json"
	"fmt"
)

// A Task can be one of:
// - AIEngineResponse
// - pullRequest
type Task interface {
	TaskType()
}

// The response from the AI Engine.
// Response should come with a HMAC-Signature-256 header.
// Contains the tests and the summary.
type AIEngineResponse struct {
	Wfid int
	Done bool
	// Tests are included if the workflow needs to continue.
	// Summary is included only if the workflow is complete.
	Tests   []byte
	Summary string
}

func (aier AIEngineResponse) TaskType()

type PullRequest struct {
	Number  int    `json:"number"`
	Action  string `json:"action"`
	Branch  string `json:"branch"`
	Title   string `json:"title"`
	Body    string `json:"body"`
	HeadSHA string `json:"headsha"`
	BaseSHA string `json:"basesha"`
	Merged  bool   `json:"merged"`
}

// Populates fields from a byte slice
func (pr *PullRequest) UnmarshalpullRequest(data []byte) error {
	var temp struct {
		Action      string `json:"action"`
		Number      int    `json:"number"`
		PullRequest struct {
			Title string `json:"title"`
			Body  string `json:"body"`
			Head  struct {
				Ref string `json:"ref"`
				Sha string `json:"sha"`
			} `json:"head"`
			Base struct {
				Sha string `json:"sha"`
			} `json:"base"`
			Merged bool `json:"merged"`
		} `json:"pull_request"`
	}

	err := json.Unmarshal(data, &temp)
	if err != nil {
		return fmt.Errorf("Failed to unmarshal json data: %w", err)
	}

	pr.Number = temp.Number
	pr.Action = temp.Action
	pr.Branch = temp.PullRequest.Head.Ref
	pr.Title = temp.PullRequest.Title
	pr.Body = temp.PullRequest.Body
	pr.HeadSHA = temp.PullRequest.Head.Sha
	pr.BaseSHA = temp.PullRequest.Base.Sha
	pr.Merged = temp.PullRequest.Merged

	return nil
}

func (pr PullRequest) TaskType()
