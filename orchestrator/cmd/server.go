package main

import (
	"context"
	_ "crypto/hmac"
	_ "crypto/sha256"
	_ "encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"time"
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

type RequestBody struct {
	Message   string `json:"message"`
	Signature string `json: "signature"`
}

type ResponseBody struct {
	Status    string    `json:"status"`
	Echo      string    `json:"echo"`
	Timestamp time.Time `json:"timestamp"`
	Signature string    `json: "signature"`
}

// TODO: temporary, fix this later
var key string = "temp-password"

// verifies if the message is legitimate
func verifyMessage(message, sig string) bool {
	return true
}

func echoHandler(w http.ResponseWriter, r *http.Request) {

	// extract the status
	customStatus := r.URL.Query().Get("status")
	if customStatus == "" {
		customStatus = "success"
	}

	// decode the request
	var body RequestBody
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&body); err != nil {
		writeJSONError(w, "Invalid JSON payload or unknown fields", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()
	if body.Message == "" {
		writeJSONError(w, "Missing required field: 'message'", http.StatusUnprocessableEntity)
		return
	}

	// testing: output the message
	fmt.Printf("Recieved message: %s\n", body.Message)

	// initialize the response
	resp := ResponseBody{
		Status:    customStatus,
		Echo:      body.Message,
		Timestamp: time.Now().UTC(),
	}

	// set up the headers
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// send the response encoded into a json
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Error("Failed to encode response", "error", err)
	}
}

// send back an error message as a json
func writeJSONError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func main() {
	// initialize log creator
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// initialize a custom multiplexer
	mux := http.NewServeMux()

	// initialize handlers for each route
	mux.Handle("POST /api/echo", http.HandlerFunc(echoHandler))
	mux.Handle("GET /", http.FileServer(http.Dir(".")))

	// initialize the server settings
	server := &http.Server{
		Addr:              ":8080",
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
