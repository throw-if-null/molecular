package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/throw-if-null/molecular/internal/api"
)

func setupServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/tasks", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			limit := r.URL.Query().Get("limit")
			var tasks []api.Task
			for i := 1; i <= 3; i++ {
				tasks = append(tasks, api.Task{TaskID: fmt.Sprintf("task-%d", i)})
			}
			if limit == "2" {
				tasks = tasks[:2]
			}
			_ = json.NewEncoder(w).Encode(tasks)
			return
		}
		if r.Method == "POST" {
			// create
			w.WriteHeader(200)
			w.Write([]byte(`{"ok":true}`))
			return
		}
		w.WriteHeader(405)
	})

	mux.HandleFunc("/v1/tasks/task-1/cancel", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			w.Write([]byte(`{"canceled":true}`))
			return
		}
		w.WriteHeader(405)
	})

	mux.HandleFunc("/v1/tasks/task-1/logs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			w.Write([]byte(`{"artifacts_root":"/tmp/x"}`))
			return
		}
		w.WriteHeader(405)
	})

	mux.HandleFunc("/v1/tasks/task-1/cleanup", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"artifacts":true,"worktree":true}`))
			return
		}
		w.WriteHeader(405)
	})

	// status endpoint for task-1
	mux.HandleFunc("/v1/tasks/task-1", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			w.Header().Set("Content-Type", "application/json")
			body := `{"task_id":"task-1","phase":"carbon","status":"running","carbon_budget":3,"helium_budget":3,"review_budget":2,"latest_attempt":{"id":42,"task_id":"task-1","role":"carbon","attempt_num":1,"status":"running","started_at":"","finished_at":"","artifacts_dir":"/tmp/x","error_summary":""}}`
			w.Write([]byte(body))
			return
		}
		w.WriteHeader(405)
	})

	return httptest.NewServer(mux)
}

func TestListCancelLogsCleanup(t *testing.T) {
	ts := setupServer()
	defer ts.Close()

	client := &http.Client{}

	// list
	buf := &bytes.Buffer{}
	oldOut, wout := captureStdout(buf)
	code := run([]string{"list"}, client, ts.URL, buf, bytes.NewBuffer(nil))
	restoreStdout(oldOut, wout)
	if code != 0 {
		t.Fatalf("list exit code: %d", code)
	}
	b, _ := io.ReadAll(buf)
	var tasks []map[string]interface{}
	if err := json.Unmarshal(b, &tasks); err != nil {
		t.Fatalf("unmarshal list: %v; body=%s", err, string(b))
	}
	if len(tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(tasks))
	}

	// list --limit 2
	buf.Reset()
	oldOut, wout = captureStdout(buf)
	code = run([]string{"list", "--limit", "2"}, client, ts.URL, buf, bytes.NewBuffer(nil))
	restoreStdout(oldOut, wout)
	if code != 0 {
		t.Fatalf("list limit exit code: %d", code)
	}
	b, _ = io.ReadAll(buf)
	if err := json.Unmarshal(b, &tasks); err != nil {
		t.Fatalf("unmarshal list limit: %v; body=%s", err, string(b))
	}
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}

	// cancel
	buf.Reset()
	oldOut, wout = captureStdout(buf)
	code = run([]string{"cancel", "task-1"}, client, ts.URL, buf, bytes.NewBuffer(nil))
	restoreStdout(oldOut, wout)
	if code != 0 {
		t.Fatalf("cancel exit code: %d", code)
	}
	b, _ = io.ReadAll(buf)
	var cres map[string]interface{}
	if err := json.Unmarshal(b, &cres); err != nil {
		t.Fatalf("unmarshal cancel: %v; body=%s", err, string(b))
	}
	if cres["canceled"] != true {
		t.Fatalf("unexpected cancel body: %v", cres)
	}

	// logs
	buf.Reset()
	code = run([]string{"logs", "task-1"}, client, ts.URL, buf, bytes.NewBuffer(nil))
	if code != 0 {
		t.Fatalf("logs exit code: %d", code)
	}
	b, _ = io.ReadAll(buf)
	var lres map[string]interface{}
	if err := json.Unmarshal(b, &lres); err != nil {
		t.Fatalf("unmarshal logs: %v; body=%s", err, string(b))
	}
	if _, ok := lres["artifacts_root"]; !ok {
		t.Fatalf("missing artifacts_root in logs")
	}

	// cleanup -> should call endpoint and print JSON
	buf.Reset()
	oldOut, wout = captureStdout(buf)
	code = run([]string{"cleanup", "task-1"}, client, ts.URL, buf, bytes.NewBuffer(nil))
	restoreStdout(oldOut, wout)
	if code != 0 {
		t.Fatalf("cleanup exit code: %d", code)
	}
	b, _ = io.ReadAll(buf)
	var cresCleanup map[string]interface{}
	if err := json.Unmarshal(b, &cresCleanup); err != nil {
		t.Fatalf("unmarshal cleanup: %v; body=%s", err, string(b))
	}
	if cresCleanup["artifacts"] != true || cresCleanup["worktree"] != true {
		t.Fatalf("unexpected cleanup body: %v", cresCleanup)
	}
}

// helpers to capture stdout/stderr
func captureStdout(w *bytes.Buffer) (*os.File, *os.File) {
	old := os.Stdout
	r, wpipe, _ := os.Pipe()
	os.Stdout = wpipe
	go func() {
		io.Copy(w, r)
		r.Close()
	}()
	return old, wpipe
}

func restoreStdout(old *os.File, wpipe *os.File) {
	_ = wpipe.Close()
	os.Stdout = old
}

func captureStderr(w *bytes.Buffer) (*os.File, *os.File) {
	old := os.Stderr
	r, wpipe, _ := os.Pipe()
	os.Stderr = wpipe
	go func() {
		io.Copy(w, r)
		r.Close()
	}()
	return old, wpipe
}

func restoreStderr(old *os.File, wpipe *os.File) {
	_ = wpipe.Close()
	os.Stderr = old
}

func TestDoctorCommand(t *testing.T) {
	// create a temp repo dir
	d, err := os.MkdirTemp("", "molecular-doctor-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(d)
	// create .molecular and a non-executable hook file
	mm := filepath.Join(d, ".molecular")
	if err := os.Mkdir(mm, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(mm, "config.toml")
	if err := os.WriteFile(cfg, []byte("x=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// create hooks: one executable, one not
	lith := filepath.Join(mm, "lithium.sh")
	chlor := filepath.Join(mm, "chlorine.sh")
	if err := os.WriteFile(lith, []byte("echo hi\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(chlor, []byte("echo bye\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// stub execLookPath to succeed for git and fail for gh
	oldLook := execLookPath
	execLookPath = func(name string) (string, error) {
		if name == "git" {
			return "/usr/bin/git", nil
		}
		return "", fmt.Errorf("not found")
	}
	defer func() { execLookPath = oldLook }()

	// run doctor in that dir
	oldWd, _ := os.Getwd()
	_ = os.Chdir(d)
	defer os.Chdir(oldWd)

	out := &bytes.Buffer{}
	code := doctorWithIO([]string{}, out, out)
	if code != 1 { // chlorine.sh not executable -> problem -> exit 1
		t.Fatalf("expected exit 1, got %d, out=%s", code, out.String())
	}

	// JSON mode
	out.Reset()
	code = doctorWithIO([]string{"--json"}, out, out)
	if code != 1 {
		t.Fatalf("expected exit 1 in json mode, got %d", code)
	}
	var rep map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("invalid json: %v, out=%s", err, out.String())
	}
	if rep["git"] != true {
		t.Fatalf("expected git true in json report")
	}
	// Now write an invalid config and ensure doctor reports a parse error
	if err := os.WriteFile(cfg, []byte("x = [1,\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	code = doctorWithIO([]string{}, out, out)
	if code != 1 {
		t.Fatalf("expected exit 1 for invalid config, got %d", code)
	}
	if !strings.Contains(out.String(), "failed to parse") {
		t.Fatalf("expected parse error message in doctor output, got: %s", out.String())
	}

}

func TestStatusOutput(t *testing.T) {
	ts := setupServer()
	defer ts.Close()

	client := &http.Client{}
	// human output
	buf := &bytes.Buffer{}
	oldOut, wout := captureStdout(buf)
	code := run([]string{"status", "task-1"}, client, ts.URL, buf, bytes.NewBuffer(nil))
	restoreStdout(oldOut, wout)
	if code != 0 {
		t.Fatalf("status exit code: %d", code)
	}
	out := buf.String()
	if !strings.Contains(out, "task-1") || !strings.Contains(out, "latest attempt") {
		t.Fatalf("unexpected status output: %s", out)
	}

	// json mode
	buf.Reset()
	oldOut, wout = captureStdout(buf)
	code = run([]string{"status", "--json", "task-1"}, client, ts.URL, buf, bytes.NewBuffer(nil))
	restoreStdout(oldOut, wout)
	if code != 0 {
		t.Fatalf("status --json exit code: %d", code)
	}
	var j map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &j); err != nil {
		t.Fatalf("invalid json output: %v; out=%s", err, buf.String())
	}
	if j["task_id"] != "task-1" {
		t.Fatalf("unexpected json task_id: %v", j["task_id"])
	}
}
