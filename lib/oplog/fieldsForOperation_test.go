package oplog

import (
	"reflect"
	"sort"
	"testing"
)

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
