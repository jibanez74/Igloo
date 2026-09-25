package tmdb

import "testing"

// cmd/api calls New whenever the stored key is non-NULL, even when it is
// blank, so the empty key must be rejected here rather than on first use.
func TestNewRejectsEmptyKey(t *testing.T) {
	_, err := New("")
	if err == nil {
		t.Fatal("expected error when API key is empty")
	}
}
