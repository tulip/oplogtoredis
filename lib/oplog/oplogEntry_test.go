package oplog

import (
	"reflect"
	"sort"
	"testing"
)

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
			got := (&oplogEntry{Data: test.in}).UpdateIsReplace()

			if got != test.expectedResult {
				t.Errorf("UpdateIsReplace(%#v) = %t; want %t",
					test.in, got, test.expectedResult)
			}
		})
	}
}

func TestChangedFields(t *testing.T) {
	tests := map[string]struct {
		input *oplogEntry
		want  []string
	}{
		"Insert": {
			input: &oplogEntry{
				Operation: "i",
				Data: map[string]interface{}{
					"foo": "a",
					"bar": 10,
				},
			},
			want: []string{"foo", "bar"},
		},

		"Replacement update": {
			input: &oplogEntry{
				Operation: "u",
				Data: map[string]interface{}{
					"foo": "a",
					"bar": 10,
				},
			},
			want: []string{"foo", "bar"},
		},

		"Delete": {
			input: &oplogEntry{
				Operation: "d",
				Data: map[string]interface{}{
					"foo": "a",
					"bar": 10,
				},
			},
			want: []string{},
		},

		"Update": {
			input: &oplogEntry{
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
			input: &oplogEntry{
				Operation: "u",
				Data: map[string]interface{}{
					"$v":   "1.0",
					"$set": map[string]interface{}{},
				},
			},
			want: []string{},
		},

		"Update, unexpected operation value type": {
			input: &oplogEntry{
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
			got := test.input.ChangedFields()

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
