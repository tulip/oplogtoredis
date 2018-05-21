package redispub

import (
	"errors"
	"testing"
	"time"

	"github.com/globalsign/mgo/bson"
)

func TestEncodeMongoTimestamp(t *testing.T) {
	tests := map[string]struct {
		in   bson.MongoTimestamp
		want string
	}{
		"Simple timestamp": {
			in:   bson.MongoTimestamp(1234),
			want: "1234",
		},
		"Zero value": {
			want: "0",
		},
		"Max value": {
			in:   bson.MongoTimestamp(9223372036854775807),
			want: "9223372036854775807",
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			got := encodeMongoTimestamp(test.in)

			if got != test.want {
				t.Errorf("encodeMongoTimestamp(%d) = \"%s\", wanted \"%s\"",
					test.in, got, test.want)
			}
		})
	}
}

func TestDecodeMongoTimestamp(t *testing.T) {
	tests := map[string]struct {
		in      string
		want    bson.MongoTimestamp
		wantErr error
	}{
		"Simple timestamp": {
			in:   "1234",
			want: bson.MongoTimestamp(1234),
		},
		"Zero value": {
			in:   "0",
			want: bson.MongoTimestamp(0),
		},
		"Max value": {
			in:   "9223372036854775807",
			want: bson.MongoTimestamp(9223372036854775807),
		},
		"Error": {
			in:      "nope",
			wantErr: errors.New("strconv.ParseInt: parsing \"nope\": invalid syntax"),
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			got, err := decodeMongoTimestamp(test.in)

			if (err == nil) && (test.wantErr != nil) {
				t.Errorf("decodeMongoTimestamp(%s) did not error, expected: %s",
					test.in, test.wantErr)
			}

			if (err != nil) && (test.wantErr == nil) {
				t.Errorf("decodeMongoTimestamp(%s) gave unexpected error: %s",
					test.in, err)
			}

			if (err != nil) && (test.wantErr != nil) && (err.Error() != test.wantErr.Error()) {
				t.Errorf("decodeMongoTimestamp(%s) gave error \"%s\", wanted \"%s\"",
					test.in, err, test.wantErr)
			}

			if got != test.want {
				t.Errorf("decodeMongoTimestamp(%s) = %d, wanted %d",
					test.in, got, test.want)
			}
		})
	}
}

func TestMongoTimestampToTime(t *testing.T) {
	tests := map[string]struct {
		in   bson.MongoTimestamp
		want time.Time
	}{
		"Simple timestamp": {
			in:   bson.MongoTimestamp(6556905427232097490),
			want: time.Unix(1526648511, 0),
		},
		"Zero value": {
			want: time.Unix(0, 0),
		},
		"Max value": {
			in:   bson.MongoTimestamp(9223372036854775807),
			want: time.Unix(2147483647, 0),
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			got := mongoTimestampToTime(test.in)

			if got != test.want {
				t.Errorf("mongoTimestampToTime(%d) = %d, wanted %d",
					test.in, got.Unix(), test.want.Unix())
			}
		})
	}
}
