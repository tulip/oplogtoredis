package helpers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func DoRequest(method string, baseURL string, path string, t *testing.T, expectedCode int) interface{} {
	req, err := http.NewRequest(method, baseURL+path, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Error creating req: %s", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		t.Fatalf("Error sending request: %s", err)
	}

	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Error eceiving response body: %s", err)
	}

	if resp.StatusCode != expectedCode {
		t.Fatalf("Expected status code %d, but got %d.\nBody was: %s", expectedCode, resp.StatusCode, respBody)
	}

	if expectedCode == 200 {
		var data interface{}
		err = json.Unmarshal(respBody, &data)
		if err != nil {
			t.Fatalf("Error parsing JSON response: %s", err)
		}

		return data
	}
	return nil
}
