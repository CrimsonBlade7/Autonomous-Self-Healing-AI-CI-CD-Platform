package types

type MsgPkg struct {
	Wfid       uint
	PullReq    PullRequest
	Path       string
	Stdout     string
	Stderr     string
	AttemptNum uint
}
