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
	"strings"
	"time"

	"github.com/CrimsonBlade7/Autonomous-AI-CI-CD-Platform/orchestrator/internal/config"
	"github.com/CrimsonBlade7/Autonomous-AI-CI-CD-Platform/orchestrator/internal/pipelines"
	"github.com/CrimsonBlade7/Autonomous-AI-CI-CD-Platform/orchestrator/internal/types"
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
func seconds(n uint) time.Duration {
	return time.Duration(n) * time.Second
}

// Sends a http request to the ai service
func SendMsgPkgRequest(ctx context.Context, mp types.MsgPkg) error {
	cli := http.Client{
		Timeout: seconds(10),
	}

	ctx, cancel := context.WithTimeout(ctx, seconds(5))
	defer cancel()

	msg, err := json.Marshal(mp)
	if err != nil {
		return fmt.Errorf("Failed to marshal the message package: %w", err)
	}
	msgReader := bytes.NewReader(msg)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, config.AIServiceUrl, msgReader)
	if err != nil {
		return fmt.Errorf("Failed to create http request: %w", err)
	}

	req.Header.Set("Accept", "application/json")

	resp, err := cli.Do(req)
	if err != nil {
		return fmt.Errorf("Failed to send http request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Bad response, status: %v", resp.StatusCode)
	}
	return nil
}

// Github webhook handler
func whHandler(prChannel chan types.PullRequest) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
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

		var pullRequest types.PullRequest
		err = pullRequest.UnmarshalJSON(body)
		if err != nil {
			slog.Error("Failed to unmsarhsal the json", "error", err)
			return
		}

		slog.Info("Job recieved", "pull request", pullRequest)

		prChannel <- pullRequest
	}
}

func patchHandler(wfm *pipelines.WorkflowManager) http.HandlerFunc {
	// TODO: patch handler unimplmented
	return func(w http.ResponseWriter, r *http.Request) {
		
	}
}

// Starts the http server.
func StartServer(ctx context.Context, prChannel chan types.PullRequest, wfm *pipelines.WorkflowManager) error {

	// initialize server
	mux := http.NewServeMux()
	mux.Handle("/", http.HandlerFunc(whHandler(prChannel)))
	mux.Handle("/patch", http.HandlerFunc(patchHandler(wfm)))

	port := fmt.Sprintf(":%s", config.Port)

	server := &http.Server{
		Addr:              port,
		Handler:           mux,          // Inject your isolated router
		ReadTimeout:       seconds(5),   // Max time to read the request body
		ReadHeaderTimeout: seconds(2),   // Max time to read just the headers
		WriteTimeout:      seconds(10),  // Max time to write the response
		IdleTimeout:       seconds(120), // Max time to keep idle connections alive
	}

	// create a context for server lifetime
	lifetime, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()

	// start the server
	go func() {
		slog.Info("Server is starting", "port", server.Addr)
		err := server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("Server error", "error", err)
			os.Exit(1)
		}
	}()

	// block until an os.Interrupt occurs
	<-lifetime.Done()

	// create a context for duration of the server shutdown
	countdown := time.Duration(config.ServerShutdownTimeLimit) * time.Second
	shutdownCtx, cancelShutdown := context.WithTimeout(ctx, countdown)
	defer cancelShutdown()

	// shutting down the server
	err := server.Shutdown(shutdownCtx)
	if err != nil {
		slog.Error("Server forced to shutdown", "error", err)
		return err
	}

	slog.Info("Server shutdown complete!")
	return nil
}
