package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

var testKey string

type Message struct {
	Body      string `json:"body"`      // message content
	Signature string `json:"signature"` // signature verification
}

// generates the HMAC key based on the message and key
func GenerateHMAC(message string) string {
	hash := hmac.New(sha256.New, []byte(testKey))
	hash.Write([]byte(message))
	return hex.EncodeToString(hash.Sum(nil))
}

// verifies if the message is legitimate
func verifyMessage(signature, message string) bool {
	return hmac.Equal([]byte(signature), []byte(GenerateHMAC(message)))
}

func CallRemoteEchoService(n int) (string, error) {

	// Configure an explicit transport timeout layer
	client := &http.Client{
		Timeout: 5 * time.Second, // Hard deadline for entire request/response lifecycle
	}

	// create the test message
	body := fmt.Sprintf("Test ping: %v", n)
	message := Message{
		Body:      body,
		Signature: GenerateHMAC(body),
	}

	buffer := new(bytes.Buffer)
	err := json.NewEncoder(buffer).Encode(message)
	if err != nil {
		return "", err
	}

	// create a context for the http request
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://localhost:8080/api/echo", buffer)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Message-Type", "echo")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	switch resp.Header.Get("Message-Type") {

	case "echo":
		var response Message
		if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
			return "", err
		}

		// verify response
		if verifyMessage(response.Signature, response.Body) {
			return response.Body, nil
		}
		return "", errors.New("Response failed verification")

	case "json-error":
		var response map[string]string

		if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
			return "", err
		}
		return "", errors.New(response["error"])
	}
	return "", errors.New("Invalid header")
}

// Go's test runner will execute this without needing a main() function
func main() {
	fmt.Println("Machine Client sending request...")
	
	for i := range 4 {
		if i < 2 {testKey = "toesmeatfeetkey123"} else {testKey = "wrongpass"}
		response, err := CallRemoteEchoService(i)
		if err != nil {
			fmt.Printf("Request failed: %v\n", err)
		} else {
			fmt.Printf("Server Responded! Response: %s\n", response)
		}


		duration := time.Duration((i+1)*100) * time.Millisecond
		time.Sleep(duration)
		fmt.Printf("Real time: %v\n", duration)
	}
}
