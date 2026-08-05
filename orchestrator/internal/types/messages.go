package types

// The request sent to the AI Engine.
// Request should come with a HMAC-Signature-256.
type Request struct {
	Wfid    uint
	PullReq PullRequest
	Path    string // TODO: remove?
	Stdout  string
	Stderr  string
}

// The response from the AI Engine.
// Response should come with a HMAC-Signature-256 header.
type Response struct {
	Wfid  uint
	Tests []byte
	// Iff summary == "", the corresponding workflow will conclude.
	Summary string
}
