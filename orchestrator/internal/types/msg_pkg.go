package types

import ()

type MsgPkg struct {
	Wfid    uint
	PullReq PullRequest
	Path    string
}
