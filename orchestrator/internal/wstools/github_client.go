package wstools

import (
	"context"
	"errors"
	"fmt"

	"github.com/benl1006/Autonomous-CI-Platform/orchestrator/internal/config"
	"github.com/benl1006/Autonomous-CI-Platform/orchestrator/internal/types"
	"github.com/go-git/go-git/v6"
	gitConfig "github.com/go-git/go-git/v6/config"
	"github.com/go-git/go-git/v6/plumbing"
	gitPlumbingClient "github.com/go-git/go-git/v6/plumbing/client"
	"github.com/go-git/go-git/v6/plumbing/transport/http"
)

type GithubClient struct{}

// Initializes the repository in the directory at path.
func (c *GithubClient) InitRepo(ctx context.Context, path string, pr types.PullRequest) (err error) {
	repo, err := git.PlainInit(path, false)
	if err != nil {
		return fmt.Errorf("Failed to initialize repo %s at %s: %w", config.RepositoryUrl, pr.HeadSHA, err)
	}

	if _, err = repo.CreateRemote(&gitConfig.RemoteConfig{
		Name: "origin",
		URLs: []string{config.RepositoryUrl},
	}); err != nil {
		return fmt.Errorf("Failed to create remote %s at %s: %w", config.RepositoryUrl, pr.HeadSHA, err)
	}

	if err := c.UpdateWorkspace(ctx, path, pr); err != nil {
		return fmt.Errorf("Failed to update the workspace: %w", err)
	}

	return nil
}

// Adds, commits, and pushes changes to remote. Returns the new sha and an error.
func (c *GithubClient) AddAllCommitPush(commitMsg, wsPath, branch, sha string) (newSha string, err error) {

	repo, err := git.PlainOpen(wsPath)
	if err != nil {
		return "", fmt.Errorf("Failed to open the repository at %s: %w", wsPath, err)
	}

	wt, err := checkoutSHA(repo, sha)
	if err != nil {
		return "", fmt.Errorf("Failed to checkout the sha: %w", err)
	}

	if err := wt.AddWithOptions(&git.AddOptions{All: true}); err != nil {
		return "", fmt.Errorf("Failed to add changes: %w", err)
	}

	hash, err := wt.Commit(commitMsg, &git.CommitOptions{All: true})
	if err != nil {
		return "", fmt.Errorf("Failed to commit changes: %w", err)
	}
	newSha = hash.String()

	remote, err := repo.Remote("origin")
	if err != nil {
		return "", fmt.Errorf("Failed to get remote: %w", err)
	}

	if err := repo.Push(&git.PushOptions{
		RemoteName:    "origin",
		RemoteURL:     remote.Config().URLs[0],
		ClientOptions: []gitPlumbingClient.Option{gitPlumbingClient.WithHTTPAuth(&http.TokenAuth{Token: config.GithubToken})},
		RefSpecs: []gitConfig.RefSpec{
			gitConfig.RefSpec(gitConfig.RefSpec(fmt.Sprintf("refs/heads/temp-branch:refs/heads/%s", branch))),
		},
		Force: true,
	}); err != nil {
		return "", fmt.Errorf("Failed to push changes: %w", err)
	}
	return newSha, nil
}

// Fetches and checksout the latest changes from remote.
func (c *GithubClient) UpdateWorkspace(ctx context.Context, wsPath string, pr types.PullRequest) (err error) {
	// fetch specific sha
	repo, err := git.PlainOpen(wsPath)
	if err != nil {
		return fmt.Errorf("Failed to open the repository at %s: %w", wsPath, err)
	}

	fetchErr := repo.FetchContext(ctx, &git.FetchOptions{
		RemoteURL:     config.RepositoryUrl,
		ClientOptions: []gitPlumbingClient.Option{gitPlumbingClient.WithHTTPAuth(&http.TokenAuth{Token: config.GithubToken})},
		RefSpecs:      []gitConfig.RefSpec{gitConfig.RefSpec(fmt.Sprintf("%s:%s", pr.HeadSHA, "refs/heads/temp-branch"))},
		Depth:         1,
	})
	if fetchErr != nil && !errors.Is(fetchErr, git.NoErrAlreadyUpToDate) {
		// attempt to fetch branch - for when "fetch by sha" is disabled
		fetchErr = repo.FetchContext(ctx, &git.FetchOptions{
			RemoteURL: config.RepositoryUrl,
			RefSpecs:  []gitConfig.RefSpec{gitConfig.RefSpec(fmt.Sprintf("refs/heads/%s:refs/heads/temp-branch", pr.Branch))},
		})
		if fetchErr != nil {
			return fmt.Errorf("Failed to fetch commit %s at %s: %w", config.RepositoryUrl, pr.HeadSHA, err)
		}
	}

	if _, err = checkoutSHA(repo, pr.HeadSHA); err != nil {
		return fmt.Errorf("Failed to checkout the sha: %w", err)
	}
	return nil
}

func checkoutSHA(repo *git.Repository, sha string) (wt *git.Worktree, err error) {
	wt, err = repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("Failed to get worktree: %w", err)
	}

	hash, ok := plumbing.FromHex(sha)
	if !ok {
		return nil, fmt.Errorf("Failed to hash the sha: %w", err)
	}

	if err = wt.Checkout(&git.CheckoutOptions{
		Hash:  hash,
		Force: true,
	}); err != nil {
		return nil, fmt.Errorf("Failed to checkout: %w", err)
	}
	return wt, nil
}
