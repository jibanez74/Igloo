package main

import (
	"bytes"
	"strconv"
	"testing"
	"time"

	"github.com/alexedwards/scs/v2"
)

// countingStore records how often the decorated store is reached so the tests
// can tell a cache hit from a database read.
type countingStore struct {
	data    map[string][]byte
	finds   int
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

func encodedTestSession(t testing.TB, userID int64) ([]byte, time.Time) {
	t.Helper()

	deadline := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
	b, err := scs.GobCodec{}.Encode(deadline, map[string]any{cookieUserID: userID, cookieSessionVersion: int64(3)})
	if err != nil {
		t.Fatalf("encode session: %v", err)
	}
	return b, deadline
}

func TestMemoSessionCodecDecodesLikeGob(t *testing.T) {
	codec := newMemoSessionCodec()
	b, deadline := encodedTestSession(t, 7)

	for i := 0; i < 3; i++ {
		gotDeadline, values, err := codec.Decode(b)
		if err != nil {
			t.Fatalf("decode %d: %v", i, err)
		}
		if !gotDeadline.Equal(deadline) {
			t.Errorf("decode %d deadline = %v, want %v", i, gotDeadline, deadline)
		}
		userID, _ := values[cookieUserID].(int64)
		version, _ := values[cookieSessionVersion].(int64)
		if userID != 7 || version != 3 || len(values) != 2 {
			t.Errorf("decode %d values = %v", i, values)
		}

		// scs writes into the map it is handed, on logout and on a password
		// change, so one request's map must never reach the next.
		values[cookieUserID] = int64(99)
		delete(values, cookieSessionVersion)
	}
}

func TestMemoSessionCodecRejectsCorruptData(t *testing.T) {
	codec := newMemoSessionCodec()

	for i := 0; i < 2; i++ {
		_, _, err := codec.Decode([]byte("not gob"))
		if err == nil {
			t.Fatalf("decode %d accepted corrupt session data", i)
		}
	}
}

func TestMemoSessionCodecStaysBounded(t *testing.T) {
	codec := newMemoSessionCodec()

	for i := 0; i < sessionCodecMemoMax+10; i++ {
		b, _ := encodedTestSession(t, int64(i))
		_, _, err := codec.Decode(b)
		if err != nil {
			t.Fatalf("decode %d: %v", i, err)
		}
	}

	if len(codec.decoded) > sessionCodecMemoMax {
		t.Errorf("memo holds %d sessions, want at most %d", len(codec.decoded), sessionCodecMemoMax)
	}

	b, _ := encodedTestSession(t, 5)
	_, values, err := codec.Decode(b)
	if err != nil {
		t.Fatalf("decode after the memo was cleared: %v", err)
	}
	userID, _ := values[cookieUserID].(int64)
	if userID != 5 {
		t.Errorf("user id after the memo was cleared = %d, want 5", userID)
	}
}

func BenchmarkSessionDecode(b *testing.B) {
	data, _ := encodedTestSession(b, 7)
	codecs := map[string]scs.Codec{"gob": scs.GobCodec{}, "memo": newMemoSessionCodec()}

	for _, name := range []string{"gob", "memo"} {
		codec := codecs[name]
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, values, err := codec.Decode(data)
				if err != nil || len(values) != 2 {
					b.Fatal(strconv.Itoa(len(values)), err)
				}
			}
		})
	}
}
