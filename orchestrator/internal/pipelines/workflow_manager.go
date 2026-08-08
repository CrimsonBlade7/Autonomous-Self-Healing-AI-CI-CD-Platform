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
	workflows map[uint]*WorkflowObject
	mutex     sync.RWMutex
}

// Creates a new workflow manager.
func NewWorkflowManager() WorkflowManager {
	return WorkflowManager{
		workflows: make(map[uint]*WorkflowObject),
		mutex:     sync.RWMutex{},
	}
}

func (wfm *WorkflowManager) Get(key uint) (*WorkflowObject, bool) {
	wfm.mutex.RLock()
	defer wfm.mutex.RUnlock()
	val, ok := wfm.workflows[key]
	return val, ok
}

func (wfm *WorkflowManager) Set(key uint, wo *WorkflowObject) {
	wfm.mutex.Lock()
	defer wfm.mutex.Unlock()
	wfm.workflows[key] = wo
}

func (wfm *WorkflowManager) Remove(key uint) {
	wfm.mutex.Lock()
	defer wfm.mutex.Unlock()
	delete(wfm.workflows, key)
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
				wf := wfm.workflows[t.Wfid].workflow
				if t.Done {
					// TODO: handle canceling workflows
					err := wf.cleanWs()
					if err != nil {
						slog.Error("Failed to clean up workflow", "error", err)
					}
				} else {
					wf.Jobs <- Job{
						JobType: RUN_TESTS,
						Data:    t.Tests,
					}
				}

			case types.PushNotification:
			default:
				panic(fmt.Sprintf("Unsupported task: %T", t))
			}
			// TODO: handle different types of prs

		}
	}
}

func (wfm *WorkflowManager) handlePullRequest(ctx context.Context, cli *client.Client, pr types.PullRequest) error {
	num := pr.Number
	wfo, ok := wfm.Get(num)
	wf := wfo.workflow
	switch pr.Action {
	case "opened":
		// Creates and starts a new workflow for a new pr
		if ok {
			panic(fmt.Sprintf("Duplicate workflow: %v", num))
		}
		nwf, err := newWorkflow(pr)
		if err != nil {
			return fmt.Errorf("Failed to create a new workflow: %w", err)
		}
		subCtx, end := context.WithCancel(ctx)
		wfm.Set(num, &WorkflowObject{
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
		wfm.Set(num, &WorkflowObject{
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

	case "edited":
		wf.Jobs <- Job{
			JobType: EDIT,
			Task:    pr,
		}
	case "synchronize":
		wf.Jobs <- Job{JobType: SYNC}
	default:
		slog.Info("Unsupported pull request action", "action", pr.Action)
	}
	return nil
}
