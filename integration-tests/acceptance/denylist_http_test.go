package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"reflect"
	"testing"
)

func doRequest(method string, path string, body map[string]interface{}, t *testing.T, expectedCode int) interface{} {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("Error encoding req body as json: %s", err)
	}
	req, err := http.NewRequest(method, os.Getenv("OTR_URL")+path, bytes.NewBuffer(jsonBody))
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

	if len(respBody) == 0 {
		return nil
	}

	var data interface{}
	err = json.Unmarshal(respBody, &data)
	if err != nil {
		t.Fatalf("Error parsing JSON response: %s", err)
	}

	return data
}

// Test the /denylist HTTP operations
func TestDenyList(t *testing.T) {
	// GET empty list of rules
	data := doRequest("GET", "/denylist", map[string]interface{}{}, t, 200)
	if !reflect.DeepEqual(data, []interface{}{}) {
		t.Fatalf("Expected empty list from blank GET, but got %#v", data)
	}
	// PUT new rule
	data = doRequest("PUT", "/denylist", map[string]interface{}{"keys": "a.b.c", "regex": "^abc$"}, t, 201)
	id, ok := data.(string)
	if !ok {
		t.Fatalf("Expected string from PUT, but got %#v", id)
	}
	// GET list with new rule in it
	data = doRequest("GET", "/denylist", map[string]interface{}{}, t, 200)
	if !reflect.DeepEqual(data, []interface{}{id}) {
		t.Fatalf("Expected singleton from GET, but got %#v", data)
	}
	// GET existing rule
	data = doRequest("GET", "/denylist/"+id, map[string]interface{}{}, t, 200)
	if !reflect.DeepEqual(data, map[string]interface{}{
		"keys":  "a.b.c",
		"regex": "^abc$",
	}) {
		t.Fatalf("Expected matched body from GET, but got %#v", data)
	}
	// PUT second rule
	data = doRequest("PUT", "/denylist", map[string]interface{}{"keys": "d.e.f", "regex": "^def$"}, t, 201)
	id2, ok := data.(string)
	if !ok {
		t.Fatalf("Expected string from PUT, but got %#v", id)
	}
	// GET second rule
	data = doRequest("GET", "/denylist/"+id2, map[string]interface{}{}, t, 200)
	if !reflect.DeepEqual(data, map[string]interface{}{
		"keys":  "d.e.f",
		"regex": "^def$",
	}) {
		t.Fatalf("Expected matched body from GET, but got %#v", data)
	}
	// GET list with both rules
	data = doRequest("GET", "/denylist", map[string]interface{}{}, t, 200)
	if !reflect.DeepEqual(data, []interface{}{id, id2}) {
		t.Fatalf("Expected doubleton from GET, but got %#v", data)
	}
	// DELETE first rule
	doRequest("DELETE", "/denylist/"+id, map[string]interface{}{}, t, 204)
	// GET first rule
	doRequest("GET", "/denylist/"+id, map[string]interface{}{}, t, 404)
	// GET list with only second rule
	data = doRequest("GET", "/denylist", map[string]interface{}{}, t, 200)
	if !reflect.DeepEqual(data, []interface{}{id2}) {
		t.Fatalf("Expected singleton from GET, but got %#V", data)
	}
}
