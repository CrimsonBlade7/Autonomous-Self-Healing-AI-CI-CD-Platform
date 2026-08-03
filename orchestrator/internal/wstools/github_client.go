package wstools

import (
	"context"
	"errors"
	"fmt"

	"github.com/CrimsonBlade7/Autonomous-Self-Healing-AI-CI-CD-Platform/orchestrator/internal/types"
	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/config"
	"github.com/go-git/go-git/v6/plumbing"
)

type GithubClient struct {}

func (c *GithubClient) CheckoutCommit(ctx context.Context, path string, pr types.PullRequest) error {
	repo, err := git.PlainInit(path, false)
	if err != nil {
		return fmt.Errorf("Failed to initialize repo %s at %s: %w", pr.Url, pr.HeadSHA, err)
	}
	_, err = repo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{pr.Url},
	})
	if err != nil {
		return fmt.Errorf("Failed to create remote %s at %s: %w", pr.Url, pr.HeadSHA, err)
	}

	// fetch specific sha
	err = repo.FetchContext(ctx, &git.FetchOptions{
		RemoteURL: pr.Url,
		RefSpecs:  []config.RefSpec{config.RefSpec(fmt.Sprintf("%s:%s", pr.HeadSHA, "refs/heads/temp-branch"))},
		Depth:     1,
	})
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		// attempt to fetch branch - for when "fetch by sha" is disabled
		err = repo.FetchContext(ctx, &git.FetchOptions{
			RemoteURL: pr.Url,
			RefSpecs:  []config.RefSpec{config.RefSpec(fmt.Sprintf("refs/heads/%s:refs/heads/temp-branch", pr.Branch))},
		})
		if err != nil {
			return fmt.Errorf("Failed to fetch commit %s at %s: %w", pr.Url, pr.HeadSHA, err)
		}
	}

	wt, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("Failed to get worktree: %w", err)
	}

	hash, ok := plumbing.FromHex(pr.HeadSHA)
	if !ok {
		return fmt.Errorf("Failed to hash the sha: %w", err)
	}
	err = wt.Checkout(&git.CheckoutOptions{
		Hash:  hash,
		Force: true,
	})
	if err != nil {
		return fmt.Errorf("Failed to checkout: %w", err)
	}
	return nil
}
