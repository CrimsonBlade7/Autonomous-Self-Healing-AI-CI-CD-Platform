package pipelines

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/CrimsonBlade7/Autonomous-AI-CI-CD-Platform/orchestrator/internal/dockertools"
	"github.com/CrimsonBlade7/Autonomous-AI-CI-CD-Platform/orchestrator/internal/types"
	"github.com/CrimsonBlade7/Autonomous-AI-CI-CD-Platform/orchestrator/internal/wstools"
	"github.com/moby/moby/client"
)

type Workflow struct {
	wfid    uint // The pr number
	pullReq types.PullRequest
	path    string
	Jobs    chan Job
	cleanup func() error
}

type Job uint

const (
	OPEN Job = iota
	CLOSE
	SYNC
	RUN_TESTS
)

// Creates a new workflow. Path and cleanup function are are uninitialized by default.
// Path and cleanup are initialized by the OPEN job.
func newWorkflow(pr types.PullRequest) (*Workflow, error) {
	wf := Workflow{
		wfid:    pr.Number,
		pullReq: pr,
		Jobs:    make(chan Job),
	}

	return &wf, nil
}

func (wf *Workflow) update(pr types.PullRequest) {
	wf.pullReq = pr
}

// Starts the job pipeline. Handles incoming jobs.
func (wf *Workflow) runWorkflow(ctx context.Context, cli *client.Client) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case job := <-wf.Jobs:
			switch job {
			case OPEN:
				p, clean, err := wstools.InitWorkspace(ctx, wf.pullReq, &wstools.GithubClient{})
				wf.path = p
				wf.cleanup = clean
				if err != nil {
					cleanerr := clean()
					if cleanerr != nil {
						return fmt.Errorf("Failed to cleanup workspace: %w", cleanerr)
					}
					return fmt.Errorf("Failed to create a temporary workspace: %w", err)
				}

				// TODO: handoff to send http request

			case CLOSE:
				// TODO: implement close pr
				return nil

			case SYNC:
				// TODO: implement sync job

			case RUN_TESTS:
				// TODO: insert tests from rag pipeline
				err := wstools.InsertTests()
				if err != nil {
					slog.Error("Failed to insert tests", "error", err)
					continue
				}
				nameFormatter := strings.NewReplacer("/", "-", "|", "-", "<", "-", ">", "-", "\"", "-")
				wsName := nameFormatter.Replace(fmt.Sprintf("%s-%v", wf.pullReq.Branch, wf.wfid))
				tag, err := dockertools.BuildImage(ctx, cli, wsName, wf.pullReq.HeadSHA, wf.path, &dockertools.RealTarBuilder{})
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

				// TODO: return logs

			default:
				panic(fmt.Sprintf("Unsupported job type: %v", job))
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
