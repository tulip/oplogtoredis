package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"reflect"
	"testing"
)

// Basic test of an update
func TestHealthz(t *testing.T) {
	// Send request
	requestURL := fmt.Sprintf("%s/healthz", os.Getenv("OTR_URL"))

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

	var data map[string]interface{}
	err = json.Unmarshal(respBody, &data)
	if err != nil {
		t.Fatalf("Error parsing JSON response: %s", err)
	}

	if !reflect.DeepEqual(data, map[string]interface{}{
		"mongoOK": true,
		"redisOK": true,
	}) {
		t.Errorf("Got incorrect response.\n    Expected: {\"ok\": true}\n    Got: %#v", data)
	}
}
