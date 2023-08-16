package oplog

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Copied from https://github.com/meteor/meteor/blob/devel/packages/mongo/oplog_v2_converter_tests.js

func TestOplogV2DeepConverter(t *testing.T) {
	tests := map[string]struct {
		in   map[string]interface{}
		want []string
	}{
		"basic": {
			in:   map[string]interface{}{"scustom": map[string]interface{}{"sEJSON$value": map[string]interface{}{"u": map[string]interface{}{"EJSONtail": "d"}}}},
			want: []string{"custom.EJSON$value.EJSONtail"},
		},
		"basic with u": {
			in:   map[string]interface{}{"u": map[string]interface{}{"d": "2", "oi": "asdas"}},
			want: []string{"d", "oi"},
		},
		"set inside an array": {
			in:   map[string]interface{}{"sasd": map[string]interface{}{"a": true, "u0": 2}},
			want: []string{"asd.0"},
		},
		"unset inside an array": {
			in:   map[string]interface{}{"sasd": map[string]interface{}{"a": true, "u0": nil}},
			want: []string{"asd.0"},
		},
		"set a new nested field inside an object": {
			in:   map[string]interface{}{"i": map[string]interface{}{"a": map[string]interface{}{"b": 2}}},
			want: []string{"a.b"},
		},
		"set a new nested field inside an object, variant": {
			in: map[string]interface{}{
				"u": map[string]interface{}{
					"count": 1,
				},
				"i": map[string]interface{}{
					"nested": map[string]interface{}{
						"state": map[string]interface{}{},
					},
				},
			},
			want: []string{"nested.state", "count"},
		},
		"set an existing nested field inside an object": {
			in: map[string]interface{}{
				"sa": map[string]interface{}{
					"i": map[string]interface{}{
						"b": 3,
						"c": 1,
					},
				},
			},
			want: []string{"a.b", "a.c"},
		},
		"unset an existing nested field inside an object": {
			in: map[string]interface{}{
				"sa": map[string]interface{}{
					"d": map[string]interface{}{
						"b": false,
					},
				},
			},
			want: []string{"a.b"},
		},
		"combine u and s": {
			in: map[string]interface{}{
				"u": map[string]interface{}{
					"c": "bar",
				},
				"sb": map[string]interface{}{
					"a":  true,
					"u0": 2,
				},
			},
			want: []string{"b.0", "c"},
		},
		"deeply nested s entries": {
			in: map[string]interface{}{
				"sservices": map[string]interface{}{
					"sresume": map[string]interface{}{
						"u": map[string]interface{}{
							"loginTokens": []interface{}{},
						},
					},
				},
			},
			want: []string{"services.resume.loginTokens"},
		},
		"set a new array": {
			in: map[string]interface{}{
				"i": map[string]interface{}{
					"tShirt": map[string]interface{}{
						"sizes": []interface{}{
							"small",
							"medium",
							"large",
						},
					},
				},
			},
			want: []string{"tShirt.sizes"},
		},
		"update specific list elements": {
			in: map[string]interface{}{
				"slist": map[string]interface{}{
					"a":  true,
					"u3": "i",
					"u4": "h",
				},
			},
			want: []string{"list.3", "list.4"},
		},
		"set whole array": {
			in: map[string]interface{}{
				"sobject": map[string]interface{}{
					"u": map[string]interface{}{
						"array": []interface{}{
							"2",
							"2",
							"4",
							"3",
						},
					},
				},
			},
			want: []string{"object.array"},
		},
		"more deep nesting": {
			in: map[string]interface{}{
				"slayout": map[string]interface{}{
					"sjourneyStepIds": map[string]interface{}{
						"sj4aqp3tiK6xCPCYu8": map[string]interface{}{
							"a":  true,
							"u2": "zTkxivNrKuBi2iJ2m",
						},
					},
				},
			},
			want: []string{"layout.journeyStepIds.j4aqp3tiK6xCPCYu8.2"},
		},
		"misleading array operator-like keys": {
			in: map[string]interface{}{
				"sarray": map[string]interface{}{
					"a": true,
					"s2": map[string]interface{}{
						"u": map[string]interface{}{
							"a": "something",
						},
					},
				},
			},
			want: []string{"array.2.a"},
		},
	}

	for testName, test := range tests {
		sort.Strings(test.want)

		t.Run(testName, func(t *testing.T) {
			got := getChangedFieldsFromOplogV2UpdateDeep(test.in, "")
			sort.Strings(got)
			assert.Equal(t, test.want, got)
		})
	}
}
