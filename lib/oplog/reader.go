package oplog

import (
	"encoding/json"

	"github.com/globalsign/mgo/bson"
	"github.com/rwynn/gtm"
	"github.com/tulip/oplogtoredis/lib/log"
	"github.com/tulip/oplogtoredis/lib/redispub"
)

// Tail processes every message from the oplog (read from `in`, which is
// probably `OpC` from a `gtm.OpCtx`), and writes outgoing messages
// to the `out` channel.
func Tail(in <-chan *gtm.Op, out chan<- *redispub.Publication, stop <-chan bool) {
	for {
		select {
		case _ = <-stop:
			// We got a stop message, return to exit the loop
			return
		case op := <-in:
			// We got an oplog entry, process it and write the result to the
			// output channel
			log.Log.Debugw("Got oplog entry", "op", op)
			msg := processOplogEntry(op)

			if msg != nil {
				out <- msg
			}
		}
	}
}

// Process a signal oplog entry. Returns the redispub.Publication that should
// be published for this oplog entry, or nil if nothing should be published.
//
// TODO PERF: Add options for filtering to specific collections or
// databases (https://github.com/tulip/oplogtoredis/issues/8)
func processOplogEntry(op *gtm.Op) *redispub.Publication {
	// Struct that matches the message format redis-oplog expects
	type outgoingMessageDocument struct {
		ID interface{} `json:"_id"`
	}
	type outgoingMessage struct {
		Event  string                  `json:"e"`
		Doc    outgoingMessageDocument `json:"d"`
		Fields []string                `json:"f"`
	}

	if op.IsCommand() {
		// Commands (such as dropping the database, modifying indices,
		// etc.) don't get sent
		return nil
	}

	database, collection := parseDBAndCollection(op)
	if collection == "system.indexes" {
		// We don't publish index creation events
		return nil
	}

	var idForChannel string
	var idForMessage interface{}

	if id, idOK := op.Id.(string); idOK {
		idForChannel = id
		idForMessage = id
	} else if id, idOK := op.Id.(bson.ObjectId); idOK {
		idHex := id.Hex()
		idForChannel = idHex
		idForMessage = map[string]string{
			"$type":  "oid",
			"$value": idHex,
		}
	} else {
		log.Log.Errorw("op.ID was not a string or ObjectID",
			"id", op.Id,
			"op", op,
			"db", database,
			"collection", collection)

		// We don't know how to handle IDs that aren't strings or ObjectIDs,
		// because we don't what what the specific channel (the channel for
		// this specific document) should be.
		return nil
	}

	// Construct the JSON we're going to send to Redis
	//
	// TODO PERF: consider a specialized JSON encoder
	// https://github.com/tulip/oplogtoredis/issues/13
	msg := outgoingMessage{
		Event:  eventNameForOperation(op),
		Doc:    outgoingMessageDocument{idForMessage},
		Fields: fieldsForOperation(op),
	}
	log.Log.Debugw("Sending outgoing message", "message", msg)
	msgJSON, err := json.Marshal(&msg)

	if err != nil {
		log.Log.Error("Error marshalling outgoing message",
			"msg", msg,
			"db", database,
			"collection", collection)

		return nil
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
	}
}

func eventNameForOperation(op *gtm.Op) string {
	if op.Operation == "d" {
		return "r"
	}
	return op.Operation
}

// Given a gtm.Op, returns (database, collection)
func parseDBAndCollection(op *gtm.Op) (string, string) {
	dbAndCollection := op.ParseNamespace()

	switch len(dbAndCollection) {
	case 0:
		// This shouldn't happen -- ParseNamespace is calling SplitN which
		// should always return at least one element
		log.Log.Error("Got empty slice when parsing database and collection",
			"namespace", op.Namespace,
			"parsedNamespace", dbAndCollection,
			"op", op)
		return "", ""
	case 1:
		// Some operations are database-level and don't have a collection
		return dbAndCollection[0], ""
	default:
		// Normal operation where the namesapce is <db>.<collection>
		return dbAndCollection[0], dbAndCollection[1]
	}
}
