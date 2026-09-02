package pipelines

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"sync"

	"github.com/benl1006/Autonomous-CI-Platform/orchestrator/internal/types"
	dockerClient "github.com/moby/moby/client"
)

// Wraps an error with a wfid.
type ErrorObject struct {
	wfid int
	err  error
}

// Manages workflows and assigns jobs.
type WorkflowManager struct {
	workflows map[int]WorkflowObject
	mutex     sync.RWMutex
	wfErrChan chan ErrorObject
}

type WorkflowObject struct {
	workflow *Workflow
	cancel   context.CancelFunc
}

// Creates a new workflow manager.
func NewWorkflowManager() *WorkflowManager {
	return &WorkflowManager{
		workflows: make(map[int]WorkflowObject),
		mutex:     sync.RWMutex{},
		wfErrChan: make(chan ErrorObject),
	}
}

// Gets the workflow from an id.
func (wfm *WorkflowManager) Get(id int) (WorkflowObject, bool) {
	wfm.mutex.RLock()
	defer wfm.mutex.RUnlock()
	val, exists := wfm.workflows[id]
	return val, exists
}

// Adds or replaces an id-workflow pair in the map.
func (wfm *WorkflowManager) Set(id int, wo WorkflowObject) {
	wfm.mutex.Lock()
	defer wfm.mutex.Unlock()
	wfm.workflows[id] = wo
}

// Removes the workflow from the map.
func (wfm *WorkflowManager) Remove(id int) {
	wfm.mutex.Lock()
	defer wfm.mutex.Unlock()
	delete(wfm.workflows, id)
}

// Starts the run pipeline. Handles incoming workflows.
func (wfm *WorkflowManager) RunWorkflowPipeline(ctx context.Context, cli *dockerClient.Client, prChan <-chan types.PullRequest, aierChan <-chan types.AIEngineResponse, pc *types.PushedCommits) {
	for {
		select {
		case <-ctx.Done():
			return

		case errObject := <-wfm.wfErrChan:
			slog.Error("Workflow failed", "wfid", errObject.wfid, "error", errObject.err)
			wf, exists := wfm.Get(errObject.wfid)
			if exists {
				wf.cancel()
				wfm.Remove(errObject.wfid)
			}

		case pr := <-prChan:
			if err := wfm.handlePullRequest(ctx, cli, pr, pc); err != nil {
				slog.Error("Failed to handle pull request", "error", err)
			}

		case aier := <-aierChan:
			wfo, exists := wfm.Get(aier.Wfid)
			if !exists {
				slog.Error("Workflow ID does not exist", "ID", aier.Wfid)
				continue
			}

			if !wfo.workflow.isRunning() {
				slog.Info("Workflow is not running and cannot recieve new tasks", "wfid", aier.Wfid)
				continue
			}

			if aier.Done {
				success := wfo.workflow.trySend(Job{
					JobType: "commit_push",
					Aier:    &aier,
				})
				if !success {
					slog.Error("Workflow is closed", "ID", aier.Wfid)
					continue
				}
			} else {
				success := wfo.workflow.trySend(Job{
					JobType: "run_tests",
					Aier:    &aier,
				})
				if !success {
					slog.Error("Workflow is closed", "ID", aier.Wfid)
					continue
				}
			}
		}
	}
}

func (wfm *WorkflowManager) handlePullRequest(ctx context.Context, cli *dockerClient.Client, pr types.PullRequest, pc *types.PushedCommits) (err error) {
	num := pr.Number
	wfo, exists := wfm.Get(num)
	var wf *Workflow

	runningActions := []string{
		"closed",
		"edited",
		"synchronize",
	}
	stoppedActions := []string{
		"opened",
		"reopened",
	}

	if exists {
		wf = wfo.workflow
		if pr.Action == "opened" {
			panic(fmt.Sprintf("Duplicate workflow: %v", pr.Number))
		}
		if slices.Contains(runningActions, pr.Action) && wf.isRunning() {
			return fmt.Errorf("Workflow is not running: %v", wf.wfid)
		}
		if slices.Contains(stoppedActions, pr.Action) && !wf.isRunning() {
			return fmt.Errorf("Workflow is running: %v", wf.wfid)
		}
	} else if pr.Action != "opened" {
		panic(fmt.Sprintf("Workflow does not exist: %v\n Action: %s", pr.Number, pr.Action))
	}

	switch pr.Action {
	case "opened":
		// Creates and starts a new workflow for a new pr
		wf := newWorkflow(pr, wfm.wfErrChan)
		wfm.openPr(ctx, cli, pr, wf, pc)
		slog.Info("New workflow started", "wfid", wf.wfid)

	case "closed":
		wfo.cancel()
		if pr.Merged {
			wfm.Remove(num)
			slog.Info("Workflow removed", "wfid", wfo.workflow.wfid)
		}
		slog.Info("Workflow stopped", "wfid", wf.wfid)

	case "reopened":
		wfm.openPr(ctx, cli, pr, wf, pc)
		slog.Info("Workflow repopened", "wfid", wfo.workflow.wfid)

	case "edited":
		success := wfo.workflow.trySend(Job{
			JobType:     "edit",
			PullRequest: &pr,
		})
		if !success {
			return fmt.Errorf("Workflow %v is closed", wfo.workflow.wfid)
		}
		slog.Info("Pull request updated", "wfid", wfo.workflow.wfid)

	case "synchronize":
		if !exists {
			panic(fmt.Sprintf("Attempting to sync a workflow that does not exist: %v", num))
		}

		wf.jobs <- Job{
			JobType:     "sync",
			PullRequest: &pr,
		}
		slog.Info("Repository synchronized", "wfid", wfo.workflow.wfid)

	default:
		slog.Info("Unsupported pull request action", "action", pr.Action)
	}
	return nil
}

// Starts a workflow on pr.
func (wfm *WorkflowManager) openPr(ctx context.Context, cli *dockerClient.Client, pr types.PullRequest, wf *Workflow, pc *types.PushedCommits) {
	subCtx, end := context.WithCancel(ctx)
	wfm.Set(pr.Number, WorkflowObject{
		workflow: wf,
		cancel:   end,
	})
	go func(ctx context.Context, cli *dockerClient.Client) {
		defer end()
		wf.runWorkflow(subCtx, cli, pc)
	}(subCtx, cli)
	wf.jobs <- Job{
		JobType:     "open",
		PullRequest: &wf.pullRequest,
	}
}
