package parse

import "testing"

func TestParseSentinelURLSetsDialTimeout(t *testing.T) {
	opts, err := ParseRedisURL("redis-sentinel://redis-sentinel:26379/0?sentinelMasterId=mymaster", true)
	if err != nil {
		t.Fatal(err)
	}
	if opts.DialTimeout != sentinelDialTimeout {
		t.Errorf("DialTimeout = %v, want %v", opts.DialTimeout, sentinelDialTimeout)
	}
}
