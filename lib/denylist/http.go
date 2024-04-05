package denylist

import (
	"encoding/json"
	"net/http"
)

// CollectionEndpoint serves the endpoints for the whole Denylist at /denylist
func CollectionEndpoint(denylist *map[string]bool) func(http.ResponseWriter, *http.Request) {
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
func SingleEndpoint(denylist *map[string]bool) func(http.ResponseWriter, *http.Request) {
	return func(response http.ResponseWriter, request *http.Request) {
		switch request.Method {
		case "GET":
			getDenylistEntry(response, request, denylist)
		case "PUT":
			createDenylistEntry(response, request, denylist)
		case "DELETE":
			deleteDenylistEntry(response, request, denylist)
		default:
			http.Error(response, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		}
	}
}

// GET /denylist
func listDenylistKeys(response http.ResponseWriter, denylist *map[string]bool) {
	keys := make([]string, len(*denylist))

	i := 0
	for k := range *denylist {
		keys[i] = k
		i++
	}

	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(http.StatusOK)
	err := json.NewEncoder(response).Encode(keys)
	if err != nil {
		http.Error(response, "couldn't encode result", http.StatusInternalServerError)
		return
	}
}

// GET /denylist/...
func getDenylistEntry(response http.ResponseWriter, request *http.Request, denylist *map[string]bool) {
	id := request.URL.Path
	_, exists := (*denylist)[id]
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
func createDenylistEntry(response http.ResponseWriter, request *http.Request, denylist *map[string]bool) {
	id := request.URL.Path
	_, exists := (*denylist)[id]
	if exists {
		response.WriteHeader(http.StatusNoContent)
		return
	}

	(*denylist)[id] = true
	response.WriteHeader(http.StatusCreated)
}

// DELETE /denylist/...
func deleteDenylistEntry(response http.ResponseWriter, request *http.Request, denylist *map[string]bool) {
	id := request.URL.Path
	_, exists := (*denylist)[id]
	if !exists {
		http.Error(response, "denylist entry not found with that id", http.StatusNotFound)
		return
	}

	delete(*denylist, id)
	response.WriteHeader(http.StatusNoContent)
}
