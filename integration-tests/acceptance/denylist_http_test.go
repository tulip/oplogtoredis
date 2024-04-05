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

func doRequest(method string, path string, t *testing.T, expectedCode int) interface{} {
	req, err := http.NewRequest(method, os.Getenv("OTR_URL")+path, &bytes.Buffer{})
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

// Test the /denylist HTTP operations
func TestDenyList(t *testing.T) {
	// GET empty list of rules
	data := doRequest("GET", "/denylist", t, 200)
	if !reflect.DeepEqual(data, []interface{}{}) {
		t.Fatalf("Expected empty list from blank GET, but got %#v", data)
	}
	// PUT new rule
	doRequest("PUT", "/denylist/abc", t, 201)
	// GET list with new rule in it
	data = doRequest("GET", "/denylist", t, 200)
	if !reflect.DeepEqual(data, []interface{}{"abc"}) {
		t.Fatalf("Expected singleton from GET, but got %#v", data)
	}
	// GET existing rule
	data = doRequest("GET", "/denylist/abc", t, 200)
	if !reflect.DeepEqual(data, "abc") {
		t.Fatalf("Expected matched body from GET, but got %#v", data)
	}
	// PUT second rule
	doRequest("PUT", "/denylist/def", t, 201)
	// GET second rule
	data = doRequest("GET", "/denylistdef/", t, 200)
	if !reflect.DeepEqual(data, "def") {
		t.Fatalf("Expected matched body from GET, but got %#v", data)
	}
	// GET list with both rules
	data = doRequest("GET", "/denylist", t, 200)
	// check both permutations, in case the server reordered them
	if !reflect.DeepEqual(data, []interface{}{"abc", "def"}) && !reflect.DeepEqual(data, []interface{}{"def", "abc"}) {
		t.Fatalf("Expected doubleton from GET, but got %#v", data)
	}
	// DELETE first rule
	doRequest("DELETE", "/denylist/abc", t, 204)
	// GET first rule
	doRequest("GET", "/denylist/abc", t, 404)
	// GET list with only second rule
	data = doRequest("GET", "/denylist", t, 200)
	if !reflect.DeepEqual(data, []interface{}{"def"}) {
		t.Fatalf("Expected singleton from GET, but got %#V", data)
	}
}
