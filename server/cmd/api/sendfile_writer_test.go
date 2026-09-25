package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

// countingReaderFrom stands in for the real response, which reaches
// sendfile(2) through net.TCPConn.ReadFrom.
type countingReaderFrom struct {
	http.ResponseWriter
	calls int
	body  bytes.Buffer
}

func (w *countingReaderFrom) ReadFrom(r io.Reader) (int64, error) {
	w.calls++
	return io.Copy(&w.body, r)
}

// plainWrapper mirrors scs's session writer: it proxies the response and
// exposes Unwrap, but not io.ReaderFrom.
type plainWrapper struct {
	http.ResponseWriter
}

func (w *plainWrapper) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func TestUnwrapReaderFromStopsAtOutermostCapableWriter(t *testing.T) {
	inner := &countingReaderFrom{ResponseWriter: httptest.NewRecorder()}
	// Two layers of ReaderFrom: unwrapping must stop at the first one found so
	// intermediate accounting, such as the request logger's byte counter, is
	// not skipped.
	middle := &countingReaderFrom{ResponseWriter: inner}
	outer := &plainWrapper{ResponseWriter: middle}

	readerFrom, found := unwrapReaderFrom(outer)
	if !found {
		t.Fatal("unwrapReaderFrom found no io.ReaderFrom")
	}

	_, err := readerFrom.ReadFrom(bytes.NewReader([]byte("body")))
	if err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}

	if middle.calls != 1 {
		t.Errorf("outermost ReaderFrom calls = %d, want 1", middle.calls)
	}
	if inner.calls != 0 {
		t.Errorf("inner ReaderFrom calls = %d, want 0", inner.calls)
	}
}

func TestRestoreSendfileLeavesCapableWriterAlone(t *testing.T) {
	capable := &countingReaderFrom{ResponseWriter: httptest.NewRecorder()}

	var seen http.ResponseWriter
	handler := restoreSendfile(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = w
	}))
	handler.ServeHTTP(capable, httptest.NewRequest(http.MethodGet, "/", nil))

	if seen != http.ResponseWriter(capable) {
		t.Error("a writer that already implements io.ReaderFrom was wrapped anyway")
	}
}

func TestRestoreSendfilePassesThroughWhenNoReaderFromExists(t *testing.T) {
	recorder := httptest.NewRecorder()

	var seen http.ResponseWriter
	handler := restoreSendfile(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = w
	}))
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	if seen != http.ResponseWriter(recorder) {
		t.Error("writer was wrapped even though no io.ReaderFrom is reachable")
	}
}

func TestSendfileResponseWriterCommitsHeaderBeforeReadFrom(t *testing.T) {
	recorder := httptest.NewRecorder()
	target := &countingReaderFrom{ResponseWriter: recorder}
	writer := &sendfileResponseWriter{ResponseWriter: recorder, readerFrom: target}

	_, err := writer.ReadFrom(bytes.NewReader([]byte("payload")))
	if err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}

	// The wrapped chain must see the header, because scs commits the session
	// there and ReadFrom bypasses it for the body.
	if recorder.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := target.body.String(); got != "payload" {
		t.Errorf("body = %q, want %q", got, "payload")
	}
}

// TestSendfileSurvivesSessionMiddleware is the regression guard for audit
// finding D6: scs's session writer implements neither io.ReaderFrom nor a
// pass-through for it, so without restoreSendfile every media byte is copied
// through a userspace buffer.
func TestSendfileSurvivesSessionMiddleware(t *testing.T) {
	app := setupSessionTestApp(t)

	capable := make(chan bool, 1)

	router := chi.NewRouter()
	router.Use(app.LoadAndSaveSession)
	router.Use(restoreSendfile)
	router.Get("/probe", func(w http.ResponseWriter, r *http.Request) {
		_, ok := w.(io.ReaderFrom)
		capable <- ok
	})

	server := httptest.NewServer(router)
	defer server.Close()

	res, err := http.Get(server.URL + "/probe")
	if err != nil {
		t.Fatalf("probe request: %v", err)
	}
	defer res.Body.Close()

	if !<-capable {
		t.Error("handler received a writer without io.ReaderFrom; sendfile is defeated")
	}
}

func TestSessionCommitControlsSendfile(t *testing.T) {
	for _, failCommit := range []bool{false, true} {
		t.Run(fmt.Sprintf("commit failure=%t", failCommit), func(t *testing.T) {
			app := setupSessionTestApp(t)
			if failCommit {
				_, err := app.DB.Exec(`CREATE TRIGGER fail_session_insert BEFORE INSERT ON sessions BEGIN SELECT RAISE(FAIL, 'private storage failure'); END`)
				if err != nil {
					t.Fatal(err)
				}
			}
			recorder := httptest.NewRecorder()
			target := &countingReaderFrom{ResponseWriter: recorder}
			var copyErr error
			handler := app.LoadAndSaveSession(restoreSendfile(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				app.SessionManager.Put(r.Context(), cookieUserID, int64(1))
				w.Header().Set("Content-Length", "7")
				_, copyErr = w.(io.ReaderFrom).ReadFrom(bytes.NewReader([]byte("payload")))
			})))
			handler.ServeHTTP(target, httptest.NewRequest(http.MethodGet, "/probe", nil))
			if failCommit {
				if copyErr == nil || target.calls != 0 || target.body.Len() != 0 {
					t.Fatalf("failed session streamed payload: calls=%d, error=%v", target.calls, copyErr)
				}
				if recorder.Code != http.StatusInternalServerError || recorder.Header().Get("Content-Length") != "" {
					t.Fatalf("invalid session error headers: status=%d headers=%v", recorder.Code, recorder.Header())
				}
			} else if copyErr != nil || target.calls != 1 || target.body.String() != "payload" {
				t.Fatalf("successful session lost sendfile: calls=%d, body=%q, error=%v", target.calls, target.body.String(), copyErr)
			}
		})
	}
}
