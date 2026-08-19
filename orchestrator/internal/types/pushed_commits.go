package types

import (
	"sync"
)

type PushedCommits struct {
	pushedCommits map[int]string
	mutex         sync.RWMutex
}

func (pc *PushedCommits) Get(num int) (string, bool) {
	pc.mutex.RLock()
	defer pc.mutex.RUnlock()
	sha, exists := pc.pushedCommits[num]
	return sha, exists
}

func (pc *PushedCommits) Set(num int, sha string) {
	pc.mutex.Lock()
	defer pc.mutex.Unlock()
	pc.pushedCommits[num] = sha
}

func (pc *PushedCommits) Remove(num int) {
	pc.mutex.Lock()
	defer pc.mutex.Unlock()
	delete(pc.pushedCommits, num)
}

func NewPushedCommits() *PushedCommits {
	return &PushedCommits{
		pushedCommits: make(map[int]string),
		mutex:         sync.RWMutex{},
	}
}
