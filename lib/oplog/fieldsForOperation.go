package oplog

import (
	"github.com/rwynn/gtm"
	"github.com/tulip/oplogtoredis/lib/log"
)

// Given a gtm.Op, returned the fields affected by that operation
//
// TODO: test this against more complicated mutations (https://github.com/tulip/oplogtoredis/issues/10)
// TODO TESTING: unit tests for this
func fieldsForOperation(op *gtm.Op) []string {
	if op.IsInsert() || gtm.UpdateIsReplace(op.Data) {
		return mapKeys(op.Data)
	} else if op.IsUpdate() {
		var fields []string
		for operationKey, operation := range op.Data {
			if operationKey == "$v" {
				// $v indicates the version of the update language and should be
				// ignored; it will likely be removed in a future version of
				// Mongo (https://jira.mongodb.org/browse/SERVER-32240)
				continue
			}

			operationMap, operationMapOK := operation.(map[string]interface{})
			if !operationMapOK {
				log.Log.Errorw("Oplog data for update contained $-prefixed key with a non-map value",
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
//
// TODO TESTING: unit tests for this
func mapKeys(m map[string]interface{}) []string {
	fields := make([]string, len(m))

	i := 0
	for key := range m {
		fields[i] = key
		i++
	}

	return fields
}
