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
	wfid             uint // The pr number
	pullRequest      types.PullRequest
	Jobs             chan Job
	path             string       // Path to the associated workspace
	cleanWs          func() error // Removes the workspace at path
	AttemptNum       uint
	currentTestsPath string // TODO: Save the tests to a seperate folder or have the ai engine send the final version at the end
}

const (
	OPEN = iota
	CLOSE
	SYNC
	EDIT
	RUN_TESTS
)

type Job struct {
	// Can be one of:
	// - OPEN
	// - CLOSE
	// - SYNC
	// - EDIT
	// - RUN_TESTS
	JobType uint
	Task    types.Task // optional
	Data    []byte     // optional
}

// Creates a new workflow. Path, cleanWs, and cancelWf function are are uninitialized by default.
// Path and cleanup are initialized by the OPEN job.
func newWorkflow(pr types.PullRequest) (*Workflow, error) {
	wf := Workflow{
		wfid:        pr.Number,
		pullRequest: pr,
		Jobs:        make(chan Job),
		AttemptNum:  0,
	}

	return &wf, nil
}

// Starts the job pipeline. Handles incoming jobs.
func (wf *Workflow) runWorkflow(ctx context.Context, cli *client.Client) error {
	for {
		select {
		case <-ctx.Done():

			return nil
		case job := <-wf.Jobs:
			switch job.JobType {
			case OPEN:
				p, clean, err := wstools.InitWorkspace(ctx, wf.pullRequest, &wstools.GithubClient{})
				wf.path = p
				wf.cleanWs = clean
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
				pr, ok := job.Task.(types.PullRequest)
				if !ok {
					panic("Edit should always come from a pull request.")
				}
				wf.pullRequest = pr

			case RUN_TESTS:
				// TODO: insert tests from rag pipeline
				err := wstools.InsertTests(job.Data, true, nil)
				if err != nil {
					slog.Error("Failed to insert tests", "error", err)
					continue
				}
				nameFormatter := strings.NewReplacer("/", "-", "|", "-", "<", "-", ">", "-", "\"", "-")
				wsName := nameFormatter.Replace(fmt.Sprintf("%s-%v", wf.pullRequest.Branch, wf.wfid))
				tag, err := dockertools.BuildImage(ctx, cli, wsName, wf.pullRequest.HeadSHA, wf.path, &dockertools.RealTarBuilder{})
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

			case EDIT:
				pr, ok := job.Task.(types.PullRequest)
				if !ok {
					panic("Edit should always come from a pull request.")
				}
				wf.pullRequest = pr

			default:
				panic(fmt.Sprintf("Unsupported job type: %v", job))
			}
		}
	}
}

func (wf *Workflow) GetWfid() uint {
	return wf.wfid
}
func (wf *Workflow) GetpullRequest() types.PullRequest {
	return wf.pullRequest
}
func (wf *Workflow) GetPath() string {
	return wf.path
}
