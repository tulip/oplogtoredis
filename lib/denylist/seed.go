package denylist

import (
	"sync"

	"github.com/tulip/oplogtoredis/lib/log"
)

var SeedEntries = []string{
	"admin",
}

func Seed(denylist *sync.Map, syncer *Syncer) error {
	for _, id := range SeedEntries {
		if _, exists := denylist.Load(id); exists {
			continue
		}

		denylist.Store(id, true)
		metricFilterEnabled.WithLabelValues(id).Set(1)
		log.Log.Infow("Denylist seed: Added entry", "id", id)

		if err := syncer.StoreDenylistEntry(denylist, id); err != nil {
			return err
		}
	}

	return nil
}
