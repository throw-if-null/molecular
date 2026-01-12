package silicon_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/throw-if-null/molecular/internal/api"
	"github.com/throw-if-null/molecular/internal/silicon"
)

func TestCleanupEndpointDeletesPathsAndIsIdempotent(t *testing.T) {
	s, cleanup := setupTestStore(t)
	defer cleanup()

	// create task
	taskID := "task-clean"
	_, _, err := s.CreateTaskOrGetExisting(&api.CreateTaskRequest{TaskID: taskID, Prompt: "p"})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	// materialize worktree and artifacts dirs
	tk, err := s.GetTask(taskID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if err := os.MkdirAll(tk.WorktreePath, 0o755); err != nil {
		t.Fatalf("mkdir worktree: %v", err)
	}
	if err := os.MkdirAll(tk.ArtifactsRoot, 0o755); err != nil {
		t.Fatalf("mkdir artifacts: %v", err)
	}
	// place files
	_ = os.WriteFile(filepath.Join(tk.WorktreePath, "x"), []byte("1"), 0o644)
	_ = os.WriteFile(filepath.Join(tk.ArtifactsRoot, "y"), []byte("2"), 0o644)

	srv := silicon.NewServer(s, 3, 3, 2)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// call cleanup
	req, _ := http.NewRequest("POST", ts.URL+"/v1/tasks/"+taskID+"/cleanup", nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("cleanup req: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 200, got %s; body=%s", res.Status, string(b))
	}

	// ensure dirs removed
	if _, err := os.Stat(tk.WorktreePath); !os.IsNotExist(err) {
		t.Fatalf("worktree not removed")
	}
	if _, err := os.Stat(tk.ArtifactsRoot); !os.IsNotExist(err) {
		t.Fatalf("artifacts not removed")
	}

	// call cleanup again -> idempotent
	req2, _ := http.NewRequest("POST", ts.URL+"/v1/tasks/"+taskID+"/cleanup", nil)
	res2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("cleanup req2: %v", err)
	}
	defer res2.Body.Close()
	if res2.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res2.Body)
		t.Fatalf("expected 200 idempotent, got %s; body=%s", res2.Status, string(b))
	}
}

func TestCleanupRejectsTraversalTaskID(t *testing.T) {
	s, cleanup := setupTestStore(t)
	defer cleanup()
	srv := silicon.NewServer(s, 3, 3, 2)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	req, _ := http.NewRequest("POST", ts.URL+"/v1/tasks/%2E%2E%2Fevil/cleanup", nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("req: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for traversal task id, got %s", res.Status)
	}
}
