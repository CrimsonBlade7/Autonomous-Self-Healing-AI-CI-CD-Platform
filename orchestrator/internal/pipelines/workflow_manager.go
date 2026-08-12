package pipelines

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/CrimsonBlade7/Autonomous-AI-CI-CD-Platform/orchestrator/internal/types"
	"github.com/moby/moby/client"
)

type WorkflowObject struct {
	workflow *Workflow
	cancel   context.CancelFunc
}

// Manages workflows and assigns jobs.
type WorkflowManager struct {
	idMap map[int]WorkflowObject
	mutex sync.RWMutex
}

// Creates a new workflow manager.
func NewWorkflowManager() *WorkflowManager {
	return &WorkflowManager{
		idMap: make(map[int]WorkflowObject),
		mutex: sync.RWMutex{},
	}
}

func (wfm *WorkflowManager) Get(id int) (WorkflowObject, bool) {
	wfm.mutex.RLock()
	defer wfm.mutex.RUnlock()
	val, exists := wfm.idMap[id]
	return val, exists
}

func (wfm *WorkflowManager) Set(id int, wo WorkflowObject) {
	wfm.mutex.Lock()
	defer wfm.mutex.Unlock()
	wfm.idMap[id] = wo
}

func (wfm *WorkflowManager) Remove(id int) {
	wfm.mutex.Lock()
	defer wfm.mutex.Unlock()
	delete(wfm.idMap, id)
}

// Starts the run pipeline. Handles incoming workflows.
func (wfm *WorkflowManager) RunWorkflowPipeline(ctx context.Context, cli *client.Client, taskChannel chan types.Task) {
	for {
		select {
		case <-ctx.Done():
			return
		case task := <-taskChannel:
			switch t := task.(type) {
			case types.PullRequest:
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
					wfo.workflow.Jobs <- Job{JobType: COMMIT_PUSH}
				} else {
					wfo.workflow.Jobs <- Job{
						JobType: RUN_TESTS,
						Data:    t.Tests,
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
		subCtx, end := context.WithCancel(ctx)
		wfm.Set(num, WorkflowObject{
			workflow: nwf,
			cancel:   end,
		})
		go func() {
			defer end()
			defer wfm.Remove(num)
			err := nwf.runWorkflow(subCtx, cli)
			if err != nil {
				slog.Error("Workflow failed", "error", err)
				end()
			}
		}()
		wf.Jobs <- Job{JobType: OPEN}

	case "closed":
		wfo.cancel()
		if pr.Merged {
			wfm.Remove(num)
		}
		wf.Jobs <- Job{JobType: CLOSE}

	case "reopened":
		subCtx, end := context.WithCancel(ctx)
		wfm.Set(num, WorkflowObject{
			workflow: wf,
			cancel:   end,
		})
		go func() {
			defer end()
			defer wfm.Remove(num)
			err := wf.runWorkflow(subCtx, cli)
			if err != nil {
				slog.Error("Workflow failed", "error", err)
				end()
			}
		}()
		wf.Jobs <- Job{JobType: OPEN}

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
