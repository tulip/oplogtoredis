package helpers

import (
	"sort"
	"testing"
)

func AssertDenylistContains(t *testing.T, data interface{}, expected []string) {
	t.Helper()

	raw, ok := data.([]interface{})
	if !ok {
		t.Fatalf("Expected a list from GET /denylist, but got %#v", data)
	}

	got := make([]string, 0, len(raw))
	for _, entry := range raw {
		s, ok := entry.(string)
		if !ok {
			t.Fatalf("Expected string entries in /denylist, but got %#v", entry)
		}
		got = append(got, s)
	}

	sort.Strings(got)
	sort.Strings(expected)

	if len(got) != len(expected) {
		t.Fatalf("Expected denylist %#v, but got %#v", expected, got)
	}
	for i := range expected {
		if got[i] != expected[i] {
			t.Fatalf("Expected denylist %#v, but got %#v", expected, got)
		}
	}
}
