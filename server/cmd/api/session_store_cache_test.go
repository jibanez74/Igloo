package main

import (
	"bytes"
	"testing"
	"time"
)

// countingStore records how often the decorated store is reached so the tests
// can tell a cache hit from a database read.
type countingStore struct {
	data    map[string][]byte
	finds   int
	commits int
	deletes int
}

func newCountingStore() *countingStore {
	return &countingStore{data: make(map[string][]byte)}
}

func (s *countingStore) Find(token string) ([]byte, bool, error) {
	s.finds++
	data, found := s.data[token]
	return data, found, nil
}

func (s *countingStore) Commit(token string, b []byte, expiry time.Time) error {
	s.commits++
	s.data[token] = b
	return nil
}

func (s *countingStore) Delete(token string) error {
	s.deletes++
	delete(s.data, token)
	return nil
}

func TestCachedSessionStoreServesRepeatedFindsFromMemory(t *testing.T) {
	backing := newCountingStore()
	backing.data["token"] = []byte("session")
	store := newCachedSessionStore(backing)

	for i := 0; i < 3; i++ {
		data, found, err := store.Find("token")
		if err != nil {
			t.Fatalf("find %d: %v", i, err)
		}
		if !found {
			t.Fatalf("find %d did not find the session", i)
		}
		if !bytes.Equal(data, []byte("session")) {
			t.Fatalf("find %d returned %q", i, data)
		}
	}

	if backing.finds != 1 {
		t.Errorf("store reads = %d, want 1", backing.finds)
	}
}

func TestCachedSessionStoreDoesNotCacheMissingSessions(t *testing.T) {
	backing := newCountingStore()
	store := newCachedSessionStore(backing)

	for i := 0; i < 2; i++ {
		_, found, err := store.Find("absent")
		if err != nil {
			t.Fatalf("find %d: %v", i, err)
		}
		if found {
			t.Fatalf("find %d reported a session that does not exist", i)
		}
	}

	// A negative result must keep reaching the store, otherwise a session
	// committed elsewhere in the same window would stay invisible.
	if backing.finds != 2 {
		t.Errorf("store reads = %d, want 2", backing.finds)
	}
}

func TestCachedSessionStoreCommitIsVisibleWithoutAStoreRead(t *testing.T) {
	backing := newCountingStore()
	store := newCachedSessionStore(backing)

	err := store.Commit("token", []byte("fresh"), time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("commit: %v", err)
	}

	data, found, err := store.Find("token")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if !found || !bytes.Equal(data, []byte("fresh")) {
		t.Fatalf("find returned (%q, %t), want the committed data", data, found)
	}
	if backing.finds != 0 {
		t.Errorf("store reads = %d, want 0", backing.finds)
	}
}

func TestCachedSessionStoreCommitRespectsExpiry(t *testing.T) {
	backing := newCountingStore()
	store := newCachedSessionStore(backing)

	// scs treats the store as the only authority on expiry, so an already
	// expired session must never be answered from memory.
	err := store.Commit("token", []byte("stale"), time.Now().Add(-time.Minute))
	if err != nil {
		t.Fatalf("commit: %v", err)
	}

	_, _, err = store.Find("token")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if backing.finds != 1 {
		t.Errorf("store reads = %d, want 1", backing.finds)
	}
}

func TestCachedSessionStoreDeleteInvalidatesImmediately(t *testing.T) {
	backing := newCountingStore()
	backing.data["token"] = []byte("session")
	store := newCachedSessionStore(backing)

	_, _, err := store.Find("token")
	if err != nil {
		t.Fatalf("warm find: %v", err)
	}

	err = store.Delete("token")
	if err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, found, err := store.Find("token")
	if err != nil {
		t.Fatalf("find after delete: %v", err)
	}
	if found {
		t.Error("a destroyed session was still served from the cache")
	}
	if backing.deletes != 1 {
		t.Errorf("store deletes = %d, want 1", backing.deletes)
	}
}
