package types

// The request sent to the AI Engine.
// Request should come with a HMAC-Signature-256.
// Contains the pull request and the logs.
type AIEngineRequest struct {
	Wfid        int // Mandatory
	PullRequest PullRequest
	Stdout      string
	Stderr      string
	ExitCode    int
}
