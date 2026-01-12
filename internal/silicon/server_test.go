package silicon_test

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"fmt"
	"github.com/throw-if-null/molecular/internal/api"
	"github.com/throw-if-null/molecular/internal/silicon"
	"github.com/throw-if-null/molecular/internal/store"
	_ "modernc.org/sqlite"
)

func setupTestStore(t *testing.T) (*store.Store, func()) {
	t.Helper()
	td, err := os.MkdirTemp("", "molecular-test-")
	if err != nil {
		t.Fatalf("tmpdir: %v", err)
	}
	dbpath := filepath.Join(td, "molecular.db")
	db, err := sql.Open("sqlite", dbpath)
	if err != nil {
		os.RemoveAll(td)
		t.Fatalf("open db: %v", err)
	}
	s := store.New(db)
	if err := s.Init(); err != nil {
		db.Close()
		os.RemoveAll(td)
		t.Fatalf("init: %v", err)
	}
	return s, func() {
		db.Close()
		os.RemoveAll(td)
	}
}

func TestListCancelLogsEndpoints(t *testing.T) {
	s, cleanup := setupTestStore(t)
	defer cleanup()

	// create tasks
	for i := 1; i <= 3; i++ {
		id := fmt.Sprintf("task-%d", i)
		_, _, err := s.CreateTaskOrGetExisting(&api.CreateTaskRequest{TaskID: id, Prompt: "p"})
		if err != nil {
			t.Fatalf("create task: %v", err)
		}
	}

	srv := silicon.NewServer(s)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// list all
	res, err := http.Get(ts.URL + "/v1/tasks")
	if err != nil {
		t.Fatalf("get list: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: %v", res.Status)
	}
	var tasks []map[string]interface{}
	body, _ := io.ReadAll(res.Body)
	if err := json.Unmarshal(body, &tasks); err != nil {
		t.Fatalf("unmarshal: %v; body=%s", err, string(body))
	}
	if len(tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(tasks))
	}

	// limit
	res2, err := http.Get(ts.URL + "/v1/tasks?limit=2")
	if err != nil {
		t.Fatalf("get list limit: %v", err)
	}
	defer res2.Body.Close()
	var tasks2 []map[string]interface{}
	body2, _ := io.ReadAll(res2.Body)
	if err := json.Unmarshal(body2, &tasks2); err != nil {
		t.Fatalf("unmarshal2: %v; body=%s", err, string(body2))
	}
	if len(tasks2) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks2))
	}

	// cancel
	req, _ := http.NewRequest("POST", ts.URL+"/v1/tasks/task-1/cancel", nil)
	cres, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	defer cres.Body.Close()
	if cres.StatusCode != http.StatusOK {
		t.Fatalf("cancel status: %v", cres.Status)
	}

	// logs
	lres, err := http.Get(ts.URL + "/v1/tasks/task-1/logs")
	if err != nil {
		t.Fatalf("logs: %v", err)
	}
	defer lres.Body.Close()
	var logs map[string]interface{}
	lb, _ := io.ReadAll(lres.Body)
	if err := json.Unmarshal(lb, &logs); err != nil {
		t.Fatalf("unmarshal logs: %v; body=%s", err, string(lb))
	}
	if _, ok := logs["artifacts_root"]; !ok {
		t.Fatalf("missing artifacts_root in logs")
	}
}
