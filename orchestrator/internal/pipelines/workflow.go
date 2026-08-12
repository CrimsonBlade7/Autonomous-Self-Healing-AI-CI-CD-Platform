package pipelines

import (
	"context"
	"fmt"
	"io"
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
	wfid             int // The pr number
	pullRequest      types.PullRequest
	Jobs             chan Job
	path             string       // Path to the associated workspace
	cleanWs          func() error // Removes the workspace at path
	attemptNum       int
	currentTestsPath string
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
	JobType int
	Task    types.Task // optional
	Data    []byte     // optional
}

// Creates a new workflow. Path, cleanWs, and cancelWf function are are uninitialized by default.
// Path and cleanup are initialized by the OPEN job.
func newWorkflow(pr types.PullRequest) (wf *Workflow, err error) {
	wf = &Workflow{
		wfid:        pr.Number,
		pullRequest: pr,
		Jobs:        make(chan Job),
		attemptNum:  0,
	}

	return wf, nil
}

// Starts the job pipeline. Handles incoming jobs.
func (wf *Workflow) runWorkflow(ctx context.Context, cli *client.Client) (err error) {
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
				if wf.attemptNum > config.MaxTestPatchingAttempts {
					return fmt.Errorf("Test generation failed: too many attempts")
				}
				wf.attemptNum++
				err := wstools.InsertTests(wf.path, job.Data)
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

				// Process the container
				contInspect, logOut, logErr, err := func() (dockertools.ContainerInspection, string, string, error) {
					contID, logOut, logErr, err := dockertools.RunContainer(ctx, cli, tag)
					if err != nil {
						return dockertools.ContainerInspection{}, "", "", fmt.Errorf("Failed to build container: %w", err)
					}

					// Close the logs and remove container
					// err is updated before this IIFE returns
					defer func() {
						err = logOut.Close()
						if err != nil {
							err = fmt.Errorf("Failed to close out logs: %w", err)
						}
						err = logErr.Close()
						if err != nil {
							err = fmt.Errorf("Failed to close error logs: %w", err)
						}
						err = dockertools.RemoveContainer(ctx, cli, contID)
						if err != nil {
							err = fmt.Errorf("Failed to remove container: %w", err)
						}
					}()

					logOutBytes, err := io.ReadAll(logOut)
					if err != nil {
						return dockertools.ContainerInspection{}, "", "", fmt.Errorf("Failed to read output logs: %w", err)
					}
					logOutString := string(logOutBytes)
					logErrBytes, err := io.ReadAll(logErr)
					if err != nil {
						return dockertools.ContainerInspection{}, "", "", fmt.Errorf("Failed to read error logs: %w", err)
					}
					logErrString := string(logErrBytes)

					inspect, err := dockertools.InspectContainer(ctx, cli, contID)
					if err != nil {
						return dockertools.ContainerInspection{}, "", "", fmt.Errorf("Failed to inspect container: %w", err)
					}

					return inspect, logOutString, logErrString, err
				}()
				if err != nil {
					slog.Error("Container failed", "error", err)
				}

				err = servertools.SendRequestAIEngine(ctx, "logs", types.AIEngineRequest{
					Wfid:     wf.wfid,
					Stdout:   logOut,
					Stderr:   logErr,
					StartTime: contInspect.StartTime,
					EndTime: contInspect.EndTime,
					Errors: contInspect.Errors,
					Status: contInspect.Status,
					OOMKilled: contInspect.OOMKilled,
					ExitCode: contInspect.ExitCode,
				})

			default:
				panic(fmt.Sprintf("Unsupported job type: %v", job))
			}
		}
	}
}

func (wf *Workflow) GetWfid() int {
	return wf.wfid
}
func (wf *Workflow) GetPullRequest() types.PullRequest {
	return wf.pullRequest
}
func (wf *Workflow) GetPath() string {
	return wf.path
}
