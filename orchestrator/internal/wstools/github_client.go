package wstools

import (
	"context"
	"errors"
	"fmt"

	"github.com/CrimsonBlade7/Autonomous-AI-CI-CD-Platform/orchestrator/internal/config"
	"github.com/CrimsonBlade7/Autonomous-AI-CI-CD-Platform/orchestrator/internal/types"
	"github.com/go-git/go-git/v6"
	gitConfig "github.com/go-git/go-git/v6/config"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/object"
)

type GithubClient struct{}

var CancelLoopErr error = errors.New("This commit originates from this platform. Pull canceled to prevent infinite loops.")

// Initializes the repository at path. Returns CancelLoopErr if the committer was this service to prevent infinite loops.
func (c *GithubClient) InitRepo(ctx context.Context, path string, pr types.PullRequest) error {
	repo, err := git.PlainInit(path, false)
	if err != nil {
		return fmt.Errorf("Failed to initialize repo %s at %s: %w", config.RepositoryUrl, pr.HeadSHA, err)
	}

	_, err = repo.CreateRemote(&gitConfig.RemoteConfig{
		Name: "origin",
		URLs: []string{config.RepositoryUrl},
	})
	if err != nil {
		return fmt.Errorf("Failed to create remote %s at %s: %w", config.RepositoryUrl, pr.HeadSHA, err)
	}

	err = c.UpdateWorkspace(ctx, path, pr)
	if err == CancelLoopErr {
		return CancelLoopErr
	} else if err != nil {
		return fmt.Errorf("Failed to update the workspace: %w", err)
	}

	return nil
}

func (c *GithubClient) CommitPush(commitMsg, wsPath, branch, sha string) error {

	repo, err := git.PlainOpen(wsPath)
	if err != nil {
		return fmt.Errorf("Failed to open the repository at %s: %w", wsPath, err)
	}

	wt, err := checkoutSHA(repo, sha)
	if err != nil {
		return fmt.Errorf("Failed to checkout the sha: %w", err)
	}

	err = wt.AddWithOptions(&git.AddOptions{All: true})
	if err != nil {
		return fmt.Errorf("Failed to add changes: %w", err)
	}

	wt.Commit(commitMsg, &git.CommitOptions{
		Author: &object.Signature{
			Name: config.BotName,
		},
	})

	remote, err := repo.Remote("origin")
	if err != nil {
		return fmt.Errorf("Failed to get remote: %w", err)
	}

	// TODO: verify if i did this correctly
	err = repo.Push(&git.PushOptions{
		RemoteName: "origin",
		RemoteURL:  remote.Config().URLs[0],
		RefSpecs: []gitConfig.RefSpec{
			gitConfig.RefSpec(gitConfig.RefSpec(fmt.Sprintf("refs/heads/temp-branch:refs/heads/%s", branch))),
		},
		Force: true,
	})
	if err != nil {
		return fmt.Errorf("Failed to push changes: %w", err)
	}
	return nil
}

func (c *GithubClient) UpdateWorkspace(ctx context.Context, wsPath string, pr types.PullRequest) error {
	// fetch specific sha
	repo, err := git.PlainOpen(wsPath)
	if err != nil {
		return fmt.Errorf("Failed to open the repository at %s: %w", wsPath, err)
	}

	err = repo.FetchContext(ctx, &git.FetchOptions{
		RemoteURL: config.RepositoryUrl,
		RefSpecs:  []gitConfig.RefSpec{gitConfig.RefSpec(fmt.Sprintf("%s:%s", pr.HeadSHA, "refs/heads/temp-branch"))},
		Depth:     1,
	})
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		// attempt to fetch branch - for when "fetch by sha" is disabled
		err = repo.FetchContext(ctx, &git.FetchOptions{
			RemoteURL: config.RepositoryUrl,
			RefSpecs:  []gitConfig.RefSpec{gitConfig.RefSpec(fmt.Sprintf("refs/heads/%s:refs/heads/temp-branch", pr.Branch))},
		})
		if err != nil {
			return fmt.Errorf("Failed to fetch commit %s at %s: %w", config.RepositoryUrl, pr.HeadSHA, err)
		}
	}

	commit, err := repo.CommitObject(plumbing.NewHash(pr.HeadSHA))
	if err != nil {
		return fmt.Errorf("Failed to retrieve commit from repo: %w", err)
	}
	if commit.Committer.Name == config.BotName {
		return CancelLoopErr
	}

	_, err = checkoutSHA(repo, pr.HeadSHA)
	if err != nil {
		return fmt.Errorf("Failed to checkout the sha: %w", err)
	}
	return nil
}

func checkoutSHA(repo *git.Repository, sha string) (*git.Worktree, error) {
	wt, err := repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("Failed to get worktree: %w", err)
	}

	hash, ok := plumbing.FromHex(sha)
	if !ok {
		return nil, fmt.Errorf("Failed to hash the sha: %w", err)
	}
	err = wt.Checkout(&git.CheckoutOptions{
		Hash:  hash,
		Force: true,
	})
	if err != nil {
		return nil, fmt.Errorf("Failed to checkout: %w", err)
	}
	return wt, nil
}
