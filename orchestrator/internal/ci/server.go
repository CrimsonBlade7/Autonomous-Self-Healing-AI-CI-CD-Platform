package ci

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/CrimsonBlade7/Autonomous-Self-Healing-AI-CI-CD-Platform/orchestrator/internal/types"
)

/*
	TextMessage denotes a text data message. The text message payload is
	interpreted as UTF-8 encoded text data.
	TextMessage = 1

	BinaryMessage denotes a binary data message.
	BinaryMessage = 2

	CloseMessage denotes a close control message. The optional message
	payload contains a numeric code and text. Use the FormatCloseMessage
	function to format a close message payload.
	CloseMessage = 8

	PingMessage denotes a ping control message. The optional message payload
	is UTF-8 encoded text.
	PingMessage = 9

	PongMessage denotes a pong control message. The optional message payload
	is UTF-8 encoded text.
	PongMessage = 10
*/

// Generates the HMAC key based on the message and secret
func GenerateHMAC(message []byte, secret string) string {
	hash := hmac.New(sha256.New, []byte(secret))
	hash.Write(message)
	return hex.EncodeToString(hash.Sum(nil))
}

// Verifies if the message is legitimate
func verifyMessage(message []byte, signature, secret string) bool {
	return hmac.Equal([]byte(signature), []byte(GenerateHMAC(message, secret)))
}

// Github webhook handler
func whHandler(secret string, jobs chan types.Job) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			slog.Error("Failed to read request body", "error", err)
			return
		}

		realSig := strings.TrimPrefix(r.Header.Get("X-Hub-Signature-256"), "sha256=")
		if !verifyMessage(body, realSig, secret) {
			w.WriteHeader(http.StatusUnauthorized)
			slog.Warn("Unauthorized request", "warn", err)
			return
		}

		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			slog.Error("Not a post request", "error", err)
			return
		}

		var pullRequest types.PullRequest
		err = pullRequest.UnmarshalJSON(body)
		if err != nil {
			slog.Error("Failed to unmsarhsal the json", "error", err)
			return
		}

		slog.Info("Job recieved", "pull request", pullRequest)

		jobs <- types.Job{
			PullReq: pullRequest,
		}
	}
}

// Starts the http server with secret s and port p
func StartServer(ctx context.Context, secret, port string, jobs chan types.Job) error {

	// initialize server
	mux := http.NewServeMux()
	mux.Handle("/", http.HandlerFunc(whHandler(secret, jobs)))
	server := &http.Server{
		Addr:              port,
		Handler:           mux,               // Inject your isolated router
		ReadTimeout:       5 * time.Second,   // Max time to read the request body
		ReadHeaderTimeout: 2 * time.Second,   // Max time to read just the headers
		WriteTimeout:      10 * time.Second,  // Max time to write the response
		IdleTimeout:       120 * time.Second, // Max time to keep idle connections alive
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
	countdown := 30 * time.Second
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
