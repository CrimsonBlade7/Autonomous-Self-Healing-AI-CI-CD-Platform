package types

import (
	"sync"
)

type PushedCommits struct {
	// Pull request number -> commit SHA
	pushedCommits map[int]map[string]struct{}
	mutex         sync.RWMutex
}

// Checks if the sha exists at num.
// Removes the sha if it exists.
func (pc *PushedCommits) IsSelfPush(num int, sha string) bool {
	pc.mutex.RLock()
	defer pc.mutex.RUnlock()

	shas, prExists := pc.pushedCommits[num]
	if !prExists {
		return false
	}

	_, shaExists := shas[sha]
	if shaExists {
		delete(shas, sha)
	}
	return shaExists
}

func (pc *PushedCommits) Add(num int, sha string) {
	pc.mutex.Lock()
	defer pc.mutex.Unlock()

	if pc.pushedCommits[num] == nil {
		pc.pushedCommits[num] = make(map[string]struct{})
	}
	pc.pushedCommits[num][sha] = struct{}{}
}

func (pc *PushedCommits) Remove(num int) {
	pc.mutex.Lock()
	defer pc.mutex.Unlock()
	delete(pc.pushedCommits, num)
}

func NewPushedCommits() *PushedCommits {
	return &PushedCommits{
		pushedCommits: make(map[int]map[string]struct{}),
		mutex:         sync.RWMutex{},
	}
}
