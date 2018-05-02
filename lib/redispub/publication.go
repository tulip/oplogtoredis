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

	// Message to send
	Msg []byte

	// The timestamp of the oplog entry. Note that this serves as *both*
	// a monotonically increasing timestamp *and* a unique identifier --
	// see https://docs.mongodb.com/manual/reference/bson-types/#timestamps
	OplogTimestamp bson.MongoTimestamp
}
