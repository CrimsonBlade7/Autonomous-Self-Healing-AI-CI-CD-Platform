package pipelines

import (
	"context"
	"log/slog"
	"sync"

	"github.com/CrimsonBlade7/Autonomous-AI-CI-CD-Platform/orchestrator/internal/types"
	"github.com/moby/moby/client"
)

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
func (wfm *WorkflowManager) RunWorkflowPipeline(ctx context.Context, cli *client.Client, prChannel chan types.PullRequest) {
	for {
		select {
		case <-ctx.Done():
			return
		case pr := <-prChannel:
			switch pr.Action {
			case "opened":
			case "closed":
			case "reopened":
			case "edited":
			case "synchronize":
			default:
				slog.Info("Unsupported pull request action", "action", pr.Action)
			}

			num := pr.Number
			wfp, ok := wfm.Get(num)
			if !ok {
				// Creates and starts a new workflow for a new pr
				wfp, err := newWorkflow(pr)
				if err != nil {
					slog.Error("Failed to create a new workflow", "error", err)
					continue
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
				wfp.Jobs <- OPEN
			} else {
				// Updates an existing workflow
				if pr.Action == "syncronize" {

				}
				wfp.update(pr)
				wfp.Jobs <- SYNC
			}
		}
	}
}
