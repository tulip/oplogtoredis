package denylist

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/tulip/oplogtoredis/lib/log"
)

var metricFilterEnabled = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Namespace: "otr",
	Subsystem: "denylist",
	Name:      "filter_enabled",
	Help:      "Gauge indicating whether the denylist filter is enabled for a particular DB name",
}, []string{"db"})

// CollectionEndpoint serves the endpoints for the whole Denylist at /denylist
func CollectionEndpoint(denylist *sync.Map, syncer *Syncer) func(http.ResponseWriter, *http.Request) {
	return func(response http.ResponseWriter, request *http.Request) {
		switch request.Method {
		case "GET":
			listDenylistKeys(response, denylist)
		default:
			http.Error(response, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		}
	}
}

// SingleEndpoint serves the endpoints for particular Denylist entries at /denylist/...
func SingleEndpoint(denylist *sync.Map, syncer *Syncer) func(http.ResponseWriter, *http.Request) {
	return func(response http.ResponseWriter, request *http.Request) {
		switch request.Method {
		case "GET":
			getDenylistEntry(response, request, denylist)
		case "PUT":
			createDenylistEntry(response, request, denylist, syncer)
		case "DELETE":
			deleteDenylistEntry(response, request, denylist, syncer)
		default:
			http.Error(response, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		}
	}
}

// GET /denylist
func listDenylistKeys(response http.ResponseWriter, denylist *sync.Map) {
	keys := []interface{}{}

	denylist.Range(func(key interface{}, value interface{}) bool {
		keys = append(keys, key)
		return true
	})

	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(http.StatusOK)
	err := json.NewEncoder(response).Encode(keys)
	if err != nil {
		http.Error(response, "couldn't encode result", http.StatusInternalServerError)
		return
	}
}

// GET /denylist/...
func getDenylistEntry(response http.ResponseWriter, request *http.Request, denylist *sync.Map) {
	id := request.URL.Path
	if strings.Contains(id, "/") {
		http.Error(response, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	_, exists := denylist.Load(id)
	if !exists {
		http.Error(response, "denylist entry not found with that id", http.StatusNotFound)
		return
	}

	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(http.StatusOK)
	err := json.NewEncoder(response).Encode(id)
	if err != nil {
		http.Error(response, "couldn't encode result", http.StatusInternalServerError)
		return
	}
}

// PUT /denylist/...
func createDenylistEntry(response http.ResponseWriter, request *http.Request, denylist *sync.Map, syncer *Syncer) {
	id := request.URL.Path
	if strings.Contains(id, "/") {
		http.Error(response, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	_, exists := denylist.Load(id)
	if exists {
		response.WriteHeader(http.StatusNoContent)
		return
	}

	denylist.Store(id, true)
	log.Log.Infow("Created denylist entry", "id", id)
	metricFilterEnabled.WithLabelValues(id).Set(1)
	err := syncer.StoreDenylistEntry(denylist, id)
	if err != nil {
		http.Error(response, "failed to persist removal of denylist entry", http.StatusInternalServerError)
		return
	}

	response.WriteHeader(http.StatusCreated)
}

// DELETE /denylist/...
func deleteDenylistEntry(response http.ResponseWriter, request *http.Request, denylist *sync.Map, syncer *Syncer) {
	id := request.URL.Path
	if strings.Contains(id, "/") {
		http.Error(response, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	_, exists := denylist.Load(id)
	if !exists {
		response.WriteHeader(http.StatusNoContent)
		return
	}

	denylist.Delete(id)
	log.Log.Infow("Deleted denylist entry", "id", id)
	metricFilterEnabled.WithLabelValues(id).Set(0)
	err := syncer.DeleteDenylistEntry(denylist, id)
	if err != nil {
		http.Error(response, "failed to persist removal of denylist entry", http.StatusInternalServerError)
		return
	}

	response.WriteHeader(http.StatusNoContent)
}
