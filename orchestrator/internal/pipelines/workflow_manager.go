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
	workflows map[uint]*Workflow
	mu        sync.RWMutex
}

// Creates a new workflow manager.
func NewWorkflowManager() WorkflowManager {
	return WorkflowManager{
		workflows: make(map[uint]*Workflow),
		mu:        sync.RWMutex{},
	}
}

func (wfm *WorkflowManager) Get(key uint) (*Workflow, bool) {
	wfm.mu.RLock()
	defer wfm.mu.RUnlock()
	val, ok := wfm.workflows[key]
	return val, ok
}

func (wfm *WorkflowManager) Set(key uint, wfp *Workflow) {
	wfm.mu.Lock()
	defer wfm.mu.Unlock()
	wfm.workflows[key] = wfp
}

func (wfm *WorkflowManager) Remove(key uint) {
	wfm.mu.Lock()
	defer wfm.mu.Unlock()
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
				wf := wfm.workflows[t.Wfid]
				if t.Done {
					// TODO: handle canceling workflows
					err := wf.cleanup()
					if err != nil {
						slog.Error("Failed to clean up workflow", "error", err)
					}
				} else {
					wf.Jobs <- Job{
						JobType: RUN_TESTS,
						Data: t.Tests,
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
	switch pr.Action {
	case "opened":
		num := pr.Number
		wfp, ok := wfm.Get(num)
		if !ok {
			// Creates and starts a new workflow for a new pr
			wfp, err := newWorkflow(pr)
			if err != nil {
				return fmt.Errorf("Failed to create a new workflow: %w", err)
			}
			wfm.Set(num, wfp)
			go func() {
				defer wfm.Remove(num)
				subCtx, cancel := context.WithCancel(ctx)
				defer cancel()
				err := wfp.runWorkflow(subCtx, cli)
				if err != nil {
					slog.Error("Workflow failed", "error", err)
					cancel()
				}
			}()
			wfp.Jobs <- Job{JobType: OPEN}
		} else {
			// Updates an existing workflow
			if pr.Action == "syncronize" {

			}
			wfp.update(pr)
			wfp.Jobs <- Job{JobType: SYNC}
		}
	case "closed":
	case "reopened":
	case "edited":
	case "synchronize":
	default:
		slog.Info("Unsupported pull request action", "action", pr.Action)
	}
	return nil
}
