package denylist

import (
	"database/sql"
	"sync"

	_ "github.com/lib/pq"
)

type Syncer struct {
	Persistent bool
	Handle     *sql.DB
}

func NewSyncer(persistenceURL string) (*Syncer, error) {
	if persistenceURL == "" {
		return &Syncer{
			Persistent: false,
		}, nil
	}

	db, err := sql.Open("postgres", persistenceURL)
	if err != nil {
		return nil, err
	}
	return &Syncer{
		Persistent: true,
		Handle:     db,
	}, nil
}

func (syncer *Syncer) LoadDenylist() (*sync.Map, error) {
	if syncer.Persistent {
		_, err := syncer.Handle.Exec("CREATE TABLE IF NOT EXISTS otr_denylist (entry VARCHAR(255) UNIQUE);")
		if err != nil {
			return nil, err
		}
		rows, err := syncer.Handle.Query("SELECT entry FROM otr_denylist;")
		if err != nil {
			return nil, err
		}
		var entry string
		denylist := sync.Map{}
		for rows.Next() {
			err = rows.Scan(&entry)
			if err != nil {
				return nil, err
			}
			denylist.Store(entry, true)
		}
		return &denylist, nil
	}

	return &sync.Map{}, nil
}

func (syncer *Syncer) StoreDenylistEntry(denylist *sync.Map, id string) error {
	if syncer.Persistent {
		_, err := syncer.Handle.Exec("INSERT INTO otr_denylist (entry) VALUES ($1) ON CONFLICT DO NOTHING;", id)
		if err != nil {
			return err
		}
		return nil
	}

	return nil
}

func (syncer *Syncer) DeleteDenylistEntry(denylist *sync.Map, id string) error {
	if syncer.Persistent {
		_, err := syncer.Handle.Exec("DELETE FROM otr_denylist WHERE entry=$1;", id)
		if err != nil {
			return err
		}
		return nil
	}

	return nil
}
