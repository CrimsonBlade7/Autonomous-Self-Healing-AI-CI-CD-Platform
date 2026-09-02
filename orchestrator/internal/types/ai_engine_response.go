package types

// The response from the AI Engine.
// Response should come with a HMAC-Signature-256 header.
// Contains the tests and the summary.
type AIEngineResponse struct {
	Wfid        int
	PullRequest PullRequest

	// Tests are ignored if Done.
	TestName string
	Tests    []byte

	// Done should always be accompanied by Summary.
	Done    bool
	Summary string
}
