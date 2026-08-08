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


func getSender(data []byte) (string, error) {
	type senderOnly struct {
		Sender struct {
			Login string `json:"login"`
		} `json:"sender"`
	}

	var temp senderOnly
	err := json.Unmarshal(data, &temp)
	if err != nil {
		return "", fmt.Errorf("Failed to unmarshal data: %w", err)
	}
	return temp.Sender.Login, nil
}

// Github webhook handler
func whHandler(taskChannel chan types.Task) http.HandlerFunc {
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
		
		sender, err := getSender(body)
		if err != nil {
			slog.Error("Failed to get the webhook sender", "error", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if  sender == config.GithubBotLogin {
			slog.Info("Webhook originates from this platform")
			return
		}

		action := r.Header.Get("X-GitHub-Event")
		switch action {
		case "push":
			var pn types.PushNotification
			err = pn.UnmarshalPushNotification(body)
			if err != nil {
				slog.Error("Failed to unmsarhsal the push notification", "error", err)
				return
			}

			slog.Info("Webhook recieved", "push notification", pn)
			taskChannel <- &pn

		case "pull_request":
			var pr types.PullRequest
			err = pr.UnmarshalpullRequest(body)
			if err != nil {
				slog.Error("Failed to unmsarhsal the pull request", "error", err)
				return
			}

			slog.Info("Webhook recieved", "pull request", pr)
			taskChannel <- &pr

		default:
			slog.Error("Unsupported webhook action", "action", action)
			return
		}
	}
}

// Handles responses from the AI Engine and sends response to respChannel.
func aiEngineResponseHandler(taskChannel chan types.Task) http.HandlerFunc {
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

		realSig := r.Header.Get("HMAC-Signature-256")
		if !verifyMessage(body, config.AIEngineSecret, realSig) {
			w.WriteHeader(http.StatusUnauthorized)
			slog.Warn("Unauthorized request", "warn", err)
			return
		}

		var resp types.AIEngineResponse
		err = json.Unmarshal(body, &resp)
		if err != nil {
			w.WriteHeader(http.StatusUnsupportedMediaType)
			slog.Error("Could not unmarshal the data into a type.Response", "error", err)
			return
		}

		taskChannel <- &resp
	}
}

// Sends a http request to the AI Engine
func SendRequestAIEngine(ctx context.Context, msgType string, msg any) error {
	cli := http.Client{
		Timeout: seconds(10),
	}

	ctx, cancel := context.WithTimeout(ctx, seconds(5))
	defer cancel()

	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("Failed to marshal the message package: %w", err)
	}
	msgReader := bytes.NewReader(msgBytes)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, config.AIEnginePort, msgReader)
	if err != nil {
		return fmt.Errorf("Failed to create http request: %w", err)
	}

	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Message-Type", msgType)

	resp, err := cli.Do(httpReq)
	if err != nil {
		return fmt.Errorf("Failed to send http request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Bad response, status: %v", resp.StatusCode)
	}
	return nil
}

// Starts the http server.
func StartServer(ctx context.Context, taskChannel chan types.Task) error {

	// initialize server
	mux := http.NewServeMux()
	mux.Handle("/", http.HandlerFunc(whHandler(taskChannel)))
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
