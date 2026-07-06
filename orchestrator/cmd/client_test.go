package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"
	"fmt"
)

func CallRemoteEchoService(n int) (*ResponseBody, error) {
	// Configure an explicit transport timeout layer
	client := &http.Client{
		Timeout: 5 * time.Second, // Hard deadline for entire request/response lifecycle
	}

	payload := RequestBody{Message: fmt.Sprintf("Internal cluster ping: %v", n)}
	jsonBytes, _ := json.Marshal(payload)

	// create a context for the http request
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://localhost:8080/api/echo?status=cluster_ok", bytes.NewBuffer(jsonBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result ResponseBody
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// Go's test runner will execute this without needing a main() function
func TestCallRemoteEchoService(t *testing.T) {
	t.Log("Machine Client sending request...")

	for i := 0; i < 10; i++ {
		res, err := CallRemoteEchoService(i)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		t.Logf("Server Responded! Status: %s | Echo: %s", res.Status, res.Echo)
		duration := time.Duration((i + 1) * 100) * time.Millisecond
		time.Sleep(duration)
		fmt.Printf("Real time: %v\n", duration)
	}
}
