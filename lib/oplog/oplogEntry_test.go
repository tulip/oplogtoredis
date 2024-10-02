package oplog

import (
	"os"
	"reflect"
	"sort"
	"strconv"
	"testing"

	"github.com/tulip/oplogtoredis/lib/config"

	"go.mongodb.org/mongo-driver/bson"
)

func rawBson(t *testing.T, data interface{}) bson.Raw {
	raw, err := bson.Marshal(data)
	if err != nil {
		t.Error("Failed to marshal test data", err)
	}
	return raw
}

func arraysMatch(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for _, valA := range a {
		found := false
		for _, valB := range b {
			if valA == valB {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func TestMapKeysRaw(t *testing.T) {
	want := []string{"key1", "key2", "key3"}
	got, err := mapKeysRaw(rawBson(t, map[string]interface{}{"key1": "one", "key2": "two", "key3": "three"}))
	if err != nil {
		t.Error("mapKeysRaw() error", err)
	}
	if !arraysMatch(got, want) {
		t.Errorf("mapKeysRaw() = %v, want %v", got, want)
	}
}

func TestCategorization(t *testing.T) {
	tests := map[string]struct {
		in       *oplogEntry
		isInsert bool
		isUpdate bool
		isRemove bool
	}{
		"insert": {
			in: &oplogEntry{
				Operation: "i",
			},
			isInsert: true,
			isUpdate: false,
			isRemove: false,
		},
		"update": {
			in: &oplogEntry{
				Operation: "u",
			},
			isInsert: false,
			isUpdate: true,
			isRemove: false,
		},
		"remove": {
			in: &oplogEntry{
				Operation: "d",
			},
			isInsert: false,
			isUpdate: false,
			isRemove: true,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			gotInsert := test.in.IsInsert()
			if gotInsert != test.isInsert {
				t.Errorf("IsInsert(%#v) = %t; want %t",
					test.in, gotInsert, test.isInsert)
			}

			gotUpdate := test.in.IsUpdate()
			if gotUpdate != test.isUpdate {
				t.Errorf("IsUpdate(%#v) = %t; want %t",
					test.in, gotUpdate, test.isUpdate)
			}

			gotRemove := test.in.IsRemove()
			if gotRemove != test.isRemove {
				t.Errorf("IsRemove(%#v) = %t; want %t",
					test.in, gotRemove, test.isRemove)
			}
		})
	}
}

func TestUpdateIsReplace(t *testing.T) {
	tests := map[string]struct {
		in             map[string]interface{}
		expectedResult bool
	}{
		"set": {
			in: map[string]interface{}{
				"$set": map[string]interface{}{"foo": "bar"},
			},
			expectedResult: false,
		},
		"unset": {
			in: map[string]interface{}{
				"$unset": map[string]interface{}{"foo": "bar"},
			},
			expectedResult: false,
		},
		"set and unset": {
			in: map[string]interface{}{
				"$set":   map[string]interface{}{"foo": "bar"},
				"$unset": map[string]interface{}{"foo": "bar"},
			},
			expectedResult: false,
		},
		"replacement": {
			in: map[string]interface{}{
				"$v":  map[string]interface{}{"foo": "bar"},
				"foo": "bar",
			},
			expectedResult: true,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			got := (&oplogEntry{Data: rawBson(t, test.in)}).UpdateIsReplace()

			if got != test.expectedResult {
				t.Errorf("UpdateIsReplace(%#v) = %t; want %t",
					test.in, got, test.expectedResult)
			}
		})
	}
}

func TestChangedFields(t *testing.T) {
	tests := map[string]struct {
		input                           *oplogEntry
		want                            []string
		enableV2ExtractDeepFieldChanges bool
	}{
		"Insert": {
			input: &oplogEntry{
				Operation: "i",
				Data: rawBson(t, map[string]interface{}{
					"foo": "a",
					"bar": 10,
				}),
			},
			want: []string{"foo", "bar"},
		},

		"Replacement update": {
			input: &oplogEntry{
				Operation: "u",
				Data: rawBson(t, map[string]interface{}{
					"foo": "a",
					"bar": 10,
				}),
			},
			want: []string{"foo", "bar"},
		},

		"Delete": {
			input: &oplogEntry{
				Operation: "d",
				Data: rawBson(t, map[string]interface{}{
					"foo": "a",
					"bar": 10,
				}),
			},
			want: []string{},
		},

		"Update": {
			input: &oplogEntry{
				Operation: "u",
				Data: rawBson(t, map[string]interface{}{
					"$v": "1.0",
					"$set": map[string]interface{}{
						"foo":     "a",
						"bar":     map[string]interface{}{"xxx": "yyy"},
						"baz.qux": 10,
					},
					"$unset": map[string]interface{}{
						"qax": true,
					},
				}),
			},
			want: []string{"foo", "bar", "baz.qux", "qax"},
		},

		"Update, no operations": {
			input: &oplogEntry{
				Operation: "u",
				Data: rawBson(t, map[string]interface{}{
					"$v":   "1.0",
					"$set": map[string]interface{}{},
				}),
			},
			want: []string{},
		},

		"Update, unexpected operation value type": {
			input: &oplogEntry{
				Operation: "u",
				Data: rawBson(t, map[string]interface{}{
					"$v":    "1.0",
					"weird": "thing",
					"$set": map[string]interface{}{
						"foo": "a",
					},
				}),
			},
			want: []string{"foo"},
		},

		"Update v2": {
			input: &oplogEntry{
				Operation: "u",
				Data: rawBson(t, map[string]interface{}{
					"$v": 2,
					"diff": map[string]interface{}{
						"i":       map[string]interface{}{"a": 1, "b": "2"},
						"u":       map[string]interface{}{"c": 1, "d": "2"},
						"d":       map[string]interface{}{"e": 1, "f": "2"},
						"sg":      10,
						"sfoobar": map[string]interface{}{},
					},
				}),
			},
			want: []string{"a", "b", "c", "d", "e", "f", "g", "foobar"},
		},

		"Update v2 deep": {
			input: &oplogEntry{
				Operation: "u",
				Data: rawBson(t, map[string]interface{}{
					"$v": 2,
					"diff": map[string]interface{}{
						"i":       map[string]interface{}{"a": 1, "b": "2"},
						"u":       map[string]interface{}{"c": 1, "d": "2"},
						"d":       map[string]interface{}{"e": 1, "f": "2"},
						"sg":      map[string]interface{}{},
						"sfoobar": map[string]interface{}{},
					},
				}),
			},
			want:                            []string{"a", "b", "c", "d", "e", "f"},
			enableV2ExtractDeepFieldChanges: true,
		},

		"Update v2, no operations": {
			input: &oplogEntry{
				Operation: "u",
				Data: rawBson(t, map[string]interface{}{
					"$v":   2,
					"diff": map[string]interface{}{},
				}),
			},
			want: []string{},
		},

		"Update v2, no operations deep": {
			input: &oplogEntry{
				Operation: "u",
				Data: rawBson(t, map[string]interface{}{
					"$v":   2,
					"diff": map[string]interface{}{},
				}),
			},
			want:                            []string{},
			enableV2ExtractDeepFieldChanges: true,
		},

		"Update v2, unexpected operation value type": {
			input: &oplogEntry{
				Operation: "u",
				Data: rawBson(t, map[string]interface{}{
					"$v":    2,
					"weird": "thing",
					"diff": map[string]interface{}{
						"i":          10,
						"otherwierd": "thing",
						"sfoo":       "bar",
					},
				}),
			},
			want: []string{"foo"},
		},

		"Update v2, unexpected operation value type deep": {
			input: &oplogEntry{
				Operation: "u",
				Data: rawBson(t, map[string]interface{}{
					"$v":    2,
					"weird": "thing",
					"diff": map[string]interface{}{
						"i":          10,
						"otherwierd": "thing",
						"sfoo":       map[string]interface{}{"u": map[string]interface{}{"x": "10"}},
					},
				}),
			},
			want:                            []string{"foo.x"},
			enableV2ExtractDeepFieldChanges: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			os.Setenv("OTR_OPLOG_V2_EXTRACT_SUBFIELD_CHANGES", strconv.FormatBool(test.enableV2ExtractDeepFieldChanges))
			os.Setenv("OTR_REDIS_URL", "redis://yyy")
			os.Setenv("OTR_MONGO_URL", "mongodb://xxx")

			if config.ParseEnv() != nil {
				t.Errorf("Failed to parse env with subfield setting %t", test.enableV2ExtractDeepFieldChanges)
			}

			got, err := test.input.ChangedFields()
			if err != nil {
				t.Error("ChangedFields() error", err)
			}

			sort.Strings(got)
			sort.Strings(test.want)

			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("fieldsForOperation(%#v) = %v, want %v", test.input, got, test.want)
			}
		})
	}
}

