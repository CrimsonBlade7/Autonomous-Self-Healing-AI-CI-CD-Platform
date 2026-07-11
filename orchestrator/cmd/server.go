package main

import (
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

	"github.com/joho/godotenv"
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

type PullRequest struct {
	Action  string `json:"action"`
	Url     string `json:"url"`
	Title   string `json:"title"`
	Body    string `json:"body"`
	HeadSHA string `json:"headsha"`
	BaseSHA string `json:"basesha"`
}

var (
	secret string
	port   string
)

// Converts a byte slice into a PullRequest struct
func (pr *PullRequest) UnmarshalJSON(data []byte) error {
	var temp struct {
		Action      string `json:"action"`
		PullRequest struct {
			Url   string `json:"url"`
			Title string `json:"title"`
			Body  string `json:"body"`
			Head  struct {
				Sha string `json:"sha"`
			} `json:"head"`
			Base struct {
				Sha string `json:"sha"`
			} `json:"base"`
		} `json:"pull_request"`
	}

	err := json.Unmarshal(data, &temp)
	if err != nil {
		return errors.New("Couldn't unmarshal the json webhook")
	}

	pr.Action = temp.Action
	pr.Url = temp.PullRequest.Url
	pr.Title = temp.PullRequest.Title
	pr.Body = temp.PullRequest.Body
	pr.HeadSHA = temp.PullRequest.Head.Sha
	pr.BaseSHA = temp.PullRequest.Base.Sha

	return nil
}

// Loads the .env variables
func loadEnv() {
	err := godotenv.Load("../.env")
	if err != nil {
		slog.Error("Error loading .env file", "error", err)
		return
	}
	secret = os.Getenv("SECRET")
	if secret == "" {
		slog.Error("Error: HMAC secret has not been set")
		return
	}
	port = os.Getenv("PORT")
	if port == "" {
		slog.Error("Error: Port has not been set")
		return
	}
}

// Generates the HMAC key based on the message and secret
func GenerateHMAC(message []byte) string {
	hash := hmac.New(sha256.New, []byte(secret))
	hash.Write(message)
	return hex.EncodeToString(hash.Sum(nil))
}

// Verifies if the message is legitimate
func verifyMessage(signature string, message []byte) bool {
	return hmac.Equal([]byte(signature), []byte(GenerateHMAC(message)))
}

// Handles the extracted pull request
func handlePullRequest(pr PullRequest) {
	// to be implemented later
}

// Github webhook handler
func whHandler(w http.ResponseWriter, r *http.Request) {

	defer r.Body.Close()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		slog.Error("Cannot read body")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	realSig := strings.TrimPrefix(string(r.Header.Get("X-Hub-Signature-256")), "sha256=")
	if !verifyMessage(realSig, body) {
		slog.Info("Signature verification failed")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var pullRequest PullRequest
	pullRequest.UnmarshalJSON(body)

	// testing: output the message
	fmt.Println("Recieved message: ")
	testPrint := "" +
		"Action: " + pullRequest.Action + "\n" +
		"Url: " + pullRequest.Url + "\n" +
		"Title: " + pullRequest.Title + "\n" +
		"Body: " + pullRequest.Body + "\n" +
		"HeadSHA: " + pullRequest.HeadSHA + "\n" +
		"BaseSHA:" + pullRequest.BaseSHA

	fmt.Println(testPrint)

	w.WriteHeader(http.StatusOK)

	go handlePullRequest(pullRequest)
}

func main() {

	loadEnv()

	// initialize log creator
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// initialize a custom multiplexer
	mux := http.NewServeMux()

	// initialize handlers for each route
	mux.Handle("/api/webhook", http.HandlerFunc(whHandler))
	mux.Handle("/", http.FileServer(http.Dir(".")))

	// initialize the server settings
	server := &http.Server{
		Addr:              port,
		Handler:           mux,               // Inject your isolated router
		ReadTimeout:       5 * time.Second,   // Max time to read the request body
		ReadHeaderTimeout: 2 * time.Second,   // Max time to read just the headers
		WriteTimeout:      10 * time.Second,  // Max time to write the response
		IdleTimeout:       120 * time.Second, // Max time to keep idle connections alive
	}

	// create a context to manage the server's lifetime
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	// end the context when the function returns
	defer stop()

	// asynchronously open the server and listen on it
	// when the server ends abnormally, force exit
	go func() {
		fmt.Printf("Server is starting on: %s\n", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Printf("Server error: %v\n", err)
			os.Exit(1)
		}
	}()

	// block until the context recieves an os.Interrupt
	<-ctx.Done()

	// create a context to manage shutting down
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 30*time.Second)
	// end the context when the function ends
	defer cancelShutdown()

	// block until the shutdown responds
	// if it does not respond or errors, the context forces it to end and it errors
	if err := server.Shutdown(shutdownCtx); err != nil {
		fmt.Printf("Server forced to shutdown by: %v", err)
	}

	fmt.Printf("Server shutdown complete!")
}
