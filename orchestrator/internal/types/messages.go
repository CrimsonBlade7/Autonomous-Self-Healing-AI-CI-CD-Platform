package types

// The request sent to the AI Engine.
// Request should come with a HMAC-Signature-256.
// Contains the pull request and the logs.
type Request struct {
	Wfid    uint
	PullReq PullRequest
	Stdout  string
	Stderr  string
}

// The response from the AI Engine.
// Response should come with a HMAC-Signature-256 header.
// Contains the tests and the summary.
type Response struct {
	Wfid uint
	// Tests and summary are mutually exclusive. // TODO: only true if i save a copy of the tests locally
	// Tests are included if the workflow needs to continue.
	// Summary is included if the workflow is complete.
	Tests   []byte
	Summary string
}
