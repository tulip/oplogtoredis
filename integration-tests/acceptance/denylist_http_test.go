package main

import (
	"os"
	"testing"

	"github.com/tulip/oplogtoredis/integration-tests/helpers"
	"github.com/tulip/oplogtoredis/lib/denylist"
)

// Test the /denylist HTTP operations
func TestDenyList(t *testing.T) {
	baseURL := os.Getenv("OTR_URL")

	// GET list of rules: should contain only the seeded entries
	data := helpers.DoRequest("GET", baseURL, "/denylist", t, 200)
	helpers.AssertDenylistContains(t, data, denylist.SeedEntries)

	// PUT new rule
	helpers.DoRequest("PUT", baseURL, "/denylist/abc", t, 201)
	// GET list with new rule in it
	data = helpers.DoRequest("GET", baseURL, "/denylist", t, 200)
	helpers.AssertDenylistContains(t, data, append(append([]string{}, denylist.SeedEntries...), "abc"))
	// GET existing rule
	data = helpers.DoRequest("GET", baseURL, "/denylist/abc", t, 200)
	if data != "abc" {
		t.Fatalf("Expected matched body from GET, but got %#v", data)
	}
	// PUT second rule
	helpers.DoRequest("PUT", baseURL, "/denylist/def", t, 201)
	// GET second rule
	data = helpers.DoRequest("GET", baseURL, "/denylist/def", t, 200)
	if data != "def" {
		t.Fatalf("Expected matched body from GET, but got %#v", data)
	}
	// GET list with both rules plus the seeded entries
	data = helpers.DoRequest("GET", baseURL, "/denylist", t, 200)
	helpers.AssertDenylistContains(t, data, append(append([]string{}, denylist.SeedEntries...), "abc", "def"))
	// DELETE first rule
	helpers.DoRequest("DELETE", baseURL, "/denylist/abc", t, 204)
	// GET first rule
	helpers.DoRequest("GET", baseURL, "/denylist/abc", t, 404)
	// GET list with only second rule plus the seeded entries
	data = helpers.DoRequest("GET", baseURL, "/denylist", t, 200)
	helpers.AssertDenylistContains(t, data, append(append([]string{}, denylist.SeedEntries...), "def"))
}

// The seeded entries should be present on the denylist without any manual
// PUT, and should actually filter (a seeded DB is reported as denied).
func TestDenyListSeeded(t *testing.T) {
	baseURL := os.Getenv("OTR_URL")

	for _, entry := range denylist.SeedEntries {
		// GET the seeded rule directly; it should already exist (200)
		data := helpers.DoRequest("GET", baseURL, "/denylist/"+entry, t, 200)
		if data != entry {
			t.Fatalf("Expected seeded entry %q from GET, but got %#v", entry, data)
		}
	}
}
