package pipelines

import (
	"context"
	"log/slog"

	"github.com/CrimsonBlade7/Autonomous-Self-Healing-AI-CI-CD-Platform/orchestrator/internal/types"
	"github.com/moby/moby/client"
)

// Starts the run pipeline. Handles incoming workflows.
func StartWorkflowPipeline(ctx context.Context, cli *client.Client, prChannel chan types.PullRequest) {
	for {
		select {
		case <-ctx.Done():
			return
		case pr := <-prChannel:
			wf, err := newWorkflow(ctx, pr)
			if err != nil {
				slog.Error("Failed to create a new workflow", "error", err)
			}

			cli, err := client.New()
			if err != nil {
				slog.Error("Failed to create a docker client", "error", err)
				continue
			}

			go func() {
				subCtx, cancel := context.WithCancel(ctx)
				defer cancel()
				err := wf.StartWorkflow(subCtx, cli)
				if err != nil {
					slog.Error("Job pipeline failed", "error", err)
					cancel()
				}
				// TODO: handoff to rag
			}()
		}
	}
}
