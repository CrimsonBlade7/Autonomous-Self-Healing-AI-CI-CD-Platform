package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	_ "log/slog"
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
	Message string `json:"message"`
}

type ResponseBody struct {
	Status    string    `json:"status"`
	Echo      string    `json:"echo"`
	Timestamp time.Time `json:"timestamp"`
}

type EchoHandler struct{}

func (h EchoHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	customStatus := r.URL.Query().Get("status")
	if customStatus == "" {
		customStatus = "success"
	}

	var body RequestBody
	decode := json.NewDecoder(r.Body)
	decode.DisallowUnknownFields()

}

func main() {
	// logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	// slog.SetDefault(logger)

	mux := http.NewServeMux()

	mux.Handle("POST /api/echo", EchoHandler{})
	mux.Handle("GET /", http.FileServer(http.Dir(".")))

	server := &http.Server{
		Addr:         ":8080",
		Handler:      mux,               // Inject your isolated router
		ReadTimeout:  5 * time.Second,   // Max time to read the request body
		WriteTimeout: 10 * time.Second,  // Max time to write the response
		IdleTimeout:  120 * time.Second, // Max time to keep idle connections alive
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	go func() {
		fmt.Printf("Server is starting on: %s\n", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Printf("Error starting server: %v\n", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelShutdown()

	if err := server.Shutdown(shutdownCtx); err != nil {
		fmt.Printf("Server forced to shutdown by: %v", err)
	}

	fmt.Printf("Server shutdown complete!")
}
