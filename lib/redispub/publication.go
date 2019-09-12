package redispub

import (
	"github.com/globalsign/mgo/bson"
)

// Publication represents a message to be sent to Redis about an
// oplog entry.
type Publication struct {
	// The two channels to send the message to
	CollectionChannel string
	SpecificChannel   string

	// Msg is the message to send.
	Msg []byte

	// OplogTimestamp is the timestamp of the oplog entry. Note that this serves as *both*
	// a monotonically increasing timestamp *and* a unique identifier --
	// see https://docs.mongodb.com/manual/reference/bson-types/#timestamps
	OplogTimestamp bson.MongoTimestamp

	// TxIdx is the index of the operation within a transaction. Used to supplement OplogTimestamp in a transaction.
	TxIdx uint
}
