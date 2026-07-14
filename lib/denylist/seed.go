package denylist

import (
	"sync"

	"github.com/tulip/oplogtoredis/lib/log"
)

var SeedEntries = []string{
	"admin",
}

func Seed(denylist *sync.Map, syncer *Syncer) error {
	for _, entry := range SeedEntries {
		if _, exists := denylist.Load(entry); exists {
			continue
		}

		denylist.Store(entry, true)
		metricFilterEnabled.WithLabelValues(entry).Set(1)
		log.Log.Infow("Denylist seed: Added entry", "id", entry)

		if err := syncer.StoreDenylistEntry(denylist, entry); err != nil {
			return err
		}
	}

	return nil
}
