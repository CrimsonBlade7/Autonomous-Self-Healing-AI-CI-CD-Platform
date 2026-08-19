package pipelines

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"sync"

	"github.com/CrimsonBlade7/Autonomous-AI-CI-CD-Platform/orchestrator/internal/types"
	"github.com/moby/moby/client"
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
func (wfm *WorkflowManager) RunWorkflowPipeline(ctx context.Context, cli *client.Client, taskChannel <-chan types.Task, pc *types.PushedCommits) {
	for {
		select {
		case <-ctx.Done():
			return

		case errObject := <-wfm.wfErrChan:
			slog.Error("Workflow failed", "wfid", errObject.wfid, "error", errObject.err)
			wf, exists := wfm.Get(errObject.wfid)
			if !exists {
				panic(fmt.Sprintf("Workflow doesn't exist: %v", errObject.wfid))
			}
			wf.cancel()
			wfm.Remove(errObject.wfid)

		case task := <-taskChannel:
			switch t := task.(type) {
			case types.PullRequest:
				if err := wfm.handlePullRequest(ctx, cli, t); err != nil {
					slog.Error("Failed to handle pull request", "error", err)
				}

			case types.AIEngineResponse:
				wfo, exists := wfm.Get(t.Wfid)
				if !exists {
					slog.Error("Workflow ID does not exist", "ID", t.Wfid)
					continue
				}

				if wfo.workflow.status != "running" {
					slog.Info("Workflow is not running and cannot recieve new tasks", "wfid", t.Wfid)
					continue
				}

				if t.Done {
					wfo.workflow.jobs <- Job{
						JobType: "commit_push",
						Task:    t,
					}
					pc.Set(t.PullRequest.Number, t.PullRequest.HeadSHA)
				} else {
					wfo.workflow.jobs <- Job{
						JobType: "run_tests",
						Task:    t,
					}
				}

			default:
				panic(fmt.Sprintf("Unsupported task: %T", t))
			}
		}
	}
}

func (wfm *WorkflowManager) handlePullRequest(ctx context.Context, cli *client.Client, pr types.PullRequest) (err error) {
	num := pr.Number
	wfo, exists := wfm.Get(num)
	wf := wfo.workflow

	runningActions := []string{
		"closed",
		"edited",
		"synchronize",
	}
	stoppedActions := []string{
		"opened",
		"reopened",
	}

	if slices.Contains(runningActions, pr.Action) && pr.Action != "running" {
		return fmt.Errorf("Workflow is not running: %v", wf.wfid)
	}
	if slices.Contains(stoppedActions, pr.Action) && pr.Action != "stopped" {
		return fmt.Errorf("Workflow is running: %v", wf.wfid)
	}

	switch pr.Action {
	case "opened":
		// Creates and starts a new workflow for a new pr
		if exists {
			panic(fmt.Sprintf("Duplicate workflow: %v", num))
		}

		nwf, err := newWorkflow(pr, wfm.wfErrChan)
		if err != nil {
			return fmt.Errorf("Failed to create a new workflow: %w", err)
		}
		wfm.openPr(ctx, cli, pr, nwf)
		slog.Info("New workflow started", "wfid", nwf.wfid)

	case "closed":
		if !exists {
			panic(fmt.Sprintf("Attempting to close a workflow that does not exist: %v", num))
		}

		wfo.cancel()
		if pr.Merged {
			wfm.Remove(num)
			slog.Info("Workflow removed", "wfid", wfo.workflow.wfid)
		}
		slog.Info("Workflow stopped", "wfid", wf.wfid)

	case "reopened":
		if !exists {
			panic(fmt.Sprintf("Attempting to reopen a workflow that does not exist: %v", num))
		}

		wfm.openPr(ctx, cli, pr, wf)
		slog.Info("Workflow repopened", "wfid", wfo.workflow.wfid)

	case "edited":
		if !exists {
			panic(fmt.Sprintf("Attempting to edit a workflow that does not exist: %v", num))
		}

		wf.jobs <- Job{
			JobType: "edit",
			Task:    pr,
		}
		slog.Info("Pull request updated", "wfid", wfo.workflow.wfid)

	case "synchronize":
		if !exists {
			panic(fmt.Sprintf("Attempting to sync a workflow that does not exist: %v", num))
		}

		wf.jobs <- Job{
			JobType: "sync",
			Task:    pr,
		}
		slog.Info("Repository synchronized", "wfid", wfo.workflow.wfid)

	default:
		slog.Info("Unsupported pull request action", "action", pr.Action)
	}
	return nil
}

func (wfm *WorkflowManager) openPr(ctx context.Context, cli *client.Client, pr types.PullRequest, wf *Workflow) {
	subCtx, end := context.WithCancel(ctx)
	wfm.Set(pr.Number, WorkflowObject{
		workflow: wf,
		cancel:   end,
	})
	go func(ctx context.Context, cli *client.Client) {
		defer end()
		wf.runWorkflow(subCtx, cli)
	}(subCtx, cli)
	wf.jobs <- Job{
		JobType: "open",
		Task:    wf.pullRequest,
	}
}
