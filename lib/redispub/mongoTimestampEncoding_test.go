package redispub

import (
	"errors"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestEncodeMongoTimestamp(t *testing.T) {
	tests := map[string]struct {
		in   primitive.Timestamp
		want string
	}{
		"Simple timestamp (I only)": {
			in:   primitive.Timestamp{I: 1234},
			want: "1234",
		},
		"Simple timestamp (T only)": {
			in:   primitive.Timestamp{T: 1234},
			want: "5299989643264",
		},
		"Simple timestamp (I and T)": {
			in:   primitive.Timestamp{T: 1234, I: 5678},
			want: "5299989648942",
		},
		"Zero value": {
			want: "0",
		},
		"Max value": {
			in:   primitive.Timestamp{T: 4294967295, I: 4294967295},
			want: "18446744073709551615",
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			got := encodeMongoTimestamp(test.in)

			if got != test.want {
				t.Errorf("encodeMongoTimestamp(%d, %d) = \"%s\", wanted \"%s\"",
					test.in.T, test.in.I, got, test.want)
			}
		})
	}
}

func TestDecodeMongoTimestamp(t *testing.T) {
	tests := map[string]struct {
		in      string
		want    primitive.Timestamp
		wantErr error
	}{
		"Simple timestamp (I only)": {
			want: primitive.Timestamp{I: 1234},
			in:   "1234",
		},
		"Simple timestamp (T only)": {
			want: primitive.Timestamp{T: 1234},
			in:   "5299989643264",
		},
		"Simple timestamp (I and T)": {
			want: primitive.Timestamp{T: 1234, I: 5678},
			in:   "5299989648942",
		},
		"Zero value": {
			in: "0",
		},
		"Max value": {
			want: primitive.Timestamp{T: 4294967295, I: 4294967295},
			in:   "18446744073709551615",
		},
		"Error": {
			in:      "nope",
			wantErr: errors.New("strconv.ParseUint: parsing \"nope\": invalid syntax"),
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

			if !got.Equal(test.want) {
				t.Errorf("decodeMongoTimestamp(%s) = %d, %d, wanted %d, %d",
					test.in, got.T, got.I, test.want.T, test.want.I)
			}
		})
	}
}

func TestMongoTimestampToTime(t *testing.T) {
	tests := map[string]struct {
		in   primitive.Timestamp
		want time.Time
	}{
		"Simple timestamp": {
			in:   primitive.Timestamp{T: 1526648511},
			want: time.Unix(1526648511, 0),
		},
		"Zero value": {
			want: time.Unix(0, 0),
		},
		"Max value": {
			in:   primitive.Timestamp{T: 2147483647},
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
