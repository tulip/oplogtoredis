package oplog

import (
	"reflect"
	"sort"
	"testing"

	"github.com/rwynn/gtm"
)

func TestFieldForOperation(t *testing.T) {
	tests := map[string]struct {
		input *gtm.Op
		want  []string
	}{
		"Insert": {
			input: &gtm.Op{
				Operation: "i",
				Data: map[string]interface{}{
					"foo": "a",
					"bar": 10,
				},
			},
			want: []string{"foo", "bar"},
		},

		"Replacement update": {
			input: &gtm.Op{
				Operation: "u",
				Data: map[string]interface{}{
					"foo": "a",
					"bar": 10,
				},
			},
			want: []string{"foo", "bar"},
		},

		"Delete": {
			input: &gtm.Op{
				Operation: "d",
				Data: map[string]interface{}{
					"foo": "a",
					"bar": 10,
				},
			},
			want: []string{},
		},

		"Update": {
			input: &gtm.Op{
				Operation: "u",
				Data: map[string]interface{}{
					"$v": "1.0",
					"$set": map[string]interface{}{
						"foo":     "a",
						"bar":     map[string]interface{}{"xxx": "yyy"},
						"baz.qux": 10,
					},
					"$unset": map[string]interface{}{
						"qax": true,
					},
				},
			},
			want: []string{"foo", "bar", "baz.qux", "qax"},
		},

		"Update, no operations": {
			input: &gtm.Op{
				Operation: "u",
				Data: map[string]interface{}{
					"$v":   "1.0",
					"$set": map[string]interface{}{},
				},
			},
			want: []string{},
		},

		"Update, unexpected operation value type": {
			input: &gtm.Op{
				Operation: "u",
				Data: map[string]interface{}{
					"$v":    "1.0",
					"weird": "thing",
					"$set": map[string]interface{}{
						"foo": "a",
					},
				},
			},
			want: []string{"foo"},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			got := fieldsForOperation(test.input)

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
