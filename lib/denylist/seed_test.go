package denylist

import (
	"sync"
	"testing"
)

// mapKeys returns the keys currently stored in a sync.Map as a set.
func mapKeys(m *sync.Map) map[string]bool {
	keys := map[string]bool{}
	m.Range(func(key, _ interface{}) bool {
		keys[key.(string)] = true
		return true
	})
	return keys
}

// nonPersistentSyncer returns a Syncer that does not touch a database, so Seed
// can be tested without Postgres.
func nonPersistentSyncer(t *testing.T) *Syncer {
	t.Helper()
	syncer, err := NewSyncer("")
	if err != nil {
		t.Fatalf("NewSyncer returned an unexpected error: %v", err)
	}
	return syncer
}

func TestSeedPopulatesEmptyDenylist(t *testing.T) {
	denylist := &sync.Map{}

	if err := Seed(denylist, nonPersistentSyncer(t)); err != nil {
		t.Fatalf("Seed returned an unexpected error: %v", err)
	}

	keys := mapKeys(denylist)
	for _, entry := range SeedEntries {
		if !keys[entry] {
			t.Errorf("Expected denylist to contain seeded entry %q, but it did not", entry)
		}
	}
	if len(keys) != len(SeedEntries) {
		t.Errorf("Expected denylist to contain exactly %d seeded entries, but got %d: %v",
			len(SeedEntries), len(keys), keys)
	}
}

func TestSeedIsIdempotent(t *testing.T) {
	denylist := &sync.Map{}
	syncer := nonPersistentSyncer(t)

	if err := Seed(denylist, syncer); err != nil {
		t.Fatalf("first Seed returned an unexpected error: %v", err)
	}
	if err := Seed(denylist, syncer); err != nil {
		t.Fatalf("second Seed returned an unexpected error: %v", err)
	}

	keys := mapKeys(denylist)
	if len(keys) != len(SeedEntries) {
		t.Errorf("Expected %d entries after seeding twice, but got %d: %v",
			len(SeedEntries), len(keys), keys)
	}
}

func TestSeedPreservesExistingEntries(t *testing.T) {
	denylist := &sync.Map{}
	denylist.Store("existing", true)

	if err := Seed(denylist, nonPersistentSyncer(t)); err != nil {
		t.Fatalf("Seed returned an unexpected error: %v", err)
	}

	if _, ok := denylist.Load("existing"); !ok {
		t.Error("Expected pre-existing entry \"existing\" to be preserved after seeding")
	}
	for _, entry := range SeedEntries {
		if _, ok := denylist.Load(entry); !ok {
			t.Errorf("Expected seeded entry %q to be present after seeding", entry)
		}
	}
}
