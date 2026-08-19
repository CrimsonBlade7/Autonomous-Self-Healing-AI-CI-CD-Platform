package pipelines

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

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
	jobs             chan Job
	path             string       // Path to the associated workspace
	cleanWs          func() error // Removes the workspace at path
	attemptNum       int
	currentTestsPath string
	errorChannel     chan<- ErrorObject

	// Can be one of:
	// - "running"
	// - "stopped"
	// - "closed"
	status string
}

type Job struct {
	// Can be one of:
	// - "open"
	// - "close"
	// - "edit"
	// - "sync"
	// - "run_tests"
	// - "commit_push"
	JobType string
	Task    types.Task
}

// Creates a new workflow. Path, cleanWs, and cancelWf function are are uninitialized by default.
// Path and cleanup are initialized by the OPEN job.
func newWorkflow(pr types.PullRequest, errChan chan<- ErrorObject) (wf *Workflow, err error) {
	wf = &Workflow{
		wfid:         pr.Number,
		pullRequest:  pr,
		jobs:         make(chan Job),
		attemptNum:   0,
		errorChannel: errChan,
	}

	return wf, nil
}

// Starts the job pipeline. Handles incoming jobs. Blocks until an error occurs.
func (wf *Workflow) runWorkflow(ctx context.Context, cli *client.Client) {
	for {
		select {
		case <-ctx.Done():
			return
		case job := <-wf.jobs:
			switch job.JobType {
			case "open":
				wf.attemptNum = 0
				path, clean, err := wstools.InitWorkspace(ctx, wf.pullRequest, &wstools.GithubClient{})
				if err != nil {
					if cleanerr := clean(); cleanerr != nil {
						wf.errorChannel <- ErrorObject{
							wfid: wf.wfid,
							err:  fmt.Errorf("Failed to cleanup workspace: %w", cleanerr),
						}
					}
					wf.errorChannel <- ErrorObject{
						wfid: wf.wfid,
						err:  fmt.Errorf("Failed to create a temporary workspace: %w", err),
					}
				}
				wf.path = path
				wf.cleanWs = clean

				if err = servertools.SendRequestAIEngine(ctx, "open", types.AIEngineRequest{
					Wfid:        wf.wfid,
					PullRequest: wf.pullRequest,
				}); err != nil {
					wf.errorChannel <- ErrorObject{
						wfid: wf.wfid,
						err:  fmt.Errorf("Failed to send request to ai engine: %w", err),
					}
				}

			case "close":
				if err := wf.cleanWs(); err != nil {
					wf.errorChannel <- ErrorObject{
						wfid: wf.wfid,
						err:  fmt.Errorf("Failed to clean up workspace: %w", err),
					}
				}
				if err := servertools.SendRequestAIEngine(ctx, "close", types.AIEngineRequest{Wfid: wf.wfid}); err != nil {
					wf.errorChannel <- ErrorObject{
						wfid: wf.wfid,
						err:  fmt.Errorf("Failed to send request to ai engine: %w", err),
					}
				}

			case "edit", "sync":
				wf.attemptNum = 0
				pr, ok := job.Task.(types.PullRequest)
				if !ok {
					panic("EDIT or SYNC should always come from a pull request.")
				}

				wf.pullRequest = pr

				// May be redundant, but exists just in case the types are relabled.
				var jt string
				if job.JobType == "edit" {
					jt = "edit"
				} else {
					jt = "sync"
				}

				if err := servertools.SendRequestAIEngine(ctx, jt, types.AIEngineRequest{
					Wfid:        wf.wfid,
					PullRequest: pr,
				}); err != nil {
					wf.errorChannel <- ErrorObject{
						wfid: wf.wfid,
						err:  fmt.Errorf("Failed to send request to ai engine: %w", err),
					}
				}

			case "run_tests":
				aier, ok := job.Task.(types.AIEngineResponse)
				if !ok {
					panic("RUN_TESTS should always come from a pull request.")
				}
				if aier.PullRequest != wf.pullRequest {
					slog.Info("Outdated response from AI Engine, pull request does not match current version")
					continue
				}
				if wf.attemptNum > config.MaxTestPatchingAttempts {
					wf.errorChannel <- ErrorObject{
						wfid: wf.wfid,
						err:  fmt.Errorf("Test generation failed: too many attempts"),
					}
				}
				wf.attemptNum++

				if err := wstools.InsertTests(filepath.Join(wf.path, aier.TestName), aier.Tests); err != nil {
					wf.errorChannel <- ErrorObject{
						wfid: wf.wfid,
						err:  fmt.Errorf("Failed to insert tests: %w", err),
					}
				}
				nameFormatter := strings.NewReplacer("/", "-", "|", "-", "<", "-", ">", "-", "\"", "-")
				wsName := nameFormatter.Replace(fmt.Sprintf("%s-%v", wf.pullRequest.Branch, wf.wfid))
				tag, err := dockertools.BuildImage(ctx, cli, wsName, wf.pullRequest.HeadSHA, wf.path, &dockertools.RealTarBuilder{})
				if err != nil {
					wf.errorChannel <- ErrorObject{
						wfid: wf.wfid,
						err:  fmt.Errorf("Failed to build image: %w", err),
					}
				}

				// Process the container
				contInspect, logOut, logErr, err := processContainer(ctx, tag, cli)
				if err != nil {
					wf.errorChannel <- ErrorObject{
						wfid: wf.wfid,
						err:  fmt.Errorf("Container failed: %w", err),
					}
				}

				if err := dockertools.RemoveImage(ctx, cli, tag); err != nil {
					wf.errorChannel <- ErrorObject{
						wfid: wf.wfid,
						err:  fmt.Errorf("Failed to remove image: %w", err),
					}
				}

				if err := servertools.SendRequestAIEngine(ctx, "logs", types.AIEngineRequest{
					Wfid:      wf.wfid,
					Stdout:    logOut,
					Stderr:    logErr,
					StartTime: contInspect.StartTime,
					EndTime:   contInspect.EndTime,
					Errors:    contInspect.Errors,
					Status:    contInspect.Status,
					OOMKilled: contInspect.OOMKilled,
					ExitCode:  contInspect.ExitCode,
				}); err != nil {
					wf.errorChannel <- ErrorObject{
						wfid: wf.wfid,
						err:  fmt.Errorf("Request to AI Engine failed: %w", err),
					}
				}

			case "commit_push":
				aier, ok := job.Task.(types.AIEngineResponse)
				if !ok {
					panic("RUN_TESTS should always come from a pull request.")
				}
				if err := wstools.WriteSummary(filepath.Join(wf.path, "summary.md"), aier.Summary); err != nil {
					wf.errorChannel <- ErrorObject{
						wfid: wf.wfid,
						err:  fmt.Errorf("Failed to write summary: %w", err),
					}
				}

			default:
				panic(fmt.Sprintf("Unsupported job type: %v", job))
			}
		}
	}
}

