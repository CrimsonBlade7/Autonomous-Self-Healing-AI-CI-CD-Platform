package pipelines

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/CrimsonBlade7/Autonomous-AI-CI-CD-Platform/orchestrator/internal/config"
	"github.com/CrimsonBlade7/Autonomous-AI-CI-CD-Platform/orchestrator/internal/dockertools"
	"github.com/CrimsonBlade7/Autonomous-AI-CI-CD-Platform/orchestrator/internal/servertools"
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
	attemptNum       uint
	currentTestsPath string // TODO: Save the tests to a seperate folder or have the ai engine send the final version at the end
}

const (
	OPEN = iota
	CLOSE
	UPDATE_PR
	RUN_TESTS
	COMMIT_PUSH
)

type Job struct {
	// Can be one of:
	// - OPEN
	// - CLOSE
	// - UPDATE_PR
	// - RUN_TESTS
	// - COMMIT_PUSH
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
		attemptNum:  0,
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
				wf.attemptNum = 0
				path, clean, err := wstools.InitWorkspace(ctx, wf.pullRequest, &wstools.GithubClient{})
				if err != nil {
					cleanerr := clean()
					if cleanerr != nil {
						return fmt.Errorf("Failed to cleanup workspace: %w", cleanerr)
					}
					return fmt.Errorf("Failed to create a temporary workspace: %w", err)
				}
				wf.path = path
				wf.cleanWs = clean

				err = servertools.SendRequestAIEngine(ctx, "open", types.AIEngineRequest{
					Wfid:        wf.wfid,
					PullRequest: wf.pullRequest,
				})
				if err != nil {
					return fmt.Errorf("Failed to send request to ai engine: %w", err)
				}

			case CLOSE:
				err := wf.cleanWs()
				if err != nil {
					return fmt.Errorf("Failed to clean up workspace: %w", err)
				}
				err = servertools.SendRequestAIEngine(ctx, "close", types.AIEngineRequest{Wfid: wf.wfid})
				if err != nil {
					return fmt.Errorf("Failed to send request to ai engine: %w", err)
				}
				return nil

			case UPDATE_PR:
				wf.attemptNum = 0
				pr, ok := job.Task.(types.PullRequest)
				if !ok {
					panic("Sync should always come from a pull request.")
				}
				wf.pullRequest = pr
				err := servertools.SendRequestAIEngine(ctx, "sync", types.AIEngineRequest{Wfid: wf.wfid})
				if err != nil {
					return fmt.Errorf("Failed to send request to ai engine: %w", err)
				}

			case RUN_TESTS:
				// TODO: insert tests from rag pipeline
				if wf.attemptNum > config.MaxTestPatchingAttempts {
					return fmt.Errorf("Test generation failed: too many attempts")
				}
				wf.attemptNum++
				err := wstools.InsertTests(wf.path, job.Data, true, nil)
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
	return wf.pullRequest
}
func (wf *Workflow) GetPath() string {
	return wf.path
}
