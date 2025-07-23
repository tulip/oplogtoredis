package oplog

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/tulip/oplogtoredis/lib/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
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
	Timestamp  primitive.Timestamp
	WallTime   time.Time
	Data       bson.Raw
	Operation  string
	Namespace  string
	Database   string
	Collection string

	TxIdx uint
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

// Returns whether this is an oplog update format v2 update (new in MongoDB 5.0)
func (op *oplogEntry) IsV2Update() bool {
	dataVersionRaw := op.Data.Lookup("$v")
	if dataVersionRaw.IsZero() {
		return false
	}

	dataVersionInt, ok := dataVersionRaw.AsInt64OK()
	if !ok {
		return false
	}

	if dataVersionInt != 2 {
		return false
	}

	diff := op.Data.Lookup("diff")
	return !diff.IsZero()
}

// If this oplogEntry is for an insert, returns whether that insert is a
// replacement (rather than a modification)
func (op *oplogEntry) UpdateIsReplace() bool {
	if data := op.Data.Lookup("$set"); !data.IsZero() {
		return false
	} else if data := op.Data.Lookup("$unset"); !data.IsZero() {
		return false
	} else if op.IsV2Update() {
		// the v2 update format is only used for modifications
		return false
	} else {
		return true
	}
}

// Given an operation, returned the fields affected by that operation
func (op *oplogEntry) ChangedFields() ([]string, error) {
	if op.IsInsert() || (op.IsUpdate() && op.UpdateIsReplace()) {
		return mapKeysRaw(op.Data)
	} else if op.IsUpdate() && op.IsV2Update() {
		// New-style update. Looks like:
		// { $v: 2, diff: { sa: "10", sb: "20", d: { c: true  } }
		return getChangedFieldsFromOplogV2Update(op)
	} else if op.IsUpdate() {
		// Old-style update. Looks like:
		// { $v: 1, $set: { "a": 10, "b": 20 }, $unset: { "c": true } }

		fields := []string{}
		elements, err := op.Data.Elements()
		if err != nil {
			metricUnprocessableChangedFields.Inc()
			log.Log.Errorw("Oplog data for non-replacement v1 update failed to unmarshal",
				"op", op.LogData(), "error", err)
			return []string{}, err
		}
		for _, element := range elements {
			operationKey := element.Key()
			if operationKey == "$v" {
				// $v indicates the update document format; it's not a changed key
				continue
			}

			operationMap, operationMapOK := element.Value().DocumentOK()
			if !operationMapOK {
				metricUnprocessableChangedFields.Inc()
				log.Log.Errorw("Oplog data for non-replacement v1 update contained a key with a non-map value",
					"op", op.LogData(), "operationKey", operationKey)
				continue
			}
			mapFields, err := mapKeysRaw(operationMap)
			if err != nil {
				return []string{}, err
			}
			fields = append(fields, mapFields...)
		}

		return fields, nil
	}

	return []string{}, nil
}

// Oplog entry log data that is sanitized to exclude document data
type oplogEntryLogData struct {
	DocID      interface{}         `json:"docID"`
	Timestamp  primitive.Timestamp `json:"timestamp"`
	WallTime   time.Time           `json:"walltime"`
	Operation  string              `json:"operation"`
	Namespace  string              `json:"namespace"`
	Database   string              `json:"database"`
	Collection string              `json:"collection"`
	TxIdx      uint                `json:"txIdx"`
}

// Creates an oplog entry log data struct, sanitized to exclude document data
func (op *oplogEntry) LogData() oplogEntryLogData {
	return oplogEntryLogData{
		DocID:      op.DocID,
		Timestamp:  op.Timestamp,
		WallTime:   op.WallTime,
		Operation:  op.Operation,
		Namespace:  op.Namespace,
		Database:   op.Database,
		Collection: op.Collection,
		TxIdx:      op.TxIdx,
	}
}

// Given a bson.Raw object, returns the top level keys of the document it represents
func mapKeysRaw(rawData bson.Raw) ([]string, error) {
	elements, err := rawData.Elements()
	if err != nil {
		log.Log.Errorw("Failed to unmarshal oplog data",
			"error", err)
		return []string{}, err
	}
	fields := make([]string, len(elements))

	for i := 0; i < len(fields); i++ {
		fields[i] = elements[i].Key()
	}

	return fields, nil
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
