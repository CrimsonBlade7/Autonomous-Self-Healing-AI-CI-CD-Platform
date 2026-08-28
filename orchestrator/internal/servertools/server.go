package servertools

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"slices"
	"strings"
	"time"

	"github.com/benl1006/Autonomous-CI-Platform/orchestrator/internal/config"
	"github.com/benl1006/Autonomous-CI-Platform/orchestrator/internal/types"
)

// Generates the HMAC key based on the message and secret
func generateHMAC(message []byte, secret string) string {
	hash := hmac.New(sha256.New, []byte(secret))
	hash.Write(message)
	return hex.EncodeToString(hash.Sum(nil))
}

// Verifies if the message is legitimate
func verifyMessage(message []byte, secret, signature string) bool {
	return hmac.Equal([]byte(signature), []byte(generateHMAC(message, secret)))
}

// Returns time.Duration of n seconds
func seconds(n int) time.Duration {
	return time.Duration(n) * time.Second
}

// Github webhook handler
func whHandler(taskChannel chan<- types.Task, pc *types.PushedCommits) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			closeErr := r.Body.Close()
			if closeErr != nil {
				slog.Error("Failed to close request body", "error", closeErr)
			}
		}()
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			slog.Error("Failed to read request body", "error", err)
			return
		}

		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			slog.Error("Not a post request", "error", err)
			return
		}

		realSig := strings.TrimPrefix(r.Header.Get("X-Hub-Signature-256"), "sha256=")
		if !verifyMessage(body, config.GithubSecret, realSig) {
			w.WriteHeader(http.StatusUnauthorized)
			slog.Warn("Unauthorized request", "warn", err)
			return
		}

		if r.Header.Get("X-GitHub-Event") != "pull_request" {
			slog.Error("Not a pull request")
			return
		}

		var pr types.PullRequest
		if err := pr.UnmarshalpullRequest(body); err != nil {
			slog.Error("Failed to unmsarhsal the pull request", "error", err)
			return
		}

		if sha, _ := pc.Get(pr.Number); pr.HeadSHA == sha {
			slog.Info("Webhook originates from this platform")
			return
		}

		slog.Info("Webhook recieved", "pull request", pr)
		taskChannel <- &pr

	}
}

// Handles responses from the AI Engine and sends response to respChannel.
func aiEngineResponseHandler(taskChannel chan<- types.Task) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if closeErr := r.Body.Close(); closeErr != nil {
				slog.Error("Failed to close request body", "error", closeErr)
			}
		}()
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			slog.Error("Failed to read request body", "error", err)
			return
		}
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			slog.Error("Not a post request", "error", err)
			return
		}

		realSig := r.Header.Get("HMAC-Signature-256")
		if !verifyMessage(body, config.AIEngineSecret, realSig) {
			w.WriteHeader(http.StatusUnauthorized)
			slog.Warn("Unauthorized request", "warn", err)
			return
		}

		var resp types.AIEngineResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			w.WriteHeader(http.StatusUnsupportedMediaType)
			slog.Error("Could not unmarshal the data into a type.Response", "error", err)
			return
		}

		taskChannel <- &resp
	}
}

// Sends a http request to the AI Engine.
// jobType can be one of: "open", "close", "logs", "edit", "sync".
// open: Start a workflow when a pr opens.
// close: Close and merge implied; end associated workflow and update rag index.
// logs: Return the logs of the last test run.
// edit: Update pr information.
// sync: update branch head
func SendRequestAIEngine(ctx context.Context, jobType string, req types.AIEngineRequest) (err error) {
	validJobTypes := []string{
		"open",
		"close",
		"edit",
		"sync",
		"logs",
	}

	if !slices.Contains(validJobTypes, jobType) {
		return fmt.Errorf("Invalid job type")
	}

	cli := http.Client{
		Timeout: seconds(config.AiEngineRequestTimeout),
	}

	msgBytes, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("Failed to marshal the message package: %w", err)
	}
	msgReader := bytes.NewReader(msgBytes)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("http://localhost:%s", config.AIEnginePort), msgReader)
	if err != nil {
		return fmt.Errorf("Failed to create http request: %w", err)
	}

	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Job-Type", jobType)

	resp, err := cli.Do(httpReq)
	if err != nil {
		return fmt.Errorf("Failed to send http request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			err = closeErr
		}
	}()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Bad response, status: %v", resp.StatusCode)
	}
	return nil
}

// Starts the http server.
func StartServer(ctx context.Context, taskChannel chan<- types.Task, pc *types.PushedCommits) (err error) {

	// initialize server
	mux := http.NewServeMux()
	mux.Handle("/", http.HandlerFunc(whHandler(taskChannel, pc)))
	mux.Handle("/patch", http.HandlerFunc(aiEngineResponseHandler(taskChannel)))

	port := fmt.Sprintf(":%s", config.Port)
	server := &http.Server{
		Addr:              port,
		Handler:           mux,                               // Inject your isolated router
		ReadHeaderTimeout: seconds(config.ReadHeaderTimeout), // Max time to read just the headers
		WriteTimeout:      seconds(config.WriteTimeout),      // Max time to write the response
	}

	// create a context for server lifetime
	lifetime, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()

	// start the server
	go func() {
		slog.Info("Server is starting", "port", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("Server error", "error", err)
			os.Exit(1)
		}
	}()

	// block until an os.Interrupt occurs
	<-lifetime.Done()

	// create a context for duration of the server shutdown
	countdown := time.Duration(config.ServerShutdownTimeout) * time.Second
	shutdownCtx, cancelShutdown := context.WithTimeout(ctx, countdown)
	defer cancelShutdown()

	// shutting down the server
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("Server forced to shutdown", "error", err)
		return err
	}

	slog.Info("Server shutdown complete!")
	return nil
}
