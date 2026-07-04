package main

import (
	"fmt"
	// "log"
	"sync"
	"github.com/gorilla/websocket"
	"net/http"
	// "github.com/gorilla/mux"
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

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var clients = make(map[*websocket.Conn]bool)
var broadcast = make(chan []byte)
var mutex = &sync.Mutex{}

func main() {
	http.HandleFunc("/ws", wsHandler)
	fmt.Println("Server starting on port 8080...")
	go broadcastHandler()
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		fmt.Println("Error starting server:", err)
	}
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("Error upgrading to websocket:", err)
		return
	}
	defer conn.Close()

	mutex.Lock()
	clients[conn] = true
	mutex.Unlock()

	connHandler(conn)
}

func connHandler(conn *websocket.Conn) {
	for {
		messageType, payload, err := conn.ReadMessage()
		if err != nil {
			fmt.Println("Error reading message:", err)
			break
		}
		fmt.Println("Message recieved")

		switch messageType {
		case websocket.TextMessage:
			fmt.Println("Recieved text message: ", string(payload))
			prefix := []byte("Broadcasting message: ")
			newPayload := append(prefix, payload...)
			broadcast <- newPayload

		case websocket.PingMessage:
			fmt.Println("Ping handling not implemented yet")
			return

		case websocket.PongMessage:
			fmt.Println("Ping handling not implemented yet")
			return

		default:
			fmt.Println("Error with message type:", err)
			return
		}
	}
}

func broadcastHandler() {
	for {
		message := <-broadcast
		mutex.Lock()
		for client := range clients {
			err := client.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				fmt.Println("Error broadcasting message:", err)
				client.Close()
				delete(clients, client)
			}
		}
		mutex.Unlock()
	}
}
