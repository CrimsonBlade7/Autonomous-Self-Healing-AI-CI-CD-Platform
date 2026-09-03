package types

import (
	"sync"
	"testing"
)

func TestPushedCommits_AddIsSelfPushAndRemove(t *testing.T) {
	t.Parallel()

	pc := NewPushedCommits()
	pc.Add(7, "abc123")
	pc.Add(7, "def456")

	if !pc.IsSelfPush(7, "abc123") {
		t.Fatal("expected abc123 to be recognized as a self-push")
	}
	if pc.IsSelfPush(7, "abc123") {
		t.Fatal("self-push SHA should be consumed on first match")
	}
	if !pc.IsSelfPush(7, "def456") {
		t.Fatal("expected def456 to still be present")
	}
	if pc.IsSelfPush(8, "abc123") {
		t.Fatal("unknown PR number should not match")
	}

	pc.Add(9, "deadbeef")
	pc.Remove(9)
	if pc.IsSelfPush(9, "deadbeef") {
		t.Fatal("removed PR should not match")
	}
}

func TestPushedCommits_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	pc := NewPushedCommits()
	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			pc.Add(1, "sha")
			_ = pc.IsSelfPush(1, "sha")
			pc.Add(1, "sha")
		}(i)
	}
	wg.Wait()
}
