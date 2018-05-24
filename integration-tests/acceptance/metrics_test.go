package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"testing"
)

// Basic test of an update
func TestMetrics(t *testing.T) {
	// Send request
	requestURL := fmt.Sprintf("%s/metrics", os.Getenv("OTR_URL"))

	resp, err := http.Get(requestURL)
	if err != nil {
		t.Fatalf("Error sending request: %s", err)
	}

	defer resp.Body.Close()

	// Get response body
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Error receiving response body: %s", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status code 200, but got %d.\n    Body was: %s",
			resp.StatusCode, respBody)
	}
}
