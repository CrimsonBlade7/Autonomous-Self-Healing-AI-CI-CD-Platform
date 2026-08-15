package pipelines

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/CrimsonBlade7/Autonomous-AI-CI-CD-Platform/orchestrator/internal/types"
	"github.com/moby/moby/client"
)

// Manages workflows and assigns jobs.
type WorkflowManager struct {
	workflows map[int]WorkflowObject
	mutex     sync.RWMutex
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
	}
}

func (wfm *WorkflowManager) Get(id int) (WorkflowObject, bool) {
	wfm.mutex.RLock()
	defer wfm.mutex.RUnlock()
	val, exists := wfm.workflows[id]
	return val, exists
}

func (wfm *WorkflowManager) Set(id int, wo WorkflowObject) {
	wfm.mutex.Lock()
	defer wfm.mutex.Unlock()
	wfm.workflows[id] = wo
}

func (wfm *WorkflowManager) Remove(id int) {
	wfm.mutex.Lock()
	defer wfm.mutex.Unlock()
	delete(wfm.workflows, id)
}

// Starts the run pipeline. Handles incoming workflows.
func (wfm *WorkflowManager) RunWorkflowPipeline(ctx context.Context, cli *client.Client, taskChannel chan types.Task, pushedCommits map[int]string) {
	for {
		select {
		case <-ctx.Done():
			return
		case task := <-taskChannel:
			switch t := task.(type) {
			case types.PullRequest:
				wfm.workflows[t.Number].cancel()
				err := wfm.handlePullRequest(ctx, cli, t)
				if err != nil {
					slog.Error("Failed to handle pull request", "error", err)
				}

			case types.AIEngineResponse:
				wfo, exists := wfm.Get(t.Wfid)
				if !exists {
					slog.Error("Workflow ID does not exist", "ID", t.Wfid)
					continue
				}
				if t.Done {
					wfo.workflow.Jobs <- Job{
						JobType: COMMIT_PUSH,
						Task:    t,
					}
					pushedCommits[t.PullRequest.Number] = t.PullRequest.HeadSHA
				} else {
					wfo.workflow.Jobs <- Job{
						JobType: RUN_TESTS,
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

	switch pr.Action {
	case "opened":
		// Creates and starts a new workflow for a new pr
		if exists {
			panic(fmt.Sprintf("Duplicate workflow: %v", num))
		}
		nwf, err := newWorkflow(pr)
		if err != nil {
			return fmt.Errorf("Failed to create a new workflow: %w", err)
		}
		err = wfm.openPr(ctx, cli, pr, nwf)
		if err != nil {
			return fmt.Errorf("Failed to open pr: %w", err)
		}

	case "closed":
		wfo.cancel()
		if pr.Merged {
			wfm.Remove(num)
		}
		wf.Jobs <- Job{
			JobType: CLOSE,
			Task:    pr,
		}

	case "reopened":
		err = wfm.openPr(ctx, cli, pr, wf)
		if err != nil {
			return fmt.Errorf("Failed to open pr: %w", err)
		}

	case "edited", "synchronize":
		wf.Jobs <- Job{
			JobType: UPDATE_PR,
			Task:    pr,
		}

	default:
		slog.Info("Unsupported pull request action", "action", pr.Action)
	}
	return nil
}

func (wfm *WorkflowManager) openPr(ctx context.Context, cli *client.Client, pr types.PullRequest, wf *Workflow) (err error) {
	subCtx, end := context.WithCancel(ctx)
	wfm.Set(pr.Number, WorkflowObject{
		workflow: wf,
		cancel:   end,
	})
	go func() {
		defer end()
		defer wfm.Remove(pr.Number)
		wfErr := wf.runWorkflow(subCtx, cli)
		if wfErr != nil {
			end()
			err = wfErr
		}
	}()
	wf.Jobs <- Job{
		JobType: OPEN,
		Task:    wf.pullRequest,
	}
	return err
}
