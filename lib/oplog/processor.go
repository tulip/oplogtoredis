package oplog

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/globalsign/mgo/bson"
	"github.com/tulip/oplogtoredis/lib/log"
	"github.com/tulip/oplogtoredis/lib/redispub"
)

// Process a signal oplog entry. Returns the redispub.Publication that should
// be published for this oplog entry, or nil if nothing should be published.
//
// TODO PERF: Add options for filtering to specific collections or
// databases (https://github.com/tulip/oplogtoredis/issues/8)
func processOplogEntry(op *oplogEntry) (*redispub.Publication, error) {
	// Struct that matches the message format redis-oplog expects
	type outgoingMessageDocument struct {
		ID interface{} `json:"_id"`
	}
	type outgoingMessage struct {
		Event  string                  `json:"e"`
		Doc    outgoingMessageDocument `json:"d"`
		Fields []string                `json:"f"`
	}

	if strings.HasPrefix(op.Collection, "system.") {
		// We don't publish index creation events
		return nil, nil
	}

	var idForChannel string
	var idForMessage interface{}

	if id, idOK := op.DocID.(string); idOK {
		idForChannel = id
		idForMessage = id
	} else if id, idOK := op.DocID.(bson.ObjectId); idOK {
		idHex := id.Hex()
		idForChannel = idHex
		idForMessage = map[string]string{
			"$type":  "oid",
			"$value": idHex,
		}
	} else {
		// We don't know how to handle IDs that aren't strings or ObjectIDs,
		// because we don't what what the specific channel (the channel for
		// this specific document) should be.
		return nil, errors.New("op.ID was not a string or ObjectID")
	}

	// Construct the JSON we're going to send to Redis
	//
	// TODO PERF: consider a specialized JSON encoder
	// https://github.com/tulip/oplogtoredis/issues/13
	msg := outgoingMessage{
		Event:  eventNameForOperation(op),
		Doc:    outgoingMessageDocument{idForMessage},
		Fields: op.ChangedFields(),
	}
	log.Log.Debugw("Sending outgoing message", "message", msg)
	msgJSON, err := json.Marshal(&msg)

	if err != nil {
		return nil, fmt.Errorf("Error marshalling outgoing message: %s", err)
	}

	// We need to publish on both the full-collection channel and the
	// single-document channel
	return &redispub.Publication{
		// The "collection" channel is used by redis-oplog for subscriptions
		// that target arbitrary selectors
		CollectionChannel: op.Namespace,

		// The "specific" channel is used by redis-oplog as a performance
		// optimization for subscriptions that target a specific ID
		SpecificChannel: op.Namespace + "::" + idForChannel,

		Msg:            msgJSON,
		OplogTimestamp: op.Timestamp,
	}, nil
}

func eventNameForOperation(op *oplogEntry) string {
	if op.Operation == "d" {
		return "r"
	}
	return op.Operation
}
