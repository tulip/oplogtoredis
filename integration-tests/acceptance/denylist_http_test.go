package main

import (
	"os"
	"reflect"
	"testing"

	"github.com/tulip/oplogtoredis/integration-tests/helpers"
)

// Test the /denylist HTTP operations
func TestDenyList(t *testing.T) {
	baseURL := os.Getenv("OTR_URL")

	// GET empty list of rules
	data := helpers.DoRequest("GET", baseURL, "/denylist", t, 200)
	if !reflect.DeepEqual(data, []interface{}{}) {
		t.Fatalf("Expected empty list from blank GET, but got %#v", data)
	}
	// PUT new rule
	helpers.DoRequest("PUT", baseURL, "/denylist/abc", t, 201)
	// GET list with new rule in it
	data = helpers.DoRequest("GET", baseURL, "/denylist", t, 200)
	if !reflect.DeepEqual(data, []interface{}{"abc"}) {
		t.Fatalf("Expected singleton from GET, but got %#v", data)
	}
	// GET existing rule
	data = helpers.DoRequest("GET", baseURL, "/denylist/abc", t, 200)
	if !reflect.DeepEqual(data, "abc") {
		t.Fatalf("Expected matched body from GET, but got %#v", data)
	}
	// PUT second rule
	helpers.DoRequest("PUT", baseURL, "/denylist/def", t, 201)
	// GET second rule
	data = helpers.DoRequest("GET", baseURL, "/denylist/def", t, 200)
	if !reflect.DeepEqual(data, "def") {
		t.Fatalf("Expected matched body from GET, but got %#v", data)
	}
	// GET list with both rules
	data = helpers.DoRequest("GET", baseURL, "/denylist", t, 200)
	// check both permutations, in case the server reordered them
	if !reflect.DeepEqual(data, []interface{}{"abc", "def"}) && !reflect.DeepEqual(data, []interface{}{"def", "abc"}) {
		t.Fatalf("Expected doubleton from GET, but got %#v", data)
	}
	// DELETE first rule
	helpers.DoRequest("DELETE", baseURL, "/denylist/abc", t, 204)
	// GET first rule
	helpers.DoRequest("GET", baseURL, "/denylist/abc", t, 404)
	// GET list with only second rule
	data = helpers.DoRequest("GET", baseURL, "/denylist", t, 200)
	if !reflect.DeepEqual(data, []interface{}{"def"}) {
		t.Fatalf("Expected singleton from GET, but got %#V", data)
	}
}
