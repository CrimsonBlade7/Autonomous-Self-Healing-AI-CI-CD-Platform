package main

import (
	"context"
	"errors"
	"fmt"
	_ "log/slog"
	"net/http"
	_ "net/http"
	"os"
	"os/signal"
	_ "sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	_ "github.com/coder/websocket/wsjson"
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

func main() {
	// logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	// slog.SetDefault(logger)

	mux := http.NewServeMux()

	mux.HandleFunc("/", wsHandler)

	server := &http.Server{
		Addr:         ":8080",
		Handler:      mux,               // Inject your isolated router
		ReadTimeout:  5 * time.Second,   // Max time to read the request body
		WriteTimeout: 10 * time.Second,  // Max time to write the response
		IdleTimeout:  120 * time.Second, // Max time to keep idle connections alive
	}

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
	)
	defer stop()

	serverError := make(chan error, 1)

	go func() {
		fmt.Printf("Server is starting on: %s\n", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverError <- err
			fmt.Printf("Error starting server: %v\n", err)
		}
	}()

	select {
	case <-serverError:
		fmt.Printf("Server startup failed.")
		return

	case <-ctx.Done():
		fmt.Printf("Shutdown signal recieved. Shutting down server...")
	}

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelShutdown()

	if err := server.Shutdown(shutdownCtx); err != nil {
		fmt.Printf("Server forced to shutdown by: %v", err)
	}

	fmt.Printf("Server shutdown complete!")
}

// Upgrades http to websocket connection
func wsHandler(w http.ResponseWriter, r *http.Request) {
	options := &websocket.AcceptOptions{
		// TODO: properly verify connection origin for websocket upgrade
		InsecureSkipVerify: true,
	}

	conn, err := websocket.Accept(w, r, options)
	if err != nil {
		fmt.Printf("Failed to accept websocket: %v", err)
		return
	}

	defer conn.CloseNow()

	for {
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		var message map[string]string

		err := wsjson.Read(ctx, conn, message)
		cancel()
		if err != nil {
			fmt.Printf("Read error (or client disconnected): %v", err)
			break
		}

		fmt.Printf("Recieved message: %v", message)
	}

}
