package pipelines

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/CrimsonBlade7/Autonomous-Self-Healing-AI-CI-CD-Platform/orchestrator/internal/dockertools"
	"github.com/CrimsonBlade7/Autonomous-Self-Healing-AI-CI-CD-Platform/orchestrator/internal/types"
	"github.com/CrimsonBlade7/Autonomous-Self-Healing-AI-CI-CD-Platform/orchestrator/internal/wstools"
	"github.com/moby/moby/client"
)

type Workflow struct {
	wfid    uint
	pullReq types.PullRequest
	path    string
	Jobs    chan Job
	cleanup func() error
}

type JobType uint

const (
	INIT JobType = iota
	UPDATE
	SYNC // TODO: handle branch syncing
)

type Job struct {
	Jt JobType
}

func newWorkflow(ctx context.Context, pr types.PullRequest) (Workflow, error) {
	p, clean, err := wstools.InitWorkspace(ctx, pr, &wstools.GithubClient{})
	wf := Workflow{
		wfid:    pr.Number,
		pullReq: pr,
		path:    p,
		Jobs:    make(chan Job),
		cleanup: clean,
	}
	if err != nil {
		cleanerr := clean()
		if cleanerr != nil {
			return Workflow{}, fmt.Errorf("Failed to cleanup workspace: %w", cleanerr)
		}
		return Workflow{}, fmt.Errorf("Failed to create a temporary workspace: %w", err)
	}
	return wf, nil
}

// Starts the job pipeline. Handles incoming jobs.
func (wf *Workflow) StartWorkflow(ctx context.Context, cli *client.Client) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case job := <-wf.Jobs:
			switch job.Jt {
			case INIT:

			case UPDATE:

				// TODO: insert tests from rag pipeline
				err := wstools.InsertTests()
				if err != nil {
					slog.Error("Failed to insert tests", "error", err)
					continue
				}

				tag, err := dockertools.BuildImage(ctx, cli, string(wf.wfid), wf.pullReq.HeadSHA, wf.path, &dockertools.RealTarBuilder{})
				if err != nil {
					slog.Error("Failed to build image", "error", err)
					continue
				}
				logs, err := dockertools.RunContainer(ctx, cli, tag)
				if err != nil {
					slog.Error("Failed to build container", "error", err)
					continue
				}
				defer logs.Close()

				fmt.Println(logs)

			default:
				panic(fmt.Sprintf("Unsupported job type: %v", job.Jt))
			}
		}
	}
}

func (wf *Workflow) GetWfid() uint {
	return wf.wfid
}
func (wf *Workflow) GetPullRequest() types.PullRequest {
	return wf.pullReq
}
func (wf *Workflow) GetPath() string {
	return wf.path
}

/*
	url := "https://github.com/CrimsonBlade7/CI-CD-Test.git"
	sha := "f27e0af69b5eaddb08a22d7542ffb584f19e0f71"
*/
