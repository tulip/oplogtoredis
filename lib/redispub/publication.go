package redispub

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Publication represents a message to be sent to Redis about an
// oplog entry.
type Publication struct {
	// The channels to send the message to
	Channels []string

	// Msg is the message to send.
	Msg []byte

	// OplogTimestamp is the timestamp of the oplog entry. Note that this serves as *both*
	// a monotonically increasing timestamp *and* a unique identifier --
	// see https://docs.mongodb.com/manual/reference/bson-types/#timestamps
	OplogTimestamp primitive.Timestamp

	// TxIdx is the index of the operation within a transaction. Used to supplement OplogTimestamp in a transaction.
	TxIdx uint

	// ParallelismKey is a number representing which parallel write loop will process this message.
	// It is a hash of the database name, assuming that a single database is the unit of ordering guarantee.
	ParallelismKey int
}