func TestMapKeys(t *testing.T) {
	tests := map[string]struct {
		input map[string]interface{}
		want  []string
	}{
		"Several keys": {
			input: map[string]interface{}{
				"key1": "foo",
				"key2": "foo",
			},
			want: []string{"key1", "key2"},
		},
		"One keys": {
			input: map[string]interface{}{
				"key1": "foo",
			},
			want: []string{"key1"},
		},
		"No keys": {
			input: map[string]interface{}{},
			want:  []string{},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			got := mapKeys(test.input)

			sort.Strings(got)
			sort.Strings(test.want)

			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("mapKeys(%#v) = %v, want %v", test.input, got, test.want)
			}
		})
	}
}

func TestUpdateIsV2Formatted(t *testing.T) {
	tests := map[string]struct {
		in             map[string]interface{}
		expectedResult bool
	}{
		"no $v": {
			in: map[string]interface{}{
				"diff": map[string]interface{}{"sa": "123"},
			},
			expectedResult: false,
		},
		"$v 1": {
			in: map[string]interface{}{
				"$v":   1,
				"diff": map[string]interface{}{"sa": "123"},
			},
			expectedResult: false,
		},
		"$v is string": {
			in: map[string]interface{}{
				"$v":   "2",
				"diff": map[string]interface{}{"sa": "123"},
			},
			expectedResult: false,
		},
		"no diff": {
			in: map[string]interface{}{
				"$v":   2,
				"$set": map[string]interface{}{"a": "123"},
			},
			expectedResult: false,
		},
		"valid v2 format": {
			in: map[string]interface{}{
				"$v":   2,
				"diff": map[string]interface{}{"sa": "123"},
			},
			expectedResult: true,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			got := (&oplogEntry{Data: rawBson(t, test.in)}).IsV2Update()

			if got != test.expectedResult {
				t.Errorf("UpdateIsV2Formatted(%#v) = %t; want %t",
					test.in, got, test.expectedResult)
			}
		})
	}
}
