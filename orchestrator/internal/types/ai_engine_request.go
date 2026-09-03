package types

import (
	"time"
)

// The request sent to the AI Engine.
// Request should come with a HMAC-Signature-256.
// Contains the pull request and the logs.
type AIEngineRequest struct {
	Wfid int // Mandatory

	// Current pull request
	PullRequest PullRequest

	// Test results
	Stdout    string
	Stderr    string
	StartTime time.Time
	EndTime   time.Time
	Errors    string // Compile or entry command errors
	Status    string // One of "created", "running", "paused", "restarting", "removing", "exited", or "dead"
	OOMKilled bool   // Killed: out of memory
	ExitCode  int
}
