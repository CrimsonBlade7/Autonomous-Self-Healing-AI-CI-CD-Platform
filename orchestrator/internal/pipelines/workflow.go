package pipelines

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/benl1006/Autonomous-CI-Platform/orchestrator/internal/config"
	"github.com/benl1006/Autonomous-CI-Platform/orchestrator/internal/dockertools"
	"github.com/benl1006/Autonomous-CI-Platform/orchestrator/internal/servertools"
	"github.com/benl1006/Autonomous-CI-Platform/orchestrator/internal/types"
	"github.com/benl1006/Autonomous-CI-Platform/orchestrator/internal/wstools"
	dockerClient "github.com/moby/moby/client"
)

type Workflow struct {
	wfid             int // The pr number
	pullRequest      types.PullRequest
	jobs             chan Job
	path             string // Path to the associated workspace.
	pathMutex        sync.RWMutex
	cleanWs          func() error // Removes the workspace at path.
	attemptNum       int
	currentTestsPath string
	errorChannel     chan<- ErrorObject

	// Can be one of:
	// - "running": currently processing a pr
	// - "stopped": temporarily paused
	// Currently a string if new statuses are added.
	status      string
	statusMutex sync.RWMutex
}

type Job struct {
	// Can be one of:
	// - "open"
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
		pathMutex:    sync.RWMutex{},
		attemptNum:   0,
		errorChannel: errChan,
		status:       "stopped",
		statusMutex:  sync.RWMutex{},
	}

	return wf, nil
}

func (wf *Workflow) GetPath() string {
	wf.pathMutex.RLock()
	defer wf.pathMutex.RUnlock()
	return wf.path
}

func (wf *Workflow) SetPath(p string) {
	wf.statusMutex.Lock()
	defer wf.statusMutex.Unlock()
	wf.path = p
}

func (wf *Workflow) GetStatus() string {
	wf.statusMutex.RLock()
	defer wf.statusMutex.RUnlock()
	return wf.path
}

func (wf *Workflow) SetRunning() {
	wf.statusMutex.Lock()
	defer wf.statusMutex.Unlock()
	wf.status = "running"
}

func (wf *Workflow) SetStopped() {
	wf.statusMutex.Lock()
	defer wf.statusMutex.Unlock()
	wf.status = "stopped"
}


// Starts the job pipeline. Handles incoming jobs. Blocks until an error occurs.
func (wf *Workflow) runWorkflow(ctx context.Context, cli *dockerClient.Client, pc *types.PushedCommits) {
	wf.SetRunning()
	for wf.status == "running" {
		select {
		case <-ctx.Done():
			wf.SetStopped()
			if wf.cleanWs != nil {
				if err := wf.cleanWs(); err != nil {
					wf.errorChannel <- ErrorObject{
						wfid: wf.wfid,
						err:  fmt.Errorf("Failed to clean up workspace: %w", err),
					}
					return
				}
			}
			newCtx, cancel := context.WithTimeout(context.Background(), time.Duration(config.AIEngineRequestCloseTimeout)*time.Second)
			defer cancel()
			if err := servertools.SendRequestAIEngine(newCtx, "close", types.AIEngineRequest{Wfid: wf.wfid}); err != nil {
				wf.errorChannel <- ErrorObject{
					wfid: wf.wfid,
					err:  fmt.Errorf("Failed to send request to ai engine: %w", err),
				}
				return
			}
			return

		case job := <-wf.jobs:
			switch job.JobType {
			case "open":
				wf.attemptNum = 0
				path, clean, err := wstools.InitWorkspace(ctx, wf.pullRequest, &wstools.GithubClient{})
				if err != nil {
					var cleanerr error
					if clean != nil {
						cleanerr = clean()
					}
					wf.errorChannel <- ErrorObject{
						wfid: wf.wfid,
						err:  fmt.Errorf("Failed to create a temporary workspace: %w", errors.Join(err, cleanerr)),
					}
					continue
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
					continue
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
					continue
				}

			case "run_tests":
				aier, ok := job.Task.(types.AIEngineResponse)
				if !ok {
					panic("RUN_TESTS should always come from a pull request.")
				}
				if aier.PullRequest != wf.pullRequest {
					continue
				}
				if wf.attemptNum > config.MaxTestPatchingAttempts {
					wf.errorChannel <- ErrorObject{
						wfid: wf.wfid,
						err:  fmt.Errorf("Test generation failed: too many attempts"),
					}
					continue
				}
				wf.attemptNum++

				if err := wstools.InsertTests(filepath.Join(wf.path, aier.TestName), aier.Tests); err != nil {
					wf.errorChannel <- ErrorObject{
						wfid: wf.wfid,
						err:  fmt.Errorf("Failed to insert tests: %w", err),
					}
					continue
				}
				nameFormatter := strings.NewReplacer("/", "-", "|", "-", "<", "-", ">", "-", "\"", "-")
				wsName := nameFormatter.Replace(fmt.Sprintf("%s-%v", wf.pullRequest.Branch, wf.wfid))
				tag, err := dockertools.BuildImage(ctx, cli, wsName, wf.pullRequest.HeadSHA, wf.path, &dockertools.RealTarBuilder{})
				if err != nil {
					wf.errorChannel <- ErrorObject{
						wfid: wf.wfid,
						err:  fmt.Errorf("Failed to build image: %w", err),
					}
					continue
				}

				// Process the container
				contInspect, logOut, logErr, err := processContainer(ctx, tag, cli)
				if err != nil {
					wf.errorChannel <- ErrorObject{
						wfid: wf.wfid,
						err:  fmt.Errorf("Container failed: %w", err),
					}
					continue
				}

				if err := dockertools.RemoveImage(ctx, cli, tag); err != nil {
					wf.errorChannel <- ErrorObject{
						wfid: wf.wfid,
						err:  fmt.Errorf("Failed to remove image: %w", err),
					}
					continue
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
					continue
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
					continue
				}
				newSha, err := wf.SendUpdatesToRemote(&wstools.GithubClient{})
				if err != nil {
					wf.errorChannel <- ErrorObject{
						wfid: wf.wfid,
						err:  fmt.Errorf("Failed to update remote: %w", err),
					}
				}
				pc.Add(wf.wfid, newSha)

			default:
				panic(fmt.Sprintf("Unsupported job type: %v", job))
			}
		}
	}
}

// Creates a container, runs it, and removes it. Returns a ContainerInspection, stdout, stderr, and an error.
func processContainer(ctx context.Context, tag string, cli *dockerClient.Client) (inspect dockertools.ContainerInspection, logOutString string, logErrString string, err error) {
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

// Adds, commits, and pushes current workspace state to remote.
func (wf *Workflow) SendUpdatesToRemote(cli wstools.GitClient) (newSha string, err error) {
	newSha, err = cli.AddAllCommitPush("", wf.path, wf.pullRequest.Branch)
	if err != nil {
		return "", fmt.Errorf("Failed to add, commit, and push changes: %w", err)
	}
	return newSha, nil
}
