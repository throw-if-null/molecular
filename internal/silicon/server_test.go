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
		t.Fatalf("open db: %v", err)
	}
	_, _ = db.Exec(`PRAGMA busy_timeout = 5000`)
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

	srv := silicon.NewServer(s, 3, 3, 2)
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

	// logs (when no attempts exist, expect 404)
	lres, err := http.Get(ts.URL + "/v1/tasks/task-1/logs")
	if err != nil {
		t.Fatalf("logs: %v", err)
	}
	defer lres.Body.Close()
	if lres.StatusCode != http.StatusNotFound {
		b, _ := io.ReadAll(lres.Body)
		t.Fatalf("expected 404 for logs with no attempts, got %s; body=%s", lres.Status, string(b))
	}

	// create an attempt and materialize its log file
	attemptID, artifactsDir, _, _, err := s.CreateAttempt("task-1", "carbon")
	if err != nil {
		t.Fatalf("create attempt: %v", err)
	}
	if err := os.MkdirAll(artifactsDir, 0o755); err != nil {
		t.Fatalf("mkdir artifacts: %v", err)
	}
	logPath := filepath.Join(artifactsDir, "log.txt")
	if err := os.WriteFile(logPath, []byte("one\ntwo\nthree\n"), 0o644); err != nil {
		t.Fatalf("write log: %v", err)
	}

	// latest attempt log
	lres2, err := http.Get(ts.URL + "/v1/tasks/task-1/logs")
	if err != nil {
		t.Fatalf("logs2: %v", err)
	}
	defer lres2.Body.Close()
	if lres2.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(lres2.Body)
		t.Fatalf("expected 200, got %s; body=%s", lres2.Status, string(b))
	}
	b2, _ := io.ReadAll(lres2.Body)
	if string(b2) != "one\ntwo\nthree\n" {
		t.Fatalf("unexpected logs body: %q", string(b2))
	}

	// tail
	lres3, err := http.Get(ts.URL + "/v1/tasks/task-1/logs?tail=2")
	if err != nil {
		t.Fatalf("logs3: %v", err)
	}
	defer lres3.Body.Close()
	if lres3.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(lres3.Body)
		t.Fatalf("expected 200, got %s; body=%s", lres3.Status, string(b))
	}
	b3, _ := io.ReadAll(lres3.Body)
	if string(b3) != "two\nthree" {
		t.Fatalf("unexpected tail body: %q", string(b3))
	}

	// by attempt id
	lres4, err := http.Get(ts.URL + fmt.Sprintf("/v1/tasks/task-1/logs?attempt_id=%d", attemptID))
	if err != nil {
		t.Fatalf("logs4: %v", err)
	}
	defer lres4.Body.Close()
	if lres4.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(lres4.Body)
		t.Fatalf("expected 200, got %s; body=%s", lres4.Status, string(b))
	}

	// by role
	lres5, err := http.Get(ts.URL + "/v1/tasks/task-1/logs?role=carbon")
	if err != nil {
		t.Fatalf("logs5: %v", err)
	}
	defer lres5.Body.Close()
	if lres5.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(lres5.Body)
		t.Fatalf("expected 200, got %s; body=%s", lres5.Status, string(b))
	}
}

func TestLogsHardCap(t *testing.T) {
	s, cleanup := setupTestStore(t)
	defer cleanup()

	_, _, err := s.CreateTaskOrGetExisting(&api.CreateTaskRequest{TaskID: "bigtask", Prompt: "p"})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	attemptID, artifactsDir, _, _, err := s.CreateAttempt("bigtask", "carbon")
	if err != nil {
		t.Fatalf("create attempt: %v", err)
	}
	if err := os.MkdirAll(artifactsDir, 0o755); err != nil {
		t.Fatalf("mkdir artifacts: %v", err)
	}
	logPath := filepath.Join(artifactsDir, "log.txt")
	// write a file larger than the cap (5 MiB)
	size := (5 << 20) + 1
	data := make([]byte, size)
	for i := range data {
		data[i] = 'a'
	}
	if err := os.WriteFile(logPath, data, 0o644); err != nil {
		t.Fatalf("write big log: %v", err)
	}

	srv := silicon.NewServer(s, 3, 3, 2)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	res, err := http.Get(ts.URL + "/v1/tasks/bigtask/logs")
	if err != nil {
		t.Fatalf("get big logs: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusRequestEntityTooLarge {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 413 for big log, got %s; body=%s", res.Status, string(b))
	}

	// also ensure that asking for tail is rejected as well (simple hard cap behavior)
	tres, err := http.Get(ts.URL + "/v1/tasks/bigtask/logs?tail=10")
	if err != nil {
		t.Fatalf("get big logs tail: %v", err)
	}
	defer tres.Body.Close()
	if tres.StatusCode != http.StatusRequestEntityTooLarge {
		b, _ := io.ReadAll(tres.Body)
		t.Fatalf("expected 413 for big log with tail, got %s; body=%s", tres.Status, string(b))
	}

	_ = attemptID
}
