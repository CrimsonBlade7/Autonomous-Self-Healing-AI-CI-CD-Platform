package types

import (
	"github.com/google/uuid"
)

type JobType int

const (
	INITIALIZE_WORKSPACE JobType = iota
	UPDATE_WORKSPACE
)

type Job struct {
	PullReqProc PullRequestProcess
	Jt          JobType
	ID          uuid.UUID
}
