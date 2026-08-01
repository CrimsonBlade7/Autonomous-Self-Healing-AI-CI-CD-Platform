package types

type JobType int

const (
	INITIALIZE_WORKSPACE JobType = iota
	UPDATE_WORKSPACE
)

type Job struct {
	Jt      JobType
	PullReq PullRequest
	ID      uint64
}