// Creates a container, runs it, and removes it. Returns a ContainerInspection, stdout, stderr, and an error.
func processContainer(ctx context.Context, tag string, cli *client.Client) (inspect dockertools.ContainerInspection, logOutString string, logErrString string, err error) {
	subContext, cancel := context.WithTimeout(ctx, time.Duration(config.ContainerTimeout)*time.Minute)
	defer cancel()
	contID, logOut, logErr, err := dockertools.RunContainer(subContext, cli, tag)
	if err != nil {
		return dockertools.ContainerInspection{}, "", "", fmt.Errorf("Failed to build container: %w", err)
	}

	// Close the logs and remove container
	// err is updated before this IIFE returns
	defer func() {
		if err := logOut.Close(); err != nil {
			err = fmt.Errorf("Failed to close out logs: %w", err)
		}
		if err := logErr.Close(); err != nil {
			err = fmt.Errorf("Failed to close error logs: %w", err)
		}
		if err := dockertools.RemoveContainer(ctx, cli, contID); err != nil {
			err = fmt.Errorf("Failed to remove container: %w", err)
		}
	}()

	logOutBytes, err := io.ReadAll(logOut)
	if err != nil {
		return dockertools.ContainerInspection{}, "", "", fmt.Errorf("Failed to read output logs: %w", err)
	}
	logOutString = string(logOutBytes)
	logErrBytes, err := io.ReadAll(logErr)
	if err != nil {
		return dockertools.ContainerInspection{}, "", "", fmt.Errorf("Failed to read error logs: %w", err)
	}
	logErrString = string(logErrBytes)

	inspect, err = dockertools.InspectContainer(ctx, cli, contID)
	if err != nil {
		return dockertools.ContainerInspection{}, "", "", fmt.Errorf("Failed to inspect container: %w", err)
	}

	return inspect, logOutString, logErrString, err
}
