package helpers

import "sort"

// OTRMessage is a message published by OTR to Redis
type OTRMessage struct {
	Event    string                 `json:"e"`
	Document map[string]interface{} `json:"d"`
	Fields   []string               `json:"f"`
}

// SortOTRMessagesByID sorts a slice of OTRMessage by ID (for order-insensitive
// comparisons)
func SortOTRMessagesByID(msgs []OTRMessage) {
	idForMsg := func(msg OTRMessage) string {
		id := msg.Document["_id"]

		// See if it's just a string ID
		stringID, ok := id.(string)
		if ok {
			return stringID
		}

		// pull out object ID
		mapID := id.(map[string]interface{})
		return mapID["$value"].(string)
	}

	sort.Slice(msgs, func(i, j int) bool {
		return idForMsg(msgs[i]) < idForMsg(msgs[j])
	})
}
