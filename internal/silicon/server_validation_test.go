package silicon_test

import (
	"bytes"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	"github.com/throw-if-null/molecular/internal/silicon"
	"github.com/throw-if-null/molecular/internal/store"
	_ "modernc.org/sqlite"
)

func setup(t *testing.T) (*store.Store, func(), *httptest.Server) {
	td, err := os.MkdirTemp("", "molecular-test-")
	if err != nil {
		t.Fatalf("tmpdir: %v", err)
	}
	dbpath := td + "/molecular.db"
	db, err := sql.Open("sqlite", dbpath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	_, _ = db.Exec(`PRAGMA busy_timeout = 5000`)
	s := store.New(db)
	if err := s.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	srv := silicon.NewServer(s, 3, 3, 2)
	ts := httptest.NewServer(srv.Handler())
	return s, func() { ts.Close(); db.Close(); os.RemoveAll(td) }, ts
}

func TestCreateTaskRejectsInvalidTaskID(t *testing.T) {
	s, cleanup, ts := setup(t)
	defer cleanup()
	_ = s

	bad := []string{"../x", "..\\x", "a/b", "a\\b", "/abs", "C:\\x"}
	for _, id := range bad {
		body := []byte(`{"task_id":"` + id + `","prompt":"p"}`)
		res, err := http.Post(ts.URL+"/v1/tasks", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("post err: %v", err)
		}
		if res.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400 for %q, got %d", id, res.StatusCode)
		}
	}
}

func TestLogsRejectsInvalidTaskID(t *testing.T) {
	_, cleanup, ts := setup(t)
	defer cleanup()

	bad := []string{"../x", "..\\x", "a/b", "a\\b", "/abs", "C:\\x"}
	for _, id := range bad {
		esc := url.PathEscape(id)
		res, err := http.Get(ts.URL + "/v1/tasks/" + esc + "/logs")
		if err != nil {
			t.Fatalf("get err: %v", err)
		}
		if res.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400 for %q, got %d", id, res.StatusCode)
		}
	}
}
