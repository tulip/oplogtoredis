package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/tulip/oplogtoredis/integration-tests/fault-injection/harness"
	"github.com/tulip/oplogtoredis/integration-tests/helpers"
	"github.com/tulip/oplogtoredis/lib/denylist"
)

// This test restarts oplogtoredis after adding denylist entries.
// We expect the entries to still be there afterward, and be removable.
func TestDenylistPersistence(t *testing.T) {
	mongo := harness.StartMongoServer()
	defer mongo.Stop()

	// Sleeping here for a while as the initial connection seems to be unreliable
	time.Sleep(time.Second * 1)

	redis := harness.StartRedisServer()
	defer redis.Stop()

	pg := harness.StartPostgresServer()
	defer pg.Stop()

	// wait before starting OTR for the auth changes to take effects
	time.Sleep(3 * time.Second)

	otr := harness.StartOTRProcessWithEnv(mongo.Addr, redis.Addr, 9000, []string{
		fmt.Sprintf("OTR_PG_PERSISTENCE_URL=%s", pg.ConnStr),
	})
	defer otr.Stop()

	time.Sleep(3 * time.Second)

	baseURL := "http://localhost:9000"
	// PUT new rule
	helpers.DoRequest("PUT", baseURL, "/denylist/abc", t, 201)
	// PUT second rule
	helpers.DoRequest("PUT", baseURL, "/denylist/def", t, 201)
	// GET list with both rules plus the seeded entries
	data := helpers.DoRequest("GET", baseURL, "/denylist", t, 200)
	expectedDenylist := append(append([]string{}, denylist.SeedEntries...), "abc", "def")
	helpers.AssertDenylistContains(t, data, expectedDenylist)

	otr.Stop()
	time.Sleep(3 * time.Second)
	otr.Start()

	time.Sleep(3 * time.Second)

	// GET list with both rules plus the seeded entries
	data = helpers.DoRequest("GET", baseURL, "/denylist", t, 200)
	helpers.AssertDenylistContains(t, data, append(append([]string{}, denylist.SeedEntries...), "abc", "def"))

	// denylist should still be modifiable

	// DELETE first rule
	helpers.DoRequest("DELETE", baseURL, "/denylist/abc", t, 204)
	// GET list with only second rule plus the seeded entries
	data = helpers.DoRequest("GET", baseURL, "/denylist", t, 200)
	helpers.AssertDenylistContains(t, data, append(append([]string{}, denylist.SeedEntries...), "def"))
}
