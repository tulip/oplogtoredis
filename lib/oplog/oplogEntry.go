package oplog

import (
	"github.com/globalsign/mgo/bson"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/tulip/oplogtoredis/lib/log"
)

const (
	operationInsert  = "i"
	operationUpdate  = "u"
	operationRemove  = "d"
	operationCommand = "c"
)

var metricUnprocessableChangedFields = promauto.NewCounter(prometheus.CounterOpts{
	Namespace: "otr",
	Subsystem: "oplog",
	Name:      "unprocessable_changed_fields",
	Help:      "Oplog messages containing data that we couldn't parse to determine changed fields",
})

// Oplog entry after basic processing to extract the document ID that was
// affected
type oplogEntry struct {
	DocID      interface{}
	Timestamp  bson.MongoTimestamp
	Data       map[string]interface{}
	Operation  string
	Namespace  string
	Database   string
	Collection string
}

// Returns whether this oplogEntry is for an insert
func (op *oplogEntry) IsInsert() bool {
	return op.Operation == operationInsert
}

// Returns whether this oplogEntry is for an update
func (op *oplogEntry) IsUpdate() bool {
	return op.Operation == operationUpdate
}

// Returns whether this oplogEntry is for a remove
func (op *oplogEntry) IsRemove() bool {
	return op.Operation == operationRemove
}

// If this oplogEntry is for an insert, returns whether that insert is a
// replacement (rather than a modification)
func (op *oplogEntry) UpdateIsReplace() bool {
	if _, ok := op.Data["$set"]; ok {
		return false
	} else if _, ok := op.Data["$unset"]; ok {
		return false
	} else {
		return true
	}
}

// Given an operation, returned the fields affected by that operation
func (op *oplogEntry) ChangedFields() []string {
	if op.IsInsert() || (op.IsUpdate() && op.UpdateIsReplace()) {
		return mapKeys(op.Data)
	} else if op.IsUpdate() {
		fields := []string{}
		for operationKey, operation := range op.Data {
			if operationKey == "$v" {
				// $v indicates the version of the update language and should be
				// ignored; it will likely be removed in a future version of
				// Mongo (https://jira.mongodb.org/browse/SERVER-32240)
				continue
			}

			operationMap, operationMapOK := operation.(map[string]interface{})
			if !operationMapOK {
				metricUnprocessableChangedFields.Inc()
				log.Log.Errorw("Oplog data for non-replacement update contained a key with a non-map value",
					"op", op)
				continue
			}

			fields = append(fields, mapKeys(operationMap)...)
		}

		return fields
	}

	return []string{}
}

// Given a map, returns the keys of that map
func mapKeys(m map[string]interface{}) []string {
	fields := make([]string, len(m))

	i := 0
	for key := range m {
		fields[i] = key
		i++
	}

	return fields
}
