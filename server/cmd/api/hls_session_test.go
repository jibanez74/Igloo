package main

import (
	"testing"
)

func TestHLSSessionKey(t *testing.T) {
	key := HLSSessionKey(123, "1080p_4mbps", 0)
	if key != "123:1080p_4mbps:0" {
		t.Errorf("HLSSessionKey(123, 1080p_4mbps, 0) = %q, want 123:1080p_4mbps:0", key)
	}
	key2 := HLSSessionKey(1, "720p_3mbps", 1)
	if key2 != "1:720p_3mbps:1" {
		t.Errorf("HLSSessionKey(1, 720p_3mbps, 1) = %q, want 1:720p_3mbps:1", key2)
	}
}
