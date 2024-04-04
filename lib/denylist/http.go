package denylist

import (
	"encoding/json"
	"net/http"
	"strings"
)

// CollectionEndpoint serves the endpoints for the whole Denylist at /denylist
func CollectionEndpoint(denylist *Denylist) func(http.ResponseWriter, *http.Request) {
	return func(response http.ResponseWriter, request *http.Request) {
		switch request.Method {
		case "GET":
			listDenylistKeys(response, denylist)
		case "PUT":
			createDenylistEntry(response, request, denylist)
		default:
			http.Error(response, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		}
	}
}

// SingleEndpoint serves the endpoints for particular Denylist entries at /denylist/...
func SingleEndpoint(denylist *Denylist) func(http.ResponseWriter, *http.Request) {
	return func(response http.ResponseWriter, request *http.Request) {
		switch request.Method {
		case "GET":
			getDenylistEntry(response, request, denylist)
		case "DELETE":
			deleteDenylistEntry(response, request, denylist)
		default:
			http.Error(response, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		}
	}
}

// GET /denylist
func listDenylistKeys(response http.ResponseWriter, denylist *Denylist) {
	keys := denylist.GetKeys()

	response.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(response).Encode(keys)
	if err != nil {
		http.Error(response, "couldn't encode result", http.StatusInternalServerError)
		return
	}
	response.WriteHeader(http.StatusOK)
}

// PUT /denylist
func createDenylistEntry(response http.ResponseWriter, request *http.Request, denylist *Denylist) {
	if request.Header.Get("Content-Type") != "application/json" {
		http.Error(response, "request must be JSON", http.StatusBadRequest)
		return
	}
	decoder := json.NewDecoder(request.Body)
	var payload map[string]string
	err := decoder.Decode(&payload)
	if err != nil {
		http.Error(response, err.Error(), http.StatusBadRequest)
		return
	}

	unparsedKeys, keysOk := payload["keys"]
	unparsedRegex, regexOk := payload["regex"]
	if !keysOk || !regexOk {
		http.Error(response, "request body must contain `keys` and `regex`", http.StatusBadRequest)
		return
	}

	id, err := denylist.AppendEntry(unparsedKeys, unparsedRegex)
	if err != nil {
		http.Error(response, err.Error(), http.StatusBadRequest)
		return
	}

	response.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(response).Encode(id)
	if err != nil {
		http.Error(response, "couldn't encode result", http.StatusInternalServerError)
		return
	}
	response.WriteHeader(http.StatusCreated)
}

// GET /denylist/...
func getDenylistEntry(response http.ResponseWriter, request *http.Request, denylist *Denylist) {
	id := request.URL.Path
	entry := denylist.GetEntry(id)
	if entry == nil {
		http.Error(response, "denylist entry not found with that id", http.StatusNotFound)
		return
	}

	payload := map[string]string{
		"keys":  strings.Join(entry.Keys, KeysSeparator),
		"regex": entry.Regex.String(),
	}

	response.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(response).Encode(payload)
	if err != nil {
		http.Error(response, "couldn't encode result", http.StatusInternalServerError)
		return
	}
	response.WriteHeader(http.StatusOK)
}

// DELETE /denylist/...
func deleteDenylistEntry(response http.ResponseWriter, request *http.Request, denylist *Denylist) {
	id := request.URL.Path
	deleted := denylist.DeleteEntry(id)

	if !deleted {
		http.Error(response, "denylist entry not found with that id", http.StatusNotFound)
		return
	}

	response.WriteHeader(http.StatusNoContent)
}
