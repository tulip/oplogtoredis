package oplog

import (
	"errors"
	"regexp"
	"strings"

	"github.com/tulip/oplogtoredis/lib/config"
	"github.com/tulip/oplogtoredis/lib/log"

	"go.mongodb.org/mongo-driver/bson"
)

// Translated from https://github.com/meteor/meteor/blob/devel/packages/mongo/oplog_v2_converter.js

var arrayIndexOperatorKeyRegex = regexp.MustCompile(`^u\d+`)

func isArrayOperator(possibleArrayOperator interface{}) bool {
	if possibleArrayOperator == nil {
		return false
	}

	switch typedPossibleArrayOperator := possibleArrayOperator.(type) {
	case map[string]interface{}:
		if (typedPossibleArrayOperator == nil) || len(typedPossibleArrayOperator) == 0 {
			return false
		}

		_, hasA := typedPossibleArrayOperator["a"]
		if !hasA {
			return false
		}

		for _, key := range mapKeys(typedPossibleArrayOperator) {
			if key != "a" && !arrayIndexOperatorKeyRegex.MatchString(key) {
				// we have found a field in here that's not valid inside
				// an array operator
				return false
			}
		}

		return true
	default:
		return false
	}
}

// Given a map, with other maps possibly nested under it, returns the
// flattened object keys. E.g.:
//
//	{
//	  a: {
//	    b: {
//	      c: [{d: 1}],
//	      e: 2
//	    },
//	    f: 3
//	  }
//	}
//
// becomes
// ['a.b.c', 'a.b.e', 'a.f']
func flatObjectKeys(prefix string, obj map[string]interface{}) []string {
	acc := []string{}

	for key, val := range obj {
		switch typedVal := val.(type) {
		case map[string]interface{}:
			if len(typedVal) == 0 {
				acc = append(acc, prefix+key)
			} else {
				acc = append(acc, flatObjectKeys(prefix+key+".", typedVal)...)
			}
		default:
			acc = append(acc, prefix+key)
		}
	}

	return acc
}

func getChangedFieldsFromOplogV2UpdateDeep(diffMap map[string]interface{}, prefix string) ([]string, error) {
	fields := []string{}

	for operationKey, operation := range diffMap {
		if operationKey == "i" || operationKey == "u" || operationKey == "d" {
			// indicates an insert, update, or delete of a whole subtree
			operationMap, operationMapOK := operation.(map[string]interface{})
			if !operationMapOK {
				metricUnprocessableChangedFields.Inc()
				log.Log.Errorw("Oplog data for non-replacement v2 update contained a i/u/d key with a non-map value",
					"op", diffMap)
				continue
			}

			fields = append(fields, flatObjectKeys(prefix, operationMap)...)
		} else if isArrayOperator(operation) {
			operationMap, operationMapOK := operation.(map[string]interface{})
			if !operationMapOK {
				metricUnprocessableChangedFields.Inc()
				log.Log.Errorw("Oplog data for non-replacement v2 update contained an array operator key with a non-map value",
					"op", diffMap)
				continue
			}

			for arrayOperatorKey := range operationMap {
				if arrayOperatorKey == "a" {
					continue
				}

				fields = append(fields, prefix+operationKey[1:]+"."+arrayOperatorKey[1:])
			}
		} else if strings.HasPrefix(operationKey, "s") {
			// indicates an insert, update, or delete of a whole subtree
			operationMap, operationMapOK := operation.(map[string]interface{})
			if !operationMapOK {
				metricUnprocessableChangedFields.Inc()
				log.Log.Errorw("Oplog data for non-replacement v2 update contained a s-field key with a non-map value",
					"op", diffMap)
				continue
			}

			// indicates a sub-field set
			subFields, err := getChangedFieldsFromOplogV2UpdateDeep(operationMap, prefix+operationKey[1:]+".")
			if err != nil {
				return []string{}, err
			}
			fields = append(fields, subFields...)
		} else if operationKey == "a" {
			// ignore
			continue
		} else {
			metricUnprocessableChangedFields.Inc()
			log.Log.Errorw("Oplog data for non-replacement v2 update contained a field that was not an i/u/d or an s-prefixed field",
				"op", diffMap)
			continue
		}

	}

	return fields, nil
}

func getChangedFieldsFromOplogV2UpdateShallow(diffRaw bson.Raw) ([]string, error) {
	fields := []string{}

	elements, err := diffRaw.Elements()
	if err != nil {
		metricUnprocessableChangedFields.Inc()
		log.Log.Errorw("Oplog data for non-replacement v1 update failed to unmarshal",
			"op", diffRaw, "error", err)
		return []string{}, err
	}
	for _, element := range elements {
		operationKey := element.Key()
		if operationKey == "i" || operationKey == "u" || operationKey == "d" {
			// indicates an insert, update, or delete of a whole subtree
			operationMap, operationMapOK := element.Value().DocumentOK()
			if !operationMapOK {
				metricUnprocessableChangedFields.Inc()
				log.Log.Errorw("Oplog data for non-replacement v2 update contained a i/u/d key with a non-map value",
					"op", diffRaw)
				continue
			}
			mapFields, err := mapKeysRaw(operationMap)
			if err != nil {
				return []string{}, err
			}
			fields = append(fields, mapFields...)
		} else if strings.HasPrefix(operationKey, "s") {
			// indicates a sub-field set
			fields = append(fields, strings.TrimPrefix(operationKey, "s"))
		} else if operationKey == "a" || strings.HasPrefix(operationKey, "o") {
			// ignore
			continue
		} else {
			metricUnprocessableChangedFields.Inc()
			log.Log.Errorw("Oplog data for non-replacement v2 update contained a field that was not an i/u/d or an s-prefixed field",
				"op", diffRaw)
			continue
		}

	}

	return fields, nil
}

func getChangedFieldsFromOplogV2Update(op *oplogEntry) ([]string, error) {
	// New-style update. Looks like:
	// { $v: 2, diff: { sa: "10", sb: "20", d: { c: true  } }
	diffRawElement := op.Data.Lookup("diff")
	if diffRawElement.IsZero() {
		metricUnprocessableChangedFields.Inc()
		log.Log.Errorw("Oplog data for non-replacement v2 update did not have a diff field",
			"op", op)
		return []string{}, errors.New("Oplog data for non-replacement v2 update did not have a diff field")
	}

	diffRaw, ok := diffRawElement.DocumentOK()

	if !ok {
		metricUnprocessableChangedFields.Inc()
		log.Log.Errorw("Oplog data for non-replacement v2 update had a diff that was not a map",
			"op", op)
		return []string{}, errors.New("Oplog data for non-replacement v2 update had a diff that was not a map")
	}

	if config.OplogV2ExtractSubfieldChanges() {
		var diffMap map[string]interface{}
		err := bson.Unmarshal(diffRaw, &diffMap)
		if err != nil {
			metricUnprocessableChangedFields.Inc()
			log.Log.Errorw("Oplog data for non-replacement v2 update had a diff that was not a map",
				"op", op)
			return []string{}, err
		}
		return getChangedFieldsFromOplogV2UpdateDeep(diffMap, "")
	} else {
		return getChangedFieldsFromOplogV2UpdateShallow(diffRaw)
	}
}
