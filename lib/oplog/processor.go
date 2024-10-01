package oplog

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"strings"

	"github.com/pkg/errors"
	"github.com/tulip/oplogtoredis/lib/log"
	"github.com/tulip/oplogtoredis/lib/redispub"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var ErrUnsupportedDocIDType = errors.New("unsupported document _id type")

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

	if op.Database == "config" {
		// The Config database holds internal MongoDB structures, such as metadata
		// about transactions and locks
		return nil, nil
	}

	var idForChannel string
	var idForMessage interface{}

	switch id := op.DocID.(type) {
	case string:
		idForChannel = id
		idForMessage = id

	case primitive.ObjectID:
		idHex := id.Hex()
		idForChannel = idHex
		idForMessage = map[string]string{
			"$type":  "oid",
			"$value": idHex,
		}

	default:
		// We don't know how to handle IDs that aren't strings or ObjectIDs,
		// because we don't what what the specific channel (the channel for
		// this specific document) should be.
		return nil, errors.Wrapf(ErrUnsupportedDocIDType, "expected string or ObjectID, got %T instead", op.DocID)
	}

	changedFields, errCF := op.ChangedFields()
	if errCF != nil {
		return nil, errors.Wrap(errCF, "error getting changed fields")
	}

	// Construct the JSON we're going to send to Redis
	//
	// TODO PERF: consider a specialized JSON encoder
	// https://github.com/tulip/oplogtoredis/issues/13
	msg := outgoingMessage{
		Event:  eventNameForOperation(op),
		Doc:    outgoingMessageDocument{idForMessage},
		Fields: changedFields,
	}
	log.Log.Debugw("Sending outgoing message", "message", msg)
	msgJSON, err := json.Marshal(&msg)

	if err != nil {
		return nil, errors.Wrap(err, "marshalling outgoing message")
	}

	hash := sha256.Sum256([]byte(op.Database))
	intSlice := hash[len(hash)-8:]

	var hashInt uint64

	err = binary.Read(bytes.NewReader(intSlice), binary.LittleEndian, &hashInt)
	if err != nil {
		panic(errors.Wrap(err, "decoding database hash as uint64"))
	}

	// We need to publish on both the full-collection channel and the
	// single-document channel
	return &redispub.Publication{
		Channels: []string{
			// The "collection" channel is used by redis-oplog for subscriptions
			// that target arbitrary selectors
			op.Namespace,
			// The "specific" channel is used by redis-oplog as a performance
			// optimization for subscriptions that target a specific ID
			op.Namespace + "::" + idForChannel,
		},
		Msg:            msgJSON,
		OplogTimestamp: op.Timestamp,

		TxIdx:          op.TxIdx,
		ParallelismKey: int(hashInt),
	}, nil
}

func eventNameForOperation(op *oplogEntry) string {
	if op.Operation == "d" {
		return "r"
	}
	return op.Operation
}
