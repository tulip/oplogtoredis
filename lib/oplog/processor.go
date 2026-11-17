package oplog

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"strings"
	"unicode/utf8"

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
	var idIsObjectID bool

	switch id := op.DocID.(type) {
	case string:
		idForChannel = id

	case primitive.ObjectID:
		idForChannel = id.Hex()
		idIsObjectID = true

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
	msgJSON := appendOutgoingMessage(nil, eventNameForOperation(op), idForChannel, idIsObjectID, changedFields)
	log.Log.Debugw("Sending outgoing message", "message", json.RawMessage(msgJSON))

	hash := sha256.Sum256([]byte(op.Database))
	hashInt := binary.LittleEndian.Uint64(hash[len(hash)-8:])

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
		WallTime:       op.WallTime,

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

// appendOutgoingMessage appends the message redis-oplog expects for one oplog
// entry to dst:
//
//	{"e":"u","d":{"_id":"someid"},"f":["a","b"]}
//
// An ObjectID _id is sent as {"$type":"oid","$value":"<hex>"}, and nil fields
// as null. The output is byte-for-byte what encoding/json produces for the
// equivalent struct, but the message shape is fixed, so it is written directly
// instead of through reflection.
func appendOutgoingMessage(dst []byte, event, id string, idIsObjectID bool, fields []string) []byte {
	// Capacity hint for the common case where nothing needs escaping: the
	// fixed punctuation, each string plus its quotes, and a comma per field.
	size := len(`{"e":"","d":{"_id":{"$type":"oid","$value":""}},"f":null}`) + len(event) + len(id)
	for _, f := range fields {
		size += len(f) + 3
	}
	if cap(dst)-len(dst) < size {
		grown := make([]byte, len(dst), len(dst)+size)
		copy(grown, dst)
		dst = grown
	}

	dst = append(dst, `{"e":`...)
	dst = appendJSONString(dst, event)
	dst = append(dst, `,"d":{"_id":`...)
	if idIsObjectID {
		dst = append(dst, `{"$type":"oid","$value":`...)
		dst = appendJSONString(dst, id)
		dst = append(dst, '}')
	} else {
		dst = appendJSONString(dst, id)
	}
	dst = append(dst, `},"f":`...)
	if fields == nil {
		dst = append(dst, "null"...)
	} else {
		dst = append(dst, '[')
		for i, f := range fields {
			if i > 0 {
				dst = append(dst, ',')
			}
			dst = appendJSONString(dst, f)
		}
		dst = append(dst, ']')
	}
	return append(dst, '}')
}

// appendJSONString appends s as a JSON string, exactly as encoding/json would.
// Plain printable ASCII is copied as-is. Anything encoding/json escapes or
// rewrites (quotes, backslashes, control characters, <, > and &, and all
// non-ASCII, which covers invalid UTF-8 and U+2028/U+2029) is rare in field
// names and IDs, so those strings are handed to encoding/json itself rather
// than re-implementing its escaping rules.
func appendJSONString(dst []byte, s string) []byte {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < 0x20 || c >= utf8.RuneSelf || c == '"' || c == '\\' || c == '<' || c == '>' || c == '&' {
			encoded, err := json.Marshal(s)
			if err != nil {
				// json.Marshal never fails for a string
				panic(errors.Wrap(err, "marshalling string"))
			}
			return append(dst, encoded...)
		}
	}
	dst = append(dst, '"')
	dst = append(dst, s...)
	return append(dst, '"')
}
